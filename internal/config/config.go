package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/dpanel-dev/installer/internal/types"
	dockerpkg "github.com/dpanel-dev/installer/pkg/docker"
)

// Config 安装配置
type Config struct {
	OS   string
	Arch string

	// === 操作类型 ===

	// === 操作类型 ===
	Action string // install, upgrade, uninstall

	// === 语言 ===
	Language string // zh, en

	// === 安装类型 ===
	InstallType string // container, binary

	// === 版本配置 ===
	Version   string // ce 社区版, pe 专业版, be 开发版
	Edition   string // standard, lite
	BaseImage string // alpine, debian - 镜像基础系统
	Registry  string // docker.io, registry.cn-hangzhou.aliyuncs.com, unavailable

	// === 容器配置 ===
	ContainerName string
	ServerHost    string // 绑定地址：0.0.0.0 或 127.0.0.1
	ServerPort    int    // 0 = 随机端口
	DataPath      string

	// === 二进制配置 ===
	BinaryPath string // 自定义二进制安装路径，为空则使用默认路径

	// === 网络配置 ===
	DNS        string
	HTTPProxy string

	// === 升级配置 ===
	UpgradeBackup bool

	// === 卸载配置 ===
	UninstallRemoveData bool

	// === TUI 临时状态 ===
	State map[string]any

	Client *dockerpkg.Client
}

// Option 配置选项函数
type Option func(*Config) error

// NewConfig 创建配置（自动检测环境 + 智能默认值）
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{
		State: make(map[string]any),
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
	}

	// 1. Docker 检测（结构性初始化）
	if cli, err := dockerpkg.New(); err == nil {
		c.Client = cli
	}

	// 2. 固定默认值
	defaults := []Option{
		WithAction(types.ActionInstall),
		WithLanguage(types.LanguageZh),
		WithVersion(types.VersionCE),
		WithEdition(types.EditionLite),
		WithContainerName("dpanel"),
		WithUpgradeBackup(true),
		WithUninstallRemoveData(false),
	}

	// 3. 根据环境 append
	if c.Client != nil {
		defaults = append(defaults, WithInstallType(types.InstallTypeContainer))
		// 容器安装：默认 Alpine，用户可在 TUI StepBaseImage 或 CLI --base-image 中选择
		defaults = append(defaults, WithBaseImage(types.BaseImageAlpine))
	} else {
		defaults = append(defaults, WithInstallType(types.InstallTypeBinary))
		// 二进制安装：按 OS 设置平台镜像
		switch c.OS {
		case "darwin":
			defaults = append(defaults, WithBaseImage(types.BaseImageDarwin))
		case "windows":
			defaults = append(defaults, WithBaseImage(types.BaseImageWindows))
		default:
			if IsMusl() {
				defaults = append(defaults, WithBaseImage(types.BaseImageAlpine))
			} else {
				defaults = append(defaults, WithBaseImage(types.BaseImageDebian))
			}
		}
	}

	defaults = append(defaults, WithServerHost(types.ServerHostAll))
	defaults = append(defaults, WithServerPort(FindAvailablePort(8080)))

	homeDir, _ := os.UserHomeDir()
	defaults = append(defaults, WithInstallPath(filepath.Join(homeDir, "dpanel")))

	// 4. 先应用默认值，再应用用户覆盖
	for _, opt := range append(defaults, opts...) {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
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

// GetRegistry 获取镜像仓库地址
func (c *Config) GetRegistry() string {
	if c.Registry == types.RegistryDockerHub || c.Registry == types.RegistryUnavailable {
		return ""
	}
	return c.Registry
}

// GetImageName 获取镜像名称
// Tag 格式：{base}[-{imageBase}]
// base: beta | beta-lite | lite | latest
// imageBase: debian | darwin | windows (alpine 无后缀)
func (c *Config) GetImageName() string {
	registry := c.GetRegistry()

	// 1. 确定镜像名称
	var name string
	switch c.Version {
	case types.VersionPE:
		name = types.ImageNamePE
	default:
		name = types.ImageNameCE
	}

	// 2. 组合 base tag
	var baseTag string
	switch {
	case c.Version == types.VersionBE && c.Edition == types.EditionLite:
		baseTag = types.TagBeta + "-" + types.TagLite
	case c.Version == types.VersionBE:
		baseTag = types.TagBeta
	case c.Edition == types.EditionLite:
		baseTag = types.TagLite
	default:
		baseTag = types.TagLatest
	}

	// 3. 追加基础系统后缀（alpine 无后缀）
	suffix := ""
	switch c.BaseImage {
	case types.BaseImageDebian:
		suffix = "debian"
	case types.BaseImageDarwin:
		suffix = "darwin"
	case types.BaseImageWindows:
		suffix = "windows"
	}

	tag := baseTag
	if suffix != "" {
		tag += "-" + suffix
	}

	if registry != "" {
		return fmt.Sprintf("%s/%s:%s", registry, name, tag)
	}
	return fmt.Sprintf("%s:%s", name, tag)
}

// SetStepValue 保存步骤选择值
func (c *Config) SetStepValue(stepName, value string) {
	c.State["step_"+stepName] = value
}

// GetStepValue 获取步骤选择值
func (c *Config) GetStepValue(stepName string) string {
	if v, ok := c.State["step_"+stepName].(string); ok {
		return v
	}
	return ""
}
