package cli

import (
	"flag"
	"fmt"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/handler"
)

// HelpError 表示用户请求帮助
type HelpError struct{}

func (e *HelpError) Error() string {
	return "help requested"
}

// VersionError 表示用户请求版本信息
type VersionError struct{}

func (e *VersionError) Error() string {
	return "version requested"
}

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
	// 使用存储的参数
	return c.run(cfg)
}

// run 运行 CLI 模式
func (c *CLI) run(cfg *config.Config) error {
	// 使用实例化时传入的参数
	args := c.args

	// 1. 解析参数
	opts, err := parseFlags(args)
	if err != nil {
		// 处理特殊的帮助和版本请求
		if _, ok := err.(*HelpError); ok {
			c.showHelp()
			return nil
		}
		if _, ok := err.(*VersionError); ok {
			c.showVersion()
			return nil
		}
		return err
	}

	// 2. 应用到 Config
	if err := cfg.ApplyOptions(opts...); err != nil {
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// 配置完成，由 main 调用 engine 执行
	return nil
}

// parseFlags 解析命令行参数
func parseFlags(args []string) ([]config.Option, error) {
	fs := flag.NewFlagSet("dpanel-installer", flag.ContinueOnError)

	// 定义标志
	var showHelp, showVersion bool
	var action, language, installType, dpanelVersion, edition, osType, registry string
	var containerName, dataPath, dockerSock, dockerHost, proxy, dns string
	var port int
	var dockerType string
	var tlsEnabled bool
	var tlsPath, sshUser, sshPassword, sshKey string

	fs.BoolVar(&showHelp, "help", false, "Show help")
	fs.BoolVar(&showHelp, "h", false, "Show help (shorthand)")
	fs.BoolVar(&showVersion, "version", false, "Show version")
	fs.BoolVar(&showVersion, "v", false, "Show version (shorthand)")
	fs.StringVar(&action, "action", "", "Action: install, upgrade, uninstall")
	fs.StringVar(&language, "language", "", "Language: zh, en")
	fs.StringVar(&installType, "install-type", "", "Install type: container, binary")
	fs.StringVar(&dpanelVersion, "dpanel-version", "", "DPanel version: community, pro, dev")
	fs.StringVar(&edition, "edition", "", "Edition: standard, lite")
	fs.StringVar(&osType, "os", "", "OS: alpine, debian")
	fs.StringVar(&registry, "registry", "", "Registry: hub, aliyun")
	fs.StringVar(&containerName, "container-name", "", "Container name")
	fs.IntVar(&port, "port", 0, "Port (0 for random)")
	fs.StringVar(&dataPath, "data-path", "", "Data path")
	fs.StringVar(&dockerType, "docker-type", "local", "Docker type: local, tcp, ssh")
	fs.StringVar(&dockerSock, "docker-sock", "", "Docker sock path")
	fs.StringVar(&dockerHost, "docker-host", "", "Docker host")
	fs.BoolVar(&tlsEnabled, "tls-enabled", false, "TLS enabled")
	fs.StringVar(&tlsPath, "tls-path", "", "TLS path")
	fs.StringVar(&sshUser, "ssh-user", "", "SSH user")
	fs.StringVar(&sshPassword, "ssh-password", "", "SSH password")
	fs.StringVar(&sshKey, "ssh-key", "", "SSH key path")
	fs.StringVar(&proxy, "proxy", "", "Proxy address")
	fs.StringVar(&dns, "dns", "", "DNS address")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// 处理 help 和 version
	if showHelp {
		return nil, &HelpError{}
	}
	if showVersion {
		return nil, &VersionError{}
	}

	// 构建 Option 列表
	opts := []config.Option{}

	if action != "" {
		opts = append(opts, config.WithAction(action))
	}
	if language != "" {
		opts = append(opts, config.WithLanguage(language))
	}
	if installType != "" {
		opts = append(opts, config.WithInstallType(installType))
	}
	if dpanelVersion != "" {
		opts = append(opts, config.WithVersion(dpanelVersion))
	}
	if edition != "" {
		opts = append(opts, config.WithEdition(edition))
	}
	if osType != "" {
		opts = append(opts, config.WithOS(osType))
	}
	if registry != "" {
		opts = append(opts, config.WithRegistry(registry))
	}
	if containerName != "" {
		opts = append(opts, config.WithContainerName(containerName))
	}
	if port > 0 {
		opts = append(opts, config.WithPort(port))
	}
	if dataPath != "" {
		opts = append(opts, config.WithDataPath(dataPath))
	}

	// Docker 连接
	switch dockerType {
	case config.DockerConnLocal:
		if dockerSock != "" {
			opts = append(opts, config.WithDockerLocal(dockerSock))
		} else {
			opts = append(opts, config.WithDockerLocal(""))
		}
	case config.DockerConnTCP:
		opts = append(opts, config.WithDockerTCP(dockerHost, 2376))
		if tlsEnabled {
			opts = append(opts, config.WithDockerTLS(true, tlsPath, "", ""))
		}
	case config.DockerConnSSH:
		opts = append(opts, config.WithDockerSSH(dockerHost, 22, sshUser))
		if sshPassword != "" {
			opts = append(opts, config.WithSSHAuth(sshPassword, ""))
		}
		if sshKey != "" {
			opts = append(opts, config.WithSSHAuth("", sshKey))
		}
	}

	// 网络配置
	if proxy != "" {
		opts = append(opts, config.WithHTTPProxy(proxy))
	}
	if dns != "" {
		opts = append(opts, config.WithDNS(dns))
	}

	return opts, nil
}

// showHelp 显示帮助信息
func (c *CLI) showHelp() {
	fmt.Println("\n")
	fmt.Println(`CLI Mode: Specify all options via command-line flags

Examples:
  dpanel-installer --action install --dpanel-version community
  dpanel-installer --action install --port 8080

Flags:`)
	fmt.Println("  -h, --help                    Show help")
	fmt.Println("  -v, --version                 Show version")
	fmt.Println("      --action <string>         Action: install, upgrade, uninstall")
	fmt.Println("      --language <string>       Language: zh, en")
	fmt.Println("      --install-type <string>   Install type: container, binary")
	fmt.Println("      --dpanel-version <string> DPanel version: community, pro, dev")
	fmt.Println("      --edition <string>        Edition: standard, lite")
	fmt.Println("      --os <string>             OS: alpine, debian")
	fmt.Println("      --registry <string>       Registry: hub, aliyun")
	fmt.Println("      --container-name <string> Container name")
	fmt.Println("      --port <int>              Port (0 for random)")
	fmt.Println("      --data-path <string>      Data path")
	fmt.Println("      --docker-type <string>    Docker type: local, tcp, ssh")
	fmt.Println("      --docker-sock <string>    Docker sock path")
	fmt.Println("      --docker-host <string>    Docker host")
	fmt.Println("      --tls-enabled             TLS enabled")
	fmt.Println("      --tls-path <string>       TLS path")
	fmt.Println("      --ssh-user <string>       SSH user")
	fmt.Println("      --ssh-password <string>   SSH password")
	fmt.Println("      --ssh-key <string>        SSH key path")
	fmt.Println("      --proxy <string>          Proxy address")
	fmt.Println("      --dns <string>            DNS address")
}

// showVersion 显示版本信息
func (c *CLI) showVersion() {
	fmt.Printf("DPanel Installer %s\n", c.version)
	fmt.Printf("Commit: %s\n", c.commit)
	fmt.Printf("Date: %s\n", c.date)
}

// 确保类型实现了接口
var _ handler.Handler = (*CLI)(nil)
