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
				Description: "Base image system: alpine or debian",
				EnumValues:  []string{types.BaseImageAlpine, types.BaseImageDebian},
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
			// Docker 连接类型
			{
				Name:        "docker-type",
				Type:        FlagTypeEnum,
				Default:     types.DockerConnLocal,
				Description: "Docker connection type: local, tcp, ssh",
				EnumValues:  []string{types.DockerConnLocal, types.DockerConnTCP, types.DockerConnSSH},
				Apply: func(value string) (config.Option, error) {
					// 这个 flag 只是标记，实际连接由其他 flags 组合处理
					return nil, nil
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
			// Docker Host (TCP/SSH)
			{
				Name:        "docker-host",
				Type:        FlagTypeString,
				Description: "Docker host address (for TCP/SSH connection)",
				Apply: func(value string) (config.Option, error) {
					// 由 parser 根据 docker-type 组合处理
					return nil, nil
				},
			},
			// TLS
			{
				Name:        "tls",
				Type:        FlagTypeBool,
				Description: "Enable TLS for TCP connection",
				Apply: func(value string) (config.Option, error) {
					// 由 parser 根据情况组合处理
					return nil, nil
				},
			},
			// TLS 路径
			{
				Name:        "tls-path",
				Type:        FlagTypeString,
				Description: "TLS certificates path",
				Apply: func(value string) (config.Option, error) {
					return config.WithContainerTLS(
						value+"/ca.pem",
						value+"/cert.pem",
						value+"/key.pem",
					), nil
				},
			},
			// SSH 用户名
			{
				Name:        "ssh-user",
				Type:        FlagTypeString,
				Description: "SSH username (for SSH connection)",
				Apply: func(value string) (config.Option, error) {
					// 由 parser 组合处理
					return nil, nil
				},
			},
			// SSH 密码
			{
				Name:        "ssh-password",
				Type:        FlagTypeString,
				Description: "SSH password",
				Apply: func(value string) (config.Option, error) {
					return config.WithContainerSSHAuth(value, ""), nil
				},
			},
			// SSH Key
			{
				Name:        "ssh-key",
				Type:        FlagTypeString,
				Description: "SSH private key path",
				Apply: func(value string) (config.Option, error) {
					return config.WithContainerSSHAuth("", value), nil
				},
			},
			// HTTP Proxy
			{
				Name:        "http-proxy",
				Type:        FlagTypeString,
				Description: "HTTP proxy address",
				Apply: func(value string) (config.Option, error) {
					return config.WithHTTPProxy(value), nil
				},
			},
			// HTTPS Proxy
			{
				Name:        "https-proxy",
				Type:        FlagTypeString,
				Description: "HTTPS proxy address",
				Apply: func(value string) (config.Option, error) {
					return config.WithHTTPSProxy(value), nil
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
			// 镜像仓库
			{
				Name:        "registry",
				Type:        FlagTypeString,
				Description: "Image registry (e.g., registry.cn-hangzhou.aliyuncs.com)",
				Apply: func(value string) (config.Option, error) {
					return config.WithRegistry(value), nil
				},
			},
		},
	},
	{
		Name:        "uninstall",
		Description: "Uninstall DPanel",
		Action:      types.ActionUninstall,
		Flags: []FlagDefinition{
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
