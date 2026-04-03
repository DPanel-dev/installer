package types

// === DPanel 版本 ===
const (
	VersionCE = "ce" // 社区版
	VersionPE = "pe" // 专业版
	VersionBE = "be" // 开发版

	VersionCommunity = VersionCE
	VersionPro       = VersionPE
	VersionDev       = VersionBE
)

// === 版本类型 ===
const (
	EditionStandard = "standard"
	EditionLite     = "lite"
)

// === 容器基础镜像系统 ===
const (
	BaseImageAlpine  = "alpine"
	BaseImageDebian  = "debian"
	BaseImageDarwin  = "darwin"
	BaseImageWindows = "windows"
)
