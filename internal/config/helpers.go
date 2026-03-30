package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dpanel-dev/installer/internal/types"
)

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
