package tui

import (
	"github.com/dpanel-dev/installer/internal/config"
)

// StepType 步骤类型
type StepType int

const (
	StepTypeMenu    StepType = iota // 菜单选择
	StepTypeInput                  // 文本输入
	StepTypeConfirm                // 确认页面
	StepTypeProgress               // 进度显示
	StepTypeComplete               // 完成页面
	StepTypeError                  // 错误页面
)

// OptionItem 选项定义（新数据结构）
type OptionItem struct {
	// 选项值（如 "community"）
	Value string

	// 显示标签
	Label string

	// 描述
	Description string

	// 禁用状态
	Disabled bool

	// 禁用原因
	DisabledReason string

	// 选择该选项后的下一步（可选）
	Next *Step
}

// StepDefinition 步骤定义（新数据结构）
type StepDefinition struct {
	ID Step

	// 步骤类型
	Type StepType

	// 标题 i18n 键
	TitleKey string

	// 输入字段的 Config 键名（输入类型使用）
	InputKey string

	// 菜单选项（菜单类型使用）
	Options []OptionItem

	// 动态选项生成器
	OptionsGenerator interface{}

	// 默认值/占位符（输入类型使用）
	DefaultValue string
	Placeholder    string

	// 下一步决策函数
	Next NextFunc
}

// NextFunc 下一步决策函数
type NextFunc func(cfg *config.Config, selectedValue string) Step

// ConfigApplier 配置应用函数
type ConfigApplier func(cfg *config.Config, value string) error

// 辅助函数：创建固定下一步的 NextFunc
func NextStep(step Step) NextFunc {
	return func(cfg *config.Config, selectedValue string) Step {
		return step
	}
}
