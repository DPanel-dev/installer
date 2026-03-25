package core

// Installation action constants
const (
	ActionInstall   = "config"
	ActionUpgrade   = "upgrade"
	ActionUninstall = "uninstall"
)

// Installation type constants
const (
	InstallTypeContainer     = "container"
	InstallTypeBinary        = "binary"
	InstallTypeInstallDocker = "install_docker"
)

// Version constants
const (
	VersionCommunity = "community"
	VersionPro       = "pro"
	VersionDev       = "dev"
)

// Edition constants
const (
	EditionStandard = "standard"
	EditionLite     = "lite"
)

// OS constants
const (
	OSAlpine = "alpine"
	OSDebian = "debian"
)

// Image registry constants
const (
	RegistryHub    = "hub"
	RegistryAliyun = "aliyun"
)

// Docker connection type constants
const (
	DockerConnLocal = "local"
	DockerConnTCP   = "tcp"
	DockerConnSSH   = "ssh"
)

// Mode constants
const (
	ModeCLI = "cli"
	ModeTUI = "tui"
)

// Valid action values
var ValidActions = []string{
	ActionInstall,
	ActionUpgrade,
	ActionUninstall,
}

// Valid config type values
var ValidInstallTypes = []string{
	InstallTypeContainer,
	InstallTypeBinary,
	InstallTypeInstallDocker,
}

// Valid version values
var ValidVersions = []string{
	VersionCommunity,
	VersionPro,
	VersionDev,
}

// Valid edition values
var ValidEditions = []string{
	EditionStandard,
	EditionLite,
}

// Valid OS values
var ValidOS = []string{
	OSAlpine,
	OSDebian,
}

// Valid registry values
var ValidRegistries = []string{
	RegistryHub,
	RegistryAliyun,
}

// Valid Docker connection type values
var ValidDockerConnTypes = []string{
	DockerConnLocal,
	DockerConnTCP,
	DockerConnSSH,
}

// IsValidAction 检查是否是有效的 action
func IsValidAction(action string) bool {
	for _, a := range ValidActions {
		if a == action {
			return true
		}
	}
	return false
}

// IsValidInstallType 检查是否是有效的安装类型
func IsValidInstallType(installType string) bool {
	for _, t := range ValidInstallTypes {
		if t == installType {
			return true
		}
	}
	return false
}

// IsValidVersion 检查是否是有效的版本
func IsValidVersion(version string) bool {
	for _, v := range ValidVersions {
		if v == version {
			return true
		}
	}
	return false
}

// IsValidEdition 检查是否是有效的版本类型
func IsValidEdition(edition string) bool {
	for _, e := range ValidEditions {
		if e == edition {
			return true
		}
	}
	return false
}

// IsValidOS 检查是否是有效的 OS
func IsValidOS(os string) bool {
	for _, o := range ValidOS {
		if o == os {
			return true
		}
	}
	return false
}

// IsValidRegistry 检查是否是有效的镜像仓库
func IsValidRegistry(registry string) bool {
	for _, r := range ValidRegistries {
		if r == registry {
			return true
		}
	}
	return false
}

// IsValidDockerConnType 检查是否是有效的 Docker 连接类型
func IsValidDockerConnType(connType string) bool {
	for _, t := range ValidDockerConnTypes {
		if t == connType {
			return true
		}
	}
	return false
}
