package types

// === 容器基础镜像系统 ===
const (
	OSAlpine = "alpine"
	OSDebian = "debian"
)

// === 镜像仓库 ===
const (
	RegistryHub    = "hub"
	RegistryAliyun = "aliyun"
)

// === Docker 连接类型 ===
const (
	DockerConnLocal = "local"
	DockerConnTCP   = "tcp"
	DockerConnSSH   = "ssh"
)
