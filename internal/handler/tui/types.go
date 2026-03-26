package tui

import (
	"github.com/dpanel-dev/installer/internal/config"
)

// ========== 步骤类型 ==========

// Step 步骤标识
type Step int

// StepType 步骤显示类型
type StepType int

const (
	StepTypeMenu    StepType = iota // 菜单选择
	StepTypeInput                   // 文本输入
	StepTypeConfirm                 // 确认页面
	StepTypeProgress                // 进度显示
	StepTypeComplete                // 完成页面
	StepTypeError                   // 错误页面
)

// ========== 步骤常量 ==========

const (
	StepLanguage Step = iota
	StepAction
	StepMirrorCheck
	StepRegistry
	StepInstallType
	StepInstallDocker      // 确认是否在线安装 Docker
	StepInstallingDocker   // 执行 Docker 在线安装
	StepVersion
	StepEdition
	StepBaseImage
	StepDockerConnection
	StepDockerConfig
	StepTLSConfig
	StepSSHConfig
	StepContainerName
	StepPort
	StepDataPath
	StepProxy
	StepDNS
	StepConfirm
	StepInstalling
	StepComplete
	StepError
)

// String 返回步骤名称
func (s Step) String() string {
	switch s {
	case StepLanguage:
		return "language"
	case StepAction:
		return "action"
	case StepMirrorCheck:
		return "mirror_check"
	case StepRegistry:
		return "registry"
	case StepInstallType:
		return "install_type"
	case StepInstallDocker:
		return "install_docker"
	case StepInstallingDocker:
		return "installing_docker"
	case StepVersion:
		return "version"
	case StepEdition:
		return "edition"
	case StepBaseImage:
		return "base_image"
	case StepDockerConnection:
		return "docker_connection"
	case StepDockerConfig:
		return "docker_config"
	case StepTLSConfig:
		return "tls_config"
	case StepSSHConfig:
		return "ssh_config"
	case StepContainerName:
		return "container_name"
	case StepPort:
		return "port"
	case StepDataPath:
		return "data_path"
	case StepProxy:
		return "proxy"
	case StepDNS:
		return "dns"
	case StepConfirm:
		return "confirm"
	case StepInstalling:
		return "installing"
	case StepComplete:
		return "complete"
	case StepError:
		return "error"
	default:
		return "unknown"
	}
}

// ========== 数据结构 ==========

// MessageType 消息类型
type MessageType int

const (
	MessageTypeInfo MessageType = iota
	MessageTypeWarning
	MessageTypeError
	MessageTypeLoading
)

// MessageContent 消息内容
type MessageContent struct {
	Type    MessageType
	Content string
}

// OptionItem 选项定义
type OptionItem struct {
	Value       string // 选项值
	Label       string // 显示标签（i18n key）
	Description string // 描述（i18n key），禁用时显示禁用原因
	Disabled    bool   // 是否禁用
}

// StepDefinition 步骤定义
type StepDefinition struct {
	Type     StepType
	TitleKey string

	// 提示信息（可选，返回 nil 则不显示）
	Message func(cfg *config.Config) *MessageContent

	// 输入类型
	DefaultValue func(cfg *config.Config) string
	Placeholder  string

	// 菜单类型（统一为函数形式）
	Options func(cfg *config.Config) []OptionItem

	// 选中/输入后更新 config
	Finish func(cfg *config.Config, value string) error

	// 决定下一步（可选，默认 Step+1）
	Next func(cfg *config.Config) Step
}

// ========== 辅助函数 ==========

// NextStep 创建固定下一步
func NextStep(step Step) func(cfg *config.Config) Step {
	return func(cfg *config.Config) Step {
		return step
	}
}
