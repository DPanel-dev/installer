package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/core"
	"github.com/dpanel-dev/installer/internal/handler"
	"github.com/dpanel-dev/installer/pkg/i18n"
)

// Step represents each step in the installation wizard
type Step int

const (
	StepLanguage Step = iota
	StepAction
	StepInstallType // Merged environment check with config type selection
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

// stepNames maps each step to its display name
var stepNames = map[Step]string{
	StepLanguage:         "select_language",
	StepAction:           "select_action",
	StepInstallType:      "install_method",
	StepVersion:          "select_version",
	StepEdition:          "select_edition",
	StepOS:               "select_os",
	StepRegistry:         "select_registry",
	StepDockerConnection: "docker_connection",
	StepDockerConfig:     "docker_host",
	StepTLSConfig:        "tls_path",
	StepSSHConfig:        "ssh_user",
	StepContainerName:    "container_name",
	StepPort:             "access_port",
	StepDataPath:         "data_path",
	StepProxy:            "proxy_address",
	StepDNS:              "dns_address",
	StepConfirm:          "confirm_install",
	StepInstalling:       "installing",
}

// DPanel / Modern DevOps theme colors
var (
	primaryColor = lipgloss.Color("#1890FF") // DPanel blue
	successColor = lipgloss.Color("#52C41A") // Success green
	errorColor   = lipgloss.Color("#FF4D4F") // Error red
	warningColor = lipgloss.Color("#FAAD14") // Warning amber
	infoColor    = lipgloss.Color("#1890FF") // Info blue

	mutedColor = lipgloss.Color("#8C8C8C") // Muted gray
	lightColor = lipgloss.Color("#BFBFBF") // Light gray

	bgColor         = lipgloss.Color("#141414") // Dark background
	bgLightColor    = lipgloss.Color("#1F1F1F") // Lighter background
	bgSelectedColor = lipgloss.Color("#0050B3") // Selected blue
	bgInputColor    = lipgloss.Color("#2A2A2A") // Input background
	bgHintColor     = lipgloss.Color("#2A2A2A") // Hint box background

	textColor        = lipgloss.Color("#E8E8E8") // Primary text
	textMutedColor   = lipgloss.Color("#8C8C8C") // Muted text
	textAccentColor  = lipgloss.Color("#40A9FF") // Accent blue
	textKeywordColor = lipgloss.Color("#1890FF") // Keyword blue
)

// Compact styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textKeywordColor).
			MarginBottom(1).
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
			Foreground(textColor).
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
			MarginTop(3)

	infoStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true).
			MarginTop(1).
			MarginBottom(1)

	// Message box styles for important prompts
	infoBoxStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Background(bgInputColor).
			Padding(1, 2).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(infoColor)

	warningBoxStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Background(bgInputColor).
			Padding(1, 2).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warningColor)

	errorBoxStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Background(bgInputColor).
			Padding(1, 2).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor)

	// Alias for backward compatibility
	hintBoxStyle = warningBoxStyle

	separatorStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1).
			MarginBottom(1)

	// Disabled style for unavailable options
	menuItemDisabledStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				PaddingLeft(2).
				MarginBottom(0).
				Italic(true)

	menuItemSelectedDisabledStyle = lipgloss.NewStyle().
					Foreground(mutedColor).
					Background(bgInputColor).
					PaddingLeft(2).
					PaddingRight(1).
					MarginBottom(0).
					Italic(true)
)

// Helper methods for responsive styling
func (m model) getResponsiveStyle(maxWidth int) lipgloss.Style {
	width := m.width
	if width > maxWidth {
		width = maxWidth
	}
	if width < 40 {
		width = 40 // Minimum width
	}
	return lipgloss.NewStyle().Width(width)
}

func (m model) getResponsiveWidth(maxWidth int) int {
	width := m.width
	if width > maxWidth {
		return maxWidth
	}
	if width < 40 {
		return 40 // Minimum width
	}
	return width
}

// Model represents the TUI state
type model struct {
	config              *config.Config
	step                Step
	cursor              int
	choices             []string
	descriptions        []string
	disabled            []bool // Track which choices are disabled
	inputValue          string
	width               int
	height              int
	quitting            bool
	error               error
	envCheck            *EnvironmentCheck
	dockerInstalled     bool
	osType              string // "windows", "darwin", "linux"
	manualDockerInstall bool   // User chose to config Docker manually
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
	// Create config with environment detection and smart defaults
	cfg, err := config.NewConfig()
	if err != nil {
		// Fallback to empty config if error
		cfg, _ = config.NewConfig()
	}

	m := model{
		config: cfg,
		step:   StepLanguage,
		width:  80,
		height: 24,
	}
	// Initialize i18n system with default language from config
	_ = i18n.Init(cfg.Language)
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
				// Skip disabled options
				for m.cursor > 0 && len(m.disabled) > m.cursor && m.disabled[m.cursor] {
					m.cursor--
				}
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
				// Skip disabled options
				for m.cursor < len(m.choices)-1 && len(m.disabled) > m.cursor && m.disabled[m.cursor] {
					m.cursor++
				}
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
		titleText := i18n.T("select_language") + " / Select Language"
		titleWidth := m.getResponsiveWidth(80)
		titleStyleWithWidth := titleStyle.Copy().Width(titleWidth)
		content.WriteString(titleStyleWithWidth.Render(titleText))
		content.WriteString("\n")
	} else {
		// Get step name
		stepName := "unknown"
		if name, ok := stepNames[m.step]; ok {
			stepName = i18n.T(name)
		}
		titleText := fmt.Sprintf("🚀 %s - %s (%d/%d)",
			i18n.T("title"),
			stepName,
			m.step,
			StepError-1)
		titleWidth := m.getResponsiveWidth(80)
		titleStyleWithWidth := titleStyle.Copy().Width(titleWidth)
		content.WriteString(titleStyleWithWidth.Render(titleText))
		content.WriteString("\n")
	}

	// Spacer before content
	content.WriteString("\n")

	// Render current step content
	switch m.step {
	case StepLanguage:
		content.WriteString("\n")
		content.WriteString(m.renderMenu())

	case StepAction:
		content.WriteString("\n")
		content.WriteString(m.renderMenu())

	case StepInstallType:
		content.WriteString("\n")
		content.WriteString(m.renderInstallType())

	case StepVersion:
		content.WriteString("\n")
		content.WriteString(m.renderMenu())

	case StepEdition:
		content.WriteString("\n")
		content.WriteString(m.renderEdition())

	case StepOS:
		content.WriteString("\n")
		content.WriteString(m.renderMenu())

	case StepRegistry:
		content.WriteString("\n")
		content.WriteString(m.renderMenu())

	case StepDockerConnection:
		content.WriteString("\n")
		content.WriteString(m.renderMenu())

	case StepDockerConfig, StepTLSConfig, StepSSHConfig,
		StepContainerName, StepPort, StepDataPath, StepProxy, StepDNS:
		content.WriteString("\n")
		content.WriteString(m.renderInput())

	case StepConfirm:
		content.WriteString("\n")
		content.WriteString(m.renderConfirm())

	case StepInstalling:
		content.WriteString("\n")
		content.WriteString(infoStyle.Render("⏳ " + i18n.T("please_wait")))
		content.WriteString("\n")

	case StepComplete:
		content.WriteString("\n")
		if m.manualDockerInstall {
			content.WriteString(infoStyle.Render("ℹ️  " + i18n.T("docker_install_manual")))
			content.WriteString("\n\n")
			content.WriteString(hintBoxStyle.Render(i18n.T("docker_download_url")))
			content.WriteString("\n\n")
			content.WriteString(infoStyle.Render("ℹ️  " + i18n.T("restart_after_docker_install")))
		} else {
			content.WriteString(successStyle.Render("✓ " + i18n.T("installation_complete")))
			content.WriteString("\n")
		}

	case StepError:
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + i18n.T("installation_failed")))
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
		// For language selection step, use hardcoded text since i18n is not initialized yet
		if m.step == StepLanguage {
			content.WriteString(helpStyle.Render("↑/↓ Navigate | Enter Confirm | Esc/Backspace Back | q/Ctrl+C Quit"))
		} else {
			content.WriteString(helpStyle.Render(i18n.T("help_navigation")))
		}
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
		"╚═════╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝\n"

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
		// Only reinitialize if language actually changed
		if m.config.Language != lang {
			m.config.Language = lang
			_ = i18n.Init(lang)
			// Re-setup choices with new language
			m.setupLanguageChoices()
		}
		m.step = StepAction
		m.setupActionChoices()

	case StepAction:
		m.config.Action = m.getSelectedAction()
		m.step = StepInstallType
		m.setupInstallTypeChoices()

	case StepInstallType:
		// Check if selected option is disabled
		if len(m.disabled) > m.cursor && m.disabled[m.cursor] {
			// Option is disabled, do nothing
			return m, nil
		}

		selection := m.getSelectedInstallType()
		if selection == core.InstallTypeInstallDocker {
			// User wants to config Docker (Linux only)
			if m.osType == "linux" {
				// Execute Docker installation script
				if err := m.installDockerLinux(); err != nil {
					m.error = err
					m.step = StepError
				} else {
					// Re-check environment after installation
					m.runEnvironmentCheck()
					// Re-setup choices with updated environment
					m.setupInstallTypeChoices()
				}
			}
		} else if selection == core.InstallTypeBinary {
			m.config.InstallType = core.InstallTypeBinary
			m.step = StepVersion
			m.setupVersionChoices()
		} else if selection == core.InstallTypeContainer {
			m.config.InstallType = core.InstallTypeContainer
			m.step = StepVersion
			m.setupVersionChoices()
		}

	case StepVersion:
		m.config.Version = m.getSelectedVersion()
		m.step = StepEdition
		m.setupEditionChoices()

	case StepEdition:
		// Check if selected option is disabled
		if len(m.disabled) > m.cursor && m.disabled[m.cursor] {
			// Option is disabled, do nothing
			return m, nil
		}

		m.config.Edition = m.getSelectedEdition()
		// For binary installation, skip OS and registry selection
		if m.config.InstallType == core.InstallTypeBinary {
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
		// Apply registry option
		_ = m.config.ApplyOptions(config.WithRegistry(m.getSelectedRegistry()))
		m.step = StepDockerConnection
		m.setupDockerConnectionChoices()

	case StepDockerConnection:
		// Apply Docker connection type option
		connType := m.getSelectedDockerConnection()
		switch connType {
		case core.DockerConnLocal:
			_ = m.config.ApplyOptions(config.WithDockerLocal(""))
		case core.DockerConnTCP:
			_ = m.config.ApplyOptions(config.WithDockerTCP("127.0.0.1", 2376))
		case core.DockerConnSSH:
			_ = m.config.ApplyOptions(config.WithDockerSSH("127.0.0.1", 22, ""))
		}

		if m.config.InstallType == core.InstallTypeContainer {
			if m.config.DockerConnType == core.DockerConnLocal {
				m.step = StepContainerName
				m.inputValue = "dpanel"
			} else if m.config.DockerConnType == core.DockerConnTCP {
				m.step = StepDockerConfig
				m.inputValue = "127.0.0.1:2376"
			} else if m.config.DockerConnType == core.DockerConnSSH {
				m.step = StepDockerConfig
				m.inputValue = "127.0.0.1:22"
			}
		} else {
			m.step = StepConfirm
			m.setupConfirmChoices()
		}

	case StepDockerConfig:
		if m.config.DockerConnType == core.DockerConnTCP {
			// Parse host:port
			_ = m.config.ApplyOptions(config.WithDockerTCP(m.inputValue, 2376))
			m.step = StepTLSConfig
			m.setupTLSChoices()
		} else if m.config.DockerConnType == core.DockerConnSSH {
			// Parse host:port
			_ = m.config.ApplyOptions(config.WithDockerSSH(m.inputValue, 22, ""))
			m.step = StepSSHConfig
			m.inputValue = ""
		} else {
			m.step = StepContainerName
			m.inputValue = "dpanel"
		}

	case StepTLSConfig:
		if m.cursor == 0 {
			_ = m.config.ApplyOptions(config.WithDockerTLS(true, "", "", ""))
			m.step = StepContainerName
			m.inputValue = "dpanel"
		} else {
			_ = m.config.ApplyOptions(config.WithDockerTLS(false, "", "", ""))
			m.step = StepContainerName
			m.inputValue = "dpanel"
		}

	case StepSSHConfig:
		if m.config.DockerSSHUser == "" {
			_ = m.config.ApplyOptions(config.WithDockerSSH(m.config.DockerSSHHost, m.config.DockerSSHPort, m.inputValue))
			m.inputValue = ""
		} else if m.config.DockerSSHPass == "" && m.config.DockerSSHKey == "" {
			_ = m.config.ApplyOptions(config.WithSSHAuth(m.inputValue, ""))
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
		// Parse port from input
		if m.inputValue != "" {
			if port, err := strconv.Atoi(m.inputValue); err == nil {
				m.config.Port = port
			}
		}
		m.step = StepDataPath
		m.inputValue = "/home/dpanel"

	case StepDataPath:
		m.config.DataPath = m.inputValue
		m.step = StepProxy
		m.inputValue = ""

	case StepProxy:
		_ = m.config.ApplyOptions(config.WithHTTPProxy(m.inputValue))
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

// StartTUI starts the TUI application with given Config
func StartTUI(cfg *config.Config) error {
	// Create model with provided config
	m := NewModelWithConfig(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Get final config and run
	finalCfg := finalModel.(model).config
	engine := core.NewEngine(finalCfg)
	return engine.Run()
}

// NewModelWithConfig creates a new model with given Config
func NewModelWithConfig(cfg *config.Config) model {
	return model{
		config: cfg,
		step:   StepLanguage,
		width:  80,
		height: 24,
	}
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
		if m.config.DockerConnType == core.DockerConnTCP {
			m.step = StepDockerConfig
			m.inputValue = ""
		} else {
			m.step = StepDockerConfig
		}
	case StepContainerName:
		if m.config.InstallType == core.InstallTypeContainer {
			if m.config.DockerConnType == core.DockerConnLocal {
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
	m.disabled = make([]bool, len(m.choices))
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
	m.disabled = make([]bool, len(m.choices))
}

func (m *model) setupInstallTypeChoices() {
	// Run environment check first
	m.runEnvironmentCheck()

	// Always show both options, but mark container as disabled if Docker not available
	dockerAvailable := m.envCheck.DockerAvailable || m.envCheck.PodmanAvailable

	if dockerAvailable {
		// Docker is available, show both options as enabled
		m.choices = []string{
			i18n.T("container_install"),
			i18n.T("binary_install"),
		}
		m.descriptions = []string{
			i18n.T("container_install_desc"),
			i18n.T("binary_install_desc"),
		}
		m.disabled = []bool{false, false}
	} else {
		// Docker not available - show both options, but container is disabled
		if m.osType == "linux" {
			// Linux: show config docker option instead of container config
			m.choices = []string{
				i18n.T("install_docker"),
				i18n.T("binary_install"),
			}
			m.descriptions = []string{
				i18n.T("install_docker_linux_desc"),
				i18n.T("binary_install_desc"),
			}
			m.disabled = []bool{false, false}
		} else {
			// Windows/macOS: show container config as disabled
			m.choices = []string{
				i18n.T("container_install"),
				i18n.T("binary_install"),
			}
			m.descriptions = []string{
				i18n.T("container_install_desc") + " " + i18n.T("container_install_disabled"),
				i18n.T("binary_install_desc"),
			}
			m.disabled = []bool{true, false}
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
	m.disabled = make([]bool, len(m.choices))
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

	// Binary installation only supports Lite Edition
	if m.config.InstallType == core.InstallTypeBinary {
		m.disabled = []bool{true, false} // Disable Standard Edition for binary installation
	} else {
		m.disabled = []bool{false, false}
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
	m.disabled = make([]bool, len(m.choices))
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
	m.disabled = make([]bool, len(m.choices))
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
	m.disabled = make([]bool, len(m.choices))
}

func (m *model) setupTLSChoices() {
	m.choices = []string{i18n.T("yes"), i18n.T("no")}
	m.descriptions = []string{
		i18n.T("enable_tls_prompt"),
		"",
	}
	m.disabled = make([]bool, len(m.choices))
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
	m.disabled = make([]bool, len(m.choices))
}

// Selection helper methods

func (m model) getSelectedAction() string {
	actions := []string{core.ActionInstall, core.ActionUpgrade, core.ActionUninstall}
	return actions[m.cursor]
}

func (m model) getSelectedInstallType() string {
	// Map choices to internal values
	// For Linux without Docker: ["install_docker", "binary_install"]
	// For others without Docker: ["binary_install"]
	// With Docker: ["container_install", "binary_install"]

	choice := m.choices[m.cursor]

	switch choice {
	case i18n.T("install_docker"):
		return core.InstallTypeInstallDocker
	case i18n.T("container_install"):
		return core.InstallTypeContainer
	case i18n.T("binary_install"):
		return core.InstallTypeBinary
	default:
		// Fallback
		if len(m.choices) == 1 {
			return core.InstallTypeBinary
		}
		return core.InstallTypeContainer
	}
}

func (m model) getSelectedVersion() string {
	versions := []string{core.VersionCommunity, core.VersionPro, core.VersionDev}
	return versions[m.cursor]
}

func (m model) getSelectedEdition() string {
	editions := []string{core.EditionStandard, core.EditionLite}
	return editions[m.cursor]
}

func (m model) getSelectedOS() string {
	osTypes := []string{core.OSDebian, core.OSAlpine}
	return osTypes[m.cursor]
}

func (m model) getSelectedRegistry() string {
	registries := []string{core.RegistryHub, core.RegistryAliyun}
	return registries[m.cursor]
}

func (m model) getSelectedDockerConnection() string {
	types := []string{core.DockerConnLocal, core.DockerConnTCP, core.DockerConnSSH}
	return types[m.cursor]
}

// Render methods

func (m model) renderMenu() string {
	var s strings.Builder
	for i, choice := range m.choices {
		// Check if this option is disabled
		isDisabled := len(m.disabled) > i && m.disabled[i]
		isSelected := i == m.cursor

		if isDisabled && isSelected {
			s.WriteString(menuItemSelectedDisabledStyle.Render(fmt.Sprintf("▸ %s", choice)))
		} else if isDisabled {
			s.WriteString(menuItemDisabledStyle.Render(fmt.Sprintf("  %s", choice)))
		} else if isSelected {
			s.WriteString(menuItemSelectedStyle.Render(fmt.Sprintf("▸ %s", choice)))
		} else {
			s.WriteString(menuItemStyle.Render(fmt.Sprintf("  %s", choice)))
		}
		s.WriteString("\n")
		if i < len(m.descriptions) && m.descriptions[i] != "" {
			if isDisabled {
				s.WriteString(descriptionStyle.Render(m.descriptions[i]))
			} else {
				s.WriteString(descriptionStyle.Render(m.descriptions[i]))
			}
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
	return s.String()
}

func (m model) renderConfirm() string {
	var s strings.Builder

	subtitleWidth := m.getResponsiveWidth(80)
	subtitleStyleWithWidth := subtitleStyle.Copy().Width(subtitleWidth)
	s.WriteString(subtitleStyleWithWidth.Render(i18n.T("configuration_summary")))
	s.WriteString("\n\n")

	cfg := m.config
	details := [][]string{
		{i18n.T("install_method"), cfg.InstallType},
		{i18n.T("select_version"), cfg.Version},
		{i18n.T("select_edition"), cfg.Edition},
		{i18n.T("select_os"), cfg.OS},
		{i18n.T("select_registry"), cfg.Registry},
		{i18n.T("container_name"), cfg.ContainerName},
		{i18n.T("access_port"), fmt.Sprintf("%d", cfg.Port)},
		{i18n.T("data_path"), cfg.DataPath},
	}

	if cfg.InstallType == core.InstallTypeContainer {
		details = append(details, []string{i18n.T("docker_connection"), cfg.DockerConnType})
		if cfg.HTTPProxy != "" {
			details = append(details, []string{i18n.T("proxy_address"), cfg.HTTPProxy})
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

// renderInstallType renders the config type selection with environment info
func (m model) renderInstallType() string {
	var s strings.Builder

	// Get OS name for display
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

	// Show environment status and helpful info
	if m.envCheck != nil {
		if m.envCheck.DockerAvailable {
			s.WriteString(successStyle.Render("✓ " + i18n.T("docker_detected")))
			s.WriteString("\n\n")
			s.WriteString(infoStyle.Render("ℹ️  " + i18n.T("container_available_prompt")))
			s.WriteString("\n\n")
		} else if m.envCheck.PodmanAvailable {
			s.WriteString(successStyle.Render("✓ " + i18n.T("podman_detected")))
			s.WriteString("\n\n")
			s.WriteString(infoStyle.Render("ℹ️  " + i18n.T("container_available_prompt")))
			s.WriteString("\n\n")
		} else {
			// Docker not available - show warning in hint box with OS info
			if m.osType == "windows" || m.osType == "darwin" {
				warningText := i18n.Tf("docker_not_found_with_os", osName) + "\n\n" +
					i18n.T("docker_download_url")
				boxWidth := m.getResponsiveWidth(76)
				warningBoxWithWidth := warningBoxStyle.Copy().Width(boxWidth)
				s.WriteString(warningBoxWithWidth.Render(warningText))
				s.WriteString("\n")
			} else if m.osType == "linux" {
				warningText := i18n.Tf("docker_not_found_linux_with_os", osName)
				boxWidth := m.getResponsiveWidth(76)
				warningBoxWithWidth := warningBoxStyle.Copy().Width(boxWidth)
				s.WriteString(warningBoxWithWidth.Render(warningText))
				s.WriteString("\n")
			}
		}
	}

	// Show menu options
	s.WriteString(m.renderMenu())

	return s.String()
}

// renderEdition renders the edition selection with info about binary installation limitations
func (m model) renderEdition() string {
	var s strings.Builder

	// Show warning for binary installation
	if m.config.InstallType == core.InstallTypeBinary {
		warningText := i18n.T("binary_install_edition_warning")
		boxWidth := m.getResponsiveWidth(76)
		warningBoxWithWidth := warningBoxStyle.Copy().Width(boxWidth)
		s.WriteString(warningBoxWithWidth.Render(warningText))
		s.WriteString("\n")
	}

	// Show menu options
	s.WriteString(m.renderMenu())

	return s.String()
}

// renderInstallType renders the config type selection with environment info

// installDockerLinux installs Docker on Linux using the official script
func (m *model) installDockerLinux() error {
	// Download and execute Docker's official config script
	// This is a placeholder - actual implementation would download and run the script
	return fmt.Errorf("docker installation script not implemented yet")
}

// TUI 实现 handler.Handler 接口
type TUI struct {
	// 预留扩展字段
}

// Option 配置选项函数
type Option func(*TUI)

// NewTUI 创建 TUI handler
func NewTUI(opts ...Option) *TUI {
	t := &TUI{}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Name 实现 handler.Handler 接口
func (t *TUI) Name() string {
	return "tui"
}

// Run 实现 handler.Handler 接口
func (t *TUI) Run(cfg *config.Config) error {
	return StartTUI(cfg)
}

// 确保类型实现了接口
var _ handler.Handler = (*TUI)(nil)
