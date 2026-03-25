package tui

import (
	"github.com/dpanel-dev/installer/internal/config"
)

// DynamicOptionGenerator 动态选项生成器类型
type DynamicOptionGenerator func(m *model, cfg *config.Config) []OptionItem

// 动态选项生成器映射
var DynamicOptionGenerators = map[Step]DynamicOptionGenerator{
	StepInstallType: generateInstallTypeOptions,
	StepEdition:     generateEditionOptions,
}

// generateInstallTypeOptions 根据环境生成安装类型选项
func generateInstallTypeOptions(m *model, cfg *config.Config) []OptionItem {
	dockerAvailable := m.envCheck.DockerAvailable || m.envCheck.PodmanAvailable

	if dockerAvailable {
		return []OptionItem{
			{
				Value:       config.InstallTypeContainer,
				Label:       "container_install",
				Description: "container_install_desc",
			},
			{
				Value:       config.InstallTypeBinary,
				Label:       "binary_install",
				Description: "binary_install_desc",
			},
		}
	}

	// Docker 不可用 - Linux 可以安装 Docker
	if m.osType == "linux" {
		return []OptionItem{
			{
				Value:       config.InstallTypeInstallDocker,
				Label:       "install_docker",
				Description: "install_docker_linux_desc",
			},
			{
				Value:       config.InstallTypeBinary,
				Label:       "binary_install",
				Description: "binary_install_desc",
			},
		}
	}

	// Docker 不可用 - Windows/macOS
	return []OptionItem{
		{
			Value:         config.InstallTypeContainer,
			Label:         "container_install",
			Description:   "container_install_desc",
			Disabled:      true,
			DisabledReason: "container_install_disabled",
		},
		{
			Value:       config.InstallTypeBinary,
			Label:       "binary_install",
			Description: "binary_install_desc",
		},
	}
}

// generateEditionOptions 根据安装类型生成版本选项
func generateEditionOptions(m *model, cfg *config.Config) []OptionItem {
	options := []OptionItem{
		{
			Value:       config.EditionStandard,
			Label:       "standard_edition",
			Description: "standard_edition_desc",
		},
		{
			Value:       config.EditionLite,
			Label:       "lite_edition",
			Description: "lite_edition_desc",
		},
	}

	// 二进制安装只支持精简版
	if cfg.InstallType == config.InstallTypeBinary {
		options[0].Disabled = true
		options[0].DisabledReason = "binary_install_edition_warning"
	}

	return options
}
