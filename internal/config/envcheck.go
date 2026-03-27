package config

import (
	"runtime"
)

// EnvCheck 环境检测结果
type EnvCheck struct {
	OS   string // windows, darwin, linux
	Arch string // amd64, arm64

	// 容器运行时（优先 Docker，备选 Podman，nil 表示不可用）
	ContainerConn *ContainerConn
}

// NewEnvCheck 创建并执行环境检测
func NewEnvCheck() *EnvCheck {
	env := &EnvCheck{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		ContainerConn: nil,
	}

	// 检测容器运行时（优先 Docker，备选 Podman）
	if conn := DetectDocker(); conn != nil {
		env.ContainerConn = conn
	} else if conn := DetectPodman(); conn != nil {
		env.ContainerConn = conn
	}

	return env
}
