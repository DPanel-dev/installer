package config

import (
	"fmt"
	"runtime"

	"github.com/dpanel-dev/installer/internal/types"
)

// Config 安装配置
type Config struct {
	// === 操作类型 ===
	Action string // install, upgrade, uninstall

	// === 语言 ===
	Language string // zh, en

	// === 安装类型 ===
	InstallType string // container, binary, install_docker

	// === 版本配置 ===
	Version  string // community, pro, dev
	Edition  string // standard, lite
	OS       string // alpine, debian
	Registry string // hub, aliyun

	// === 宺器配置 ===
	ContainerName string
	Port          int // 0 = 随机端口
	DataPath      string

	// === Docker 连接 ===
	DockerConnType string // local, tcp, ssh
	DockerSockPath string
	DockerTCPHost  string
	DockerTCPPort  int
	DockerSSHHost  string
	DockerSSHPort  int
	DockerSSHUser  string
	DockerSSHPass  string
	DockerSSHKey   string
	DockerTLS      bool

	// === 网络配置 ===
	DNS        string
	HTTPProxy  string
	HTTPSProxy string

	// === 升级配置 ===
	UpgradeBackup bool

	// === 卸载配置 ===
	UninstallRemoveData bool

	// === 环境检测结果 ===
	Env types.EnvCheck
}

// Option 配置选项函数
type Option func(*Config) error

// NewConfig 创建配置（自动检测环境 + 智能默认值）
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}

	// 1. 执行环境检测
	c.detectEnvironment()

	// 2. 根据环境设置最优默认值
	c.applySmartDefaults()

	// 3. 应用用户自定义选项
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// detectEnvironment 执行环境检测
func (c *Config) detectEnvironment() {
	c.Env = types.EnvCheck{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// 检测 Docker 引擎类型
	c.Env.DockerEngine = detectDockerEngineType()

	// 检测 Podman：有命令即可（无守护进程架构）
	c.Env.PodmanExists = podmanExists()

	// 镜像源连通性在用户选择时实时检测
}

// TestAndSetRegistry 测试镜像源连通性并设置 Registry
// 先测试 docker.io，失败则测试阿里云，都失败则 Registry 为空
func (c *Config) TestAndSetRegistry() {
	if testRegistryConnectivity("registry.hub.docker.com") {
		c.Registry = "" // 使用 docker.io
		return
	}
	if testRegistryConnectivity("registry.cn-hangzhou.aliyuncs.com") {
		c.Registry = "aliyun"
		return
	}
	c.Registry = "" // 都不可用，保持空表示无法安装
}

// CanInstall 检查是否可以执行安装操作（Registry 非空或已测试可用）
func (c *Config) CanInstall() bool {
	// 如果 Registry 为空，实时测试连通性
	if c.Registry == "" {
		c.TestAndSetRegistry()
	}
	// 再次检查：如果仍为空，说明两个源都不可用
	// 但空值本身也可能表示使用默认 docker.io，需要额外标记
	return c.Registry != "" || testRegistryConnectivity("registry.hub.docker.com")
}

// CanUpgrade 检查是否可以执行升级操作
func (c *Config) CanUpgrade() bool {
	return c.CanInstall()
}

// applySmartDefaults 根据环境设置最优默认值
func (c *Config) applySmartDefaults() {
	// 操作类型
	c.Action = "install"

	// 语言
	c.Language = "zh"

	// ===== 安装类型 =====
	if c.Env.DockerEngine == types.DockerEngineLocal {
		c.InstallType = "container"
		c.DockerConnType = "local"
		c.DockerSockPath = "/var/run/docker.sock"
	} else if c.Env.PodmanExists {
		c.InstallType = "container"
		c.DockerConnType = "local"
		c.DockerSockPath = getPodmanSockPath()
	} else {
		if c.Env.OS == "linux" {
			c.InstallType = "install_docker"
		} else {
			c.InstallType = "binary"
		}
	}

	// ===== 版本配置 =====
	c.Version = "community"
	c.Edition = "lite"
	c.OS = "debian"

	// ===== 镜像源：默认为空（使用 docker.io）， =====
	// 用户可在 TUI 中根据网络情况选择
	c.Registry = ""

	// ===== 容器配置 =====
	c.ContainerName = "dpanel"
	c.Port = 0

	// 数据路径根据系统选择
	switch c.Env.OS {
	case "windows":
		c.DataPath = `C:\dpanel\data`
	case "darwin":
		c.DataPath = "/Users/Shared/dpanel"
	default:
		c.DataPath = "/home/dpanel"
	}

	// ===== 网络配置 =====
	c.DNS = ""
	c.HTTPProxy = ""
	c.HTTPSProxy = ""

	// ===== 升级/卸载配置 =====
	c.UpgradeBackup = true
	c.UninstallRemoveData = false
}

// ApplyOptions 批量应用选项
func (c *Config) ApplyOptions(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

// IsContainerAvailable 容器安装是否可用
func (c *Config) IsContainerAvailable() bool {
	return c.Env.ContainerAvailable()
}

// GetRegistry 获取镜像仓库地址
func (c *Config) GetRegistry() string {
	if c.Registry == "aliyun" {
		return "registry.cn-hangzhou.aliyuncs.com"
	}
	return ""
}

// GetImageName 获取镜像名称
func (c *Config) GetImageName() string {
	registry := c.GetRegistry()

	var name string
	switch c.Version {
	case "community":
		name = "dpanel/dpanel"
	case "pro":
		name = "dpanel/dpanel-pe"
	default:
		name = "dpanel/dpanel"
	}

	var tag string
	if c.Edition == "lite" {
		tag = "lite"
	} else if c.Version == "dev" {
		tag = "beta"
	} else {
		tag = "latest"
	}

	if registry != "" {
		return fmt.Sprintf("%s/%s:%s", registry, name, tag)
	}
	return fmt.Sprintf("%s:%s", name, tag)
}
