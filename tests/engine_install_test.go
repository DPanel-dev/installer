package tests

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/core"
	"github.com/dpanel-dev/installer/internal/types"
	dockerpkg "github.com/dpanel-dev/installer/pkg/docker"
	"github.com/joho/godotenv"
	dockerclient "github.com/moby/moby/client"
)

// ========== 镜像地址测试（纯单元测试，不依赖 Docker） ==========

func TestGetImageName(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		edition   string
		baseImage string
		registry  string
		want      string
	}{
		// ===== CE =====
		{"ce_std_alpine", types.VersionCE, types.EditionStandard, types.BaseImageAlpine, "", "dpanel/dpanel:latest"},
		{"ce_std_debian", types.VersionCE, types.EditionStandard, types.BaseImageDebian, "", "dpanel/dpanel:latest-debian"},
		{"ce_std_darwin", types.VersionCE, types.EditionStandard, types.BaseImageDarwin, "", "dpanel/dpanel:latest-darwin"},
		{"ce_std_windows", types.VersionCE, types.EditionStandard, types.BaseImageWindows, "", "dpanel/dpanel:latest-windows"},
		{"ce_lite_alpine", types.VersionCE, types.EditionLite, types.BaseImageAlpine, "", "dpanel/dpanel:lite"},
		{"ce_lite_debian", types.VersionCE, types.EditionLite, types.BaseImageDebian, "", "dpanel/dpanel:lite-debian"},
		{"ce_lite_darwin", types.VersionCE, types.EditionLite, types.BaseImageDarwin, "", "dpanel/dpanel:lite-darwin"},
		{"ce_lite_windows", types.VersionCE, types.EditionLite, types.BaseImageWindows, "", "dpanel/dpanel:lite-windows"},

		// ===== PE =====
		{"pe_std_alpine", types.VersionPE, types.EditionStandard, types.BaseImageAlpine, "", "dpanel/dpanel-pe:latest"},
		{"pe_std_debian", types.VersionPE, types.EditionStandard, types.BaseImageDebian, "", "dpanel/dpanel-pe:latest-debian"},
		{"pe_std_darwin", types.VersionPE, types.EditionStandard, types.BaseImageDarwin, "", "dpanel/dpanel-pe:latest-darwin"},
		{"pe_std_windows", types.VersionPE, types.EditionStandard, types.BaseImageWindows, "", "dpanel/dpanel-pe:latest-windows"},
		{"pe_lite_alpine", types.VersionPE, types.EditionLite, types.BaseImageAlpine, "", "dpanel/dpanel-pe:lite"},
		{"pe_lite_debian", types.VersionPE, types.EditionLite, types.BaseImageDebian, "", "dpanel/dpanel-pe:lite-debian"},
		{"pe_lite_darwin", types.VersionPE, types.EditionLite, types.BaseImageDarwin, "", "dpanel/dpanel-pe:lite-darwin"},
		{"pe_lite_windows", types.VersionPE, types.EditionLite, types.BaseImageWindows, "", "dpanel/dpanel-pe:lite-windows"},

		// ===== BE =====
		{"be_std_alpine", types.VersionBE, types.EditionStandard, types.BaseImageAlpine, "", "dpanel/dpanel:beta"},
		{"be_std_debian", types.VersionBE, types.EditionStandard, types.BaseImageDebian, "", "dpanel/dpanel:beta-debian"},
		{"be_std_darwin", types.VersionBE, types.EditionStandard, types.BaseImageDarwin, "", "dpanel/dpanel:beta-darwin"},
		{"be_std_windows", types.VersionBE, types.EditionStandard, types.BaseImageWindows, "", "dpanel/dpanel:beta-windows"},
		{"be_lite_alpine", types.VersionBE, types.EditionLite, types.BaseImageAlpine, "", "dpanel/dpanel:beta-lite"},
		{"be_lite_debian", types.VersionBE, types.EditionLite, types.BaseImageDebian, "", "dpanel/dpanel:beta-lite-debian"},
		{"be_lite_darwin", types.VersionBE, types.EditionLite, types.BaseImageDarwin, "", "dpanel/dpanel:beta-lite-darwin"},
		{"be_lite_windows", types.VersionBE, types.EditionLite, types.BaseImageWindows, "", "dpanel/dpanel:beta-lite-windows"},

		// ===== Registry =====
		{"ce_lite_hub", types.VersionCE, types.EditionLite, types.BaseImageAlpine, types.RegistryDockerHub, "dpanel/dpanel:lite"},
		{"ce_lite_aliyun", types.VersionCE, types.EditionLite, types.BaseImageAlpine, types.RegistryAliYun, "registry.cn-hangzhou.aliyuncs.com/dpanel/dpanel:lite"},
		{"ce_lite_unavailable", types.VersionCE, types.EditionLite, types.BaseImageAlpine, types.RegistryUnavailable, "dpanel/dpanel:lite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.NewConfig(
				config.WithVersion(tt.version),
				config.WithEdition(tt.edition),
				config.WithBaseImage(tt.baseImage),
				config.WithRegistry(tt.registry),
			)
			if err != nil {
				t.Fatalf("NewConfig() error: %v", err)
			}
			got := cfg.GetImageName()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ========== 容器安装测试（需要 Docker） ==========

func TestEngineContainerInstall(t *testing.T) {
	cli, err := dockerpkg.New()
	if err != nil {
		t.Skip("Docker not available, skipping")
	}

	const containerName = "dpanel-test-install"

	cfg, err := config.NewConfig(
		config.WithAction(types.ActionInstall),
		config.WithInstallType(types.InstallTypeContainer),
		config.WithVersion(types.VersionCE),
		config.WithEdition(types.EditionLite),
		config.WithBaseImage(types.BaseImageAlpine),
		config.WithContainerName(containerName),
		config.WithDataPath(filepath.Join(t.TempDir(), "data")),
		config.WithServerPort(0),
		config.WithClient(cli),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	// 清理
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = cli.Client.ContainerRemove(ctx, containerName, dockerclient.ContainerRemoveOptions{Force: true})
	}()

	// 执行安装
	engine := core.NewEngine(cfg)
	if err := engine.Run(); err != nil {
		t.Fatalf("engine.Run() error: %v", err)
	}

	// Inspect 验证
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inspect, err := cli.Client.ContainerInspect(ctx, containerName, dockerclient.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("ContainerInspect() error: %v", err)
	}

	c := inspect.Container

	// 运行状态
	if !c.State.Running {
		t.Errorf("state: got %q, want running", c.State.Status)
	}

	// 镜像名称
	if c.Config.Image != cfg.GetImageName() {
		t.Errorf("image: got %q, want %q", c.Config.Image, cfg.GetImageName())
	}

	// 环境变量 APP_NAME
	if !hasEnv(c.Config.Env, "APP_NAME", containerName) {
		t.Errorf("env: missing APP_NAME=%s in %v", containerName, c.Config.Env)
	}

	// Hostname
	wantHostname := containerName + ".pod.dpanel.local"
	if c.Config.Hostname != wantHostname {
		t.Errorf("hostname: got %q, want %q", c.Config.Hostname, wantHostname)
	}

	// Volume: 数据目录
	if !contains(c.HostConfig.Binds, cfg.DataPath+":/dpanel") {
		t.Errorf("binds: missing %s:/dpanel in %v", cfg.DataPath, c.HostConfig.Binds)
	}

	// Volume: Docker sock
	if !containsAny(c.HostConfig.Binds, ":/var/run/docker.sock") {
		t.Errorf("binds: missing docker.sock mount")
	}

	// 重启策略
	if c.HostConfig.RestartPolicy.Name != "always" {
		t.Errorf("restart policy: got %q, want always", c.HostConfig.RestartPolicy.Name)
	}

	// 日志配置
	if c.HostConfig.LogConfig.Type != "json-file" {
		t.Errorf("log driver: got %q, want json-file", c.HostConfig.LogConfig.Type)
	}
}

// ========== 二进制安装测试（需要网络） ==========

func TestEngineBinaryInstall(t *testing.T) {
	installDir := "./install-test"
	installPath := filepath.Join(installDir, "dpanel")
	os.RemoveAll(installDir)

	cfg, err := config.NewConfig(
		config.WithAction(types.ActionInstall),
		config.WithInstallType(types.InstallTypeBinary),
		config.WithVersion(types.VersionBE),
		config.WithEdition(types.EditionLite),
		config.WithBaseImage(types.BaseImageDarwin),
		config.WithRegistry(types.RegistryAliYun),
		config.WithDataPath(filepath.Join(installDir, "data")),
		config.WithBinaryPath(installPath),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	engine := core.NewEngine(cfg)
	if err := engine.Run(); err != nil {
		t.Fatalf("engine.Run() error: %v", err)
	}

	// 验证二进制文件
	info, err := os.Stat(cfg.BinaryPath)
	if err != nil {
		t.Fatalf("Stat(%q) error: %v", cfg.BinaryPath, err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("mode: got %v, want executable", info.Mode())
	}
	if info.Size() == 0 {
		t.Errorf("size: got 0, want non-empty binary")
	}

	// 验证 config.yaml
	configYaml := filepath.Join(installDir, "config.yaml")
	if ci, err := os.Stat(configYaml); err != nil {
		t.Errorf("Stat(%q) error: %v", configYaml, err)
	} else if ci.Size() == 0 {
		t.Errorf("config.yaml: got 0 bytes, want non-empty")
	}

	// 验证进程存活（engine 已启动二进制，PID 记录在 .env 中）
	envData, err := os.ReadFile(filepath.Join(installDir, ".env"))
	if err != nil {
		t.Fatalf("read .env file error: %v", err)
	}
	envMap, err := godotenv.Parse(strings.NewReader(string(envData)))
	if err != nil {
		t.Fatalf("parse .env error: %v", err)
	}
	pidStr, ok := envMap["PID"]
	if !ok || pidStr == "" {
		t.Fatalf(".env missing PID field")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		t.Fatalf("invalid pid %q: %v", pidStr, err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		t.Fatalf("FindProcess(%d) error: %v", pid, err)
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("process %d not alive: %v", pid, err)
	}
	t.Logf("binary started, PID: %d", pid)

	// 清理：停止进程
	defer func() {
		_ = proc.Signal(syscall.SIGTERM)
	}()
}

func TestEngineBinaryUninstall(t *testing.T) {
	installPath := "./data/dpanel"
	os.RemoveAll(installPath)

	dataDir := t.TempDir()

	// 安装
	installCfg, err := config.NewConfig(
		config.WithAction(types.ActionInstall),
		config.WithInstallType(types.InstallTypeBinary),
		config.WithVersion(types.VersionCE),
		config.WithEdition(types.EditionLite),
		config.WithBaseImage(types.BaseImageAlpine),
		config.WithDataPath(filepath.Join(dataDir, "data")),
		config.WithBinaryPath(installPath),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	engine := core.NewEngine(installCfg)
	if err := engine.Run(); err != nil {
		t.Fatalf("install: engine.Run() error: %v", err)
	}
	if _, err := os.Stat(installPath); err != nil {
		t.Fatalf("install: binary not found at %q", installPath)
	}

	// 卸载（保留数据）
	uninstallCfg, err := config.NewConfig(
		config.WithAction(types.ActionUninstall),
		config.WithInstallType(types.InstallTypeBinary),
		config.WithDataPath(filepath.Join(dataDir, "data")),
		config.WithBinaryPath(installPath),
		config.WithUninstallRemoveData(false),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	engine = core.NewEngine(uninstallCfg)
	if err := engine.Run(); err != nil {
		t.Fatalf("uninstall: engine.Run() error: %v", err)
	}

	// 验证二进制已删除
	if _, err := os.Stat(installPath); !os.IsNotExist(err) {
		t.Errorf("uninstall: binary still exists at %q", installPath)
	}

	// 验证数据目录保留
	if _, err := os.Stat(filepath.Join(dataDir, "data")); err != nil {
		t.Errorf("uninstall: data directory should be preserved, got error: %v", err)
	}
}

// ========== 辅助函数 ==========

func hasEnv(envs []string, key, value string) bool {
	want := key + "=" + value
	for _, e := range envs {
		if e == want {
			return true
		}
	}
	return false
}

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func containsAny(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
