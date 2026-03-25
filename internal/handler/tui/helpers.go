package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/pkg/i18n"
)

// GetStepDefinition 获取步骤定义
func GetStepDefinition(step Step) StepDefinition {
	if def, ok := StepDefinitions[step]; ok {
		return def
	}
	// 返回默认定义
	return StepDefinition{
		ID:    step,
		Type:  StepTypeMenu,
		Next:  NextStep(step),
	}
}

// ApplyConfig 应用配置到 Config
func ApplyConfig(cfg *config.Config, step Step, value string) error {
	if applier, ok := ConfigAppliers[step]; ok {
		return applier(cfg, value)
	}
	return nil
}

// GetNextStep 获取下一步
func GetNextStep(step Step, cfg *config.Config, selectedValue string) Step {
	if def, ok := StepDefinitions[step]; ok && def.Next != nil {
		return def.Next(cfg, selectedValue)
	}
	// 默认返回下一步（假设步骤是连续的）
	return Step(int(step) + 1)
}

// ========== 渲染辅助函数 ==========

// renderLogo 渲染 DPANEL ASCII 艺术 logo
func renderLogo() string {
	logoStyle := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	logo := "██████╗ ██████╗  █████╗ ███╗   ██╗███████╗██╗     \n" +
		"██╔══██╗██╔══██╗██╔══██╗████╗  ██║██╔════╝██║     \n" +
		"██║  ██║██████╔╝███████║██╔██╗ ██║█████╗  ██║     \n" +
		"██║  ██║██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║     \n" +
		"██████╔╝██║     ██║  ██║██║ ╚████║███████╗███████╗\n" +
		"╚═════╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝\n"

	return logoStyle.Render(logo)
}

// ========== 步骤判断辅助函数 ==========

// isInputStep 判断指定步骤是否为输入步骤
func isInputStep(step Step) bool {
	return step == StepDockerConfig ||
		step == StepTLSConfig ||
		step == StepSSHConfig ||
		step == StepContainerName ||
		step == StepPort ||
		step == StepDataPath ||
		step == StepProxy ||
		step == StepDNS
}

// getStepTitle 获取指定步骤的标题
func getStepTitle(step Step) string {
	switch step {
	case StepDockerConfig:
		return i18n.T("docker_host")
	case StepTLSConfig:
		return i18n.T("tls_path")
	case StepSSHConfig:
		return i18n.T("ssh_user")
	case StepContainerName:
		return i18n.T("container_name")
	case StepPort:
		return i18n.T("access_port")
	case StepDataPath:
		return i18n.T("data_path")
	case StepProxy:
		return i18n.T("proxy_address")
	case StepDNS:
		return i18n.T("dns_address")
	default:
		return ""
	}
}

// ========== 环境检测辅助函数 ==========

// checkDockerCommand 检查 Docker 命令是否存在
func checkDockerCommand() (exists bool, version string) {
	if path, err := exec.LookPath("docker"); err == nil {
		return true, getDockerVersion(path)
	}
	return false, ""
}

// checkPodmanCommand 检查 Podman 命令是否存在
func checkPodmanCommand() (exists bool, version string) {
	if path, err := exec.LookPath("podman"); err == nil {
		return true, getPodmanVersion(path)
	}
	return false, ""
}

// getDockerVersion 获取 Docker 版本
func getDockerVersion(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	// 解析版本：Docker version 24.0.7, build afdd53b
	// 简化处理，只返回前几行
	return string(out)
}

// getPodmanVersion 获取 Podman 版本
func getPodmanVersion(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// testDockerService 测试 Docker/Podman 服务是否可用
func testDockerService(cmd string) bool {
	out, err := exec.Command(cmd, "ps").CombinedOutput()
	if err != nil {
		return false
	}
	// docker ps 成功会输出表头
	return fmt.Sprintf("%s", out)[:13] == "CONTAINER ID" // 简化检查
}

// detectOS 检测操作系统
func detectOS() string {
	return runtime.GOOS
}

// installDockerLinux 在 Linux 上安装 Docker
func installDockerLinux() error {
	// TODO: 实现真实的 Docker 安装逻辑
	return fmt.Errorf("docker installation script not implemented yet")
}

// ========== 步骤验证辅助函数 ==========

// validateContainerName 验证容器名称
func validateContainerName(value string) error {
	if value == "" {
		return fmt.Errorf("container name cannot be empty")
	}
	if len(value) > 64 {
		return fmt.Errorf("container name too long (max 64 characters)")
	}
	return nil
}

// validatePort 验证端口号
func validatePort(value string) error {
	if value == "" {
		return nil // 允许空值（使用默认）
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid port number")
	}
	if port < 0 || port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535")
	}
	return nil
}

