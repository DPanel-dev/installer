package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dpanel-dev/installer/internal/install"
	"github.com/dpanel-dev/installer/pkg/i18n"
)

// Step represents each step in the installation wizard
type Step int

const (
	StepLanguage Step = iota
	StepAction
	StepEnvironmentCheck
	StepInstallDocker
	StepInstallType
	StepVersion
	StepEdition
	StepOS
	StepRegistry
	StepDockerConnection
	StepDockerConfig
	StepTLSConfig
	StepSSHConfig
	StepContainerName
	StepPort
	StepDataPath
	StepProxy
	StepDNS
	StepConfirm
	StepInstalling
	StepComplete
	StepError
)

// Claude Code / VS Code Dark theme colors
var (
	primaryColor = lipgloss.Color("#007ACC") // VS Code blue
	successColor = lipgloss.Color("#4EC9B0") // VS Code teal
	errorColor   = lipgloss.Color("#F14C4C") // VS Code red
	warningColor = lipgloss.Color("#CE9178") // VS Code orange
	mutedColor   = lipgloss.Color("#858585") // VS Code gray

	bgColor         = lipgloss.Color("#1E1E1E") // VS Code background
	bgLightColor    = lipgloss.Color("#252526") // VS Code sidebar
	bgSelectedColor = lipgloss.Color("#094771") // VS Code selection
	bgInputColor    = lipgloss.Color("#3C3C3C") // VS Code input background

	textColor        = lipgloss.Color("#D4D4D4") // VS Code foreground
	textMutedColor   = lipgloss.Color("#858585") // VS Code comment
	textAccentColor  = lipgloss.Color("#4FC1FF") // VS Code bright blue
	textKeywordColor = lipgloss.Color("#569CD6") // VS Code keyword
	textStringColor  = lipgloss.Color("#CE9178") // VS Code string
)

// Compact styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(0)

	subtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textKeywordColor).
			MarginBottom(0).
			MarginTop(0)

	menuItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2).
			MarginBottom(0)

	menuItemSelectedStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(bgSelectedColor).
				PaddingLeft(2).
				PaddingRight(1).
				Bold(true).
				MarginBottom(0)

	descriptionStyle = lipgloss.NewStyle().
				Foreground(textMutedColor).
				PaddingLeft(4).
				MarginBottom(0)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(textKeywordColor).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Foreground(textStringColor).
			Background(bgInputColor).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(0)

	infoStyle = lipgloss.NewStyle().
			Foreground(textMutedColor).
			MarginBottom(0)
)

// Model represents the TUI state
type model struct {
	config          *install.Config
	step            Step
	cursor          int
	choices         []string
	descriptions    []string
	inputValue      string
	width           int
	height          int
	quitting        bool
	error           error
	envCheck        *EnvironmentCheck
	dockerInstalled bool
	osType          string // "windows", "darwin", "linux"
}

// EnvironmentCheck holds environment check results
type EnvironmentCheck struct {
	DockerAvailable bool
	PodmanAvailable bool
	HasPermission   bool
	DockerVersion   string
	PodmanVersion   string
}

// InitialModel creates the initial TUI model
func InitialModel() model {
	cfg := install.NewConfig()
	m := model{
		config: cfg,
		step:   StepLanguage,
		width:  80,
		height: 24,
	}
	m.setupLanguageChoices()
	return m
}

// Init initializes the model
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "esc", "left", "h":
			// Go back to previous step
			newModel, cmd := m.goBack()
			return newModel, cmd
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			return m.handleEnter()
		case "backspace":
			// If in input step and has content, delete last character
			// Otherwise, go back to previous step
			if m.isInputStep() && len(m.inputValue) > 0 {
				m.inputValue = m.inputValue[:len(m.inputValue)-1]
			} else {
				newModel, cmd := m.goBack()
				return newModel, cmd
			}
		default:
			// Handle text input for input steps
			if m.isInputStep() && len(msg.String()) == 1 {
				m.inputValue += msg.String()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View renders the TUI
func (m model) View() string {
	if m.quitting {
		return ""
	}

	var content strings.Builder

	// Top padding
	content.WriteString("\n")

	// Logo - show on all pages
	content.WriteString(renderLogo())
	content.WriteString("\n")

	// Title with step indicator
	if m.step == StepLanguage {
		content.WriteString(titleStyle.Render(i18n.T("select_language") + " / Select Language"))
		content.WriteString("\n")
	} else {
		content.WriteString(titleStyle.Render(i18n.Tf("title_with_step", m.step, StepError)))
		content.WriteString("\n")
	}

	// Spacer before content
	content.WriteString("\n")

	// Render current step content
	switch m.step {
	case StepLanguage:
		content.WriteString(m.renderMenu())

	case StepAction:
		content.WriteString(subtitleStyle.Render(i18n.T("select_action")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepEnvironmentCheck:
		content.WriteString(subtitleStyle.Render(i18n.T("environment_check")))
		content.WriteString("\n\n")
		content.WriteString(m.renderEnvironmentCheck())

	case StepInstallDocker:
		content.WriteString(subtitleStyle.Render(i18n.T("install_docker")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepInstallType:
		content.WriteString(subtitleStyle.Render(i18n.T("install_method")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepVersion:
		content.WriteString(subtitleStyle.Render(i18n.T("select_version")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepEdition:
		content.WriteString(subtitleStyle.Render(i18n.T("select_edition")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepOS:
		content.WriteString(subtitleStyle.Render(i18n.T("select_os")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepRegistry:
		content.WriteString(subtitleStyle.Render(i18n.T("select_registry")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepDockerConnection:
		content.WriteString(subtitleStyle.Render(i18n.T("docker_connection")))
		content.WriteString("\n\n")
		content.WriteString(m.renderMenu())

	case StepDockerConfig, StepTLSConfig, StepSSHConfig,
		StepContainerName, StepPort, StepDataPath, StepProxy, StepDNS:
		content.WriteString(subtitleStyle.Render(m.getStepTitle()))
		content.WriteString("\n\n")
		content.WriteString(m.renderInput())

	case StepConfirm:
		content.WriteString(subtitleStyle.Render(i18n.T("confirm_install")))
		content.WriteString("\n\n")
		content.WriteString(m.renderConfirm())

	case StepInstalling:
		content.WriteString(subtitleStyle.Render(i18n.T("installing")))
		content.WriteString("\n\n")
		content.WriteString(infoStyle.Render(i18n.T("please_wait")))
		content.WriteString("\n")

	case StepComplete:
		content.WriteString(successStyle.Render(i18n.T("installation_complete")))
		content.WriteString("\n")

	case StepError:
		content.WriteString(errorStyle.Render(i18n.T("installation_failed")))
		content.WriteString("\n\n")
		if m.error != nil {
			content.WriteString(m.error.Error())
			content.WriteString("\n")
		}
	}

	// Spacer before help text
	if m.step != StepComplete && m.step != StepError {
		content.WriteString("\n")
	}

	// Help text (except for final steps)
	if m.step != StepComplete && m.step != StepError {
		content.WriteString(helpStyle.Render(i18n.T("help_navigation")))
	} else {
		content.WriteString(helpStyle.Render(i18n.T("quit_prompt")))
	}

	content.WriteString("\n")

	// Return content
	return content.String()
}

// renderLogo renders the DPANEL ASCII art logo
func renderLogo() string {
	logoStyle := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	// 重新设计的 DPANEL ASCII 艺术字
	logo := "██████╗ ██████╗  █████╗ ███╗   ██╗███████╗██╗     \n" +
		"██╔══██╗██╔══██╗██╔══██╗████╗  ██║██╔════╝██║     \n" +
		"██║  ██║██████╔╝███████║██╔██╗ ██║█████╗  ██║     \n" +
		"██║  ██║██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║     \n" +
		"██████╔╝██║     ██║  ██║██║ ╚████║███████╗███████╗\n" +
		"╚═════╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝\n\n\n\n\n"

	return logoStyle.Render(logo)
}

// handleEnter handles the Enter key press
func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepLanguage:
		lang := "en"
		if m.cursor == 1 {
			lang = "zh"
		}
		m.config.Language = lang
		_ = i18n.Init(lang)
		m.step = StepAction
		m.setupActionChoices()

	case StepAction:
		m.config.Action = m.getSelectedAction()
		if m.config.Action == "install" {
			m.step = StepEnvironmentCheck
			m.runEnvironmentCheck()
		} else if m.config.Action == "upgrade" {
			// For upgrade, skip to environment check
			m.step = StepEnvironmentCheck
			m.runEnvironmentCheck()
		} else if m.config.Action == "uninstall" {
			// For uninstall, skip to environment check
			m.step = StepEnvironmentCheck
			m.runEnvironmentCheck()
		}

	case StepEnvironmentCheck:
		if !m.envCheck.DockerAvailable && !m.envCheck.PodmanAvailable {
			m.step = StepInstallDocker
			m.setupInstallDockerChoices()
		} else {
			// Based on action, decide next step
			if m.config.Action == "install" {
				m.step = StepInstallType
				m.setupInstallTypeChoices()
			} else if m.config.Action == "upgrade" || m.config.Action == "uninstall" {
				// For upgrade and uninstall, skip to confirm step
				m.config.InstallType = "container" // Default to container
				m.config.Version = "community"     // Use default
				m.config.Edition = "standard"      // Use default
				m.config.OS = "alpine"             // Use default
				m.config.ImageRegistry = "hub"     // Use default
				m.config.ContainerName = "dpanel"  // Use default
				m.config.Port = 8080               // Use default
				m.config.DataPath = "/home/dpanel" // Use default
				m.config.DockerConnection = &install.DockerConnection{
					Type:     "local",
					SockPath: "/var/run/docker.sock",
				}
				m.step = StepConfirm
			}
		}

	case StepInstallDocker:
		if m.osType == "linux" {
			// Linux: option 0 is install Docker, option 1 is skip
			if m.cursor == 0 {
				m.error = fmt.Errorf("automatic Docker installation is not implemented yet")
				m.step = StepError
			} else {
				// Skip Docker installation, use binary
				m.config.InstallType = "binary"
				m.step = StepVersion
				m.setupVersionChoices()
			}
		} else {
			// Windows/macOS: only option is to skip (binary installation)
			m.config.InstallType = "binary"
			m.step = StepVersion
			m.setupVersionChoices()
		}

	case StepInstallType:
		m.config.InstallType = m.getSelectedInstallType()
		m.step = StepVersion
		m.setupVersionChoices()

	case StepVersion:
		m.config.Version = m.getSelectedVersion()
		m.step = StepEdition
		m.setupEditionChoices()

	case StepEdition:
		m.config.Edition = m.getSelectedEdition()
		// For binary installation, skip OS and registry selection
		if m.config.InstallType == "binary" {
			m.step = StepContainerName
			m.inputValue = "dpanel"
		} else {
			m.step = StepOS
			m.setupOSChoices()
		}

	case StepOS:
		m.config.OS = m.getSelectedOS()
		m.step = StepRegistry
		m.setupRegistryChoices()

	case StepRegistry:
		m.config.ImageRegistry = m.getSelectedRegistry()
		m.step = StepDockerConnection
		m.setupDockerConnectionChoices()

	case StepDockerConnection:
		m.config.DockerConnection.Type = m.getSelectedDockerConnection()
		if m.config.InstallType == "container" {
			if m.config.DockerConnection.Type == "local" {
				m.step = StepContainerName
				m.inputValue = "dpanel"
			} else if m.config.DockerConnection.Type == "tcp" {
				m.step = StepDockerConfig
				m.inputValue = "127.0.0.1:2376"
			} else if m.config.DockerConnection.Type == "ssh" {
				m.step = StepDockerConfig
				m.inputValue = "127.0.0.1:22"
			}
		} else {
			m.step = StepConfirm
			m.setupConfirmChoices()
		}

	case StepDockerConfig:
		if m.config.DockerConnection.Type == "tcp" {
			m.config.DockerConnection.Host = m.inputValue
			m.step = StepTLSConfig
			m.setupTLSChoices()
		} else if m.config.DockerConnection.Type == "ssh" {
			m.config.DockerConnection.Host = m.inputValue
			m.step = StepSSHConfig
			m.inputValue = ""
		} else {
			m.step = StepContainerName
			m.inputValue = "dpanel"
		}

	case StepTLSConfig:
		if m.cursor == 0 {
			m.config.DockerConnection.TLSEnabled = true
			m.step = StepContainerName
			m.inputValue = "dpanel"
		} else {
			m.config.DockerConnection.TLSEnabled = false
			m.step = StepContainerName
			m.inputValue = "dpanel"
		}

	case StepSSHConfig:
		if m.config.DockerConnection.SSHUser == "" {
			m.config.DockerConnection.SSHUser = m.inputValue
			m.inputValue = ""
		} else if m.config.DockerConnection.SSHPass == "" && m.config.DockerConnection.SSHKey == "" {
			m.config.DockerConnection.SSHPass = m.inputValue
			m.step = StepContainerName
			m.inputValue = "dpanel"
		} else {
			m.step = StepContainerName
			m.inputValue = "dpanel"
		}

	case StepContainerName:
		m.config.ContainerName = m.inputValue
		m.step = StepPort
		m.inputValue = ""

	case StepPort:
		if port, err := install.ParsePort(m.inputValue); err == nil {
			m.config.Port = port
		}
		m.step = StepDataPath
		m.inputValue = "/home/dpanel"

	case StepDataPath:
		m.config.DataPath = m.inputValue
		m.step = StepProxy
		m.inputValue = ""

	case StepProxy:
		m.config.Proxy = m.inputValue
		m.step = StepDNS
		m.inputValue = ""

	case StepDNS:
		m.config.DNS = m.inputValue
		m.step = StepConfirm
		m.setupConfirmChoices()

	case StepConfirm:
		if m.cursor == 0 {
			m.step = StepInstalling
			return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
				return installMsg{}
			})
		} else {
			m.step = StepLanguage
			m.setupLanguageChoices()
		}
	}

	m.cursor = 0
	return m, nil
}

// isInputStep checks if current step is an input step
func (m model) isInputStep() bool {
	return m.step == StepDockerConfig ||
		m.step == StepTLSConfig ||
		m.step == StepSSHConfig ||
		m.step == StepContainerName ||
		m.step == StepPort ||
		m.step == StepDataPath ||
		m.step == StepProxy ||
		m.step == StepDNS
}

// getStepTitle returns the title for the current step
func (m model) getStepTitle() string {
	switch m.step {
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

// installMsg is a message to trigger installation
type installMsg struct{}

// StartTUI starts the TUI application
func StartTUI() error {
	p := tea.NewProgram(InitialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// goBack handles going back to the previous step
func (m model) goBack() (tea.Model, tea.Cmd) {
	switch m.step {
	case StepLanguage:
		// Can't go back from first step
		return m, nil
	case StepAction:
		m.step = StepLanguage
		m.setupLanguageChoices()
	case StepEnvironmentCheck:
		m.step = StepAction
		m.setupActionChoices()
	case StepInstallDocker:
		m.step = StepEnvironmentCheck
		m.runEnvironmentCheck()
	case StepInstallType:
		m.step = StepAction
		m.setupActionChoices()
	case StepVersion:
		m.step = StepInstallType
		m.setupInstallTypeChoices()
	case StepEdition:
		m.step = StepVersion
		m.setupVersionChoices()
	case StepOS:
		m.step = StepEdition
		m.setupEditionChoices()
	case StepRegistry:
		m.step = StepOS
		m.setupOSChoices()
	case StepDockerConnection:
		m.step = StepRegistry
		m.setupRegistryChoices()
	case StepDockerConfig:
		m.step = StepDockerConnection
		m.setupDockerConnectionChoices()
	case StepTLSConfig:
		m.step = StepDockerConfig
		m.inputValue = ""
	case StepSSHConfig:
		if m.config.DockerConnection.Type == "tcp" {
			m.step = StepDockerConfig
			m.inputValue = ""
		} else {
			m.step = StepDockerConfig
		}
	case StepContainerName:
		if m.config.InstallType == "container" {
			if m.config.DockerConnection.Type == "local" {
				m.step = StepDockerConnection
				m.setupDockerConnectionChoices()
			} else {
				m.step = StepSSHConfig
				m.inputValue = ""
			}
		} else {
			m.step = StepVersion
			m.setupVersionChoices()
		}
	case StepPort:
		m.step = StepContainerName
		m.inputValue = "dpanel"
	case StepDataPath:
		m.step = StepPort
		m.inputValue = ""
	case StepProxy:
		m.step = StepDataPath
		m.inputValue = "/home/dpanel"
	case StepDNS:
		m.step = StepProxy
		m.inputValue = ""
	case StepConfirm:
		m.step = StepDNS
		m.inputValue = ""
	case StepComplete, StepError:
		return m, nil
	default:
		if m.step > StepAction {
			m.step = StepAction
			m.setupActionChoices()
		}
	}

	m.cursor = 0
	return m, nil
}

// Setup methods for each step

func (m *model) setupLanguageChoices() {
	m.choices = []string{"English", "简体中文"}
	m.descriptions = []string{
		"Use English as the display language",
		"使用简体中文作为显示语言",
	}
}

func (m *model) setupActionChoices() {
	m.choices = []string{
		i18n.T("install_panel"),
		i18n.T("upgrade_panel"),
		i18n.T("uninstall_panel"),
	}
	m.descriptions = []string{
		i18n.T("install_panel_desc"),
		i18n.T("upgrade_panel_desc"),
		i18n.T("uninstall_panel_desc"),
	}
}

func (m *model) setupInstallDockerChoices() {
	// Different options based on OS
	if m.osType == "linux" {
		// Linux: can auto-install Docker
		m.choices = []string{
			i18n.T("install_docker"),
			i18n.T("skip_docker_install"),
		}
		m.descriptions = []string{
			i18n.T("install_docker_desc"),
			i18n.T("skip_docker_install_desc"),
		}
	} else {
		// Windows/macOS: manual installation required
		m.choices = []string{
			i18n.T("skip_docker_install"),
		}
		m.descriptions = []string{
			i18n.T("manual_docker_prompt"),
		}
	}
}

func (m *model) setupInstallTypeChoices() {
	if !m.dockerInstalled {
		m.choices = []string{i18n.T("binary_install")}
		m.descriptions = []string{i18n.T("binary_only_notice")}
	} else {
		m.choices = []string{
			i18n.T("container_install"),
			i18n.T("binary_install"),
		}
		m.descriptions = []string{
			i18n.T("container_install_desc"),
			i18n.T("binary_install_desc"),
		}
	}
}

func (m *model) setupVersionChoices() {
	m.choices = []string{
		i18n.T("community_edition"),
		i18n.T("professional_edition"),
		i18n.T("development_edition"),
	}
	m.descriptions = []string{
		i18n.T("community_edition_desc"),
		i18n.T("professional_edition_desc"),
		i18n.T("development_edition_desc"),
	}
}

func (m *model) setupEditionChoices() {
	m.choices = []string{
		i18n.T("standard_edition"),
		i18n.T("lite_edition"),
	}
	m.descriptions = []string{
		i18n.T("standard_edition_desc"),
		i18n.T("lite_edition_desc"),
	}
}

func (m *model) setupOSChoices() {
	m.choices = []string{
		i18n.T("debian"),
		i18n.T("alpine"),
	}
	m.descriptions = []string{
		i18n.T("debian_desc"),
		i18n.T("alpine_desc"),
	}
}

func (m *model) setupRegistryChoices() {
	m.choices = []string{
		i18n.T("docker_hub"),
		i18n.T("aliyun"),
	}
	m.descriptions = []string{
		i18n.T("docker_hub_desc"),
		i18n.T("aliyun_desc"),
	}
}

func (m *model) setupDockerConnectionChoices() {
	m.choices = []string{
		i18n.T("local_sock"),
		i18n.T("remote_tcp"),
		i18n.T("remote_ssh"),
	}
	m.descriptions = []string{
		i18n.T("local_sock_desc"),
		i18n.T("remote_tcp_desc"),
		i18n.T("remote_ssh_desc"),
	}
}

func (m *model) setupTLSChoices() {
	m.choices = []string{i18n.T("yes"), i18n.T("no")}
	m.descriptions = []string{
		i18n.T("enable_tls_prompt"),
		"",
	}
}

func (m *model) setupConfirmChoices() {
	m.choices = []string{
		i18n.T("start_installation"),
		i18n.T("back_to_modify"),
	}
	m.descriptions = []string{
		i18n.T("confirm"),
		i18n.T("cancel"),
	}
}

// Selection helper methods

func (m model) getSelectedAction() string {
	actions := []string{"install", "upgrade", "uninstall"}
	return actions[m.cursor]
}

func (m model) getSelectedInstallType() string {
	if !m.dockerInstalled {
		return "binary"
	}
	types := []string{"container", "binary"}
	return types[m.cursor]
}

func (m model) getSelectedVersion() string {
	versions := []string{"community", "pro", "dev"}
	return versions[m.cursor]
}

func (m model) getSelectedEdition() string {
	editions := []string{"standard", "lite"}
	return editions[m.cursor]
}

func (m model) getSelectedOS() string {
	osTypes := []string{"debian", "alpine"}
	return osTypes[m.cursor]
}

func (m model) getSelectedRegistry() string {
	registries := []string{"hub", "aliyun"}
	return registries[m.cursor]
}

func (m model) getSelectedDockerConnection() string {
	types := []string{"local", "tcp", "ssh"}
	return types[m.cursor]
}

// Render methods

func (m model) renderMenu() string {
	var s strings.Builder
	for i, choice := range m.choices {
		if i == m.cursor {
			s.WriteString(menuItemSelectedStyle.Render(fmt.Sprintf("▸ %s", choice)))
		} else {
			s.WriteString(menuItemStyle.Render(fmt.Sprintf("  %s", choice)))
		}
		s.WriteString("\n")
		if i < len(m.descriptions) && m.descriptions[i] != "" {
			s.WriteString(descriptionStyle.Render(m.descriptions[i]))
			s.WriteString("\n")
		}
	}
	return s.String()
}

func (m model) renderInput() string {
	var s strings.Builder
	s.WriteString(inputLabelStyle.Render(m.getStepTitle() + ":"))
	s.WriteString("\n\n")
	s.WriteString(inputStyle.Render(m.inputValue + "█"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render(i18n.T("press_enter")))
	return s.String()
}

func (m model) renderEnvironmentCheck() string {
	var s strings.Builder

	// Show OS information
	osName := ""
	switch m.osType {
	case "windows":
		osName = "Windows"
	case "darwin":
		osName = "macOS"
	case "linux":
		osName = "Linux"
	default:
		osName = m.osType
	}
	s.WriteString(infoStyle.Render(i18n.T("os_label") + ": " + osName))
	s.WriteString("\n")

	if m.envCheck != nil {
		if m.envCheck.DockerAvailable {
			s.WriteString(successStyle.Render("✓ " + i18n.T("docker_detected")))
			s.WriteString("\n")
		} else if m.envCheck.PodmanAvailable {
			s.WriteString(successStyle.Render("✓ " + i18n.T("podman_detected")))
			s.WriteString("\n")
		} else {
			s.WriteString(errorStyle.Render("✗ " + i18n.T("docker_not_found")))
			s.WriteString("\n")
		}

		if m.envCheck.DockerAvailable || m.envCheck.PodmanAvailable {
			if m.envCheck.HasPermission {
				s.WriteString(successStyle.Render("✓ " + i18n.T("permission_ok")))
			} else {
				s.WriteString(errorStyle.Render("✗ " + i18n.T("permission_denied")))
			}
			s.WriteString("\n")
		}

		// Show guidance if no Docker available
		if !m.envCheck.DockerAvailable && !m.envCheck.PodmanAvailable {
			s.WriteString("\n")
			if m.osType == "linux" {
				s.WriteString(infoStyle.Render(i18n.T("docker_choice_linux")))
			} else {
				s.WriteString(infoStyle.Render(i18n.T("docker_install_manual")))
				s.WriteString("\n")
				if m.osType == "windows" {
					s.WriteString(infoStyle.Render(i18n.T("docker_download_windows")))
				} else if m.osType == "darwin" {
					s.WriteString(infoStyle.Render(i18n.T("docker_download_macos")))
				}
				s.WriteString("\n")
				s.WriteString(infoStyle.Render(i18n.T("docker_continue_binary")))
			}
		}
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render(i18n.T("press_enter")))
	return s.String()
}

func (m model) renderConfirm() string {
	var s strings.Builder

	s.WriteString(subtitleStyle.Render(i18n.T("configuration_summary")))
	s.WriteString("\n\n")

	cfg := m.config
	details := [][]string{
		{i18n.T("install_method"), cfg.InstallType},
		{i18n.T("select_version"), cfg.Version},
		{i18n.T("select_edition"), cfg.Edition},
		{i18n.T("select_os"), cfg.OS},
		{i18n.T("select_registry"), cfg.ImageRegistry},
		{i18n.T("container_name"), cfg.ContainerName},
		{i18n.T("access_port"), fmt.Sprintf("%d", cfg.Port)},
		{i18n.T("data_path"), cfg.DataPath},
	}

	if cfg.InstallType == "container" {
		details = append(details, []string{i18n.T("docker_connection"), cfg.DockerConnection.Type})
		if cfg.Proxy != "" {
			details = append(details, []string{i18n.T("proxy_address"), cfg.Proxy})
		}
		if cfg.DNS != "" {
			details = append(details, []string{i18n.T("dns_address"), cfg.DNS})
		}
	}

	for _, detail := range details {
		s.WriteString(fmt.Sprintf("%s: %s\n", detail[0], detail[1]))
	}

	s.WriteString("\n")
	s.WriteString(m.renderMenu())

	return s.String()
}

// Environment check

func (m *model) runEnvironmentCheck() {
	m.envCheck = &EnvironmentCheck{}

	// Detect OS type
	m.osType = runtime.GOOS

	// Check for Docker command
	dockerExists := false
	if _, err := exec.LookPath("docker"); err == nil {
		m.envCheck.DockerAvailable = true
		dockerExists = true
	}

	// Check for Podman command
	if _, err := exec.LookPath("podman"); err == nil {
		m.envCheck.PodmanAvailable = true
	}

	// Test Docker service (not just command)
	if dockerExists {
		cmd := exec.Command("docker", "ps")
		if err := cmd.Run(); err != nil {
			m.envCheck.DockerAvailable = false
			m.envCheck.HasPermission = false
		} else {
			m.envCheck.HasPermission = true
		}
	}

	// Test Podman service if Docker failed
	if !m.envCheck.DockerAvailable && m.envCheck.PodmanAvailable {
		cmd := exec.Command("podman", "ps")
		if err := cmd.Run(); err != nil {
			m.envCheck.PodmanAvailable = false
			m.envCheck.HasPermission = false
		} else {
			m.envCheck.HasPermission = true
		}
	}

	m.dockerInstalled = m.envCheck.DockerAvailable || m.envCheck.PodmanAvailable
}
