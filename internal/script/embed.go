// Package script provides embedded shell scripts for installation.
package script

import _ "embed"

// Docker installation scripts for different Linux distributions
var (
	// DockerInstallLinux is the Docker installation script for standard Linux
	// Uses get.docker.com official script with Chinese mirror support
	//go:embed docker_install_linux.sh
	DockerInstallLinux string

	// DockerInstallAlpine is the Docker installation script for Alpine Linux
	// Uses apk package manager
	//go:embed docker_install_alpine.sh
	DockerInstallAlpine string
)
