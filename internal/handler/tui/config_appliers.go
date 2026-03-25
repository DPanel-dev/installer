package tui

import (
	"fmt"
	"strconv"

	"github.com/dpanel-dev/installer/internal/config"
)

// ConfigAppliers 配置应用器映射
var ConfigAppliers = map[Step]ConfigApplier{
	StepAction:           applyAction,
	StepInstallType:      applyInstallType,
	StepVersion:          applyVersion,
	StepEdition:          applyEdition,
	StepOS:               applyOS,
	StepRegistry:         applyRegistry,
	StepDockerConnection: applyDockerConnection,
	StepContainerName:    applyContainerName,
	StepPort:             applyPort,
	StepDataPath:         applyDataPath,
	StepProxy:            applyProxy,
	StepDNS:              applyDNS,
}

func applyAction(cfg *config.Config, value string) error {
	return cfg.ApplyOptions(config.WithAction(value))
}

func applyInstallType(cfg *config.Config, value string) error {
	if value == config.InstallTypeInstallDocker {
		return fmt.Errorf("docker installation not implemented")
	}
	cfg.InstallType = value
	return nil
}

func applyVersion(cfg *config.Config, value string) error {
	cfg.Version = value
	return nil
}

func applyEdition(cfg *config.Config, value string) error {
	cfg.Edition = value
	return nil
}

func applyOS(cfg *config.Config, value string) error {
	cfg.OS = value
	return nil
}

func applyRegistry(cfg *config.Config, value string) error {
	return cfg.ApplyOptions(config.WithRegistry(value))
}

func applyDockerConnection(cfg *config.Config, value string) error {
	switch value {
	case config.DockerConnLocal:
		return cfg.ApplyOptions(config.WithDockerLocal(""))
	case config.DockerConnTCP:
		return cfg.ApplyOptions(config.WithDockerTCP("127.0.0.1", 2376))
	case config.DockerConnSSH:
		return cfg.ApplyOptions(config.WithDockerSSH("127.0.0.1", 22, ""))
	}
	return fmt.Errorf("invalid docker connection type: %s", value)
}

func applyContainerName(cfg *config.Config, value string) error {
	cfg.ContainerName = value
	return nil
}

func applyPort(cfg *config.Config, value string) error {
	if value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			cfg.Port = port
		}
	}
	return nil
}

func applyDataPath(cfg *config.Config, value string) error {
	cfg.DataPath = value
	return nil
}

func applyProxy(cfg *config.Config, value string) error {
	return cfg.ApplyOptions(config.WithHTTPProxy(value))
}

func applyDNS(cfg *config.Config, value string) error {
	cfg.DNS = value
	return nil
}
