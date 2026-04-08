package main

import (
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
	// Setup logging (file only)
	setupLogger()

	// Run the installer
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run 运行安装器
func run() error {
	args := os.Args[1:]

	// 无参数 → TUI，有参数 → CLI
	if len(args) == 0 {
		cfg, err := config.NewConfig()
		if err != nil {
			slog.Error("Failed to create config", "error", err)
			return err
		}
		slog.Debug("Starting installer", "mode", "tui", "config", cfg)
		return tui.NewTUI().Run(cfg)
	}

	// CLI 模式：cobra 负责解析参数和管理 config
	slog.Debug("Starting installer", "mode", "cli", "args", args)
	return cli.NewCLI(
		cli.WithArgs(args),
		cli.WithVersionInfo(version, commit, date),
	).Run(nil)
}

// setupLogger 设置日志记录器（仅文件输出）
// CLI handler 内部自行决定是否增加控制台输出
func setupLogger() {
	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
	}
	execDir := filepath.Dir(execPath)
	logPath := filepath.Join(execDir, "run.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		slog.Error("Failed to open log file", "path", logPath, "error", err)
		return
	}

	fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	slog.SetDefault(slog.New(fileHandler))
}
