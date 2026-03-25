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
	StepInstallType
	StepVersion
	StepEdition
	StepOS
	StepRegistry
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

// ========== 数据结构 ==========

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

	// 输入类型
	DefaultValue string
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

// prevStepMap 后退步骤映射
var prevStepMap = map[Step]Step{
	StepAction:           StepLanguage,
	StepInstallType:      StepAction,
	StepVersion:          StepInstallType,
	StepEdition:          StepVersion,
	StepOS:               StepEdition,
	StepRegistry:         StepOS,
	StepDockerConnection: StepRegistry,
	StepDockerConfig:     StepDockerConnection,
	StepTLSConfig:        StepDockerConfig,
	StepSSHConfig:        StepDockerConfig,
	StepContainerName:    StepDockerConnection, // 简化，可能需要根据情况调整
	StepPort:             StepContainerName,
	StepDataPath:         StepPort,
	StepProxy:            StepDataPath,
	StepDNS:              StepProxy,
	StepConfirm:          StepDNS,
}
