package types

// InstanceInfo 表示一个已发现的 DPanel 实例
type InstanceInfo struct {
	Type    string // InstallTypeContainer 或 InstallTypeBinary
	Name    string // 实例名称
	Running bool   // 是否运行中
	ID      string // 容器 ID 或 PID
}
