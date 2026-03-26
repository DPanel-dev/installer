package tui

import (
	"strconv"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
	"github.com/dpanel-dev/installer/pkg/i18n"
)

// StepDefinitions 步骤定义注册表
var StepDefinitions = map[Step]StepDefinition{
	// ========== 语言选择 ==========
	StepLanguage: {
		Type:     StepTypeMenu,
		TitleKey: "select_language",
		Options: func(cfg *config.Config) []OptionItem {
			// 语言选择时 i18n 未初始化，使用硬编码文本
			return []OptionItem{
				{Value: "en", Label: "English", Description: "English language interface"},
				{Value: "zh", Label: "简体中文", Description: "简体中文界面"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.Language = value
			// 初始化语言包
			if err := i18n.Init(value); err != nil {
				return err
			}
			return nil
		},
		Next: NextStep(StepAction),
	},

	// ========== 操作选择 ==========
	StepAction: {
		Type:     StepTypeMenu,
		TitleKey: "select_action",
		Options: func(cfg *config.Config) []OptionItem {
			canInstall := cfg.Registry != "unavailable"
			return []OptionItem{
				{Value: types.ActionInstall, Label: "install_panel", Description: "install_panel_desc", Disabled: !canInstall},
				{Value: types.ActionUpgrade, Label: "upgrade_panel", Description: "upgrade_panel_desc", Disabled: !canInstall},
				{Value: types.ActionUninstall, Label: "uninstall_panel", Description: "uninstall_panel_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.Action = value
			return nil
		},
		Next: NextStep(StepInstallType),
	},

	// ========== 安装方式 ==========
	StepInstallType: {
		Type:     StepTypeMenu,
		TitleKey: "install_method",
		Options: func(cfg *config.Config) []OptionItem {
			if cfg.Env.ContainerConn != nil {
				return []OptionItem{
					{Value: types.InstallTypeContainer, Label: "container_install", Description: "container_install_desc"},
					{Value: types.InstallTypeBinary, Label: "binary_install", Description: "binary_install_desc"},
				}
			}
			// Docker 不可用 - Linux
			if cfg.Env.OS == "linux" {
				return []OptionItem{
					{Value: types.InstallTypeContainer, Label: "container_install", Description: "install_docker_linux_desc"},
					{Value: types.InstallTypeBinary, Label: "binary_install", Description: "binary_install_desc"},
				}
			}
			// Windows/macOS Docker 不可用
			return []OptionItem{
				{Value: types.InstallTypeContainer, Label: "container_install", Description: "container_install_disabled", Disabled: true},
				{Value: types.InstallTypeBinary, Label: "binary_install", Description: "binary_install_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.InstallType = value
			return nil
		},
		Next: NextStep(StepVersion),
	},

	// ========== 版本选择 ==========
	StepVersion: {
		Type:     StepTypeMenu,
		TitleKey: "select_version",
		Options: func(cfg *config.Config) []OptionItem {
			return []OptionItem{
				{Value: types.VersionCommunity, Label: "community_edition", Description: "community_edition_desc"},
				{Value: types.VersionPro, Label: "professional_edition", Description: "professional_edition_desc"},
				{Value: types.VersionDev, Label: "development_edition", Description: "development_edition_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.Version = value
			return nil
		},
		Next: NextStep(StepEdition),
	},

	// ========== 版本类型 ==========
	StepEdition: {
		Type:     StepTypeMenu,
		TitleKey: "select_edition",
		Options: func(cfg *config.Config) []OptionItem {
			items := []OptionItem{
				{Value: types.EditionStandard, Label: "standard_edition", Description: "standard_edition_desc"},
				{Value: types.EditionLite, Label: "lite_edition", Description: "lite_edition_desc"},
			}
			// 二进制安装只支持精简版
			if cfg.InstallType == types.InstallTypeBinary {
				items[0].Disabled = true
				items[0].Description = "binary_install_edition_warning"
			}
			return items
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.Edition = value
			return nil
		},
		Next: func(cfg *config.Config) Step {
			if cfg.InstallType == types.InstallTypeBinary {
				return StepContainerName
			}
			return StepBaseImage
		},
	},

	// ========== 基础镜像系统 ==========
	StepBaseImage: {
		Type:     StepTypeMenu,
		TitleKey: "select_base_image",
		Options: func(cfg *config.Config) []OptionItem {
			return []OptionItem{
				{Value: types.BaseImageAlpine, Label: "alpine", Description: "alpine_desc"},
				{Value: types.BaseImageDebian, Label: "debian", Description: "debian_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.BaseImage = value
			return nil
		},
		Next: NextStep(StepRegistry),
	},

	// ========== 镜像仓库 ==========
	StepRegistry: {
		Type:     StepTypeMenu,
		TitleKey: "select_registry",
		Options: func(cfg *config.Config) []OptionItem {
			return []OptionItem{
				{Value: types.RegistryDockerHub, Label: "docker_hub", Description: "docker_hub_desc"},
				{Value: types.RegistryAliYun, Label: "aliyun", Description: "aliyun_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.Registry = value
			return nil
		},
		Next: NextStep(StepDockerConnection),
	},

	// ========== Docker 连接方式 ==========
	StepDockerConnection: {
		Type:     StepTypeMenu,
		TitleKey: "docker_connection",
		Options: func(cfg *config.Config) []OptionItem {
			return []OptionItem{
				{Value: string(types.ContainerConnTypeSock), Label: "local_sock", Description: "local_sock_desc"},
				{Value: string(types.ContainerConnTypeTCP), Label: "remote_tcp", Description: "remote_tcp_desc"},
				{Value: string(types.ContainerConnTypeSSH), Label: "remote_ssh", Description: "remote_ssh_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			// 初始化容器连接配置
			if cfg.Env.ContainerConn == nil {
				cfg.Env.ContainerConn = &config.ContainerConn{
					Engine: types.ContainerEngineDocker,
				}
			}
			cfg.Env.ContainerConn.Type = value
			return nil
		},
		Next: func(cfg *config.Config) Step {
			if cfg.Env.ContainerConn == nil {
				return StepContainerName
			}
			switch cfg.Env.ContainerConn.Type {
			case types.ContainerConnTypeTCP:
				return StepDockerConfig
			case types.ContainerConnTypeSSH:
				return StepSSHConfig
			default:
				return StepContainerName
			}
		},
	},

	// ========== Docker 配置（TCP） ==========
	StepDockerConfig: {
		Type:         StepTypeInput,
		TitleKey:     "docker_host",
		Placeholder:  "tcp://localhost:2375",
		DefaultValue: "tcp://localhost:2375",
		Finish: func(cfg *config.Config, value string) error {
			if cfg.Env.ContainerConn == nil {
				cfg.Env.ContainerConn = &config.ContainerConn{
					Engine: types.ContainerEngineDocker,
					Type:   types.ContainerConnTypeTCP,
				}
			}
			cfg.Env.ContainerConn.Address = value
			return nil
		},
		Next: NextStep(StepTLSConfig),
	},

	// ========== TLS 配置 ==========
	StepTLSConfig: {
		Type:     StepTypeMenu,
		TitleKey: "tls_config",
		Options: func(cfg *config.Config) []OptionItem {
			return []OptionItem{
				{Value: "yes", Label: "yes", Description: "enable_tls_desc"},
				{Value: "no", Label: "no", Description: "disable_tls_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			if cfg.Env.ContainerConn != nil {
				cfg.Env.ContainerConn.TLSVerify = value == "yes"
			}
			return nil
		},
		Next: NextStep(StepContainerName),
	},

	// ========== SSH 配置 ==========
	StepSSHConfig: {
		Type:        StepTypeInput,
		TitleKey:    "ssh_host",
		Placeholder: "ssh://host:22",
		Finish: func(cfg *config.Config, value string) error {
			if cfg.Env.ContainerConn == nil {
				cfg.Env.ContainerConn = &config.ContainerConn{
					Engine: types.ContainerEngineDocker,
					Type:   types.ContainerConnTypeSSH,
				}
			}
			cfg.Env.ContainerConn.Address = value
			return nil
		},
		Next: NextStep(StepContainerName),
	},

	// ========== 容器名称 ==========
	StepContainerName: {
		Type:         StepTypeInput,
		TitleKey:     "container_name",
		DefaultValue: "dpanel",
		Finish: func(cfg *config.Config, value string) error {
			cfg.ContainerName = value
			return nil
		},
		Next: NextStep(StepPort),
	},

	// ========== 端口 ==========
	StepPort: {
		Type:         StepTypeInput,
		TitleKey:     "access_port",
		DefaultValue: "80",
		Finish: func(cfg *config.Config, value string) error {
			if value != "" {
				if port, err := strconv.Atoi(value); err == nil {
					cfg.Port = port
				}
			}
			return nil
		},
		Next: NextStep(StepDataPath),
	},

	// ========== 数据路径 ==========
	StepDataPath: {
		Type:         StepTypeInput,
		TitleKey:     "data_path",
		DefaultValue: "/home/dpanel",
		Finish: func(cfg *config.Config, value string) error {
			cfg.DataPath = value
			return nil
		},
		Next: NextStep(StepProxy),
	},

	// ========== 代理 ==========
	StepProxy: {
		Type:        StepTypeInput,
		TitleKey:    "proxy_address",
		Placeholder: "http://proxy.example.com:8080",
		Finish: func(cfg *config.Config, value string) error {
			cfg.HTTPProxy = value
			return nil
		},
		Next: NextStep(StepDNS),
	},

	// ========== DNS ==========
	StepDNS: {
		Type:        StepTypeInput,
		TitleKey:    "dns_address",
		Placeholder: "8.8.8.8",
		Finish: func(cfg *config.Config, value string) error {
			cfg.DNS = value
			return nil
		},
		Next: NextStep(StepConfirm),
	},

	// ========== 确认安装 ==========
	StepConfirm: {
		Type:     StepTypeConfirm,
		TitleKey: "confirm_install",
		Options: func(cfg *config.Config) []OptionItem {
			return []OptionItem{
				{Value: "confirm", Label: "start_installation", Description: "confirm_desc"},
				{Value: "cancel", Label: "back_to_modify", Description: "cancel_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			return nil
		},
		Next: func(cfg *config.Config) Step {
			return StepInstalling
		},
	},
}

// GetStepDef 获取步骤定义
func GetStepDef(step Step) StepDefinition {
	if def, ok := StepDefinitions[step]; ok {
		return def
	}
	return StepDefinition{Type: StepTypeError, TitleKey: "unknown_step"}
}

// GetPrevStep 获取上一步
func GetPrevStep(step Step) Step {
	if prev, ok := prevStepMap[step]; ok {
		return prev
	}
	if step > StepLanguage {
		return step - 1
	}
	return step
}
