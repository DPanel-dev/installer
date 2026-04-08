package core

import (
	"fmt"
	"log/slog"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
)

// Engine handles installer execution.
type Engine struct {
	Config *config.Config
}

// NewEngine creates a new installation engine.
func NewEngine(cfg *config.Config) *Engine {
	return &Engine{Config: cfg}
}

// Run executes the configured action.
func (e *Engine) Run() error {
	// 二进制安装：BaseImage 必须与当前 OS 匹配
	// 处理用户有 Docker（默认 container+alpine）但通过 CLI/TUI 切换为 binary 的情况
	if e.Config.InstallType == types.InstallTypeBinary {
		switch e.Config.OS {
		case "darwin":
			e.Config.BaseImage = types.BaseImageDarwin
		case "windows":
			e.Config.BaseImage = types.BaseImageWindows
		default:
			// Linux: 保持用户选择的 alpine 或 debian
		}
	}

	e.logRuntimeConfig()

	switch e.Config.Action {
	case types.ActionInstall:
		switch e.Config.InstallType {
		case types.InstallTypeContainer:
			return e.installContainer()
		case types.InstallTypeBinary:
			return e.installBinary()
		}
	case types.ActionUpgrade:
		switch e.Config.InstallType {
		case types.InstallTypeContainer:
			return func() error {
				if err := e.backupContainer(); err != nil {
					return err
				}
				return e.installContainer()
			}()
		case types.InstallTypeBinary:
			return e.upgradeBinary()
		}
	case types.ActionUninstall:
		switch e.Config.InstallType {
		case types.InstallTypeContainer:
			return e.uninstallContainer()
		case types.InstallTypeBinary:
			return e.uninstallBinary()
		}
	}

	return fmt.Errorf("unsupported action/install type: %s/%s", e.Config.Action, e.Config.InstallType)
}

func (e *Engine) logRuntimeConfig() {
	cfg := e.Config
	slog.Info("Config System", "os", cfg.OS, "arch", cfg.Arch)
	slog.Info("Config Version", "version", cfg.Version, "edition", cfg.Edition, "base_image", cfg.BaseImage)
	slog.Info("Config Paths", "binary", cfg.BinaryPath, "data", cfg.DataPath)
	if cfg.InstallType == types.InstallTypeContainer {
		slog.Info("Config Container", "name", cfg.ContainerName, "port", cfg.ServerPort, "registry", cfg.Registry)
	}
	if cfg.DNS != "" {
		slog.Info("Config Network", "dns", cfg.DNS)
	}
	if cfg.HTTPProxy != "" {
		slog.Info("Config Network", "proxy", cfg.HTTPProxy)
	}
}
