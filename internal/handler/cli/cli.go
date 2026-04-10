package cli

import (
	"fmt"
	"log/slog"

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
	flagType       string
	flagVersion    string
	flagEdition    string
	flagBaseImage  string
	flagName       string
	flagServerHost string
	flagServerPort int
	flagDataPath   string
	flagDockerSock string
	flagEnvProxy      string
	flagEnvDNS        string
	flagEnableBackup  bool
	flagEnableDeleteData bool
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

// buildInstallCmd 构建 install 子命令
func (c *CLI) buildInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install DPanel",
		Long: `Install a new DPanel instance.

The instance is identified by --name (default: "dpanel").
Use --data-path to specify the data directory (required for container install,
binary install uses it as root: <data-path>/dpanel-<name> for binary, <data-path>/data/ for data).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.applyAndRun(cmd, types.ActionInstall)
		},
	}

	cmd.Flags().StringVar(&c.flagName, "name", "", "Instance name (default: dpanel)")
	cmd.Flags().StringVar(&c.flagDataPath, "data-path", "", "Data directory (required for container; binary root dir)")
	cmd.Flags().StringVar(&c.flagType, "type", "", "Install type: container or binary (auto-detected if not specified)")
	cmd.Flags().StringVar(&c.flagVersion, "version", "", "DPanel version: ce (community), pe (pro), be (dev)")
	cmd.Flags().StringVar(&c.flagEdition, "edition", "", "Edition: standard or lite (standard only for container install)")
	cmd.Flags().StringVar(&c.flagBaseImage, "base-image", "", "Base image system: alpine, debian, darwin, windows (only for binary install, auto-detected by default)")
	cmd.Flags().StringVar(&c.flagServerHost, "server-host", "", "Server bind host (default: 0.0.0.0)")
	cmd.Flags().IntVar(&c.flagServerPort, "server-port", 0, "Server port (0 for random)")
	cmd.Flags().StringVar(&c.flagDockerSock, "docker-sock", "", "Docker socket path (for local connection)")
	cmd.Flags().StringVar(&c.flagEnvProxy, "env-proxy", "", "Proxy environment variable (used for both HTTP and HTTPS)")
	cmd.Flags().StringVar(&c.flagEnvDNS, "env-dns", "", "DNS environment variable")

	return cmd
}

// buildUpgradeCmd 构建 upgrade 子命令
func (c *CLI) buildUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade existing DPanel installation",
		Long: `Upgrade an existing DPanel installation.

Requires --name to identify the instance.
The type (container/binary) is auto-detected by searching for the name.
Existing configuration is preserved; only --version/--edition override the image,
and --proxy/--dns override environment variables.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.applyAndRun(cmd, types.ActionUpgrade)
		},
	}

	cmd.Flags().StringVar(&c.flagName, "name", "", "Instance name (required)")
	cmd.Flags().StringVar(&c.flagVersion, "version", "", "DPanel version: ce (community), pe (pro), be (dev) (keep existing if not specified)")
	cmd.Flags().StringVar(&c.flagEdition, "edition", "", "Edition: standard or lite (keep existing if not specified)")
	cmd.Flags().StringVar(&c.flagEnvProxy, "env-proxy", "", "Proxy environment variable (override existing)")
	cmd.Flags().StringVar(&c.flagEnvDNS, "env-dns", "", "DNS environment variable (override existing)")
	cmd.Flags().BoolVar(&c.flagEnableBackup, "enable-backup", true, "Create backup before upgrade")
	cmd.Flags().StringVar(&c.flagDockerSock, "docker-sock", "", "Docker socket path (for local connection)")

	return cmd
}

// buildUninstallCmd 构建 uninstall 子命令
func (c *CLI) buildUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall DPanel",
		Long: `Uninstall a DPanel installation.

Requires --name to identify the instance.
The type (container/binary) is auto-detected by searching for the name.
Use --remove-data to also delete the data directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.applyAndRun(cmd, types.ActionUninstall)
		},
	}

	cmd.Flags().StringVar(&c.flagName, "name", "", "Instance name (required)")
	cmd.Flags().BoolVar(&c.flagEnableDeleteData, "enable-delete-data", false, "Delete data directory")
	cmd.Flags().StringVar(&c.flagDockerSock, "docker-sock", "", "Docker socket path (for local connection)")

	return cmd
}

// applyAndRun 应用 flags 到 config，查找实例，确认提示，然后执行
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

	// 所有命令必须显式指定 --name
	if !cmd.Flags().Changed("name") {
		return fmt.Errorf(`required flag(s) "--name" not set`)
	}

	// Install：根据 Docker 可用性推断 type，检测同名实例提示转 upgrade
	// Upgrade/Uninstall：通过 DiscoverInstances 查找现有实例
	switch action {
	case types.ActionInstall:
		if cfg.InstallType == "" {
			if cfg.Client != nil {
				cfg.InstallType = types.InstallTypeContainer
			} else {
				cfg.InstallType = types.InstallTypeBinary
			}
		}
		// 检测同名实例，提示切换到 upgrade
		if inst := cfg.FindInstance(cfg.Name); inst != nil {
			slog.Warn("Already installed, switching to upgrade mode", "type", inst.Type)
			cfg.Action = types.ActionUpgrade
			cfg.InstallType = inst.Type
		}

	case types.ActionUpgrade, types.ActionUninstall:
		inst := cfg.FindInstance(cfg.Name)
		if inst == nil {
			return fmt.Errorf(`%q not found`, cfg.Name)
		}
		cfg.InstallType = inst.Type
	}

	// 创建最终 Driver
	var driver types.Driver
	switch cfg.InstallType {
	case types.InstallTypeContainer:
		driver = core.NewContainerDriver(cfg)
	case types.InstallTypeBinary:
		driver = core.NewBinaryDriver(cfg)
	}

	// 设置进度回调
	if c.flagProgress != ProgressQuiet {
		if bd, ok := driver.(*core.BinaryDriver); ok {
			bd.ProgressFunc = ShowProgress
			bd.ProgressDone = FinishProgress
		}
		if cd, ok := driver.(*core.ContainerDriver); ok {
			cd.ProgressFunc = ShowProgress
			cd.ProgressDone = FinishProgress
		}
	}

	// 显示配置信息
	core.LogConfig(cfg)

	// 检测状态并提示
	status := driver.Status()
	if status.Running {
		slog.Warn("Already running", "id", status.ID)
	}

	// 真正的新安装才需要 data-path
	if cfg.Action == types.ActionInstall && cfg.DataPath == "" {
		return fmt.Errorf(`required flag(s) "--data-path" not set`)
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

	// 调度执行
	switch cfg.Action {
	case types.ActionInstall:
		err = driver.Install()
	case types.ActionUpgrade:
		err = driver.Upgrade()
	case types.ActionUninstall:
		err = driver.Uninstall()
	}
	if err != nil {
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

	if cmd.Flags().Changed("name") {
		opts = append(opts, config.WithName(c.flagName))
	}
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
	if cmd.Flags().Changed("server-host") {
		opts = append(opts, config.WithServerHost(c.flagServerHost))
	}
	if cmd.Flags().Changed("server-port") {
		opts = append(opts, config.WithServerPort(c.flagServerPort))
	}
	if cmd.Flags().Changed("data-path") {
		opts = append(opts, config.WithDataPath(c.flagDataPath))
	}
	if cmd.Flags().Changed("docker-sock") {
		opts = append(opts, config.WithContainerSock(c.flagDockerSock))
	}
	if cmd.Flags().Changed("env-proxy") {
		opts = append(opts, config.WithEnvProxy(c.flagEnvProxy))
	}
	if cmd.Flags().Changed("env-dns") {
		opts = append(opts, config.WithEnvDNS(c.flagEnvDNS))
	}
	if cmd.Flags().Changed("enable-backup") {
		opts = append(opts, config.WithEnableBackup(c.flagEnableBackup))
	}
	if cmd.Flags().Changed("enable-delete-data") {
		opts = append(opts, config.WithEnableDeleteData(c.flagEnableDeleteData))
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
