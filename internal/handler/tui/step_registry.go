package tui

import (
	"strconv"

	"github.com/dpanel-dev/installer/internal/config"
)

// StepRegistry 步骤注册表
var StepDefinitions = map[Step]StepDefinition{
	StepLanguage: {
		ID:       StepLanguage,
		Type:     StepTypeMenu,
		TitleKey: "select_language",
		Next: func(cfg *config.Config, selectedValue string) Step {
			cfg.Language = selectedValue
			return StepAction
		},
	},

	StepAction: {
		ID:       StepAction,
		Type:     StepTypeMenu,
		TitleKey: "select_action",
		Options: []OptionItem{
			{
				Value:       config.ActionInstall,
				Label:       "install_panel",
				Description: "install_panel_desc",
			},
			{
				Value:       config.ActionUpgrade,
				Label:       "upgrade_panel",
				Description: "upgrade_panel_desc",
			},
			{
				Value:       config.ActionUninstall,
				Label:       "uninstall_panel",
				Description: "uninstall_panel_desc",
			},
		},
		Next: NextStep(StepInstallType),
	},

	StepInstallType: {
		ID:       StepInstallType,
		Type:     StepTypeMenu,
		TitleKey: "install_method",
		OptionsGenerator: func(m *model, cfg *config.Config) []OptionItem {
			return DynamicOptionGenerators[StepInstallType](m, cfg)
		},
		Next: func(cfg *config.Config, selectedValue string) Step {
			cfg.InstallType = selectedValue
			return StepVersion
		},
	},

	StepVersion: {
		ID:       StepVersion,
		Type:     StepTypeMenu,
		TitleKey: "select_version",
		Options: []OptionItem{
			{
				Value:       config.VersionCommunity,
				Label:       "community_edition",
				Description: "community_edition_desc",
			},
			{
				Value:       config.VersionPro,
				Label:       "professional_edition",
				Description: "professional_edition_desc",
			},
			{
				Value:       config.VersionDev,
				Label:       "development_edition",
				Description: "development_edition_desc",
			},
		},
		Next: func(cfg *config.Config, selectedValue string) Step {
			cfg.Version = selectedValue
			return StepEdition
		},
	},

	StepEdition: {
		ID:       StepEdition,
		Type:     StepTypeMenu,
		TitleKey: "select_edition",
		OptionsGenerator: func(m *model, cfg *config.Config) []OptionItem {
			return DynamicOptionGenerators[StepEdition](m, cfg)
		},
		Next: func(cfg *config.Config, selectedValue string) Step {
			cfg.Edition = selectedValue
			if cfg.InstallType == config.InstallTypeBinary {
				return StepContainerName
			}
			return StepOS
		},
	},

	StepContainerName: {
		ID:           StepContainerName,
		Type:         StepTypeInput,
		TitleKey:     "container_name",
		InputKey:     "ContainerName",
		DefaultValue: "dpanel",
		Next: func(cfg *config.Config, selectedValue string) Step {
			cfg.ContainerName = selectedValue
			return StepPort
		},
	},

	StepPort: {
		ID:           StepPort,
		Type:         StepTypeInput,
		TitleKey:     "access_port",
		InputKey:     "Port",
		DefaultValue: "0",
		Next: func(cfg *config.Config, selectedValue string) Step {
			if selectedValue != "" {
				if port, err := strconv.Atoi(selectedValue); err == nil {
					cfg.Port = port
				}
			}
			return StepDataPath
		},
	},

	StepDataPath: {
		ID:           StepDataPath,
		Type:         StepTypeInput,
		TitleKey:     "data_path",
		InputKey:     "DataPath",
		DefaultValue: "/home/dpanel",
		Next: func(cfg *config.Config, selectedValue string) Step {
			cfg.DataPath = selectedValue
			return StepProxy
		},
	},

	StepProxy: {
		ID:          StepProxy,
		Type:        StepTypeInput,
		TitleKey:    "proxy_address",
		InputKey:    "",
		Placeholder: "http://proxy.example.com:8080",
		Next:        NextStep(StepDNS),
	},

	StepDNS: {
		ID:          StepDNS,
		Type:        StepTypeInput,
		TitleKey:    "dns_address",
		InputKey:    "",
		Placeholder: "8.8.8.8",
		Next:        NextStep(StepConfirm),
	},

	StepConfirm: {
		ID:       StepConfirm,
		Type:     StepTypeConfirm,
		TitleKey: "confirm_install",
		Options: []OptionItem{
			{
				Value:       "confirm",
				Label:       "start_installation",
				Description: "confirm",
			},
			{
				Value:       "cancel",
				Label:       "back_to_modify",
				Description: "cancel",
			},
		},
		Next: func(cfg *config.Config, selectedValue string) Step {
			if selectedValue == "confirm" {
				return StepInstalling
			}
			return StepLanguage
		},
	},
}
