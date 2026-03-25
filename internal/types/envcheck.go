package types

// DockerEngineType Docker 引擎检测类型
type DockerEngineType int

const (
	DockerEngineNone DockerEngineType = iota // 无 Docker
	DockerEngineLocal                        // 本地 Docker Engine
	DockerEngineRemote                       // 远程 Docker 连接
)

// EnvCheck 环境检测结果
type EnvCheck struct {
	OS   string // windows, darwin, linux
	Arch string // amd64, arm64

	// 容器运行时
	DockerEngine DockerEngineType // Docker 引擎类型：无/本地/远程
	PodmanExists bool             // Podman 是否存在（有命令即可）

	// 网络环境
	HubAccessible    bool // Docker Hub 是否可访问
	AliyunAccessible bool // 阿里云镜像是否可访问
}

// ContainerAvailable 容器安装是否可用（本地 Docker 或 Podman）
func (e *EnvCheck) ContainerAvailable() bool {
	return e.DockerEngine == DockerEngineLocal || e.PodmanExists
}

// AnyRegistryAvailable 检查是否有任何镜像源可用
func (e *EnvCheck) AnyRegistryAvailable() bool {
	return e.HubAccessible || e.AliyunAccessible
}
