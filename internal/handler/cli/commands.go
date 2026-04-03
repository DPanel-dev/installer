package cli

import (
	"fmt"
	"strconv"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
)

// Commands 命令定义注册表
var Commands = []CommandDefinition{
	{
		Name:        "install",
		Description: "Install DPanel",
		Action:      types.ActionInstall,
		Flags: []FlagDefinition{
			// 安装类型
			{
				Name:        "type",
				Type:        FlagTypeEnum,
				Default:     types.InstallTypeContainer,
				Description: "Install type: container or binary",
				EnumValues:  []string{types.InstallTypeContainer, types.InstallTypeBinary},
				Apply: func(value string) (config.Option, error) {
					return config.WithInstallType(value), nil
				},
			},
			// 版本
			{
				Name:        "version",
				Type:        FlagTypeEnum,
				Default:     types.VersionCE,
				Description: "DPanel version: ce (community), pe (pro), be (dev)",
				EnumValues:  []string{types.VersionCE, types.VersionPE, types.VersionBE},
				Apply: func(value string) (config.Option, error) {
					return config.WithVersion(value), nil
				},
			},
			// 版本类型
			{
				Name:        "edition",
				Type:        FlagTypeEnum,
				Default:     types.EditionLite,
				Description: "Edition: standard or lite",
				EnumValues:  []string{types.EditionStandard, types.EditionLite},
				Apply: func(value string) (config.Option, error) {
					return config.WithEdition(value), nil
				},
			},
			// 基础镜像
			{
				Name:        "base-image",
				Type:        FlagTypeEnum,
				Default:     types.BaseImageAlpine,
				Description: "Base image system: alpine, debian, darwin, windows",
				EnumValues:  []string{types.BaseImageAlpine, types.BaseImageDebian, types.BaseImageDarwin, types.BaseImageWindows},
				Apply: func(value string) (config.Option, error) {
					return config.WithBaseImage(value), nil
				},
			},
			// 镜像仓库
			{
				Name:        "registry",
				Type:        FlagTypeString,
				Description: "Image registry (e.g., registry.cn-hangzhou.aliyuncs.com)",
				Apply: func(value string) (config.Option, error) {
					return config.WithRegistry(value), nil
				},
			},
			// 容器名称
			{
				Name:        "name",
				Type:        FlagTypeString,
				Default:     "dpanel",
				Description: "Container name",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("container name cannot be empty")
					}
					return config.WithContainerName(value), nil
				},
			},
			// 端口
			{
				Name:        "port",
				Type:        FlagTypeInt,
				Default:     "8080",
				Description: "Port to expose (0 for random)",
				Apply: func(value string) (config.Option, error) {
					port, err := strconv.Atoi(value)
					if err != nil {
						return nil, fmt.Errorf("invalid port: %s", value)
					}
					return config.WithPort(port), nil
				},
			},
			// 数据路径
			{
				Name:        "data-path",
				Type:        FlagTypeString,
				Description: "Data storage path",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("data path cannot be empty")
					}
					return config.WithDataPath(value), nil
				},
			},
			// 二进制路径
			{
				Name:        "binary-path",
				Type:        FlagTypeString,
				Description: "Binary installation path (for binary install type)",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("binary path cannot be empty")
					}
					return config.WithBinaryPath(value), nil
				},
			},
			// Docker Socket
			{
				Name:        "docker-sock",
				Type:        FlagTypeString,
				Description: "Docker socket path (for local connection)",
				Apply: func(value string) (config.Option, error) {
					return config.WithContainerSock(value), nil
				},
			},
			// Proxy
			{
				Name:        "proxy",
				Type:        FlagTypeString,
				Description: "Proxy address (used for both HTTP and HTTPS)",
				Apply: func(value string) (config.Option, error) {
					return config.WithHTTPProxy(value), nil
				},
			},
			// DNS
			{
				Name:        "dns",
				Type:        FlagTypeString,
				Description: "DNS server address",
				Apply: func(value string) (config.Option, error) {
					return config.WithDNS(value), nil
				},
			},
		},
	},
	{
		Name:        "upgrade",
		Description: "Upgrade existing DPanel installation",
		Action:      types.ActionUpgrade,
		Flags: []FlagDefinition{
			// 安装类型（可选，自动检测）
			{
				Name:        "type",
				Type:        FlagTypeEnum,
				Description: "Install type: container or binary (auto-detected if not specified)",
				EnumValues:  []string{types.InstallTypeContainer, types.InstallTypeBinary},
				Apply: func(value string) (config.Option, error) {
					return config.WithInstallType(value), nil
				},
			},
			// 版本
			{
				Name:        "version",
				Type:        FlagTypeEnum,
				Default:     types.VersionCE,
				Description: "DPanel version: ce (community), pe (pro), be (dev)",
				EnumValues:  []string{types.VersionCE, types.VersionPE, types.VersionBE},
				Apply: func(value string) (config.Option, error) {
					return config.WithVersion(value), nil
				},
			},
			// 版本类型
			{
				Name:        "edition",
				Type:        FlagTypeEnum,
				Default:     types.EditionLite,
				Description: "Edition: standard or lite",
				EnumValues:  []string{types.EditionStandard, types.EditionLite},
				Apply: func(value string) (config.Option, error) {
					return config.WithEdition(value), nil
				},
			},
			// 基础镜像
			{
				Name:        "base-image",
				Type:        FlagTypeEnum,
				Default:     types.BaseImageAlpine,
				Description: "Base image system: alpine, debian, darwin, windows",
				EnumValues:  []string{types.BaseImageAlpine, types.BaseImageDebian, types.BaseImageDarwin, types.BaseImageWindows},
				Apply: func(value string) (config.Option, error) {
					return config.WithBaseImage(value), nil
				},
			},
			// 镜像仓库
			{
				Name:        "registry",
				Type:        FlagTypeString,
				Description: "Image registry (e.g., registry.cn-hangzhou.aliyuncs.com)",
				Apply: func(value string) (config.Option, error) {
					return config.WithRegistry(value), nil
				},
			},
			// 容器名称
			{
				Name:        "name",
				Type:        FlagTypeString,
				Default:     "dpanel",
				Description: "Container name",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("container name cannot be empty")
					}
					return config.WithContainerName(value), nil
				},
			},
			// 端口
			{
				Name:        "port",
				Type:        FlagTypeInt,
				Default:     "8080",
				Description: "Port to expose (0 for random)",
				Apply: func(value string) (config.Option, error) {
					port, err := strconv.Atoi(value)
					if err != nil {
						return nil, fmt.Errorf("invalid port: %s", value)
					}
					return config.WithPort(port), nil
				},
			},
			// 数据路径
			{
				Name:        "data-path",
				Type:        FlagTypeString,
				Description: "Data storage path",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("data path cannot be empty")
					}
					return config.WithDataPath(value), nil
				},
			},
			// 二进制路径
			{
				Name:        "binary-path",
				Type:        FlagTypeString,
				Description: "Binary installation path (for binary install type)",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("binary path cannot be empty")
					}
					return config.WithBinaryPath(value), nil
				},
			},
			// Docker Socket
			{
				Name:        "docker-sock",
				Type:        FlagTypeString,
				Description: "Docker socket path (for local connection)",
				Apply: func(value string) (config.Option, error) {
					return config.WithContainerSock(value), nil
				},
			},
			// 是否备份
			{
				Name:        "backup",
				Type:        FlagTypeBool,
				Default:     "true",
				Description: "Create backup before upgrade",
				Apply: func(value string) (config.Option, error) {
					backup := value == "true" || value == "1"
					return config.WithUpgradeBackup(backup), nil
				},
			},
		},
	},
	{
		Name:        "uninstall",
		Description: "Uninstall DPanel",
		Action:      types.ActionUninstall,
		Flags: []FlagDefinition{
			// 安装类型（可选，自动检测）
			{
				Name:        "type",
				Type:        FlagTypeEnum,
				Description: "Install type: container or binary (auto-detected if not specified)",
				EnumValues:  []string{types.InstallTypeContainer, types.InstallTypeBinary},
				Apply: func(value string) (config.Option, error) {
					return config.WithInstallType(value), nil
				},
			},
			// 容器名称
			{
				Name:        "name",
				Type:        FlagTypeString,
				Default:     "dpanel",
				Description: "Container name",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("container name cannot be empty")
					}
					return config.WithContainerName(value), nil
				},
			},
			// 数据路径
			{
				Name:        "data-path",
				Type:        FlagTypeString,
				Description: "Data storage path",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("data path cannot be empty")
					}
					return config.WithDataPath(value), nil
				},
			},
			// 二进制路径
			{
				Name:        "binary-path",
				Type:        FlagTypeString,
				Description: "Binary installation path (for binary install type)",
				Apply: func(value string) (config.Option, error) {
					if value == "" {
						return nil, fmt.Errorf("binary path cannot be empty")
					}
					return config.WithBinaryPath(value), nil
				},
			},
			// Docker Socket
			{
				Name:        "docker-sock",
				Type:        FlagTypeString,
				Description: "Docker socket path (for local connection)",
				Apply: func(value string) (config.Option, error) {
					return config.WithContainerSock(value), nil
				},
			},
			// 是否删除数据
			{
				Name:        "remove-data",
				Type:        FlagTypeBool,
				Default:     "false",
				Description: "Remove data directory",
				Apply: func(value string) (config.Option, error) {
					remove := value == "true" || value == "1"
					return config.WithUninstallRemoveData(remove), nil
				},
			},
		},
	},
}

// GetCommand 根据名称获取命令定义
func GetCommand(name string) *CommandDefinition {
	for i := range Commands {
		if Commands[i].Name == name {
			return &Commands[i]
		}
	}
	return nil
}
