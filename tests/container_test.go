package tests

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/core"
	"github.com/dpanel-dev/installer/internal/types"
	dockerpkg "github.com/dpanel-dev/installer/pkg/docker"
	dockerclient "github.com/moby/moby/client"
)

// ========== 容器安装测试（需要 Docker） ==========

func dockerClient(t *testing.T) *dockerpkg.Client {
	t.Helper()
	cli, err := dockerpkg.New()
	if err != nil {
		t.Skip("Docker not available, skipping")
	}
	return cli
}

func testDataPath(t *testing.T) string {
	t.Helper()
	rel := filepath.Join("install-test", "data")
	abs, err := filepath.Abs(rel)
	if err != nil {
		t.Fatalf("Abs(%q) error: %v", rel, err)
	}
	_ = os.RemoveAll(abs)
	return abs
}

func containerExists(t *testing.T, cli *dockerpkg.Client, name string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := cli.Client.ContainerInspect(ctx, name, dockerclient.ContainerInspectOptions{})
	return err == nil
}

func containerRunning(t *testing.T, cli *dockerpkg.Client, name string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	inspect, err := cli.Client.ContainerInspect(ctx, name, dockerclient.ContainerInspectOptions{})
	if err != nil {
		return false
	}
	return inspect.Container.State.Running
}

func TestContainer(t *testing.T) {
	cli := dockerClient(t)
	const name = "dpanel-test"
	dataPath := testDataPath(t)

	// 清理残留容器
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cli.Client.ContainerRemove(ctx, name, dockerclient.ContainerRemoveOptions{Force: true})
	})

	// ========== Step 1: 安装（完整配置） ==========
	t.Log("=== Step 1: Install ===")
	port := config.FindAvailablePort(8888)
	if port == 0 {
		t.Fatal("no available port found")
	}
	t.Logf("using port %d", port)

	cfg, err := config.NewConfig(
		config.WithAction(types.ActionInstall),
		config.WithInstallType(types.InstallTypeContainer),
		config.WithVersion(types.VersionCE),
		config.WithEdition(types.EditionLite),
		config.WithBaseImage(types.BaseImageAlpine),
		config.WithName(name),
		config.WithDataPath(dataPath),
		config.WithServerPort(port),
		config.WithClient(cli),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	driver := core.NewContainerDriver(cfg)
	if err := driver.Install(); err != nil {
		t.Fatalf("install error: %v", err)
	}

	// 验证容器运行
	if !containerRunning(t, cli, name) {
		t.Fatal("container not running after install")
	}

	// 验证 HTTP 访问
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	for {
		resp, err := http.Get(url)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("HTTP status: got %d, want %d", resp.StatusCode, http.StatusOK)
		}
		body := make([]byte, 512)
		n, _ := resp.Body.Read(body)
		bodyStr := string(body[:n])
		if !strings.Contains(bodyStr, "<html") && !strings.Contains(bodyStr, "<!DOCTYPE") {
			t.Errorf("response is not HTML, got: %s", bodyStr[:min(n, 200)])
		}
		t.Logf("HTTP %s → %d (%s)", url, resp.StatusCode, bodyStr[:min(n, 200)])
		break
	}

	// 验证数据目录不为空
	entries, err := os.ReadDir(dataPath)
	if err != nil {
		t.Fatalf("ReadDir(%q) error: %v", dataPath, err)
	}
	if len(entries) == 0 {
		t.Fatal("data directory should not be empty after install")
	}
	t.Logf("data directory: %d entries", len(entries))

	// ========== Step 2: 升级（只传 name + 环境变量覆盖） ==========
	t.Log("=== Step 2: Upgrade (name only + env override) ===")
	upgradeCfg, err := config.NewConfig(
		config.WithAction(types.ActionUpgrade),
		config.WithName(name),
		config.WithClient(cli),
		config.WithEnableBackup(false),
		config.WithEnvProxy("http://test-proxy:8080"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	upgradeDriver := core.NewContainerDriver(upgradeCfg)
	if err := upgradeDriver.Upgrade(); err != nil {
		t.Fatalf("upgrade error: %v", err)
	}

	// 验证容器仍运行
	if !containerRunning(t, cli, name) {
		t.Fatal("container not running after upgrade")
	}

	// 验证配置保留（APP_NAME、端口绑定、挂载路径）+ 环境变量覆盖
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	inspect2, err := cli.Client.ContainerInspect(ctx2, name, dockerclient.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("ContainerInspect after upgrade error: %v", err)
	}
	if !hasEnv(inspect2.Container.Config.Env, "APP_NAME", name) {
		t.Errorf("config not preserved: missing APP_NAME=%s in env", name)
	}
	if !hasEnv(inspect2.Container.Config.Env, "HTTP_PROXY", "http://test-proxy:8080") {
		t.Error("env override failed: missing HTTP_PROXY")
	}
	if !containsAny(inspect2.Container.HostConfig.Binds, ":/dpanel") {
		t.Error("config not preserved: missing /dpanel bind mount")
	}
	if len(inspect2.Container.HostConfig.PortBindings) == 0 {
		t.Error("config not preserved: missing port bindings")
	}
	t.Logf("upgrade preserved: binds=%v ports=%v", inspect2.Container.HostConfig.Binds, inspect2.Container.HostConfig.PortBindings)

	// ========== Step 3: 卸载 - 保留数据（只传 name） ==========
	t.Log("=== Step 3: Uninstall (name only, preserve data) ===")
	preserveCfg, err := config.NewConfig(
		config.WithAction(types.ActionUninstall),
		config.WithName(name),
		config.WithClient(cli),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}
	if err := core.NewContainerDriver(preserveCfg).Uninstall(); err != nil {
		t.Fatalf("uninstall (preserve) error: %v", err)
	}

	if containerExists(t, cli, name) {
		t.Error("container should be removed after uninstall")
	}
	if _, err := os.Stat(dataPath); err != nil {
		t.Errorf("data directory should be preserved, got error: %v", err)
	}

	// ========== Step 4: 再次安装 ==========
	t.Log("=== Step 4: Re-install ===")
	cfg.Action = types.ActionInstall
	if err := core.NewContainerDriver(cfg).Install(); err != nil {
		t.Fatalf("re-install error: %v", err)
	}
	if !containerRunning(t, cli, name) {
		t.Fatal("container not running after re-install")
	}

	// ========== Step 5: 卸载 - 删除数据（只传 name + remove-data） ==========
	t.Log("=== Step 5: Uninstall (name + remove-data) ===")
	removeCfg, err := config.NewConfig(
		config.WithAction(types.ActionUninstall),
		config.WithName(name),
		config.WithClient(cli),
		config.WithEnableDeleteData(true),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}
	if err := core.NewContainerDriver(removeCfg).Uninstall(); err != nil {
		t.Fatalf("uninstall (remove data) error: %v", err)
	}

	if containerExists(t, cli, name) {
		t.Error("container should be removed after uninstall")
	}
	if _, err := os.Stat(dataPath); !os.IsNotExist(err) {
		t.Errorf("data directory should be removed, still exists at %q", dataPath)
	}
}
