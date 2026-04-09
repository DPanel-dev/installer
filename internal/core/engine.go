package core

import (
	"fmt"
	"log/slog"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
)

// Engine handles installer execution.
type Engine struct {
	Config        *config.Config
	ProgressFunc  func(complete, total int64) // 拉取进度回调
	ProgressDone  func()                     // 拉取完成回调
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

	// 自动检测 Registry（安装/升级时）
	if e.Config.Registry == "" && e.Config.Action != types.ActionUninstall {
		slog.Info("Registry", "status", "detecting")
		hubLatency := config.TestRegistryLatency(types.RegistryDockerHub)
		aliLatency := config.TestRegistryLatency(types.RegistryAliYun)

		e.Config.Registry = types.RegistryUnavailable
		if aliLatency > 0 {
			e.Config.Registry = types.RegistryAliYun
		}
		if hubLatency > 0 && (aliLatency <= 0 || hubLatency <= aliLatency) {
			e.Config.Registry = types.RegistryDockerHub
		}

		slog.Info("Registry", "selected", e.Config.Registry, "hub_ms", hubLatency, "aliyun_ms", aliLatency)
	}

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

// LogConfig 输出运行时配置信息
func (e *Engine) LogConfig() {
	cfg := e.Config
	dockerStatus := "not available"
	if cfg.Client != nil {
		dockerStatus = "available"
	}
	slog.Info("Config", "os", cfg.OS, "arch", cfg.Arch, "docker", dockerStatus)
	slog.Info("Config", "type", cfg.InstallType, "version", cfg.Version, "edition", cfg.Edition, "base_image", cfg.BaseImage)
	slog.Info("Config", "name", cfg.ContainerName, "port", cfg.ServerPort, "binary", cfg.BinaryPath, "data", cfg.DataPath)
	if cfg.DNS != "" {
		slog.Info("Config", "dns", cfg.DNS)
	}
	if cfg.HTTPProxy != "" {
		slog.Info("Config", "proxy", cfg.HTTPProxy)
	}
}
