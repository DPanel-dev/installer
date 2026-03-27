package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dpanel-dev/installer/internal/types"
)

// DetectDocker 检测 Docker 并返回连接配置
func DetectDocker() *ContainerConn {
	// 1. 命令不存在
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}

	// 2. 使用 docker context inspect 获取连接信息
	out, err := exec.Command("docker", "context", "inspect", "--format", "json").CombinedOutput()
	if err != nil {
		slog.Debug("docker context inspect failed", "error", err)
		return nil
	}

	// dockerContext Docker context 信息（JSON 解析用）
	type dockerContext struct {
		Endpoints struct {
			Docker struct {
				Host          string `json:"Host"`
				SkipTLSVerify bool   `json:"SkipTLSVerify"`
			} `json:"docker"`
		} `json:"Endpoints"`
	}

	// 3. 解析 JSON（可能是数组）
	var contexts []dockerContext
	if err := json.Unmarshal(out, &contexts); err != nil {
		slog.Debug("Failed to parse docker context", "error", err)
		return nil
	}

	if len(contexts) == 0 {
		return nil
	}

	ctx := contexts[0]
	host := ctx.Endpoints.Docker.Host

	// 4. 默认值
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	conn := &ContainerConn{
		Engine:    types.ContainerEngineDocker,
		TLSVerify: !ctx.Endpoints.Docker.SkipTLSVerify,
	}

	// 5. 根据地址判断类型
	if strings.HasPrefix(host, "unix://") {
		conn.Type = types.ContainerConnTypeSock
		conn.Address = host
		return conn
	}

	if strings.HasPrefix(host, "npipe://") {
		conn.Type = types.ContainerConnTypeSock
		conn.Address = host
		return conn
	}

	if strings.HasPrefix(host, "tcp://") {
		conn.Type = types.ContainerConnTypeTCP
		conn.Address = host
		return conn
	}

	if strings.HasPrefix(host, "ssh://") {
		conn.Type = types.ContainerConnTypeSSH
		conn.Address = host
		return conn
	}

	// 未知格式，尝试作为 TCP
	conn.Type = types.ContainerConnTypeTCP
	conn.Address = host
	return conn
}

// DetectPodman 检测 Podman 并返回连接配置
func DetectPodman() *ContainerConn {
	// 1. 命令不存在
	if _, err := exec.LookPath("podman"); err != nil {
		return nil
	}

	conn := &ContainerConn{
		Engine: types.ContainerEnginePodman,
	}

	// 2. 获取 sock 路径
	switch runtime.GOOS {
	case "linux":
		// 优先 XDG_RUNTIME_DIR
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			sockPath := filepath.Join(dir, "podman", "podman.sock")
			if _, err := os.Stat(sockPath); err == nil {
				conn.Type = types.ContainerConnTypeSock
				conn.Address = "unix://" + sockPath
				return conn
			}
		}
		// 备选 /run/user/{uid}
		if uid := os.Getuid(); uid > 0 {
			sockPath := fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
			if _, err := os.Stat(sockPath); err == nil {
				conn.Type = types.ContainerConnTypeSock
				conn.Address = "unix://" + sockPath
				return conn
			}
		}
		// rootful sock
		if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
			conn.Type = types.ContainerConnTypeSock
			conn.Address = "unix:///run/podman/podman.sock"
			return conn
		}
	case "darwin":
		// macOS
		home, _ := os.UserHomeDir()
		sockPath := filepath.Join(home, ".local", "share", "containers", "podman", "machine", "default", "podman.sock")
		if _, err := os.Stat(sockPath); err == nil {
			conn.Type = types.ContainerConnTypeSock
			conn.Address = "unix://" + sockPath
			return conn
		}
	}

	// 有命令但没找到 sock，返回 nil（Podman 存在但未运行）
	return nil
}

// TestRegistryConnectivity 测试镜像仓库连通性
func TestRegistryConnectivity(host string) bool {
	latency := TestRegistryLatency(host)
	return latency > 0
}

// TestRegistryLatency 测试镜像仓库延迟（毫秒），0 表示不可用
func TestRegistryLatency(host string) int {
	if host == types.RegistryDockerHub {
		host = "index.docker.io"
	}
	url := fmt.Sprintf("https://%s/v2/", host)
	timeout := 5 * time.Second

	client := http.Client{
		Timeout: timeout,
	}

	start := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		slog.Debug("Registry latency test failed", "host", host, "error", err)
		return 0
	}
	defer resp.Body.Close()

	latency := int(time.Since(start).Milliseconds())

	// 200, 401, 403 都表示服务可达（401/403 是需要认证但服务存在）
	available := resp.StatusCode < 500
	if !available {
		slog.Debug("Registry latency test unavailable", "host", host, "status", resp.StatusCode)
		return 0
	}

	slog.Debug("Registry latency test result", "host", host, "latency_ms", latency, "status", resp.StatusCode)
	return latency
}

// IsMusl 检测系统是否使用 musl libc（如 Alpine）
func IsMusl() bool {
	// 方法1：检查 ldd 输出
	if out, err := exec.Command("ldd", "--version").CombinedOutput(); err == nil {
		if strings.Contains(strings.ToLower(string(out)), "musl") {
			return true
		}
	}

	// 方法2：检查 /lib/ld-musl-*.so.1 文件
	if files, err := filepath.Glob("/lib/ld-musl-*.so.1"); err == nil && len(files) > 0 {
		return true
	}

	// 方法3：检查 /etc/os-release 中是否包含 Alpine
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), "alpine") {
			return true
		}
	}

	return false
}

// FindAvailablePort 查找可用端口，从 startPort 开始查找
func FindAvailablePort(startPort int) int {
	for port := startPort; port < 65535; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return startPort // 如果都不可用，返回默认端口
}
