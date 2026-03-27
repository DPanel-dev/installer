package tui

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/handler"
	"github.com/dpanel-dev/installer/internal/types"
	"github.com/dpanel-dev/installer/pkg/i18n"
)

// TUI 实现 handler.Handler 和 tea.Model 接口
type TUI struct {
	// 配置
	cfg *config.Config

	// 状态
	step        Step
	cursor      int
	inputValue  string
	inputCursor int // 输入光标位置
	width       int
	height      int
	quitting    bool
	err         error

	// 缓存当前步骤定义
	currentDef StepDefinition

	// 步骤历史（记录访问路径，用于回退）
	stepHistory []Step

	// 临时状态存储（如浏览模式状态等）
	state map[string]any
}

// Option 配置选项函数
type Option func(*TUI)

// NewTUI 创建 TUI 实例
func NewTUI(opts ...Option) *TUI {
	t := &TUI{
		width:       80,
		height:      24,
		stepHistory: make([]Step, 0),
		state:       make(map[string]any),
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

	// 用户主动退出（包括 Ctrl+C / 完成页任意键退出）时，不继续执行引擎
	if t.quitting {
		return nil
	}

	// 检查是否成功完成
	if t.step != StepComplete {
		return nil // 用户中途退出
	}

	// TUI 流程完成，允许主流程执行引擎
	t.cfg.Finished = true

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
	case progressTickMsg:
		return t.doProgressTick()
	case preRunDoneMsg:
		return t.handlePreRunDone(msg)
	case progressDoneMsg:
		return t.handleProgressDone(msg)
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
	t.currentDef = GetStepDef(t.step)
	t.cursor = 0
	t.inputValue = ""

	options := t.getOptions()

	// 确定 finalValue：Options[0].Value（输入类型）→ 历史值覆盖
	var finalValue string
	if t.currentDef.Type == StepTypeInput || t.currentDef.Type == StepTypePathInput {
		if len(options) > 0 && options[0].Value != "" {
			finalValue = options[0].Value
		}
	}
	if saved := t.cfg.GetStepValue(t.step.String()); saved != "" {
		finalValue = saved
	}

	// 如果 finalValue 为空则取第一个非禁用的选项
	for i, opt := range options {
		if !opt.Disabled && (t.currentDef.Type == StepTypeMenu || t.currentDef.Type == StepTypeConfirm) {
			t.cursor = i
			break
		}
	}

	// 应用 finalValue
	if finalValue != "" {
		switch t.currentDef.Type {
		case StepTypeInput, StepTypePathInput:
			t.inputValue = finalValue
			t.inputCursor = len(finalValue) // 光标放在末尾
		case StepTypeMenu, StepTypeConfirm:
			for i, opt := range options {
				if opt.Value == finalValue {
					t.cursor = i
					break
				}
			}
		}
	} else {
		// 无默认值时，光标也重置
		t.inputCursor = 0
	}

	// 清理浏览状态（切换步骤时）
	delete(t.state, "browse")

	// 进度步骤统一初始化计时状态
	if t.currentDef.Type == StepTypeProgress {
		t.state["progress_elapsed_seconds"] = 0
	} else {
		if !t.isPreRunActive() {
			delete(t.state, "progress_elapsed_seconds")
		}
	}
}

func (t *TUI) isPreRunActive() bool {
	v, _ := t.state["prerun_active"].(bool)
	return v
}

// getBrowseState 获取浏览状态
func (t *TUI) getBrowseState() *BrowseState {
	if bs, ok := t.state["browse"].(*BrowseState); ok {
		return bs
	}
	return nil
}

// setBrowseState 设置浏览状态
func (t *TUI) setBrowseState(bs *BrowseState) {
	t.state["browse"] = bs
}

// getMaxShow 计算浏览模式的最大可见行数
func (t *TUI) getMaxShow() int {
	fixedHeight := 14 // Logo + 标题 + 路径 + 帮助 + 边距
	maxShow := t.height - fixedHeight
	if maxShow < 5 {
		maxShow = 5
	}
	return maxShow
}

// clearBrowseState 清除浏览状态
func (t *TUI) clearBrowseState() {
	delete(t.state, "browse")
}

// enterBrowseMode 进入浏览模式
func (t *TUI) enterBrowseMode() {
	dir := t.inputValue
	if dir == "" {
		u, _ := user.Current()
		if u != nil {
			dir = u.HomeDir
		}
	}

	bs := &BrowseState{
		Dir:    dir,
		Cursor: 0,
		Offset: 0,
	}
	bs.LoadEntries()
	t.setBrowseState(bs)
	t.currentDef.Type = StepTypeBrowse
}

// exitBrowseMode 退出浏览模式
func (t *TUI) exitBrowseMode(confirm bool) {
	bs := t.getBrowseState()
	if bs != nil && confirm && len(bs.Entries) > 0 && bs.Cursor < len(bs.Entries) {
		// 选择光标所在的目录
		selected := bs.Entries[bs.Cursor]
		t.inputValue = filepath.Join(bs.Dir, selected.Name())
		t.inputCursor = len(t.inputValue)
	}
	t.clearBrowseState()
	// 恢复为 PathInput 类型
	t.currentDef = GetStepDef(t.step)
}

// handleKey 处理按键
func (t *TUI) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// 完成页支持任意键退出
	if t.step == StepComplete {
		t.quitting = true
		return t, tea.Quit
	}

	// 进入步骤前的 PreRun 执行中：仅允许 Ctrl+C 退出
	if t.isPreRunActive() {
		if key == "ctrl+c" {
			t.quitting = true
			return t, tea.Quit
		}
		return t, nil
	}

	// 浏览模式单独处理
	if t.currentDef.Type == StepTypeBrowse {
		return t.handleBrowseKey(key)
	}

	switch key {
	case "ctrl+c":
		t.quitting = true
		return t, tea.Quit

	case "esc":
		return t.goBack()

	case "backspace", "left", "right":
		t.handleTextInput(key)
		return t, nil

	case "up":
		t.moveCursor(-1)
		return t, nil

	case "down":
		t.moveCursor(1)
		return t, nil

	case "enter":
		return t.handleEnter()

	case "tab":
		// 路径输入：进入浏览模式
		if t.currentDef.Type == StepTypePathInput {
			t.enterBrowseMode()
		}
		return t, nil

	default:
		t.handleTextInput(key)
		return t, nil
	}
}

// handleBrowseKey 处理浏览模式按键
func (t *TUI) handleBrowseKey(key string) (tea.Model, tea.Cmd) {
	bs := t.getBrowseState()
	if bs == nil {
		return t, nil
	}

	switch key {
	case "up":
		bs.MoveCursor(-1, t.getMaxShow())
	case "down":
		bs.MoveCursor(1, t.getMaxShow())
	case "backspace", "left":
		// 返回上一级目录
		parent := filepath.Dir(bs.Dir)
		if parent != bs.Dir {
			bs.Dir = parent
			bs.LoadEntries()
		}
	case " ", "right":
		// 进入选中目录
		if len(bs.Entries) > 0 && bs.Cursor < len(bs.Entries) {
			selected := bs.Entries[bs.Cursor]
			bs.Dir = filepath.Join(bs.Dir, selected.Name())
			bs.LoadEntries()
		}
	case "enter":
		// 确认选择
		t.exitBrowseMode(true)
	case "esc":
		// 取消，返回输入模式
		t.exitBrowseMode(false)
	case "ctrl+c":
		t.quitting = true
		return t, tea.Quit
	}
	return t, nil
}

// handleTextInput 处理文本输入类型的按键
func (t *TUI) handleTextInput(key string) bool {
	// 只处理输入类型
	if t.currentDef.Type != StepTypeInput && t.currentDef.Type != StepTypePathInput {
		return false
	}

	switch key {
	case "backspace":
		if t.inputCursor > 0 {
			t.inputValue = t.inputValue[:t.inputCursor-1] + t.inputValue[t.inputCursor:]
			t.inputCursor--
		}
	case "left":
		if t.inputCursor > 0 {
			t.inputCursor--
		}
	case "right":
		if t.inputCursor < len(t.inputValue) {
			t.inputCursor++
		}
	default:
		if len(key) == 1 {
			t.inputValue = t.inputValue[:t.inputCursor] + key + t.inputValue[t.inputCursor:]
			t.inputCursor++
		}
	}
	return true
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
	// 浏览模式：确认选择
	if t.currentDef.Type == StepTypeBrowse {
		t.exitBrowseMode(true)
		return t, nil
	}

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

	case StepTypeInput, StepTypePathInput:
		value = t.inputValue
		// 如果输入为空，使用 Options[0].Value 作为默认值
		if value == "" {
			options := t.getOptions()
			if len(options) > 0 {
				value = options[0].Value
			}
		}
	}

	// 记录选择值（用于回退时恢复）
	if value != "" {
		t.cfg.SetStepValue(t.step.String(), value)
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
		t.stepHistory = nil // 清空历史
		t.step = StepLanguage
		t.initStep()
		return t, nil
	}

	// 记录当前步骤到历史（进入下一步前）
	t.stepHistory = append(t.stepHistory, t.step)

	// 获取下一步
	var nextStep Step
	if t.currentDef.Next != nil {
		nextStep = t.currentDef.Next(t.cfg)
	} else {
		nextStep = t.step + 1
	}

	return t.enterStep(nextStep)
}

// handleProgressDone 处理异步过程步骤完成
func (t *TUI) handleProgressDone(msg progressDoneMsg) (tea.Model, tea.Cmd) {
	if t.currentDef.Type != StepTypeProgress {
		return t, nil
	}

	if msg.err != nil {
		t.err = msg.err
		t.step = StepError
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
	return t, nil
}

// handlePreRunDone 处理进入步骤前的 PreRun 完成
func (t *TUI) handlePreRunDone(msg preRunDoneMsg) (tea.Model, tea.Cmd) {
	if !t.isPreRunActive() {
		return t, nil
	}

	t.state["prerun_active"] = false

	if msg.err != nil {
		t.err = msg.err
		t.step = StepError
		t.initStep()
		return t, nil
	}

	// PreRun 完成后进入步骤正常展示状态
	t.initStep()
	return t, nil
}

func (t *TUI) enterStep(step Step) (tea.Model, tea.Cmd) {
	t.step = step
	t.initStep()

	// 进入步骤前的耗时准备：PreRun 异步执行 + 统一读秒
	if t.currentDef.PreRun != nil {
		t.state["prerun_active"] = true
		t.state["progress_elapsed_seconds"] = 0
		return t, tea.Batch(
			tea.Tick(time.Second, func(time.Time) tea.Msg {
				return progressTickMsg{}
			}),
			preRunDoneCmd(t.currentDef.PreRun, t.cfg),
		)
	}

	// 进度步骤统一：异步执行 Finish + 读秒
	if t.currentDef.Type == StepTypeProgress {
		return t, tea.Batch(
			tea.Tick(time.Second, func(time.Time) tea.Msg {
				return progressTickMsg{}
			}),
			progressDoneCmd(progressStepRunner(t.currentDef, t.cfg)),
		)
	}

	return t, nil
}

// doProgressTick 执行进度步骤的统一读秒刷新
func (t *TUI) doProgressTick() (tea.Model, tea.Cmd) {
	if t.currentDef.Type != StepTypeProgress && !t.isPreRunActive() {
		return t, nil
	}

	elapsed, _ := t.state["progress_elapsed_seconds"].(int)
	elapsed++
	t.state["progress_elapsed_seconds"] = elapsed

	return t, tea.Tick(time.Second, func(time.Time) tea.Msg {
		return progressTickMsg{}
	})
}

// goBack 返回上一步
func (t *TUI) goBack() (tea.Model, tea.Cmd) {
	if t.isPreRunActive() {
		return t, nil
	}

	// 浏览模式：退出浏览模式（不确认选择）
	if t.currentDef.Type == StepTypeBrowse {
		t.exitBrowseMode(false)
		return t, nil
	}

	if t.step <= StepLanguage || t.step == StepComplete || t.step == StepError {
		return t, nil
	}

	// 从历史中取出上一步
	if len(t.stepHistory) == 0 {
		return t, nil
	}

	prevStep := t.stepHistory[len(t.stepHistory)-1]
	t.stepHistory = t.stepHistory[:len(t.stepHistory)-1]

	t.step = prevStep
	t.initStep()
	return t, nil
}

// ========== 渲染方法 ==========

func (t *TUI) renderTitle() string {
	var title string
	if t.step == StepLanguage {
		title = "🚀 DPanel - " + "选择语言" + " / Select Language" + fmt.Sprintf(" (%d/%d)", t.step, StepComplete)
	} else {
		stepName := i18n.T(t.currentDef.TitleKey)
		title = fmt.Sprintf("🚀 DPanel - %s (%d/%d)", stepName, t.step, StepComplete)
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

	// PreRun 阶段统一显示请稍候读秒
	if t.isPreRunActive() {
		elapsed, _ := t.state["progress_elapsed_seconds"].(int)
		msg := fmt.Sprintf("%s (已运行 %ds)", i18n.T("please_wait"), elapsed)
		b.WriteString(infoStyle.Render("⏳ " + msg))
		b.WriteString("\n")
		return b.String()
	}

	// 渲染步骤提示信息（橙色 + > 内容格式）
	if t.currentDef.Message != nil {
		if msg := t.currentDef.Message(t.cfg); msg != nil && msg.Content != "" {
			// 按 | 分隔多行，每行都以 > 开头
			lines := strings.Split(msg.Content, "|")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					b.WriteString(messageStyle.Render("> " + line))
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
	}

	switch t.currentDef.Type {
	case StepTypeMenu:
		b.WriteString(t.renderMenu())

	case StepTypeInput:
		b.WriteString(t.renderInput())

	case StepTypePathInput:
		b.WriteString(t.renderPathInput())

	case StepTypeBrowse:
		b.WriteString(t.renderBrowse())

	case StepTypeConfirm:
		b.WriteString(t.renderConfirm())

	case StepTypeProgress:
		msg := i18n.T("please_wait")
		elapsed, _ := t.state["progress_elapsed_seconds"].(int)
		msg = fmt.Sprintf("%s (已运行 %ds)", msg, elapsed)
		b.WriteString(infoStyle.Render("⏳ " + msg))
		b.WriteString("\n")

	case StepTypeComplete:
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

	// 输入行（与菜单选中项相同样式）
	var display string
	if t.inputValue == "" {
		// 显示空输入框 + 光标
		display = menuItemSelectedStyle.Render("  ▸ █")
	} else {
		// 显示输入值 + 光标
		before := t.inputValue[:t.inputCursor]
		after := t.inputValue[t.inputCursor:]
		display = menuItemSelectedStyle.Render("  ▸ " + before + "█" + after)
	}
	b.WriteString(display)
	b.WriteString("\n")

	options := t.getOptions()

	// 说明文字（与菜单选项相同样式）
	if len(options) > 0 && options[0].Description != "" {
		desc := options[0].Description
		if !isTranslated(desc) {
			desc = i18n.T(desc)
		}
		b.WriteString(descriptionStyle.Render("  " + desc))
		b.WriteString("\n")
	}

	return b.String()
}

func (t *TUI) renderPathInput() string {
	var b strings.Builder

	options := t.getOptions()

	// 输入行（与菜单选中项相同样式）
	var display string
	if t.inputValue == "" {
		// 显示占位符或空输入框 + 光标
		if len(options) > 0 && options[0].Label != "" {
			placeholder := options[0].Label
			if !isTranslated(placeholder) {
				placeholder = i18n.T(placeholder)
			}
			display = menuItemSelectedStyle.Render("  ▸ " + placeholder)
		} else {
			// Label 为空时，显示空输入框 + 光标
			display = menuItemSelectedStyle.Render("  ▸ █")
		}
	} else {
		// 显示输入值 + 光标
		before := t.inputValue[:t.inputCursor]
		after := t.inputValue[t.inputCursor:]
		display = menuItemSelectedStyle.Render("  ▸ " + before + "█" + after)
	}
	b.WriteString(display)
	b.WriteString("\n")

	// 说明文字（与菜单选项相同样式）
	if len(options) > 0 && options[0].Description != "" {
		desc := options[0].Description
		if !isTranslated(desc) {
			desc = i18n.T(desc)
		}
		b.WriteString(descriptionStyle.Render("  " + desc))
		b.WriteString("\n")
	}

	return b.String()
}

func (t *TUI) renderBrowse() string {
	var b strings.Builder
	bs := t.getBrowseState()
	if bs == nil {
		return ""
	}

	// 当前目录
	b.WriteString(lipgloss.NewStyle().Foreground(primaryColor).Render("📁 " + bs.Dir))
	b.WriteString("\n\n")

	// 目录列表
	if len(bs.Entries) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render("  (空目录或无法读取)"))
	} else {
		maxShow := t.getMaxShow()

		total := len(bs.Entries)
		showScrollbar := total > maxShow

		// 计算滚动条滑块位置（基于光标在总数中的位置）
		var thumbPos int
		if total > 1 {
			thumbPos = int(float64(bs.Cursor) / float64(total-1) * float64(maxShow-1))
		}

		// 显示可见条目
		end := min(bs.Offset+maxShow, total)
		for i := bs.Offset; i < end; i++ {
			entry := bs.Entries[i]
			selected := i == bs.Cursor

			// 滚动条（左侧固定位置）
			var line string
			if showScrollbar {
				visibleIdx := i - bs.Offset
				if visibleIdx == thumbPos {
					line = lipgloss.NewStyle().Foreground(primaryColor).Render("█ ")
				} else {
					line = lipgloss.NewStyle().Foreground(mutedColor).Render("│ ")
				}
			} else {
				line = "  "
			}

			// 选中标记 + 图标 + 名称（两个空格在图标和名称之间）
			if selected {
				line += menuItemSelectedStyle.Render("▸ 📁  " + entry.Name())
			} else {
				line += menuItemStyle.Render("  📁  " + entry.Name())
			}

			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	return b.String()
}

func (t *TUI) renderConfirm() string {
	var b strings.Builder

	cfg := t.cfg

	// 通用配置
	details := [][2]string{
		{i18n.T("install_method"), cfg.InstallType},
		{i18n.T("select_registry"), cfg.Registry},
		{i18n.T("select_version"), cfg.Version},
		{i18n.T("select_edition"), cfg.Edition},
		{i18n.T("container_name"), cfg.ContainerName},
		{i18n.T("access_port"), strconv.Itoa(cfg.Port)},
		{i18n.T("data_path"), cfg.DataPath},
	}

	// 容器安装特有配置
	if cfg.InstallType == types.InstallTypeContainer {
		details = append(details, [2]string{i18n.T("select_base_image"), cfg.BaseImage})

		connType := "local"
		if cfg.Env.ContainerConn != nil {
			connType = string(cfg.Env.ContainerConn.Type)
		}
		details = append(details, [2]string{i18n.T("docker_connection"), connType})

		if cfg.HTTPProxy != "" {
			details = append(details, [2]string{i18n.T("proxy_address"), cfg.HTTPProxy})
		}
		if cfg.DNS != "" {
			details = append(details, [2]string{i18n.T("dns_address"), cfg.DNS})
		}
	}

	// 使用统一的 messageStyle 格式
	for _, d := range details {
		line := fmt.Sprintf("> %s: %s", d[0], d[1])
		b.WriteString(messageStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(t.renderMenu())

	return b.String()
}

func (t *TUI) renderHelp() string {
	if t.step == StepLanguage {
		return helpStyle.Render("↑/↓ Navigate | Enter Confirm | Esc Back | Ctrl+C Quit") + "\n"
	}

	if t.currentDef.Type == StepTypeInput {
		return helpStyle.Render("Enter Confirm | Esc Back | Ctrl+C Quit") + "\n"
	}

	if t.currentDef.Type == StepTypePathInput {
		return helpStyle.Render("Tab Browse | Enter Confirm | Esc Back | Ctrl+C Quit") + "\n"
	}

	if t.currentDef.Type == StepTypeBrowse {
		return helpStyle.Render("↑/↓ Navigate | Space Enter | Enter Confirm | Esc Cancel | Ctrl+C Quit") + "\n"
	}

	return helpStyle.Render("↑/↓ Navigate | Enter Confirm | Esc Back | Ctrl+C Quit") + "\n"
}

// ========== 辅助函数 ==========

func isTranslated(s string) bool {
	return strings.ContainsAny(s, "中文简体") || strings.Contains(s, " ")
}

// ========== 消息类型 ==========

type progressTickMsg struct{}
type progressDoneMsg struct {
	err error
}
type preRunDoneMsg struct {
	err error
}

func progressDoneCmd(fn func() error) tea.Cmd {
	return func() tea.Msg {
		return progressDoneMsg{err: fn()}
	}
}

func preRunDoneCmd(fn func(*config.Config) error, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		return preRunDoneMsg{err: fn(cfg)}
	}
}

func progressStepRunner(def StepDefinition, cfg *config.Config) func() error {
	if def.Finish != nil {
		return func() error {
			return def.Finish(cfg, "")
		}
	}
	return func() error { return nil }
}

// ========== 接口验证 ==========

var _ handler.Handler = (*TUI)(nil)
var _ tea.Model = (*TUI)(nil)
