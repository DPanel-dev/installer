package types

// === 镜像名称 ===
const (
	ImageNameCE = "dpanel/dpanel"    // 社区版
	ImageNamePE = "dpanel/dpanel-pe" // 专业版
)

// === 镜像 Tag（Alpine 基础） ===
const (
	TagLatest   = "latest"    // standard + alpine
	TagLite     = "lite"      // lite + alpine
	TagBeta     = "beta"      // be + standard + alpine
	TagBetaLite = "beta-lite" // be + lite + alpine
)

// === 镜像 Tag（Debian 基础） ===
const (
	TagLatestDebian   = "latest-debian"    // standard + debian
	TagLiteDebian     = "lite-debian"      // lite + debian
	TagBetaDebian     = "beta-debian"      // be + standard + debian
	TagBetaLiteDebian = "beta-lite-debian" // be + lite + debian
)

// === 旧常量（兼容） ===
const (
	ImageNameCommunity = ImageNameCE
	ImageNamePro       = ImageNamePE
)
