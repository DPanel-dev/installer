package config

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/dpanel-dev/installer/internal/types"
	dockerclient "github.com/moby/moby/client"
	"github.com/shirou/gopsutil/v3/process"
)

// DiscoverInstances 发现所有 DPanel 实例（容器 + 二进制）
func (c *Config) DiscoverInstances() []types.InstanceInfo {
	var instances []types.InstanceInfo

	// 1. 扫描容器：按镜像名识别
	if c.Client != nil {
		instances = append(instances, c.discoverContainers()...)
	}

	// 2. 扫描二进制进程：按进程名前缀识别
	instances = append(instances, discoverBinaries()...)

	return instances
}

// FindInstance 按 name 查找单个实例
func (c *Config) FindInstance(name string) *types.InstanceInfo {
	for _, inst := range c.DiscoverInstances() {
		if inst.Name == name {
			return &inst
		}
	}
	return nil
}

// discoverContainers 扫描所有 dpanel 容器
func (c *Config) discoverContainers() []types.InstanceInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Client.Client.ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return nil
	}

	var instances []types.InstanceInfo
	for _, item := range result.Items {
		if !isDPanelImage(item.Image) {
			continue
		}

		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		if name == "" {
			continue
		}

		instances = append(instances, types.InstanceInfo{
			Type:    types.InstallTypeContainer,
			Name:    name,
			Running: item.State == "running",
			ID:      item.ID,
		})
	}
	return instances
}

// discoverBinaries 扫描所有 dpanel- 开头的二进制进程
func discoverBinaries() []types.InstanceInfo {
	all, err := process.Processes()
	if err != nil {
		return nil
	}

	var instances []types.InstanceInfo
	for _, p := range all {
		pName, err := p.Name()
		if err != nil || !strings.HasPrefix(pName, "dpanel-") {
			continue
		}

		name := strings.TrimPrefix(pName, "dpanel-")
		if name == "" {
			continue
		}

		instances = append(instances, types.InstanceInfo{
			Type:    types.InstallTypeBinary,
			Name:    name,
			Running: true,
			ID:      strconv.Itoa(int(p.Pid)),
		})
	}
	return instances
}

// isDPanelImage 判断镜像名是否为 dpanel 镜像
func isDPanelImage(image string) bool {
	lower := strings.ToLower(image)
	return strings.Contains(lower, "/dpanel")
}
