package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/handler"
	"github.com/dpanel-dev/installer/internal/types"
)

// CLI 实现 handler.Handler 接口
type CLI struct {
	args    []string
	version string
	commit  string
	date    string
}

// Option 配置选项函数
type Option func(*CLI)

// NewCLI 创建 CLI handler
func NewCLI(opts ...Option) *CLI {
	c := &CLI{
		version: "dev",
		commit:  "none",
		date:    "unknown",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithArgs 设置命令行参数
func WithArgs(args []string) Option {
	return func(c *CLI) {
		c.args = args
	}
}

// WithVersionInfo 设置版本信息
func WithVersionInfo(version, commit, date string) Option {
	return func(c *CLI) {
		c.version = version
		c.commit = commit
		c.date = date
	}
}

// Name 实现 handler.Handler 接口
func (c *CLI) Name() string {
	return "cli"
}

// Run 实现 handler.Handler 接口
func (c *CLI) Run(cfg *config.Config) error {
	return c.run(cfg)
}

// run 运行 CLI 模式
func (c *CLI) run(cfg *config.Config) error {
	args := c.args

	// 1. 处理全局 flags (--help, --version)
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-h":
			c.showRootHelp()
			return nil
		case "--version", "-v":
			c.showVersion()
			return nil
		}
	}

	// 2. 无子命令时显示帮助
	if len(args) == 0 {
		c.showRootHelp()
		return nil
	}

	// 3. 查找子命令
	cmdName := args[0]
	cmd := GetCommand(cmdName)
	if cmd == nil {
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	// 4. 解析子命令 flags
	opts, err := c.parseCommandFlags(cmd, args[1:])
	if err != nil {
		if err.Error() == "help requested" {
			c.showCommandHelp(cmd)
			return nil
		}
		return err
	}

	// 5. 设置 Action
	cfg.Action = cmd.Action

	// 6. 应用到 Config
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("failed to apply config: %w", err)
		}
	}

	return nil
}

// parseCommandFlags 解析子命令的 flags
func (c *CLI) parseCommandFlags(cmd *CommandDefinition, args []string) ([]func(*config.Config) error, error) {
	// 解析结果存储
	values := make(map[string]string)

	// 遍历 args 解析 flags
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// --help
		if arg == "--help" || arg == "-h" {
			return nil, fmt.Errorf("help requested")
		}

		// 解析 flag
		if strings.HasPrefix(arg, "--") {
			// 长格式: --name value 或 --name=value
			name, value := parseLongFlag(arg)
			if value == "" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				value = args[i+1]
				i++
			}
			values[name] = value
		}
	}

	// 查找 flag 定义并应用
	opts := []func(*config.Config) error{}
	for _, flag := range cmd.Flags {
		// 检查是否提供了该 flag
		value, found := values[flag.Name]

		// 如果未提供但有默认值，使用默认值
		if !found && flag.Default != "" {
			value = flag.Default
			found = true
		}

		// 跳过未提供的 flag
		if !found {
			continue
		}

		// 验证枚举值
		if flag.Type == FlagTypeEnum && len(flag.EnumValues) > 0 {
			valid := false
			for _, ev := range flag.EnumValues {
				if ev == value {
					valid = true
					break
				}
			}
			if !valid {
				return nil, fmt.Errorf("invalid value '%s' for --%s, must be one of: %s",
					value, flag.Name, strings.Join(flag.EnumValues, ", "))
			}
		}

		// 应用 flag
		if flag.Apply != nil {
			opt, err := flag.Apply(value)
			if err != nil {
				return nil, fmt.Errorf("--%s: %w", flag.Name, err)
			}
			if opt != nil {
				opts = append(opts, func(cfg *config.Config) error {
					return opt(cfg)
				})
			}
		}
	}

	// 特殊处理：组合 Docker 连接参数
	if dockerType, ok := values["docker-type"]; ok {
		dockerHost, _ := values["docker-host"]
		tls, _ := values["tls"]
		tlsPath, _ := values["tls-path"]
		sshUser, _ := values["ssh-user"]
		sshPassword, _ := values["ssh-password"]
		sshKey, _ := values["ssh-key"]

		switch dockerType {
		case types.DockerConnLocal:
			// 本地连接由 WithContainerSock 处理
		case types.DockerConnTCP:
			if dockerHost != "" {
				tlsVerify := tls == "true"
				opts = append(opts, func(cfg *config.Config) error {
					return config.WithContainerTCP(dockerHost, 2376, tlsVerify)(cfg)
				})
				if tlsVerify && tlsPath != "" {
					opts = append(opts, func(cfg *config.Config) error {
						return config.WithContainerTLS(
							tlsPath+"/ca.pem",
							tlsPath+"/cert.pem",
							tlsPath+"/key.pem",
						)(cfg)
					})
				}
			}
		case types.DockerConnSSH:
			if dockerHost != "" {
				opts = append(opts, func(cfg *config.Config) error {
					return config.WithContainerSSH(dockerHost, 22, sshUser)(cfg)
				})
				if sshPassword != "" || sshKey != "" {
					opts = append(opts, func(cfg *config.Config) error {
						return config.WithContainerSSHAuth(sshPassword, sshKey)(cfg)
					})
				}
			}
		}
	}

	return opts, nil
}

// parseLongFlag 解析长格式 flag
func parseLongFlag(arg string) (name, value string) {
	// 去掉 --
	arg = arg[2:]
	// 检查是否包含 =
	if idx := strings.Index(arg, "="); idx >= 0 {
		return arg[:idx], arg[idx+1:]
	}
	return arg, ""
}

// showRootHelp 显示根帮助
func (c *CLI) showRootHelp() {
	fmt.Println()
	fmt.Println("DPanel Installer - Install, upgrade, and manage DPanel")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  dpanel-installer [command]")
	fmt.Println()
	fmt.Println("Commands:")
	for _, cmd := range Commands {
		fmt.Printf("  %-12s  %s\n", cmd.Name, cmd.Description)
	}
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --help       Show help")
	fmt.Println("  --version    Show version")
	fmt.Println()
	fmt.Println("Use \"dpanel-installer [command] --help\" for more information about a command.")
	fmt.Println()
}

// showCommandHelp 显示子命令帮助
func (c *CLI) showCommandHelp(cmd *CommandDefinition) {
	fmt.Println()
	fmt.Printf("Usage: dpanel-installer %s [flags]\n\n", cmd.Name)
	fmt.Println(cmd.Description)
	fmt.Println()
	fmt.Println("Flags:")

	for _, flag := range cmd.Flags {
		// 构建 flag 字符串
		flagStr := fmt.Sprintf("    --%s", flag.Name)

		// 添加类型信息
		var typeInfo string
		switch flag.Type {
		case FlagTypeEnum:
			typeInfo = fmt.Sprintf("[%s]", strings.Join(flag.EnumValues, "|"))
		case FlagTypeInt:
			typeInfo = "<int>"
		case FlagTypeBool:
			typeInfo = ""
		default:
			typeInfo = "<string>"
		}

		// 添加默认值
		var defaultInfo string
		if flag.Default != "" {
			defaultInfo = fmt.Sprintf(" (default: %s)", flag.Default)
		}

		// 格式化输出
		fmt.Printf("  %-20s %-15s %s%s\n", flagStr, typeInfo, flag.Description, defaultInfo)
	}

	fmt.Println()
	fmt.Println("  --help            Show help")
	fmt.Println()
}

// showVersion 显示版本信息
func (c *CLI) showVersion() {
	fmt.Printf("DPanel Installer %s\n", c.version)
	fmt.Printf("Commit: %s\n", c.commit)
	fmt.Printf("Date: %s\n", c.date)
}

// parseBool 解析 bool 值
func parseBool(s string) bool {
	return s == "true" || s == "1" || s == "yes"
}

// parseInt 解析 int 值
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// 确保类型实现了接口
var _ handler.Handler = (*CLI)(nil)
