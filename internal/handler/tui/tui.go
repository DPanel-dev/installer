package tui

import (
	"fmt"
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

// TUI 实现 handler.Handler 和 tea.Model 接口
type TUI struct {
	// 配置
	cfg *config.Config

	// 状态
	step       Step
	cursor     int
	inputValue string
	width      int
	height     int
	quitting   bool
	err        error

	// 缓存当前步骤定义
	currentDef StepDefinition

	// 历史记录：步骤 -> 选中值标签
	history map[Step]string
}

// Option 配置选项函数
type Option func(*TUI)

// NewTUI 创建 TUI 实例
func NewTUI(opts ...Option) *TUI {
	t := &TUI{
		width:   80,
		height:  24,
		history: make(map[Step]string),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// WithConfig 设置配置
func WithConfig(cfg *config.Config) Option {
	return func(t *TUI) {
		t.cfg = cfg
	}
}

// ========== handler.Handler 接口 ==========

func (t *TUI) Name() string {
	return "tui"
}

func (t *TUI) Run(cfg *config.Config) error {
	t.cfg = cfg
	t.step = StepLanguage
	t.initStep()

	p := tea.NewProgram(t, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		return err
	}

	// 检查是否成功完成
	if t.step != StepComplete {
		return nil // 用户中途退出
	}

	// 配置完成，由 main 调用 engine 执行
	return nil
}

// ========== tea.Model 接口 ==========

func (t *TUI) Init() tea.Cmd {
	return nil
}

func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return t.handleKey(msg)
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		return t, nil
	}
	return t, nil
}

func (t *TUI) View() string {
	if t.quitting {
		return ""
	}

	var b strings.Builder

	// Logo
	b.WriteString("\n")
	b.WriteString(renderLogo())
	b.WriteString("\n")

	// 标题
	b.WriteString(t.renderTitle())
	b.WriteString("\n")

	// 内容
	b.WriteString(t.renderContent())

	// 帮助
	b.WriteString(t.renderHelp())

	return b.String()
}

// ========== 内部方法 ==========

// getOptions 获取当前步骤的选项
func (t *TUI) getOptions() []OptionItem {
	if t.currentDef.Options != nil {
		return t.currentDef.Options(t.cfg)
	}
	return nil
}

// initStep 初始化当前步骤
func (t *TUI) initStep() {
	// 缓存步骤定义
	t.currentDef = GetStepDef(t.step)

	// 设置默认值
	t.inputValue = t.currentDef.DefaultValue

	// 重置光标
	t.cursor = 0

	// 恢复历史选择
	if savedValue, ok := t.history[t.step]; ok {
		switch t.currentDef.Type {
		case StepTypeMenu, StepTypeConfirm:
			// 查找并恢复光标位置
			options := t.getOptions()
			for i, opt := range options {
				if opt.Value == savedValue {
					t.cursor = i
					break
				}
			}
		case StepTypeInput:
			// 恢复输入值
			t.inputValue = savedValue
		}
	}
}

// handleKey 处理按键
func (t *TUI) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c", "q":
		t.quitting = true
		return t, tea.Quit

	case "esc", "left", "h", "backspace":
		return t.goBack()

	case "up", "k":
		t.moveCursor(-1)
		return t, nil

	case "down", "j":
		t.moveCursor(1)
		return t, nil

	case "enter":
		return t.handleEnter()

	default:
		// 输入步骤处理文本输入
		if t.currentDef.Type == StepTypeInput && len(key) == 1 {
			t.inputValue += key
		}
		return t, nil
	}
}

// moveCursor 移动光标
func (t *TUI) moveCursor(delta int) {
	options := t.getOptions()
	if len(options) == 0 {
		return
	}

	newCursor := t.cursor + delta
	if newCursor < 0 {
		newCursor = 0
	}
	if newCursor >= len(options) {
		newCursor = len(options) - 1
	}

	// 跳过禁用选项
	for newCursor >= 0 && newCursor < len(options) && options[newCursor].Disabled {
		if delta > 0 {
			newCursor++
		} else {
			newCursor--
		}
	}

	if newCursor >= 0 && newCursor < len(options) {
		t.cursor = newCursor
	}
}

// handleEnter 处理回车
func (t *TUI) handleEnter() (tea.Model, tea.Cmd) {
	// 获取选中值
	var value string
	switch t.currentDef.Type {
	case StepTypeMenu, StepTypeConfirm:
		options := t.getOptions()
		if len(options) == 0 {
			return t, nil
		}
		if options[t.cursor].Disabled {
			return t, nil
		}
		value = options[t.cursor].Value

	case StepTypeInput:
		value = t.inputValue
		if value == "" {
			value = t.currentDef.DefaultValue
		}
	}

	// 记录历史（用于回退时恢复选中状态）
	if value != "" {
		t.history[t.step] = value
	}

	// 执行 Finish
	if t.currentDef.Finish != nil {
		if err := t.currentDef.Finish(t.cfg, value); err != nil {
			t.err = err
			t.step = StepError
			t.initStep()
			return t, nil
		}
	}

	// 确认页面选择取消
	if t.step == StepConfirm && value == "cancel" {
		t.step = StepLanguage
		t.initStep()
		return t, nil
	}

	// 获取下一步
	if t.currentDef.Next != nil {
		t.step = t.currentDef.Next(t.cfg)
	} else {
		t.step++
	}

	t.initStep()

	// 开始安装
	if t.step == StepInstalling {
		return t, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return installMsg{}
		})
	}

	return t, nil
}

// goBack 返回上一步
func (t *TUI) goBack() (tea.Model, tea.Cmd) {
	if t.step <= StepLanguage || t.step == StepComplete || t.step == StepError {
		return t, nil
	}

	// 输入步骤：有内容时删除字符
	if t.currentDef.Type == StepTypeInput && len(t.inputValue) > 0 {
		t.inputValue = t.inputValue[:len(t.inputValue)-1]
		return t, nil
	}

	t.step = GetPrevStep(t.step)
	t.initStep()
	return t, nil
}

// ========== 渲染方法 ==========

func (t *TUI) renderTitle() string {
	var title string
	if t.step == StepLanguage {
		title = "🚀 DPanel - " + "选择语言" + " / Select Language" + fmt.Sprintf(" (%d/%d)", t.step, StepConfirm)
	} else {
		stepName := i18n.T(t.currentDef.TitleKey)
		title = fmt.Sprintf("🚀 DPanel - %s (%d/%d)", stepName, t.step, StepConfirm)
	}

	width := min(t.width, 80)
	if width < 40 {
		width = 40
	}
	return titleStyle.Width(width).Render(title)
}

func (t *TUI) renderContent() string {
	var b strings.Builder
	b.WriteString("\n")

	switch t.currentDef.Type {
	case StepTypeMenu:
		b.WriteString(t.renderMenu())

	case StepTypeInput:
		b.WriteString(t.renderInput())

	case StepTypeConfirm:
		b.WriteString(t.renderConfirm())

	case StepTypeProgress:
		b.WriteString(infoStyle.Render("⏳ " + i18n.T("please_wait")))
		b.WriteString("\n")

	case StepTypeComplete:
		b.WriteString(successStyle.Render("✓ " + i18n.T("installation_complete")))
		b.WriteString("\n")

	case StepTypeError:
		b.WriteString(errorStyle.Render("✗ " + i18n.T("installation_failed")))
		b.WriteString("\n\n")
		if t.err != nil {
			b.WriteString(t.err.Error())
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (t *TUI) renderMenu() string {
	var b strings.Builder
	options := t.getOptions()

	for i, opt := range options {
		selected := i == t.cursor

		var style lipgloss.Style
		prefix := "  "

		switch {
		case opt.Disabled && selected:
			style = menuItemSelectedDisabledStyle
			prefix = "▸ "
		case opt.Disabled:
			style = menuItemDisabledStyle
		case selected:
			style = menuItemSelectedStyle
			prefix = "▸ "
		default:
			style = menuItemStyle
		}

		label := opt.Label
		if !isTranslated(label) {
			label = i18n.T(label)
		}
		b.WriteString(style.Render(prefix + label))
		b.WriteString("\n")

		if opt.Description != "" {
			desc := opt.Description
			if !isTranslated(desc) {
				desc = i18n.T(desc)
			}
			b.WriteString(descriptionStyle.Render(desc))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (t *TUI) renderInput() string {
	var b strings.Builder
	title := i18n.T(t.currentDef.TitleKey)
	b.WriteString(inputLabelStyle.Render(title + ":"))
	b.WriteString("\n\n")

	display := t.inputValue
	if display == "" && t.currentDef.Placeholder != "" {
		display = lipgloss.NewStyle().Foreground(mutedColor).Render(t.currentDef.Placeholder)
	} else {
		display = inputStyle.Render(display + "█")
	}
	b.WriteString(display)
	b.WriteString("\n")

	return b.String()
}

func (t *TUI) renderConfirm() string {
	var b strings.Builder

	width := min(t.width, 80)
	if width < 40 {
		width = 40
	}
	b.WriteString(subtitleStyle.Width(width).Render(i18n.T("configuration_summary")))
	b.WriteString("\n\n")

	cfg := t.cfg
	details := [][2]string{
		{i18n.T("install_method"), cfg.InstallType},
		{i18n.T("select_version"), cfg.Version},
		{i18n.T("select_edition"), cfg.Edition},
		{i18n.T("container_name"), cfg.ContainerName},
		{i18n.T("access_port"), strconv.Itoa(cfg.Port)},
		{i18n.T("data_path"), cfg.DataPath},
	}

	if cfg.InstallType == core.InstallTypeContainer {
		details = append(details, [2]string{i18n.T("docker_connection"), cfg.DockerConnType})
		if cfg.HTTPProxy != "" {
			details = append(details, [2]string{i18n.T("proxy_address"), cfg.HTTPProxy})
		}
		if cfg.DNS != "" {
			details = append(details, [2]string{i18n.T("dns_address"), cfg.DNS})
		}
	}

	for _, d := range details {
		fmt.Fprintf(&b, "  %s: %s\n", d[0], d[1])
	}

	b.WriteString("\n")
	b.WriteString(t.renderMenu())

	return b.String()
}

func (t *TUI) renderHelp() string {
	if t.step == StepComplete || t.step == StepError {
		return helpStyle.Render("Press 'q' to quit") + "\n"
	}

	if t.step == StepLanguage {
		return helpStyle.Render("↑/↓ Navigate | Enter Confirm | Esc Back | q/Ctrl+C Quit") + "\n"
	}

	if t.currentDef.Type == StepTypeInput {
		return helpStyle.Render("Enter Confirm | Esc Back | q/Ctrl+C Quit") + "\n"
	}

	return helpStyle.Render("↑/↓ Navigate | Enter Confirm | Esc Back | q/Ctrl+C Quit") + "\n"
}

// ========== 辅助函数 ==========

func isTranslated(s string) bool {
	return strings.ContainsAny(s, "中文简体") || strings.Contains(s, " ")
}

// ========== 消息类型 ==========

type installMsg struct{}

// ========== 接口验证 ==========

var _ handler.Handler = (*TUI)(nil)
var _ tea.Model = (*TUI)(nil)
