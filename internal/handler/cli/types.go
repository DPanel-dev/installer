package cli

import (
	"github.com/dpanel-dev/installer/internal/config"
)

// FlagType 定义 flag 的类型
type FlagType int

const (
	FlagTypeString FlagType = iota
	FlagTypeInt
	FlagTypeBool
	FlagTypeEnum
)

// FlagDefinition 定义一个命令行 flag
type FlagDefinition struct {
	Name        string                                     // flag 名称，如 "port"
	Type        FlagType                                   // flag 类型
	Default     string                                     // 默认值
	Description string                                     // 帮助描述（英文）
	EnumValues  []string                                   // 枚举值（仅 FlagTypeEnum 时有效）
	Apply       func(value string) (config.Option, error) // 返回 config.Option
}

// CommandDefinition 定义一个子命令
type CommandDefinition struct {
	Name        string            // 命令名称，如 "install"
	Description string            // 帮助描述（英文）
	Flags       []FlagDefinition  // 该命令的 flags
	Action      string            // 对应的 config.Action 值
}
