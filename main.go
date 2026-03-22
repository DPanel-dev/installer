package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dpanel-dev/installer/internal/install"
	"github.com/dpanel-dev/installer/internal/ui/tui"
	"github.com/spf13/cobra"
)

var (
	// Version information (set via ldflags)
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Mode flag
	mode string

	// Action flags
	action string

	// Installation flags
	language      string
	installType   string
	versionFlag   string
	edition       string
	osType        string
	imageRegistry string
	containerName string
	port          int
	dataPath      string
	dockerSock    string
	dockerHost    string
	dockerType    string
	proxy         string
	dns           string
	tlsEnabled    bool
	tlsPath       string
	sshUser       string
	sshPassword   string
	sshKey        string
)

var rootCmd = &cobra.Command{
	Use:   "dpanel-installer",
	Short: "DPanel Installer",
	Long: `DPanel Installer - A tool for installing and managing DPanel.

Available modes:
  - TUI mode: Interactive terminal UI started with --mode tui
  - CLI mode: Programmatic installation via command-line flags (default)

Examples:
  # Start TUI mode
  dpanel-installer --mode tui

  # CLI mode installation
  dpanel-installer --action install --version community --edition lite

  # Show version
  dpanel-installer --version`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no action or mode specified, show help
		if action == "" && mode == "" {
			_ = cmd.Help()
			return
		}

		setupLogger()

		// Execute based on mode
		if mode == "tui" {
			runTUI()
		} else {
			// CLI mode
			if err := runCLI(); err != nil {
				slog.Error("Installation failed", "error", err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
	},
}

func init() {
	// Mode flag
	rootCmd.Flags().StringVar(&mode, "mode", "", "Run mode: tui, cli (default: cli)")

	// Action flags
	rootCmd.Flags().StringVarP(&action, "action", "a", "", "Action to perform: install, upgrade, uninstall")

	// Common flags
	rootCmd.Flags().StringVarP(&language, "language", "l", "en", "Output language: en, zh")
	rootCmd.Flags().StringVarP(&installType, "type", "t", "container", "Installation type: container, binary")
	rootCmd.Flags().StringVar(&versionFlag, "version", "community", "Version: community, pro, dev")
	rootCmd.Flags().StringVarP(&edition, "edition", "e", "lite", "Edition: standard, lite")
	rootCmd.Flags().StringVar(&osType, "os", "debian", "Base OS: alpine, debian")
	rootCmd.Flags().StringVar(&imageRegistry, "registry", "hub", "Image registry: hub, aliyun")

	// Container specific flags
	rootCmd.Flags().StringVarP(&containerName, "name", "n", "dpanel", "Container name")
	rootCmd.Flags().IntVarP(&port, "port", "p", 0, "Access port (0 for random)")
	rootCmd.Flags().StringVarP(&dataPath, "data-path", "d", "/home/dpanel", "Data storage directory")
	rootCmd.Flags().StringVar(&proxy, "proxy", "", "Proxy address for panel container")
	rootCmd.Flags().StringVar(&dns, "dns", "", "DNS address for container")

	// Docker connection flags
	rootCmd.Flags().StringVar(&dockerType, "docker-type", "local", "Docker connection type: local, tcp, ssh")
	rootCmd.Flags().StringVar(&dockerSock, "docker-sock", "/var/run/docker.sock", "Docker sock file path")
	rootCmd.Flags().StringVar(&dockerHost, "docker-host", "", "Docker remote host (ip:port for tcp/ssh)")
	rootCmd.Flags().BoolVar(&tlsEnabled, "tls", false, "Enable TLS for TCP connection")
	rootCmd.Flags().StringVar(&tlsPath, "tls-path", "", "TLS certificate directory")

	// SSH connection flags
	rootCmd.Flags().StringVar(&sshUser, "ssh-user", "", "SSH username")
	rootCmd.Flags().StringVar(&sshPassword, "ssh-password", "", "SSH password")
	rootCmd.Flags().StringVar(&sshKey, "ssh-key", "", "SSH private key path")

	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runTUI() {
	slog.Info("Starting DPanel Installer in TUI mode")
	if err := tui.StartTUI(); err != nil {
		slog.Error("TUI Error", "error", err)
		fmt.Fprintf(os.Stderr, "TUI Error: %v\n", err)
		os.Exit(1)
	}
}

func runCLI() error {
	slog.Info("Starting DPanel installer in CLI mode",
		"action", action,
		"version", versionFlag,
		"edition", edition,
	)

	// Validate action is specified
	if action == "" {
		return fmt.Errorf("--action flag is required (install, upgrade, uninstall)")
	}

	// Create config with default values
	cfg := install.NewConfig()

	// Override with CLI flags
	cfg.Action = action
	cfg.Language = language
	cfg.InstallType = installType
	cfg.Version = versionFlag
	cfg.Edition = edition
	cfg.OS = osType
	cfg.ImageRegistry = imageRegistry
	cfg.ContainerName = containerName
	cfg.Port = port
	cfg.DataPath = dataPath
	cfg.Proxy = proxy
	cfg.DNS = dns

	// Set docker connection config
	cfg.DockerConnection = &install.DockerConnection{
		Type:       dockerType,
		SockPath:   dockerSock,
		Host:       dockerHost,
		TLSEnabled: tlsEnabled,
		TLSPath:    tlsPath,
		SSHUser:    sshUser,
		SSHPass:    sshPassword,
		SSHKey:     sshKey,
	}

	// Validate config
	if err := validateConfig(cfg); err != nil {
		return err
	}

	// Create and run engine
	engine := install.NewEngine(cfg)
	if err := engine.Run(); err != nil {
		return err
	}

	slog.Info("Installation completed successfully")
	return nil
}

func validateConfig(cfg *install.Config) error {
	// Validate action
	validActions := map[string]bool{"install": true, "upgrade": true, "uninstall": true}
	if !validActions[cfg.Action] {
		return fmt.Errorf("invalid action: %s (must be install, upgrade, or uninstall)", cfg.Action)
	}

	// Validate install type
	validTypes := map[string]bool{"container": true, "binary": true}
	if !validTypes[cfg.InstallType] {
		return fmt.Errorf("invalid install type: %s (must be container or binary)", cfg.InstallType)
	}

	// Validate version
	validVersions := map[string]bool{"community": true, "pro": true, "dev": true}
	if !validVersions[cfg.Version] {
		return fmt.Errorf("invalid version: %s (must be community, pro, or dev)", cfg.Version)
	}

	// Validate edition
	validEditions := map[string]bool{"standard": true, "lite": true}
	if !validEditions[cfg.Edition] {
		return fmt.Errorf("invalid edition: %s (must be standard or lite)", cfg.Edition)
	}

	// Validate OS
	validOS := map[string]bool{"alpine": true, "debian": true}
	if !validOS[cfg.OS] {
		return fmt.Errorf("invalid OS: %s (must be alpine or debian)", cfg.OS)
	}

	// Validate registry
	validRegistries := map[string]bool{"hub": true, "aliyun": true}
	if !validRegistries[cfg.ImageRegistry] {
		return fmt.Errorf("invalid registry: %s (must be hub or aliyun)", cfg.ImageRegistry)
	}

	// Validate docker connection type
	validDockerTypes := map[string]bool{"local": true, "tcp": true, "ssh": true}
	if !validDockerTypes[cfg.DockerConnection.Type] {
		return fmt.Errorf("invalid docker type: %s (must be local, tcp, or ssh)", cfg.DockerConnection.Type)
	}

	// Validate port range
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be between 0 and 65535, 0 for random)", cfg.Port)
	}

	return nil
}

func setupLogger() {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
	}
	execDir := filepath.Dir(execPath)
	logPath := filepath.Join(execDir, "run.log")

	// Create log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}

	// Setup slog with JSON file output
	fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	multiHandler := newMultiHandler(fileHandler)
	slog.SetDefault(slog.New(multiHandler))
}

// multiHandler writes logs to multiple handlers
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}
