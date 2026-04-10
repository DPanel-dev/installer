package types

import "os"

// RuntimeStatus 表示当前安装目标的运行状态
type RuntimeStatus struct {
	Exists  bool   // 文件或容器是否存在
	Running bool   // 进程或容器是否在运行
	ID      string // 二进制存 PID(s)，容器存 ContainerID
}

// ExtractTarget 定义从 OCI 镜像提取文件的规则
type ExtractTarget struct {
	ImagePath string      // OCI 镜像内路径
	Name      string      // 本地文件名
	Mode      os.FileMode // 文件权限
}

// Driver 定义安装驱动的统一接口
type Driver interface {
	Status() RuntimeStatus
	Install() error
	Upgrade() error
	Uninstall() error
	Backup() error
	Start() error
	Stop() error
	ResolveImage() string // 解析镜像地址（触发 BaseImage/Registry 补填）
}
