package config

import (
	"fmt"
	"strings"

	"github.com/dpanel-dev/installer/internal/types"
	dockerpkg "github.com/dpanel-dev/installer/pkg/docker"
	dockerclient "github.com/moby/moby/client"
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

// WithName 设置实例名称（容器名 / 二进制进程名，全局唯一）
func WithName(name string) Option {
	return func(c *Config) error {
		if name == "" {
			return fmt.Errorf("name cannot be empty")
		}
		c.Name = name
		return nil
	}
}

// WithServerHost 设置服务绑定地址
func WithServerHost(host string) Option {
	return func(c *Config) error {
		c.ServerHost = host
		return nil
	}
}

// WithServerPort 设置服务端口
func WithServerPort(port int) Option {
	return func(c *Config) error {
		c.ServerPort = port
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
		if address == "" {
			address = dockerpkg.DefaultDockerSockPath
		}

		host := dockerpkg.NormalizeHost(address)
		cli, err := dockerpkg.New(dockerclient.WithHost(host))
		if err != nil {
			return err
		}

		if c.Client != nil && c.Client != cli && c.Client.Client != nil {
			_ = c.Client.Client.Close()
		}
		c.Client = cli
		return nil
	}
}

// === 网络配置 Options ===

// WithEnvDNS 设置 DNS 环境变量
func WithEnvDNS(dns string) Option {
	return func(c *Config) error {
		c.DNS = dns
		return nil
	}
}

// WithEnvProxy 设置代理环境变量（同时用于 HTTP 和 HTTPS）
func WithEnvProxy(proxy string) Option {
	return func(c *Config) error {
		c.HTTPProxy = proxy
		return nil
	}
}

// === 升级配置 Options ===

// WithEnableBackup 设置是否备份
func WithEnableBackup(backup bool) Option {
	return func(c *Config) error {
		c.UpgradeBackup = backup
		return nil
	}
}

// === 卸载配置 Options ===

// WithEnableDeleteData 设置是否删除数据
func WithEnableDeleteData(remove bool) Option {
	return func(c *Config) error {
		c.UninstallRemoveData = remove
		return nil
	}
}

// === 二进制配置 Options ===

// WithBinaryPath 设置二进制安装路径（Windows 自动补 .exe 后缀）
func WithBinaryPath(path string) Option {
	return func(c *Config) error {
		if path == "" {
			return fmt.Errorf("binary path cannot be empty")
		}
		if c.OS == types.BaseImageWindows && !strings.HasSuffix(strings.ToLower(path), ".exe") {
			path += ".exe"
		}
		c.BinaryPath = path
		return nil
	}
}

// WithOS 设置操作系统（仅用于测试覆盖）
func WithOS(os string) Option {
	return func(c *Config) error {
		c.OS = os
		return nil
	}
}

// WithArch 设置架构（仅用于测试覆盖）
func WithArch(arch string) Option {
	return func(c *Config) error {
		c.Arch = arch
		return nil
	}
}

// WithClient 设置 Docker client
func WithClient(cli *dockerpkg.Client) Option {
	return func(c *Config) error {
		c.Client = cli
		return nil
	}
}
