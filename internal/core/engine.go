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
	e.logRuntimeConfig()

	switch e.Config.Action {
	case types.ActionInstall:
		switch e.Config.InstallType {
		case types.InstallTypeContainer:
			slog.Info("Running container install")
			return e.installContainer()
		case types.InstallTypeBinary:
			slog.Info("Running binary install")
			return e.installBinary()
		}
	case types.ActionUpgrade:
		switch e.Config.InstallType {
		case types.InstallTypeContainer:
			slog.Info("Running container upgrade")
			return func() error {
				if err := e.backupContainer(); err != nil {
					return err
				}
				return e.installContainer()
			}()
		case types.InstallTypeBinary:
			slog.Info("Running binary upgrade")
			return e.upgradeBinary()
		}
	case types.ActionUninstall:
		switch e.Config.InstallType {
		case types.InstallTypeContainer:
			slog.Info("Running container uninstall")
			return e.uninstallContainer()
		case types.InstallTypeBinary:
			slog.Info("Running binary uninstall")
			return e.uninstallBinary()
		}
	}

	return fmt.Errorf("unsupported action/install type: %s/%s", e.Config.Action, e.Config.InstallType)
}

func (e *Engine) logRuntimeConfig() {
	cfg := e.Config
	slog.Info("Installation config",
		"os", cfg.OS,
		"arch", cfg.Arch,
		"action", cfg.Action,
		"language", cfg.Language,
		"install_type", cfg.InstallType,
		"version", cfg.Version,
		"edition", cfg.Edition,
		"base_image", cfg.BaseImage,
		"registry", cfg.Registry,
		"container_name", cfg.ContainerName,
		"port", cfg.Port,
		"data_path", cfg.DataPath,
		"dns", cfg.DNS,
		"http_proxy", cfg.HTTPProxy,
		"https_proxy", cfg.HTTPProxy,
		"upgrade_backup", cfg.UpgradeBackup,
		"uninstall_remove_data", cfg.UninstallRemoveData,
	)
}
