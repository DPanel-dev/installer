package config

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dpanel-dev/installer/internal/types"
)

// detectDockerEngineType 检测 Docker 引擎类型
func detectDockerEngineType() types.DockerEngineType {
	// 1. 壽令不存在
	if _, err := exec.LookPath("docker"); err != nil {
		return types.DockerEngineNone
	}

	// 2. docker info 获取信息
	out, err := exec.Command("docker", "info", "-f", "{{.OperatingSystem}}").CombinedOutput()
	if err != nil {
		return types.DockerEngineNone // daemon 没运行
	}

	// 3. Docker Desktop = 本地
	if strings.Contains(strings.ToLower(string(out)), "docker desktop") {
		return types.DockerEngineLocal
	}

	// 4. Windows 非 Docker Desktop = 远程
	if runtime.GOOS == "windows" {
		return types.DockerEngineRemote
	}

	// 5. 检查 DOCKER_HOST
	host := os.Getenv("DOCKER_HOST")

	// 没设置或 unix sock = 本地
	if host == "" || strings.HasPrefix(host, "unix://") {
		return types.DockerEngineLocal
	}

	// TCP 127.x 或 localhost = 本地
	if strings.HasPrefix(host, "tcp://127.") || strings.HasPrefix(host, "tcp://localhost") {
		return types.DockerEngineLocal
	}

	// 其他 = 远程
	return types.DockerEngineRemote
}

// testRegistryConnectivity 测试镜像仓库连通性
// host: 镜像仓库地址，如 registry.hub.docker.com
func testRegistryConnectivity(host string) bool {
	url := fmt.Sprintf("https://%s/v2/", host)
	timeout := 5 * time.Second

	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		slog.Debug("Registry connectivity test failed", "host", host, "error", err)
	 return false
 }
    defer resp.Body.Close()

    // 200, 401, 403 都表示服务可达（401/403 是需要认证但服务存在）
    available := resp.StatusCode < 500
    slog.Debug("Registry connectivity test result", "host", host, "available", available, "status", resp.StatusCode)
    return available
}

// getPodmanSockPath 获取 Podman sock 路径
func getPodmanSockPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
        return filepath.Join(dir, "podman", "podman.sock")
    }
    return "/run/user/1000/podman/podman.sock"
}

// podmanExists 检测 Podman 命令是否存在
func podmanExists() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}
