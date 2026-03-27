package config

import "github.com/dpanel-dev/installer/internal/types"

// ContainerConn 容器连接配置（统一 Docker 和 Podman）
type ContainerConn struct {
	Engine  string // docker 或 podman
	Type    string // sock, tcp, ssh
	Address string // unix:///var/run/docker.sock, tcp://host:port, ssh://host:port

	// TCP 专用（TLS）
	TLSVerify bool   // 是否需要 TLS 验证
	TLSCACert string // CA 证书路径
	TLSCert   string // 客户端证书路径
	TLSKey    string // 客户端私钥路径

	// SSH 专用
	SSHUsername string // SSH 用户名
	SSHPassword string // SSH 密码
	SSHKeyPath  string // 私钥文件路径
}

// IsDocker 判断是否为 Docker
func (c *ContainerConn) IsDocker() bool {
	return c != nil && c.Engine == types.ContainerEngineDocker
}

// IsPodman 判断是否为 Podman
func (c *ContainerConn) IsPodman() bool {
	return c != nil && c.Engine == types.ContainerEnginePodman
}

// IsLocal 判断是否为本地容器引擎
func (c *ContainerConn) IsLocal() bool {
	if c == nil {
		return false
	}
	return c.Type == types.ContainerConnTypeSock
}

// IsRemote 判断是否为远程容器引擎
func (c *ContainerConn) IsRemote() bool {
	return c != nil && !c.IsLocal()
}
