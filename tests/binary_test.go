package tests

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/core"
	"github.com/dpanel-dev/installer/internal/types"
	"github.com/joho/godotenv"
)

// ========== 二进制安装测试（需要网络） ==========

func TestBinary(t *testing.T) {
	const name = "dpanel-test"
	installDir := filepath.Join("install-test", name)
	_ = os.RemoveAll("install-test")

	// ========== Step 1: 安装（完整配置） ==========
	t.Log("=== Step 1: Install ===")
	cfg, err := config.NewConfig(
		config.WithAction(types.ActionInstall),
		config.WithInstallType(types.InstallTypeBinary),
		config.WithVersion(types.VersionBE),
		config.WithEdition(types.EditionLite),
		config.WithBaseImage(types.BaseImageDarwin),
		config.WithRegistry(types.RegistryAliYun),
		config.WithName(name),
		config.WithDataPath(installDir),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	driver := core.NewBinaryDriver(cfg)
	if err := driver.Install(); err != nil {
		t.Fatalf("install error: %v", err)
	}

	// 验证二进制文件路径：DataPath/dpanel-{name}
	expectedBin := filepath.Join(installDir, "dpanel-"+name)
	info, err := os.Stat(expectedBin)
	if err != nil {
		t.Fatalf("binary not found at %q: %v", expectedBin, err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("mode: got %v, want executable", info.Mode())
	}
	if info.Size() == 0 {
		t.Error("binary is empty")
	}
	t.Logf("binary installed at %s (%d bytes)", expectedBin, info.Size())

	// 验证 config.yaml
	configYaml := filepath.Join(installDir, "config.yaml")
	if ci, err := os.Stat(configYaml); err != nil {
		t.Errorf("config.yaml not found: %v", err)
	} else if ci.Size() == 0 {
		t.Error("config.yaml is empty")
	}

	// 验证数据目录：DataPath/data/
	dataDir := filepath.Join(installDir, "data")
	if entries, err := os.ReadDir(dataDir); err != nil {
		t.Logf("data dir not created yet (ok for first install): %v", err)
	} else {
		t.Logf("data directory: %d entries", len(entries))
	}

	// 验证进程存活
	proc := findBinaryProcess(t, installDir)
	defer func() { _ = proc.Signal(syscall.SIGTERM) }()
	t.Logf("Step 1 done, PID: %d", proc.Pid)

	// ========== Step 2: 升级（只传 name） ==========
	t.Log("=== Step 2: Upgrade (name only) ===")
	upgradeCfg, err := config.NewConfig(
		config.WithAction(types.ActionUpgrade),
		config.WithInstallType(types.InstallTypeBinary),
		config.WithName(name),
		config.WithDataPath(installDir),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	upgradeDriver := core.NewBinaryDriver(upgradeCfg)
	if err := upgradeDriver.Upgrade(); err != nil {
		t.Fatalf("upgrade error: %v", err)
	}

	// 验证升级后二进制仍存在
	info2, err := os.Stat(expectedBin)
	if err != nil {
		t.Fatalf("binary not found after upgrade: %v", err)
	}
	if info2.Size() == 0 {
		t.Error("binary is empty after upgrade")
	}
	t.Logf("upgrade done, binary size: %d bytes", info2.Size())

	// 验证进程仍在运行
	proc2 := findBinaryProcess(t, installDir)
	defer func() { _ = proc2.Signal(syscall.SIGTERM) }()
	t.Logf("Step 2 done, PID: %d", proc2.Pid)

	// ========== Step 3: 卸载 - 保留数据（只传 name） ==========
	t.Log("=== Step 3: Uninstall (name only, preserve data) ===")
	preserveCfg, err := config.NewConfig(
		config.WithAction(types.ActionUninstall),
		config.WithInstallType(types.InstallTypeBinary),
		config.WithName(name),
		config.WithDataPath(installDir),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	preserveDriver := core.NewBinaryDriver(preserveCfg)
	if err := preserveDriver.Uninstall(); err != nil {
		t.Fatalf("uninstall (preserve) error: %v", err)
	}

	// 验证二进制已删除
	if _, err := os.Stat(expectedBin); !os.IsNotExist(err) {
		t.Errorf("binary still exists at %q", expectedBin)
	}

	// 验证数据目录保留
	dataDir2 := filepath.Join(installDir, "data")
	if _, err := os.Stat(dataDir2); err != nil {
		t.Errorf("data directory should be preserved, got error: %v", err)
	}
	t.Log("Step 3 done, binary removed, data preserved")

	// ========== Step 4: 再次安装 ==========
	t.Log("=== Step 4: Re-install ===")
	cfg.Action = types.ActionInstall
	reDriver := core.NewBinaryDriver(cfg)
	if err := reDriver.Install(); err != nil {
		t.Fatalf("re-install error: %v", err)
	}
	if _, err := os.Stat(expectedBin); err != nil {
		t.Fatalf("binary not found after re-install: %v", err)
	}
	t.Log("Step 4 done")

	// ========== Step 5: 卸载 - 删除数据 ==========
	t.Log("=== Step 5: Uninstall (name + remove-data) ===")
	removeCfg, err := config.NewConfig(
		config.WithAction(types.ActionUninstall),
		config.WithInstallType(types.InstallTypeBinary),
		config.WithName(name),
		config.WithDataPath(installDir),
		config.WithEnableDeleteData(true),
	)
	if err != nil {
		t.Fatalf("NewConfig() error: %v", err)
	}

	removeDriver := core.NewBinaryDriver(removeCfg)
	if err := removeDriver.Uninstall(); err != nil {
		t.Fatalf("uninstall (remove data) error: %v", err)
	}

	// 验证二进制已删除
	if _, err := os.Stat(expectedBin); !os.IsNotExist(err) {
		t.Errorf("binary still exists at %q", expectedBin)
	}

	// 验证数据目录已删除
	dataDir3 := filepath.Join(installDir, "data")
	if _, err := os.Stat(dataDir3); !os.IsNotExist(err) {
		t.Errorf("data directory should be removed, still exists at %q", dataDir3)
	}
	t.Log("Step 5 done, all cleaned up")
}

// findBinaryProcess 从 .env 读取 PID 并验证进程存活
func findBinaryProcess(t *testing.T, installDir string) *os.Process {
	t.Helper()

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
		t.Fatal(".env missing PID field")
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
	return proc
}
