package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// ========== 颜色定义 ==========

var (
	primaryColor = lipgloss.Color("#1890FF")
	successColor = lipgloss.Color("#52C41A")
	errorColor   = lipgloss.Color("#FF4D4F")
	warningColor = lipgloss.Color("#FAAD14")
	infoColor    = lipgloss.Color("#1890FF")
	mutedColor   = lipgloss.Color("#8C8C8C")
	orangeColor  = lipgloss.Color("#FFA500")

	bgSelectedColor = lipgloss.Color("#0050B3")
	bgInputColor    = lipgloss.Color("#2A2A2A")

	textColor      = lipgloss.Color("#E8E8E8")
	textMutedColor = lipgloss.Color("#8C8C8C")
)

// ========== 样式定义 ==========

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	menuItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			PaddingLeft(2)

	menuItemSelectedStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(bgSelectedColor).
				PaddingLeft(2).
				PaddingRight(1).
				Bold(true)

	menuItemDisabledStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				PaddingLeft(2).
				Italic(true)

	menuItemSelectedDisabledStyle = lipgloss.NewStyle().
					Foreground(mutedColor).
					Background(bgInputColor).
					PaddingLeft(2).
					PaddingRight(1).
					Italic(true)

	descriptionStyle = lipgloss.NewStyle().
				Foreground(textMutedColor).
				PaddingLeft(4)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
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

	warningBoxStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Background(bgInputColor).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warningColor)

	hintBoxStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Background(bgInputColor).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(infoColor)

	errorBoxStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Background(bgInputColor).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor)

	// 统一消息样式（橙色）
	messageStyle = lipgloss.NewStyle().
			Foreground(orangeColor)
)

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
