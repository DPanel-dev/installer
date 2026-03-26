package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/script"
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
		Next: func(cfg *config.Config) Step {
			switch cfg.Action {
			case types.ActionInstall, types.ActionUpgrade:
				// 安装/升级：检测镜像源
				return StepMirrorCheck
			case types.ActionUninstall:
				// 卸载：直接到容器名称
				return StepContainerName
			default:
				return StepInstallType
			}
		},
	},

	// ========== 镜像源检测 ==========
	StepMirrorCheck: {
		Type:     StepTypeProgress,
		TitleKey: "registry_check",
		Finish: func(cfg *config.Config, _ string) error {
			// 检测两个镜像源的延迟
			dockerHubLatency := config.TestRegistryLatency(types.RegistryDockerHub)
			aliYunLatency := config.TestRegistryLatency(types.RegistryAliYun)

			// 存储检测结果
			cfg.State["docker_hub_latency"] = dockerHubLatency
			cfg.State["aliyun_latency"] = aliYunLatency

			// 如果都不可用，记录错误但不立即返回
			if dockerHubLatency == 0 && aliYunLatency == 0 {
				cfg.Registry = "unavailable"
				cfg.State["mirror_check_error"] = i18n.T("no_registry_available")
			}

			return nil
		},
		Next: func(cfg *config.Config) Step {
			if cfg.Registry == "unavailable" {
				return StepError
			}
			return StepRegistry
		},
	},

	// ========== 安装方式 ==========
	StepInstallType: {
		Type:     StepTypeMenu,
		TitleKey: "install_method",
		Options: func(cfg *config.Config) []OptionItem {
			// 有本地容器连接（Docker/Podman 可用）
			if cfg.Env.ContainerConn != nil && ((cfg.Env.ContainerConn.IsDocker() && cfg.Env.ContainerConn.IsLocal()) || cfg.Env.ContainerConn.IsPodman()) {
				return []OptionItem{
					{Value: types.InstallTypeContainer, Label: "container_install", Description: "container_install_desc"},
					{Value: types.InstallTypeBinary, Label: "binary_install", Description: "binary_install_desc"},
				}
			}

			// 没有本地容器连接
			if runtime.GOOS == "linux" {
				// Linux：可以在线安装 Docker（提示在 TUI 提示区域显示）
				return []OptionItem{
					{Value: types.InstallTypeContainer, Label: "container_install", Description: "container_install_desc"},
					{Value: types.InstallTypeBinary, Label: "binary_install", Description: "binary_install_desc"},
				}
			}

			// Windows/macOS：容器安装禁用（提示在 TUI 提示区域显示）
			return []OptionItem{
				{Value: types.InstallTypeContainer, Label: "container_install", Description: "container_install_desc", Disabled: true},
				{Value: types.InstallTypeBinary, Label: "binary_install", Description: "binary_install_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.InstallType = value
			return nil
		},
		Next: func(cfg *config.Config) Step {
			// 选择二进制安装 -> 跳转到版本选择
			if cfg.InstallType == types.InstallTypeBinary {
				return StepVersion
			}

			// 选择容器安装
			// 有本地容器连接 -> 跳转到版本选择
			if cfg.Env.ContainerConn != nil {
				return StepVersion
			}

			// 没有本地容器连接 + Linux -> 跳转到确认在线安装 Docker
			if cfg.Env.OS == "linux" {
				return StepInstallDocker
			}

			// 其他情况（不应该到达这里，因为容器安装已禁用）
			return StepVersion
		},
	},

	// ========== 确认在线安装 Docker ==========
	StepInstallDocker: {
		Type:     StepTypeMenu,
		TitleKey: "install_docker_prompt",
		Options: func(cfg *config.Config) []OptionItem {
			return []OptionItem{
				{Value: "yes", Label: "install_docker_online", Description: "install_docker_online_desc"},
				{Value: "no", Label: "skip_docker_install", Description: "skip_docker_install_desc"},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.State["install_docker_choice"] = value
			return nil
		},
		Next: func(cfg *config.Config) Step {
			choice, _ := cfg.State["install_docker_choice"].(string)
			if choice == "yes" {
				// 选择安装 -> 执行安装
				return StepInstallingDocker
			}
			// 选择跳过 -> 切换到二进制安装
			cfg.InstallType = types.InstallTypeBinary
			return StepVersion
		},
	},

	// ========== 执行 Docker 在线安装 ==========
	StepInstallingDocker: {
		Type:     StepTypeProgress,
		TitleKey: "installing_docker",
		Finish: func(cfg *config.Config, _ string) error {
			// 1. 选择对应的脚本
			var scriptContent string
			if _, err := os.Stat("/etc/alpine-release"); err == nil {
				// Alpine Linux
				scriptContent = script.DockerInstallAlpine
			} else {
				// 标准 Linux
				scriptContent = script.DockerInstallLinux
			}

			// 2. 创建临时文件
			tmpDir := os.TempDir()
			scriptPath := filepath.Join(tmpDir, "dpanel-docker-install.sh")

			if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
				cfg.State["docker_install_error"] = err.Error()
				return nil
			}
			defer os.Remove(scriptPath)

			// 3. 执行脚本
			cmd := exec.Command("sh", scriptPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				cfg.State["docker_install_error"] = err.Error()
				return nil
			}

			// 4. 验证安装
			if _, err := exec.LookPath("docker"); err != nil {
				cfg.State["docker_install_error"] = "docker command not found after installation"
				return nil
			}

			cfg.State["docker_install_success"] = true
			return nil
		},
		Next: func(cfg *config.Config) Step {
			success, _ := cfg.State["docker_install_success"].(bool)
			if success {
				// 安装成功 -> 重新检测环境
				cfg.Env = config.NewEnvCheck()
				return StepVersion
			}
			// 安装失败 -> 切换到二进制安装
			cfg.InstallType = types.InstallTypeBinary
			return StepVersion
		},
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
		Next: NextStep(StepDockerConnection),
	},

	// ========== 镜像仓库 ==========
	StepRegistry: {
		Type:     StepTypeMenu,
		TitleKey: "select_registry",
		Options: func(cfg *config.Config) []OptionItem {
			// 从 State 中读取检测结果
			dockerHubLatency, _ := cfg.State["docker_hub_latency"].(int)
			aliYunLatency, _ := cfg.State["aliyun_latency"].(int)

			// 构建描述（包含延迟信息）
			var dockerHubDesc, aliYunDesc string
			var dockerHubDisabled, aliYunDisabled bool

			if dockerHubLatency > 0 {
				dockerHubDesc = i18n.T("docker_hub_desc") + i18n.Tf("registry_latency", dockerHubLatency)
			} else {
				dockerHubDesc = i18n.T("docker_hub_desc") + i18n.T("registry_unavailable")
				dockerHubDisabled = true
			}

			if aliYunLatency > 0 {
				aliYunDesc = i18n.T("aliyun_desc") + i18n.Tf("registry_latency", aliYunLatency)
			} else {
				aliYunDesc = i18n.T("aliyun_desc") + i18n.T("registry_unavailable")
				aliYunDisabled = true
			}

			return []OptionItem{
				{Value: types.RegistryDockerHub, Label: "docker_hub", Description: dockerHubDesc, Disabled: dockerHubDisabled},
				{Value: types.RegistryAliYun, Label: "aliyun", Description: aliYunDesc, Disabled: aliYunDisabled},
			}
		},
		Finish: func(cfg *config.Config, value string) error {
			cfg.Registry = value
			return nil
		},
		Next: NextStep(StepInstallType),
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
		DefaultValue: func(cfg *config.Config) string {
			return cfg.ContainerName
		},
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
		DefaultValue: func(cfg *config.Config) string {
			return cfg.DataPath
		},
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
