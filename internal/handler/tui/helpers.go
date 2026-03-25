package tui

import (
	"github.com/dpanel-dev/installer/internal/config"
)

// GetStepDefinition 获取步骤定义
func GetStepDefinition(step Step) StepDefinition {
	if def, ok := StepDefinitions[step]; ok {
		return def
	}
	// 返回默认定义
	return StepDefinition{
		ID:    step,
		Type:  StepTypeMenu,
		Next:  NextStep(step),
	}
}

// ApplyConfig 应用配置到 Config
func ApplyConfig(cfg *config.Config, step Step, value string) error {
	if applier, ok := ConfigAppliers[step]; ok {
		return applier(cfg, value)
	}
	return nil
}

// GetNextStep 获取下一步
func GetNextStep(step Step, cfg *config.Config, selectedValue string) Step {
	if def, ok := StepDefinitions[step]; ok && def.Next != nil {
		return def.Next(cfg, selectedValue)
	}
	// 默认返回下一步（假设步骤是连续的）
	return Step(int(step) + 1)
}
