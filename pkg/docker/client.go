package docker

import (
	"context"
	"runtime"
	"strings"
	"time"

	dockerclient "github.com/moby/moby/client"
)

// Client 封装统一的 Docker SDK client 创建入口。
type Client struct {
	Client *dockerclient.Client
}

// New 创建 Docker SDK client。默认使用基础环境配置，额外 opts 用于覆盖或补充。
func New(opts ...dockerclient.Opt) (*Client, error) {
	if len(opts) > 0 {
		return newClient(opts...)
	}

	// 1. 默认 Docker 探测，始终先走默认路径，再叠加用户传入参数。
	cli, err := newClient()
	if err == nil {
		return cli, nil
	}

	// 1. Docker 本地 endpoint 候选。
	for _, host := range dockerHosts() {
		cli, err = newClient(dockerclient.WithHost(host))
		if err == nil {
			return cli, nil
		}
	}

	// 2. Podman 本地 endpoint 候选。
	for _, host := range podmanHosts() {
		cli, err = newClient(dockerclient.WithHost(host))
		if err == nil {
			return cli, nil
		}
	}

	return nil, err
}

func newClient(opts ...dockerclient.Opt) (*Client, error) {
	opts = append([]dockerclient.Opt{
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionFromEnv(),
	}, opts...)

	cli, err := dockerclient.New(opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx, dockerclient.PingOptions{}); err != nil {
		_ = cli.Close()
		return nil, err
	}

	return &Client{
		Client: cli,
	}, nil
}

// DaemonHost 返回适合容器 bind 挂载的 Docker/Podman socket 路径。
//
// 各平台处理：
//   - macOS Docker Desktop: VirtioFS 无法挂载 ~/.docker/run/docker.sock，使用 /var/run/docker.sock
//   - Windows Docker Desktop: npipe 无法挂载到容器，返回空字符串
//   - Linux / Podman / rootless: 返回实际检测到的 socket 路径
func (c *Client) DaemonHost() string {
	if c == nil || c.Client == nil {
		return DefaultDockerSockPath
	}

	host := c.Client.DaemonHost()
	path := SockPathFromHost(host)

	// Windows named pipe 无法挂载到容器
	if strings.HasPrefix(host, "npipe://") {
		return ""
	}

	// macOS Docker Desktop: VirtioFS 限制，必须用 /var/run/docker.sock
	if runtime.GOOS == "darwin" && !c.IsPodman() {
		return "/var/run/docker.sock"
	}

	if path == "" {
		return DefaultDockerSockPath
	}
	return path
}

// IsPodman 检测当前连接是否为 Podman
func (c *Client) IsPodman() bool {
	if c == nil || c.Client == nil {
		return false
	}
	return strings.Contains(c.Client.DaemonHost(), "podman")
}
