package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
	containerapi "github.com/moby/moby/api/types/container"
	networkapi "github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
)

// ========== ContainerDriver ==========

// ContainerDriver 容器安装驱动
type ContainerDriver struct {
	Config       *config.Config
	status       types.RuntimeStatus
	ProgressFunc  func(complete, total int64)
	ProgressDone  func()
}

// NewContainerDriver 创建容器安装驱动（只做状态检测，不修改 Config）
func NewContainerDriver(cfg *config.Config) *ContainerDriver {
	d := &ContainerDriver{Config: cfg}

	// 检测容器状态
	if cfg.Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		containerID, err := findContainerID(ctx, cfg.Client.Client, cfg.Name)
		if err == nil && containerID != "" {
			d.status.Exists = true
			d.status.ID = containerID

			inspect, err := cfg.Client.Client.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
			if err == nil && inspect.Container.State.Running {
				d.status.Running = true
			}
		}
	}

	return d
}

// ResolveImage 解析镜像地址（补填 Registry 后返回）
func (c *ContainerDriver) ResolveImage() string {
	cfg := c.Config
	if cfg.Registry == "" && cfg.Action != types.ActionUninstall {
		cfg.Registry = detectRegistry()
	}
	return cfg.GetImageName()
}

// Status 返回当前运行状态
func (c *ContainerDriver) Status() types.RuntimeStatus {
	return c.status
}

// Install 安装容器（全新安装或覆盖安装）
func (c *ContainerDriver) Install() error {
	cfg := c.Config

	if err := os.MkdirAll(cfg.DataPath, 0755); err != nil {
		return fmt.Errorf("create data path failed: %w", err)
	}

	cli := cfg.Client.Client

	// 拉镜像使用独立的长超时
	pullCtx, pullCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer pullCancel()

	slog.Info("Install", "pull", cfg.GetImageName())
	if err := c.pullImage(pullCtx, cli, cfg.GetImageName()); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 删旧容器（如果存在）
	containerID, err := findContainerID(ctx, cli, cfg.Name)
	if err != nil {
		return err
	}
	if containerID != "" {
		slog.Info("Install", "remove", cfg.Name)
		if _, err := cli.ContainerRemove(ctx, containerID, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}
	}

	slog.Info("Install", "create", cfg.Name, "port", cfg.ServerPort)
	createOpts, err := c.containerCreateOptions()
	if err != nil {
		return err
	}

	created, err := cli.ContainerCreate(ctx, createOpts)
	if err != nil {
		return err
	}

	if _, err := cli.ContainerStart(ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
		return err
	}

	// 写 .env 到安装目录
	if err := writeEnv(cfg); err != nil {
		return err
	}

	slog.Info("Install", "started", created.ID[:12])

	c.status.Exists = true
	c.status.ID = created.ID
	c.status.Running = true

	return nil
}

// Upgrade 升级容器：保留旧容器配置，仅允许用户覆盖镜像和环境变量
func (c *ContainerDriver) Upgrade() error {
	cfg := c.Config
	cli := cfg.Client.Client

	if !c.status.Exists {
		return fmt.Errorf("container %s not found", cfg.Name)
	}

	// 1. Inspect 旧容器，提取完整配置
	ctx := context.Background()
	inspect, err := cli.ContainerInspect(ctx, c.status.ID, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspect container failed: %w", err)
	}
	old := inspect.Container

	// 2. 确定新镜像：用户通过 --version/--edition 指定时用新镜像，否则沿用旧镜像
	image := old.Config.Image
	if cfg.Registry == "" && cfg.Action != types.ActionUninstall {
		cfg.Registry = detectRegistry()
	}
	if newImage := cfg.GetImageName(); newImage != "" && cfg.Registry != "" {
		image = newImage
	}

	// 3. 合并环境变量：旧容器 env + 用户覆盖（proxy/dns 等）
	envMap := make(map[string]string)
	for _, line := range old.Config.Env {
		if k, v, ok := strings.Cut(line, "="); ok {
			envMap[k] = v
		}
	}
	// 用户通过 CLI 指定的覆盖项
	if cfg.HTTPProxy != "" {
		envMap["HTTP_PROXY"] = cfg.HTTPProxy
		envMap["HTTPS_PROXY"] = cfg.HTTPProxy
	}
	if cfg.DNS != "" {
		envMap["DP_DNS"] = cfg.DNS
	}
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}

	// 4. 拉取新镜像
	pullCtx, pullCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer pullCancel()
	slog.Info("Upgrade", "pull", image)
	if err := c.pullImage(pullCtx, cli, image); err != nil {
		return err
	}

	// 5. 备份
	if err := c.Backup(); err != nil {
		return err
	}

	// 6. 删旧容器
	removeCtx, removeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer removeCancel()
	slog.Info("Upgrade", "remove", cfg.Name)
	if _, err := cli.ContainerRemove(removeCtx, c.status.ID, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
		return err
	}

	// 7. 用旧配置重建容器
	createOpts := dockerclient.ContainerCreateOptions{
		Config: &containerapi.Config{
			Image:        image,
			Hostname:     old.Config.Hostname,
			Env:          env,
			ExposedPorts: old.Config.ExposedPorts,
		},
		HostConfig: &containerapi.HostConfig{
			Binds:         old.HostConfig.Binds,
			PortBindings:  old.HostConfig.PortBindings,
			RestartPolicy: old.HostConfig.RestartPolicy,
			LogConfig:     old.HostConfig.LogConfig,
			DNS:           old.HostConfig.DNS,
			ExtraHosts:    old.HostConfig.ExtraHosts,
		},
		Name: cfg.Name,
	}

	// 用户覆盖 DNS
	if cfg.DNS != "" {
		addr, err := netip.ParseAddr(cfg.DNS)
		if err != nil {
			return fmt.Errorf("invalid dns address: %w", err)
		}
		createOpts.HostConfig.DNS = []netip.Addr{addr}
	}

	slog.Info("Upgrade", "create", cfg.Name)
	created, err := cli.ContainerCreate(ctx, createOpts)
	if err != nil {
		return err
	}

	if _, err := cli.ContainerStart(ctx, created.ID, dockerclient.ContainerStartOptions{}); err != nil {
		return err
	}

	slog.Info("Upgrade", "started", created.ID[:12])
	c.status.ID = created.ID
	c.status.Running = true
	return nil
}

// Uninstall 卸载容器
func (c *ContainerDriver) Uninstall() error {
	cfg := c.Config

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli := cfg.Client.Client

	containerID, err := findContainerID(ctx, cli, cfg.Name)
	if err != nil {
		return err
	}
	if containerID != "" {
		// inspect 获取镜像名和数据路径
		imageName := cfg.GetImageName()
		dataPath := cfg.DataPath
		if inspect, err := cli.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{}); err == nil {
			imageName = inspect.Container.Config.Image
			// 从 binds 解析数据目录
			if dataPath == "" {
				for _, bind := range inspect.Container.HostConfig.Binds {
					if strings.HasSuffix(bind, ":/dpanel") {
						dataPath = strings.TrimSuffix(bind, ":/dpanel")
						break
					}
				}
			}
		}

		slog.Info("Uninstall", "remove", cfg.Name)
		if _, err := cli.ContainerRemove(ctx, containerID, dockerclient.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}

		// 删除镜像
		slog.Info("Uninstall", "remove_image", imageName)
		if _, err := cli.ImageRemove(ctx, imageName, dockerclient.ImageRemoveOptions{Force: true}); err != nil {
			slog.Warn("Uninstall", "remove_image_error", err)
		}

		// 删除数据目录
		if cfg.UninstallRemoveData && dataPath != "" {
			slog.Info("Uninstall", "remove_data", dataPath)
			if err := os.RemoveAll(dataPath); err != nil {
				return fmt.Errorf("remove data path failed: %w", err)
			}
		}
	}

	// 清理 .env
	_ = os.Remove(envPath(cfg))

	return nil
}

// Backup 备份容器
func (c *ContainerDriver) Backup() error {
	if !c.Config.UpgradeBackup {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli := c.Config.Client.Client

	containerID, err := findContainerID(ctx, cli, c.Config.Name)
	if err != nil {
		return err
	}
	if containerID == "" {
		return nil
	}

	backupName := fmt.Sprintf("%s-backup-%s", c.Config.Name, time.Now().Format("20060102150405"))
	if _, err := cli.ContainerStop(ctx, containerID, dockerclient.ContainerStopOptions{}); err != nil {
		return err
	}
	if _, err := cli.ContainerRename(ctx, containerID, dockerclient.ContainerRenameOptions{NewName: backupName}); err != nil {
		return err
	}

	slog.Info("Upgrade", "backup", backupName)
	return nil
}

// Start 启动容器
func (c *ContainerDriver) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli := c.Config.Client.Client

	containerID, err := findContainerID(ctx, cli, c.Config.Name)
	if err != nil {
		return err
	}
	if containerID == "" {
		return fmt.Errorf("container %s not found", c.Config.Name)
	}

	_, err = cli.ContainerStart(ctx, containerID, dockerclient.ContainerStartOptions{})
	if err == nil {
		c.status.Running = true
	}
	return err
}

// Stop 停止容器
func (c *ContainerDriver) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli := c.Config.Client.Client

	containerID, err := findContainerID(ctx, cli, c.Config.Name)
	if err != nil {
		return err
	}
	if containerID == "" {
		return nil
	}

	_, err = cli.ContainerStop(ctx, containerID, dockerclient.ContainerStopOptions{})
	if err == nil {
		c.status.Running = false
	}
	return err
}

// ========== 私有方法 ==========

// containerCreateOptions 构建容器创建参数
func (c *ContainerDriver) containerCreateOptions() (dockerclient.ContainerCreateOptions, error) {
	cfg := c.Config

	exposedPorts := networkapi.PortSet{}
	portBindings := networkapi.PortMap{}

	addPortBinding := func(hostPort, containerPort string) error {
		port, err := networkapi.ParsePort(containerPort + "/tcp")
		if err != nil {
			return err
		}
		exposedPorts[port] = struct{}{}
		portBindings[port] = []networkapi.PortBinding{{HostIP: netip.Addr{}, HostPort: hostPort}}
		return nil
	}

	if cfg.Edition == types.EditionStandard {
		if err := addPortBinding("80", "80"); err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
		if err := addPortBinding("443", "443"); err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
	}
	if cfg.ServerPort > 0 {
		if err := addPortBinding(fmt.Sprintf("%d", cfg.ServerPort), "8080"); err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
	} else {
		port, err := networkapi.ParsePort("8080/tcp")
		if err != nil {
			return dockerclient.ContainerCreateOptions{}, err
		}
		exposedPorts[port] = struct{}{}
	}

	// 从安装目录 .env 读取用户自定义变量，合并到容器 env
	env := []string{fmt.Sprintf("APP_NAME=%s", cfg.Name)}

	// 读取 .env 中的用户自定义变量
	ePath := envPath(cfg)
	if envMap, err := ReadEnv(ePath); err == nil {
		for k, v := range envMap {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// 安装器管理的 key 覆盖 .env 中的旧值
	// 容器内不传 storage path 和 server port，使用镜像默认值
	installEnv := buildInstallEnv(cfg)
	delete(installEnv, "DP_SYSTEM_STORAGE_LOCAL_PATH")
	delete(installEnv, "STORAGE_LOCAL_PATH")
	delete(installEnv, "APP_SERVER_PORT")
	delete(installEnv, "APP_SERVER_HOST")
	for k, v := range installEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// 过滤 .env 中残留的 storage/host/port（installEnv 已删除，需从 env 列表中也移除）
	filtered := make([]string, 0, len(env))
	skipKeys := map[string]bool{
		"DP_SYSTEM_STORAGE_LOCAL_PATH": true,
		"STORAGE_LOCAL_PATH":           true,
		"APP_SERVER_PORT":              true,
		"APP_SERVER_HOST":              true,
	}
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if !skipKeys[k] {
			filtered = append(filtered, e)
		}
	}
	env = filtered

	hostConfig := &containerapi.HostConfig{
		Binds: buildBinds(cfg),
		PortBindings: portBindings,
		RestartPolicy: containerapi.RestartPolicy{
			Name: "always",
		},
		LogConfig: containerapi.LogConfig{
			Type: "json-file",
			Config: map[string]string{
				"max-size": "5m",
				"max-file": "10",
			},
		},
	}
	if cfg.DNS != "" {
		addr, err := netip.ParseAddr(cfg.DNS)
		if err != nil {
			return dockerclient.ContainerCreateOptions{}, fmt.Errorf("invalid dns address: %w", err)
		}
		hostConfig.DNS = []netip.Addr{addr}
	}
	if !cfg.Client.IsPodman() {
		hostConfig.ExtraHosts = []string{"host.dpanel.local:host-gateway"}
	}

	return dockerclient.ContainerCreateOptions{
		Config: &containerapi.Config{
			Image:        cfg.GetImageName(),
			Hostname:     fmt.Sprintf("%s.pod.dpanel.local", cfg.Name),
			Env:          env,
			ExposedPorts: networkapi.PortSet(exposedPorts),
		},
		HostConfig: hostConfig,
		Name:       cfg.Name,
	}, nil
}

// buildBinds 构建容器挂载列表
func buildBinds(cfg *config.Config) []string {
	binds := []string{fmt.Sprintf("%s:/dpanel", cfg.DataPath)}
	if sock := cfg.Client.DaemonHost(); sock != "" {
		binds = append([]string{fmt.Sprintf("%s:/var/run/docker.sock", sock)}, binds...)
	}
	return binds
}

// findContainerID 查找容器 ID
func findContainerID(ctx context.Context, cli *dockerclient.Client, name string) (string, error) {
	result, err := cli.ContainerList(ctx, dockerclient.ContainerListOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("list containers failed: %w", err)
	}

	for _, item := range result.Items {
		for _, n := range item.Names {
			if strings.TrimPrefix(n, "/") == name {
				return item.ID, nil
			}
		}
	}

	return "", nil
}

// pullImage 拉取镜像并解析进度
func (c *ContainerDriver) pullImage(ctx context.Context, cli *dockerclient.Client, image string) error {
	resp, err := cli.ImagePull(ctx, image, dockerclient.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image failed: %w", err)
	}
	defer resp.Close()

	type progressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	}
	type pullStatus struct {
		Status         string         `json:"status"`
		ProgressDetail progressDetail `json:"progressDetail"`
		ID             string         `json:"id"`
	}

	// 追踪各层的下载进度
	layers := make(map[string]progressDetail)
	scanner := bufio.NewScanner(resp)
	for scanner.Scan() {
		var s pullStatus
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			continue
		}
		if s.Status == "Downloading" && s.ProgressDetail.Total > 0 {
			layers[s.ID] = s.ProgressDetail
			if c.ProgressFunc != nil {
				var complete, total int64
				for _, d := range layers {
					complete += d.Current
					total += d.Total
				}
				c.ProgressFunc(complete, total)
			}
		}
	}

	if err := resp.Wait(ctx); err != nil {
		return fmt.Errorf("wait image pull failed: %w", err)
	}

	// 判断 scanner 自身错误（如读取中断）
	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("read pull response failed: %w", err)
	}

	if c.ProgressDone != nil {
		c.ProgressDone()
	}
	return nil
}

// exportContainerEnv 从容器导出环境变量到安装目录 .env
func exportContainerEnv(cfg *config.Config, containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cli := cfg.Client.Client
	inspect, err := cli.ContainerInspect(ctx, containerID, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspect container failed: %w", err)
	}

	env := make(map[string]string)
	for _, line := range inspect.Container.Config.Env {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	// 去掉不需要持久化的系统变量
	delete(env, "PATH")
	delete(env, "HOME")
	delete(env, "HOSTNAME")

	ePath := envPath(cfg)
	if err := WriteEnv(ePath, env); err != nil {
		return err
	}

	slog.Info("Upgrade", "export", ePath)
	return nil
}

// LogConfig 输出运行时配置信息
func LogConfig(cfg *config.Config) {
	dockerStatus := "not available"
	if cfg.Client != nil {
		dockerStatus = "available"
	}
	slog.Info("Config", "os", cfg.OS, "arch", cfg.Arch, "docker", dockerStatus)
	slog.Info("Config", "type", cfg.InstallType, "version", cfg.Version, "edition", cfg.Edition, "base_image", cfg.BaseImage)
	slog.Info("Config", "name", cfg.Name, "port", cfg.ServerPort, "binary", cfg.BinaryPath, "data", cfg.DataPath)
	if cfg.DNS != "" {
		slog.Info("Config", "dns", cfg.DNS)
	}
	if cfg.HTTPProxy != "" {
		slog.Info("Config", "proxy", cfg.HTTPProxy)
	}
}

// GetAccessURLs 获取访问地址列表
func GetAccessURLs(cfg *config.Config) []string {
	if cfg.ServerPort <= 0 {
		return nil
	}

	port := cfg.ServerPort
	urls := []string{fmt.Sprintf("http://127.0.0.1:%d", port)}

	if cfg.ServerHost != types.ServerHostLocal {
		if localIP := config.GetLocalIP(); localIP != "127.0.0.1" {
			urls = append(urls, fmt.Sprintf("http://%s:%d", localIP, port))
		}
		if publicIP := config.GetPublicIP(); publicIP != "" {
			urls = append(urls, fmt.Sprintf("http://%s:%d", publicIP, port))
		}
	}

	return urls
}
