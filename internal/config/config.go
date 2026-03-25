package config

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// === 常量定义 ===

// Installation action constants
const (
	ActionInstall   = "install"
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

// Config 安装配置
type Config struct {
	// === 操作类型 ===
	Action string // config, upgrade, uninstall

	// === 语言 ===
	Language string // zh, en

	// === 安装类型 ===
	// 根据环境自动选择最优值：container, binary, install_docker
	InstallType string

	// === 版本配置 ===
	Version  string // community, pro, dev
	Edition  string // standard, lite
	OS       string // alpine, debian
	Registry string // hub, aliyun

	// === 容器配置 ===
	ContainerName string
	Port          int // 0 = 随机端口
	DataPath      string

	// === Docker 连接 ===
	DockerConnType string // local, tcp, ssh
	DockerSockPath string
	DockerTCPHost  string
	DockerTCPPort  int
	DockerSSHHost  string
	DockerSSHPort  int
	DockerSSHUser  string
	DockerSSHPass  string
	DockerSSHKey   string
	DockerTLS      bool
	DockerTLSCert  string
	DockerTLSKey   string
	DockerTLSCA    string

	// === 网络配置 ===
	DNS        string
	HTTPProxy  string
	HTTPSProxy string

	// === 升级配置 ===
	UpgradeTargetVersion string
	UpgradeBackup        bool

	// === 卸载配置 ===
	UninstallRemoveData bool

	// === 环境检测结果 ===
	// 自动检测，用于判断选项可用性
	Env EnvCheck
}

// EnvCheck 环境检测结果
type EnvCheck struct {
	// 系统信息
	OS   string // windows, darwin, linux
	Arch string // amd64, arm64

	// Docker/Podman
	DockerExists    bool
	DockerVersion   string
	DockerAvailable bool
	PodmanExists    bool
	PodmanVersion   string
	PodmanAvailable bool

	// 网络
	HubAccessible    bool
	AliyunAccessible bool

	// 现有安装
	ExistingContainers []string
	ExistingBinaries   []string

	// 检测时间
	CheckTime time.Time
}

// Option 配置选项函数
type Option func(*Config) error

// NewConfig 创建配置（自动检测环境 + 智能默认值）
func NewConfig(opts ...Option) (*Config, error) {
	c := &Config{}

	// 1. 执行环境检测
	c.detectEnvironment()

	// 2. 根据环境设置最优默认值
	c.applySmartDefaults()

	// 3. 应用用户自定义选项
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// detectEnvironment 执行环境检测
func (c *Config) detectEnvironment() {
	c.Env = EnvCheck{
		OS:        detectOS(),
		Arch:      runtime.GOARCH,
		CheckTime: time.Now(),
	}

	// 检测 Docker
	if path, err := exec.LookPath("docker"); err == nil {
		c.Env.DockerExists = true
		c.Env.DockerVersion = getDockerVersion(path)
		c.Env.DockerAvailable = testDockerService("docker")
	}

	// 检测 Podman
	if path, err := exec.LookPath("podman"); err == nil {
		c.Env.PodmanExists = true
		c.Env.PodmanVersion = getPodmanVersion(path)
		c.Env.PodmanAvailable = testDockerService("podman")
	}

	// 检测网络
	c.Env.HubAccessible = testRegistryConnectivity("hub")
	c.Env.AliyunAccessible = testRegistryConnectivity("aliyun")

	// 检测现有安装
	c.Env.ExistingContainers = getExistingContainers()
	c.Env.ExistingBinaries = getExistingBinaries()
}

// applySmartDefaults 根据环境设置最优默认值
func (c *Config) applySmartDefaults() {
	// 操作类型
	c.Action = ActionInstall

	// 语言
	c.Language = "zh"

	// ===== 安装类型 =====
	// 优先级：Docker > Podman > Binary
	if c.Env.DockerAvailable {
		c.InstallType = InstallTypeContainer
		c.DockerConnType = DockerConnLocal
		c.DockerSockPath = "/var/run/docker.sock"
	} else if c.Env.PodmanAvailable {
		c.InstallType = InstallTypeContainer
		c.DockerConnType = DockerConnLocal
		c.DockerSockPath = getPodmanSockPath()
	} else {
		// Linux 且无 Docker/Podman 时，可以选择安装 Docker
		if c.Env.OS == "linux" {
			c.InstallType = InstallTypeInstallDocker
		} else {
			c.InstallType = InstallTypeBinary
		}
	}

	// ===== 版本配置 =====
	c.Version = VersionCommunity
	c.Edition = EditionLite // 默认精简版，适合新手
	c.OS = OSDebian         // 默认 debian，更稳定

	// ===== 镜像源 =====
	// Hub 可访问优先用 Hub，否则用 Aliyun
	if c.Env.HubAccessible {
		c.Registry = RegistryHub
	} else {
		c.Registry = RegistryAliyun
	}

	// ===== 容器配置 =====
	c.ContainerName = "dpanel"
	c.Port = 0 // 随机端口

	// 数据路径根据系统选择
	switch c.Env.OS {
	case "windows":
		c.DataPath = `C:\dpanel\data`
	case "darwin":
		c.DataPath = "/Users/Shared/dpanel"
	default: // linux
		c.DataPath = "/home/dpanel"
	}

	// ===== 网络配置 =====
	c.DNS = ""
	c.HTTPProxy = ""
	c.HTTPSProxy = ""

	// ===== 升级/卸载配置 =====
	c.UpgradeBackup = true
	c.UninstallRemoveData = false
}

// ApplyOptions 批量应用选项
func (c *Config) ApplyOptions(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 基础验证
	if c.Action == "" {
		return fmt.Errorf("action is required")
	}

	// 根据操作类型验证
	switch c.Action {
	case ActionInstall:
		return c.validateInstall()
	case ActionUpgrade:
		return c.validateUpgrade()
	case ActionUninstall:
		return c.validateUninstall()
	}

	return nil
}

// validateInstall 验证安装配置
func (c *Config) validateInstall() error {
	// 检查镜像源是否可用
	if c.InstallType == InstallTypeContainer {
		if !c.Env.DockerAvailable && !c.Env.PodmanAvailable {
			return fmt.Errorf("container installation requires Docker or Podman")
		}
	}

	// 标准版需要容器安装
	if c.Edition == EditionStandard && c.InstallType != InstallTypeContainer {
		return fmt.Errorf("standard edition only supports container installation")
	}

	// 容器名称必填
	if c.InstallType == InstallTypeContainer && c.ContainerName == "" {
		return fmt.Errorf("container name is required")
	}

	return nil
}

// validateUpgrade 验证升级配置
func (c *Config) validateUpgrade() error {
	if len(c.Env.ExistingContainers) == 0 && len(c.Env.ExistingBinaries) == 0 {
		return fmt.Errorf("no existing installation found")
	}
	return nil
}

// validateUninstall 验证卸载配置
func (c *Config) validateUninstall() error {
	if len(c.Env.ExistingContainers) == 0 && len(c.Env.ExistingBinaries) == 0 {
		return fmt.Errorf("no existing installation found")
	}
	return nil
}

// IsDockerAvailable Docker 是否可用
func (c *Config) IsDockerAvailable() bool {
	return c.Env.DockerAvailable || c.Env.PodmanAvailable
}

// GetRegistry 根据配置获取镜像仓库
func (c *Config) GetRegistry() string {
	if c.Registry == "aliyun" {
		return "registry.cn-hangzhou.aliyuncs.com"
	}
	return ""
}

// GetImageName 获取镜像名称
func (c *Config) GetImageName() string {
	registry := c.GetRegistry()

	var name string
	switch c.Version {
	case "community":
		name = "dpanel/dpanel"
	case "pro":
		name = "dpanel/dpanel-pe"
	case "dev":
		name = "dpanel/dpanel"
	}

	var tag string
	if c.Edition == "lite" {
		tag = "lite"
	} else if c.Version == "dev" {
		tag = "beta"
	} else {
		tag = "latest"
	}

	if registry != "" {
		return fmt.Sprintf("%s/%s:%s", registry, name, tag)
	}
	return fmt.Sprintf("%s:%s", name, tag)
}

// === 辅助函数 ===

// detectOS 检测操作系统
func detectOS() string {
	return runtime.GOOS
}

// getDockerVersion 获取 Docker 版本
func getDockerVersion(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	// 解析版本：Docker version 24.0.7, build afdd53b
	parts := strings.Split(string(out), " ")
	if len(parts) >= 3 {
		return strings.TrimSuffix(parts[2], ",")
	}
	return ""
}

// getPodmanVersion 获取 Podman 版本
func getPodmanVersion(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	// 解析版本：podman version 4.7.2
	parts := strings.Split(string(out), " ")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// testDockerService 测试 Docker/Podman 服务是否可用
func testDockerService(cmd string) bool {
	// 执行 docker ps 或 podman ps 测试
	out, err := exec.Command(cmd, "ps").CombinedOutput()
	if err != nil {
		return false
	}
	// docker ps 成功会输出表头
	return strings.Contains(string(out), "CONTAINER ID")
}

// getPodmanSockPath 获取 Podman sock 路径
func getPodmanSockPath() string {
	// 优先使用 XDG_RUNTIME_DIR
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/podman/podman.sock"
	}
	return "/run/user/1000/podman/podman.sock"
}

// testRegistryConnectivity 测试镜像仓库连通性
func testRegistryConnectivity(registry string) bool {
	// TODO: 实现真实的连通性测试
	// 暂时返回 true，后续可以用 HTTP 请求测试
	return true
}

// getExistingContainers 获取现有容器列表
func getExistingContainers() []string {
	// TODO: 实现 docker ps 获取现有容器
	return []string{}
}

// getExistingBinaries 获取现有二进制文件列表
func getExistingBinaries() []string {
	// TODO: 实现查找现有二进制文件
	return []string{}
}

// === 验证函数 ===

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
