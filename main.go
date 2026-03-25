package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/handler/cli"
	"github.com/dpanel-dev/installer/internal/handler/tui"
)

var (
	// Version information (set via ldflags)
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Setup logging
	setupLogger()

	// Run the installer
	if err := run(); err != nil {
		slog.Error("Installation failed", "error", err)
		os.Exit(1)
	}
}

// run 运行安装器
func run() error {
	args := os.Args[1:]

	// Create default Config
	cfg, err := config.NewConfig()
	if err != nil {
		slog.Error("Failed to create config", "error", err)
		return err
	}
	slog.Debug("Starting installer", "config", cfg, "args", args)

	// 无参数 → TUI，有参数 → CLI
	if len(args) == 0 {
		slog.Debug("Starting installer", "mode", "tui")
		return tui.NewTUI().Run(cfg)
	}

	slog.Debug("Starting installer", "mode", "cli")

	return cli.NewCLI(
		cli.WithArgs(args),
		cli.WithVersionInfo(version, commit, date),
	).Run(cfg)
}

// setupLogger 设置日志记录器
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
		slog.Error("Failed to open log file", "path", logPath, "error", err)
		return
	}

	// Setup slog with JSON file output
	fileHandler := slog.NewJSONHandler(io.MultiWriter(os.Stdout, logFile), &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	slog.SetDefault(slog.New(fileHandler))
}
