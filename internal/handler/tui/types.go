package tui

import (
	"os"

	"github.com/dpanel-dev/installer/internal/config"
)

// ========== 步骤类型 ==========

// Step 步骤标识
type Step int

// StepType 步骤显示类型
type StepType int

const (
	StepTypeMenu      StepType = iota // 菜单选择
	StepTypeInput                     // 文本输入
	StepTypePathInput                 // 路径输入（支持输入和浏览切换）
	StepTypeBrowse                    // 路径浏览
	StepTypeConfirm                   // 确认页面
	StepTypeProgress                  // 进度显示
	StepTypeComplete                  // 完成页面
	StepTypeError                     // 错误页面
)

// ========== 步骤常量 ==========

const (
	StepNone Step = iota
	StepLanguage
	StepAction
	StepMirrorCheck
	StepRegistry
	StepInstallType
	StepInstallDocker      // 确认是否在线安装 Docker
	StepInstallingDocker   // 执行 Docker 在线安装
	StepVersion
	StepEdition
	StepBaseImage
	StepDockerSock         // Docker Sock 文件路径
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

// String 返回步骤名称
func (s Step) String() string {
	switch s {
	case StepLanguage:
		return "language"
	case StepAction:
		return "action"
	case StepMirrorCheck:
		return "mirror_check"
	case StepRegistry:
		return "registry"
	case StepInstallType:
		return "install_type"
	case StepInstallDocker:
		return "install_docker"
	case StepInstallingDocker:
		return "installing_docker"
	case StepVersion:
		return "version"
	case StepEdition:
		return "edition"
	case StepBaseImage:
		return "base_image"
	case StepDockerSock:
		return "docker_sock"
	case StepContainerName:
		return "container_name"
	case StepPort:
		return "port"
	case StepDataPath:
		return "data_path"
	case StepProxy:
		return "proxy"
	case StepDNS:
		return "dns"
	case StepConfirm:
		return "confirm"
	case StepInstalling:
		return "installing"
	case StepComplete:
		return "complete"
	case StepError:
		return "error"
	default:
		return "unknown"
	}
}

// ========== 数据结构 ==========

// MessageType 消息类型
type MessageType int

const (
	MessageTypeInfo MessageType = iota
	MessageTypeWarning
	MessageTypeError
	MessageTypeLoading
)

// MessageContent 消息内容
type MessageContent struct {
	Type    MessageType
	Content string
}

// OptionItem 选项/输入定义（统一结构）
type OptionItem struct {
	Value       string // 选项值 / 输入默认值
	Label       string // 显示标签（i18n key）
	Description string // 描述（i18n key），灰色显示在下方
	Disabled    bool   // 是否禁用
}

// StepDefinition 步骤定义
type StepDefinition struct {
	Type     StepType
	TitleKey string

	// 提示信息（可选，返回 nil 则不显示）
	Message func(cfg *config.Config) *MessageContent

	// 统一选项（菜单类型：多个选项；输入类型：一个选项）
	// 对于输入类型，Options[0] 提供：
	//   - Value: 默认值
	//   - Label: 输入框标题（通常与 TitleKey 相同）
	//   - Description: 灰色说明文字
	Options func(cfg *config.Config) []OptionItem

	// 选中/输入后更新 config
	Finish func(cfg *config.Config, value string) error

	// 决定下一步（可选，默认 Step+1）
	Next func(cfg *config.Config) Step
}

// BrowseState 浏览模式状态
type BrowseState struct {
	Dir     string        // 当前目录
	Entries []os.DirEntry // 目录列表
	Cursor  int           // 光标位置
	Offset  int           // 滚动偏移
}

// LoadEntries 加载目录内容
func (bs *BrowseState) LoadEntries() {
	entries, err := os.ReadDir(bs.Dir)
	if err != nil {
		bs.Entries = nil
		bs.Cursor = 0
		bs.Offset = 0
		return
	}

	// 过滤只保留目录
	var dirs []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		}
	}
	bs.Entries = dirs
	bs.Cursor = 0
	bs.Offset = 0
}

// MoveCursor 移动光标（带滚动）
// maxShow: 可见行数
func (bs *BrowseState) MoveCursor(delta int, maxShow int) {
	if len(bs.Entries) == 0 {
		return
	}

	bs.Cursor += delta
	if bs.Cursor < 0 {
		bs.Cursor = 0
	}
	if bs.Cursor >= len(bs.Entries) {
		bs.Cursor = len(bs.Entries) - 1
	}

	// 滚动窗口
	if maxShow < 5 {
		maxShow = 5
	}
	if bs.Cursor < bs.Offset {
		bs.Offset = bs.Cursor
	}
	if bs.Cursor >= bs.Offset+maxShow {
		bs.Offset = bs.Cursor - maxShow + 1
	}
}

// ========== 辅助函数 ==========

// NextStep 创建固定下一步
func NextStep(step Step) func(cfg *config.Config) Step {
	return func(cfg *config.Config) Step {
		return step
	}
}
