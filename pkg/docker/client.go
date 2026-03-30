package docker

import (
	"context"
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
