package types

// === 镜像名称 ===
const (
	ImageNameCE = "dpanel/dpanel"    // 社区版
	ImageNamePE = "dpanel/dpanel-pe" // 专业版
)

// === 镜像仓库 ===
const (
	RegistryDockerHub   = "docker.io" // Docker Hub Index
	RegistryAliYun      = "registry.cn-hangzhou.aliyuncs.com"
	RegistryUnavailable = "unavailable"
)

// === 镜像 Tag（Alpine 基础） ===
const (
	TagLatest   = "latest"    // standard + alpine
	TagLite     = "lite"      // lite + alpine
	TagBeta     = "beta"      // be + standard + alpine
	TagBetaLite = "beta-lite" // be + lite + alpine
)

const (
	TagDebian = "debian"
)
