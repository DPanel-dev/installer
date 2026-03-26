package types

// === 容器基础镜像系统 ===
const (
	BaseImageAlpine = "alpine"
	BaseImageDebian = "debian"
)

// === 镜像仓库 ===
const (
	RegistryDockerHub    = "docker.io" // Docker Hub Index
	RegistryAliYun = "registry.cn-hangzhou.aliyuncs.com"
	RegistryUnavailable  = "unavailable"
)

// === 容器引擎类型 ===
const (
	ContainerEngineDocker = "docker"
	ContainerEnginePodman = "podman"
)

// === 容器连接类型 ===
const (
	ContainerConnTypeSock = "sock" // Unix socket / Windows named pipe
	ContainerConnTypeTCP  = "tcp"  // TCP 连接
	ContainerConnTypeSSH  = "ssh"  // SSH 连接
)

// === Docker 连接类型（兼容旧代码） ===
const (
	DockerConnLocal = "local"
	DockerConnTCP   = "tcp"
	DockerConnSSH   = "ssh"
)
