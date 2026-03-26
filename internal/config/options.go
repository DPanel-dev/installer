package config

import (
	"fmt"

	"github.com/dpanel-dev/installer/internal/types"
)

// === 基础配置 Options ===

// WithAction 设置操作类型
func WithAction(action string) Option {
	return func(c *Config) error {
		c.Action = action
		return nil
	}
}

// WithLanguage 设置语言
func WithLanguage(lang string) Option {
	return func(c *Config) error {
		c.Language = lang
		return nil
	}
}

// WithInstallType 设置安装类型
func WithInstallType(installType string) Option {
	return func(c *Config) error {
		c.InstallType = installType
		return nil
	}
}

// === 版本配置 Options ===

// WithVersion 设置版本
func WithVersion(version string) Option {
	return func(c *Config) error {
		c.Version = version
		return nil
	}
}

// WithEdition 设置版本类型
func WithEdition(edition string) Option {
	return func(c *Config) error {
		c.Edition = edition
		return nil
	}
}

// WithBaseImage 设置基础镜像系统
func WithBaseImage(baseImage string) Option {
	return func(c *Config) error {
		c.BaseImage = baseImage
		return nil
	}
}

// WithRegistry 设置镜像仓库
func WithRegistry(registry string) Option {
	return func(c *Config) error {
		c.Registry = registry
		return nil
	}
}

// === 容器配置 Options ===

// WithContainerName 设置容器名称
func WithContainerName(name string) Option {
	return func(c *Config) error {
		if name == "" {
			return fmt.Errorf("container name cannot be empty")
		}
		c.ContainerName = name
		return nil
	}
}

// WithPort 设置端口
func WithPort(port int) Option {
	return func(c *Config) error {
		c.Port = port
		return nil
	}
}

// WithDataPath 设置数据路径
func WithDataPath(path string) Option {
	return func(c *Config) error {
		if path == "" {
			return fmt.Errorf("data path cannot be empty")
		}
		c.DataPath = path
		return nil
	}
}

// === 容器连接 Options ===

// WithContainerSock 设置本地 socket 连接
func WithContainerSock(address string) Option {
	return func(c *Config) error {
		if c.Env.ContainerConn == nil {
			c.Env.ContainerConn = &ContainerConn{Engine: types.ContainerEngineDocker}
		}
		c.Env.ContainerConn.Type = types.ContainerConnTypeSock
		if address == "" {
			c.Env.ContainerConn.Address = "unix:///var/run/docker.sock"
		} else {
			c.Env.ContainerConn.Address = address
		}
		return nil
	}
}

// WithContainerTCP 设置 TCP 连接
func WithContainerTCP(host string, port int, tlsVerify bool) Option {
	return func(c *Config) error {
		if host == "" {
			return fmt.Errorf("TCP host cannot be empty")
		}
		if c.Env.ContainerConn == nil {
			c.Env.ContainerConn = &ContainerConn{Engine: types.ContainerEngineDocker}
		}
		c.Env.ContainerConn.Type = types.ContainerConnTypeTCP
		c.Env.ContainerConn.Address = fmt.Sprintf("tcp://%s:%d", host, port)
		c.Env.ContainerConn.TLSVerify = tlsVerify
		return nil
	}
}

// WithContainerSSH 设置 SSH 连接
func WithContainerSSH(host string, port int, username string) Option {
	return func(c *Config) error {
		if host == "" {
			return fmt.Errorf("SSH host cannot be empty")
		}
		if c.Env.ContainerConn == nil {
			c.Env.ContainerConn = &ContainerConn{Engine: types.ContainerEngineDocker}
		}
		c.Env.ContainerConn.Type = types.ContainerConnTypeSSH
		c.Env.ContainerConn.Address = fmt.Sprintf("ssh://%s:%d", host, port)
		c.Env.ContainerConn.SSHUsername = username
		return nil
	}
}

// WithContainerTLS 设置 TLS 配置
func WithContainerTLS(caCert, cert, key string) Option {
	return func(c *Config) error {
		if c.Env.ContainerConn == nil {
			c.Env.ContainerConn = &ContainerConn{Engine: types.ContainerEngineDocker}
		}
		c.Env.ContainerConn.TLSVerify = true
		c.Env.ContainerConn.TLSCACert = caCert
		c.Env.ContainerConn.TLSCert = cert
		c.Env.ContainerConn.TLSKey = key
		return nil
	}
}

// WithContainerSSHAuth 设置 SSH 认证
func WithContainerSSHAuth(password, keyPath string) Option {
	return func(c *Config) error {
		if c.Env.ContainerConn == nil {
			c.Env.ContainerConn = &ContainerConn{Engine: types.ContainerEngineDocker}
		}
		c.Env.ContainerConn.SSHPassword = password
		c.Env.ContainerConn.SSHKeyPath = keyPath
		return nil
	}
}

// === 网络配置 Options ===

// WithDNS 设置 DNS
func WithDNS(dns string) Option {
	return func(c *Config) error {
		c.DNS = dns
		return nil
	}
}

// WithHTTPProxy 设置 HTTP 代理
func WithHTTPProxy(proxy string) Option {
	return func(c *Config) error {
		c.HTTPProxy = proxy
		return nil
	}
}

// WithHTTPSProxy 设置 HTTPS 代理
func WithHTTPSProxy(proxy string) Option {
	return func(c *Config) error {
		c.HTTPSProxy = proxy
		return nil
	}
}

// === 升级配置 Options ===

// WithUpgradeBackup 设置是否备份
func WithUpgradeBackup(backup bool) Option {
	return func(c *Config) error {
		c.UpgradeBackup = backup
		return nil
	}
}

// === 卸载配置 Options ===

// WithUninstallRemoveData 设置是否删除数据
func WithUninstallRemoveData(remove bool) Option {
	return func(c *Config) error {
		c.UninstallRemoveData = remove
		return nil
	}
}
