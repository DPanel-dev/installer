package config

import (
	"fmt"
)

// === 基础配置 Options ===

// WithAction 设置操作类型
func WithAction(action string) Option {
	return func(c *Config) error {
		if !IsValidAction(action) {
			return fmt.Errorf("invalid action: %s, must be one of: config, upgrade, uninstall", action)
		}
		c.Action = action
		return nil
	}
}

// WithLanguage 设置语言
func WithLanguage(lang string) Option {
	return func(c *Config) error {
		if lang != "zh" && lang != "en" {
			return fmt.Errorf("invalid language: %s, must be 'zh' or 'en'", lang)
		}
		c.Language = lang
		return nil
	}
}

// WithInstallType 设置安装类型
func WithInstallType(installType string) Option {
	return func(c *Config) error {
		if !IsValidInstallType(installType) {
			return fmt.Errorf("invalid config type: %s", installType)
		}

		// 验证：选择容器安装时必须有 Docker/Podman
		if installType == InstallTypeContainer && !c.Env.DockerAvailable && !c.Env.PodmanAvailable {
			return fmt.Errorf("container installation requires Docker or Podman to be available")
		}

		// 验证：install_docker 只在 Linux 可用
		if installType == InstallTypeInstallDocker && c.Env.OS != "linux" {
			return fmt.Errorf("automatic Docker installation is only available on Linux")
		}

		c.InstallType = installType
		return nil
	}
}

// === 版本配置 Options ===

// WithVersion 设置版本
func WithVersion(version string) Option {
	return func(c *Config) error {
		if !IsValidVersion(version) {
			return fmt.Errorf("invalid version: %s, must be one of: community, pro, dev", version)
		}
		c.Version = version
		return nil
	}
}

// WithEdition 设置版本类型
func WithEdition(edition string) Option {
	return func(c *Config) error {
		if !IsValidEdition(edition) {
			return fmt.Errorf("invalid edition: %s, must be 'standard' or 'lite'", edition)
		}

		// 验证：二进制安装只支持 lite
		if c.InstallType == InstallTypeBinary && edition == EditionStandard {
			return fmt.Errorf("standard edition only supports container installation")
		}

		c.Edition = edition
		return nil
	}
}

// WithOS 设置构建系统
func WithOS(os string) Option {
	return func(c *Config) error {
		if !IsValidOS(os) {
			return fmt.Errorf("invalid OS: %s, must be 'alpine' or 'debian'", os)
		}
		c.OS = os
		return nil
	}
}

// WithRegistry 设置镜像源
func WithRegistry(registry string) Option {
	return func(c *Config) error {
		if !IsValidRegistry(registry) {
			return fmt.Errorf("invalid registry: %s, must be 'hub' or 'aliyun'", registry)
		}

		// 验证：选择的镜像源必须可用
		if registry == RegistryHub && !c.Env.HubAccessible {
			return fmt.Errorf("hub registry is not accessible from your environment")
		}
		if registry == RegistryAliyun && !c.Env.AliyunAccessible {
			return fmt.Errorf("aliyun registry is not accessible from your environment")
		}

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

		// 验证：容器名称不能已存在
		for _, existing := range c.Env.ExistingContainers {
			if existing == name {
				return fmt.Errorf("container '%s' already exists", name)
			}
		}

		c.ContainerName = name
		return nil
	}
}

// WithPort 设置端口
func WithPort(port int) Option {
	return func(c *Config) error {
		if port < 0 || port > 65535 {
			return fmt.Errorf("invalid port: %d, must be between 0 and 65535", port)
		}
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

		// TODO: 验证目录必须为空（或不存在）

		c.DataPath = path
		return nil
	}
}

// === Docker 连接 Options ===

// WithDockerLocal 设置本地 Docker 连接
func WithDockerLocal(sockPath string) Option {
	return func(c *Config) error {
		if sockPath == "" {
			sockPath = "/var/run/docker.sock"
		}

		// TODO: 验证 sock 文件必须存在

		c.DockerConnType = "local"
		c.DockerSockPath = sockPath
		return nil
	}
}

// WithDockerTCP 设置 TCP 连接
func WithDockerTCP(host string, port int) Option {
	return func(c *Config) error {
		if host == "" {
			return fmt.Errorf("TCP host cannot be empty")
		}
		if port <= 0 || port > 65535 {
			return fmt.Errorf("invalid TCP port: %d", port)
		}

		c.DockerConnType = "tcp"
		c.DockerTCPHost = host
		c.DockerTCPPort = port
		return nil
	}
}

// WithDockerSSH 设置 SSH 连接
func WithDockerSSH(host string, port int, user string) Option {
	return func(c *Config) error {
		if host == "" {
			return fmt.Errorf("SSH host cannot be empty")
		}
		if port <= 0 || port > 65535 {
			return fmt.Errorf("invalid SSH port: %d", port)
		}
		if user == "" {
			return fmt.Errorf("SSH user cannot be empty")
		}

		c.DockerConnType = "ssh"
		c.DockerSSHHost = host
		c.DockerSSHPort = port
		c.DockerSSHUser = user
		return nil
	}
}

// WithDockerTLS 设置 TLS 配置
func WithDockerTLS(enabled bool, certPath, keyPath, caPath string) Option {
	return func(c *Config) error {
		if c.DockerConnType != DockerConnTCP {
			return fmt.Errorf("TLS is only supported for TCP connections")
		}
		c.DockerTLS = enabled
		c.DockerTLSCert = certPath
		c.DockerTLSKey = keyPath
		c.DockerTLSCA = caPath
		return nil
	}
}

// WithSSHAuth 设置 SSH 认证
func WithSSHAuth(password, keyPath string) Option {
	return func(c *Config) error {
		if c.DockerConnType != DockerConnSSH {
			return fmt.Errorf("SSH auth is only for SSH connections")
		}
		if password == "" && keyPath == "" {
			return fmt.Errorf("either password or key path must be provided")
		}
		c.DockerSSHPass = password
		c.DockerSSHKey = keyPath
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

// WithUpgradeTargetVersion 设置目标版本
func WithUpgradeTargetVersion(version string) Option {
	return func(c *Config) error {
		c.UpgradeTargetVersion = version
		return nil
	}
}

// WithUpgradeBackup 设置是否备份容器
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
