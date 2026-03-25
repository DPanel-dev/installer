package handler

import (
	"github.com/dpanel-dev/installer/internal/config"
)

// Handler 定义统一的处理器接口
type Handler interface {
	// Name 返回处理器名称
	Name() string

	// Run 运行处理器
	Run(cfg *config.Config) error
}
