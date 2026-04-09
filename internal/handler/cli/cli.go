package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/core"
	"github.com/dpanel-dev/installer/internal/handler"
	"github.com/dpanel-dev/installer/internal/types"
)

// CLI 实现 handler.Handler 接口
type CLI struct {
	args    []string
	version string
	commit  string
	date    string

	// 全局 flags
	flagProgress string
	flagYes      bool

	// 子命令 flags（通过 cobra 绑定）
	flagType        string
	flagVersion     string
	flagEdition     string
	flagBaseImage   string
	flagName        string
	flagServerHost  string
	flagServerPort  int
	flagInstallPath string
	flagDataPath    string
	flagDockerSock  string
	flagProxy       string
	flagDNS         string
	flagBackup      bool
	flagRemoveData  bool
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
// cfg 参数由 TUI 使用，CLI 模式忽略（cobra 内部自行创建 config）
func (c *CLI) Run(cfg *config.Config) error {
	return c.Parse()
}

// buildRootCmd 构建根命令
func (c *CLI) buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "dpanel-installer",
		Short:   "Install, upgrade, and manage DPanel",
		Version: fmt.Sprintf("%s (commit: %s, date: %s)", c.version, c.commit, c.date),
	}

	// 全局 flags
	rootCmd.PersistentFlags().StringVar(&c.flagProgress, "progress", ProgressPlain, "Progress output mode: plain, quiet")
	rootCmd.PersistentFlags().BoolVarP(&c.flagYes, "yes", "y", false, "Auto-confirm prompts")

	// 子命令
	rootCmd.AddCommand(c.buildInstallCmd())
	rootCmd.AddCommand(c.buildUpgradeCmd())
	rootCmd.AddCommand(c.buildUninstallCmd())

	// 无子命令时显示帮助
	rootCmd.SetArgs(c.args)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true}) // 隐藏默认 help 子命令

	return rootCmd
}

// addCommonFlags 添加通用 flags（install/upgrade 共享）
func (c *CLI) addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&c.flagType, "type", "", "Install type: container or binary")
	cmd.Flags().StringVar(&c.flagVersion, "version", "", "DPanel version: ce (community), pe (pro), be (dev)")
	cmd.Flags().StringVar(&c.flagEdition, "edition", "", "Edition: standard or lite (standard only for container install)")
	cmd.Flags().StringVar(&c.flagServerHost, "server-host", "", "Server bind host (default: 0.0.0.0)")
	cmd.Flags().IntVar(&c.flagServerPort, "server-port", 0, "Server port (0 for random)")
	cmd.Flags().StringVar(&c.flagInstallPath, "install-path", "", "Installation directory (auto-derives binary and data paths)")
	cmd.Flags().StringVar(&c.flagDataPath, "data-path", "", "Data storage path (overrides auto-derived path from --install-path)")
	cmd.Flags().StringVar(&c.flagDockerSock, "docker-sock", "", "Docker socket path (for local connection)")
}

// buildInstallCmd 构建 install 子命令
func (c *CLI) buildInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install DPanel",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.applyAndRun(cmd, types.ActionInstall)
		},
	}

	c.addCommonFlags(cmd)
	cmd.Flags().StringVar(&c.flagBaseImage, "base-image", "", "Base image system: alpine, debian, darwin, windows (only for binary install, auto-detected by default)")
	cmd.Flags().StringVar(&c.flagName, "name", "", "Container name")
	cmd.Flags().StringVar(&c.flagProxy, "proxy", "", "Proxy address (used for both HTTP and HTTPS)")
	cmd.Flags().StringVar(&c.flagDNS, "dns", "", "DNS server address")

	return cmd
}

// buildUpgradeCmd 构建 upgrade 子命令
func (c *CLI) buildUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade existing DPanel installation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.applyAndRun(cmd, types.ActionUpgrade)
		},
	}

	c.addCommonFlags(cmd)
	cmd.Flags().StringVar(&c.flagBaseImage, "base-image", "", "Base image system: alpine, debian, darwin, windows (only for binary install, auto-detected by default)")
	cmd.Flags().StringVar(&c.flagName, "name", "", "Container name")
	cmd.Flags().BoolVar(&c.flagBackup, "backup", true, "Create backup before upgrade")

	return cmd
}

// buildUninstallCmd 构建 uninstall 子命令
func (c *CLI) buildUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall DPanel",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.applyAndRun(cmd, types.ActionUninstall)
		},
	}

	cmd.Flags().StringVar(&c.flagType, "type", "", "Install type: container or binary (auto-detected if not specified)")
	cmd.Flags().StringVar(&c.flagInstallPath, "install-path", "", "Installation directory (auto-derives binary and data paths)")
	cmd.Flags().StringVar(&c.flagDataPath, "data-path", "", "Data storage path (overrides auto-derived path from --install-path)")
	cmd.Flags().StringVar(&c.flagDockerSock, "docker-sock", "", "Docker socket path (for local connection)")
	cmd.Flags().StringVar(&c.flagName, "name", "", "Container name")
	cmd.Flags().BoolVar(&c.flagRemoveData, "remove-data", false, "Remove data directory")

	return cmd
}

// applyAndRun 应用 flags 到 config，确认提示，然后执行
func (c *CLI) applyAndRun(cmd *cobra.Command, action string) error {
	// CLI 模式：增加控制台日志
	setupConsoleLogger(c.flagProgress)

	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}

	// 设置 Action
	cfg.Action = action

	// 应用 flags 到 config（只应用用户显式设置的 flag）
	opts := c.buildOptions(cmd)
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return err
		}
	}

	// 二进制安装且用户未指定 base-image：根据 OS 自动修正
	if cfg.InstallType == types.InstallTypeBinary && !cmd.Flags().Changed("base-image") {
		switch cfg.OS {
		case "darwin":
			cfg.BaseImage = types.BaseImageDarwin
		case "windows":
			cfg.BaseImage = types.BaseImageWindows
		default:
			if config.IsMusl() {
				cfg.BaseImage = types.BaseImageAlpine
			} else {
				cfg.BaseImage = types.BaseImageDebian
			}
		}
	}

	engine := core.NewEngine(cfg)

	// 设置进度回调
	if c.flagProgress != ProgressQuiet {
		engine.ProgressFunc = ShowProgress
		engine.ProgressDone = FinishProgress
	}

	// 先显示配置信息
	engine.LogConfig()

	// 安装时检测是否已存在 → 自动转为升级
	if cfg.Action == types.ActionInstall {
		if _, err := os.Stat(cfg.BinaryPath); err == nil {
			slog.Warn("Already installed")
			cfg.Action = types.ActionUpgrade
		}
	}

	// 需要确认的操作（uninstall / upgrade），除非 -y
	if !c.flagYes {
		switch cfg.Action {
		case types.ActionUninstall:
			msg := "Uninstall DPanel?"
			if cfg.UninstallRemoveData {
				msg += " All data will be permanently deleted."
			}
			if !parseYes(msg) {
				fmt.Println("Aborted.")
				return nil
			}
		case types.ActionUpgrade:
			if !parseYes("Upgrade with new version?") {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}

	if err := engine.Run(); err != nil {
		return err
	}

	// 安装/升级成功后输出访问地址
	c.printAccessURL(cfg)
	return nil
}

// printAccessURL 输出安装完成后的访问地址
func (c *CLI) printAccessURL(cfg *config.Config) {
	if cfg.ServerPort <= 0 {
		return
	}

	port := cfg.ServerPort
	slog.Info("Done", "local", fmt.Sprintf("http://127.0.0.1:%d", port))

	if cfg.ServerHost != types.ServerHostLocal {
		if localIP := config.GetLocalIP(); localIP != "127.0.0.1" {
			slog.Info("Done", "internal", fmt.Sprintf("http://%s:%d", localIP, port))
		}
		if publicIP := config.GetPublicIP(); publicIP != "" {
			slog.Info("Done", "external", fmt.Sprintf("http://%s:%d", publicIP, port))
		}
	}
}

// buildOptions 从 cobra flags 构建 config.Option 列表
func (c *CLI) buildOptions(cmd *cobra.Command) []config.Option {
	var opts []config.Option

	if cmd.Flags().Changed("type") {
		opts = append(opts, config.WithInstallType(c.flagType))
	}
	if cmd.Flags().Changed("version") {
		opts = append(opts, config.WithVersion(c.flagVersion))
	}
	if cmd.Flags().Changed("edition") {
		opts = append(opts, config.WithEdition(c.flagEdition))
	}
	if cmd.Flags().Changed("base-image") {
		opts = append(opts, config.WithBaseImage(c.flagBaseImage))
	}
	if cmd.Flags().Changed("name") {
		opts = append(opts, config.WithContainerName(c.flagName))
	}
	if cmd.Flags().Changed("server-host") {
		opts = append(opts, config.WithServerHost(c.flagServerHost))
	}
	if cmd.Flags().Changed("server-port") {
		opts = append(opts, config.WithServerPort(c.flagServerPort))
	}
	if cmd.Flags().Changed("install-path") {
		opts = append(opts, config.WithInstallPath(c.flagInstallPath))
	}
	if cmd.Flags().Changed("data-path") {
		opts = append(opts, config.WithDataPath(c.flagDataPath))
	}
	if cmd.Flags().Changed("docker-sock") {
		opts = append(opts, config.WithContainerSock(c.flagDockerSock))
	}
	if cmd.Flags().Changed("proxy") {
		opts = append(opts, config.WithHTTPProxy(c.flagProxy))
	}
	if cmd.Flags().Changed("dns") {
		opts = append(opts, config.WithDNS(c.flagDNS))
	}
	if cmd.Flags().Changed("backup") {
		opts = append(opts, config.WithUpgradeBackup(c.flagBackup))
	}
	if cmd.Flags().Changed("remove-data") {
		opts = append(opts, config.WithUninstallRemoveData(c.flagRemoveData))
	}

	return opts
}

// Parse 执行命令解析并返回 Run 结果
// 这是一个桥接方法，让 main.go 中的调用方式保持不变
func (c *CLI) Parse() error {
	rootCmd := c.buildRootCmd()
	rootCmd.SilenceUsage = true
	_, err := rootCmd.ExecuteC()
	return err
}

// 确保类型实现了接口
var _ handler.Handler = (*CLI)(nil)
