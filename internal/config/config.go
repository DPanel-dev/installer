package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dpanel-dev/installer/internal/types"
	dockerpkg "github.com/dpanel-dev/installer/pkg/docker"
)

// Config 安装配置
type Config struct {
	OS   string
	Arch string

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
	Port          int // 0 = 随机端口
	DataPath      string

	// === 网络配置 ===
	DNS        string
	HTTPProxy  string
	HTTPSProxy string

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
		State:       make(map[string]any),
		InstallType: types.InstallTypeBinary,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
	}

	if cli, err := dockerpkg.New(); err == nil {
		c.Client = cli
		c.InstallType = types.InstallTypeContainer
	}

	// 2. 镜像源默认为空，由 TUI 在运行时检测
	c.Registry = ""

	// 3. 根据环境设置最优默认值
	// 操作类型
	c.Action = types.ActionInstall

	// 语言
	c.Language = types.LanguageZh

	// ===== 版本配置 =====
	c.Version = types.VersionCE
	c.Edition = types.EditionLite

	// 基础镜像：二进制安装时根据系统 libc 类型自动选择
	c.BaseImage = types.ImageBaseAlpine

	if c.InstallType == types.InstallTypeBinary {
		if IsMusl() {
			c.BaseImage = types.ImageBaseAlpine
		} else {
			c.BaseImage = types.ImageBaseDebian
		}
	}

	// ===== 容器配置 =====
	c.ContainerName = "dpanel"
	c.Port = FindAvailablePort(8080)

	// 数据路径根据系统选择
	homeDir, _ := os.UserHomeDir()
	c.DataPath = filepath.Join(homeDir, "dpanel", "data")

	// ===== 网络配置 =====
	c.DNS = ""
	c.HTTPProxy = ""
	c.HTTPSProxy = ""

	// ===== 升级/卸载配置 =====
	c.UpgradeBackup = true
	c.UninstallRemoveData = false

	// 4. 应用用户自定义选项
	for _, opt := range opts {
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
// 组合规则：版本(ce/pe/be) + 版本类型(standard/lite) + 基础镜像(alpine/debian)
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

	// 2. 组合 Tag
	// 格式：[beta-][lite-][debian]
	var tagParts []string

	// 开发版前缀
	if c.Version == types.VersionBE {
		tagParts = append(tagParts, types.TagBeta)
	}

	// 精简版
	if c.Edition == types.EditionLite {
		tagParts = append(tagParts, types.TagLite)
	}

	// Debian 基础镜像
	if c.BaseImage == types.ImageBaseDebian {
		tagParts = append(tagParts, types.TagDebian)
	}

	// 组合 Tag
	tag := strings.Join(tagParts, "-")
	if tag == "" {
		tag = "latest"
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
