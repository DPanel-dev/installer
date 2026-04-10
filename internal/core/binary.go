package core

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	update "github.com/inconshreveable/go-update"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
)

// 默认提取目标
var defaultExtractTargets = []types.ExtractTarget{
	{ImagePath: "/app/server/dpanel", Name: "dpanel", Mode: 0755},
	{ImagePath: "/app/server/config.yaml", Name: "config.yaml", Mode: 0644},
}

// downloadPath 返回安装程序目录下的临时目录
func downloadPath() string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), "download")
}

// progressReader 包装 io.Reader，追踪读取进度
type progressReader struct {
	r    io.Reader
	read int64
	fn   func(complete, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.read += int64(n)
	if pr.fn != nil {
		pr.fn(pr.read, 0) // total=0 表示未知
	}
	return n, err
}

// ========== BinaryDriver ==========

// BinaryDriver 二进制安装驱动
type BinaryDriver struct {
	Config        *config.Config
	status        types.RuntimeStatus
	pids          []int32
	ProgressFunc  func(complete, total int64)
	ProgressDone  func()
}

// NewBinaryDriver 创建二进制安装驱动（只做状态检测，不修改 Config）
func NewBinaryDriver(cfg *config.Config) *BinaryDriver {
	d := &BinaryDriver{Config: cfg}

	// 推算 BinaryPath：DataPath/dpanel-{Name}（如果未显式设置）
	if cfg.BinaryPath == "" && cfg.DataPath != "" && cfg.Name != "" {
		binName := "dpanel-" + cfg.Name
		if cfg.OS == "windows" {
			binName += ".exe"
		}
		cfg.BinaryPath = filepath.Join(cfg.DataPath, binName)
	}

	// 检测 binary 文件
	if cfg.BinaryPath != "" {
		_, err := os.Stat(cfg.BinaryPath)
		d.status.Exists = err == nil
	}

	// 检测进程（按进程名 dpanel-{name} 查找）
	binProcessName := "dpanel-" + cfg.Name
	procs, _ := findProcessesByName(binProcessName)
	if len(procs) > 0 {
		d.status.Running = true
		d.pids = make([]int32, len(procs))
		var ids []string
		for i, p := range procs {
			d.pids[i] = p.Pid
			ids = append(ids, strconv.Itoa(int(p.Pid)))
		}
		d.status.ID = strings.Join(ids, ",")

		// 从进程解析 BinaryPath（upgrade/uninstall 场景）
		if cfg.BinaryPath == "" {
			if exe, err := procs[0].Exe(); err == nil {
				cfg.BinaryPath = exe
				if cfg.DataPath == "" {
					cfg.DataPath = filepath.Dir(exe)
				}
				d.status.Exists = true
			}
		}
	}

	return d
}

// ResolveImage 解析镜像地址（补填 BaseImage/Registry 后返回）
func (d *BinaryDriver) ResolveImage() string {
	cfg := d.Config
	if cfg.BaseImage == "" {
		switch cfg.OS {
		case "darwin":
			cfg.BaseImage = types.BaseImageDarwin
		case "windows":
			cfg.BaseImage = types.BaseImageWindows
		default:
			if config.IsMusl() {
				cfg.BaseImage = types.BaseImageAlpine
			} else {
				cfg.BaseImage = types.BaseImageDebian
			}
		}
	}
	if cfg.Registry == "" && cfg.Action != types.ActionUninstall {
		cfg.Registry = detectRegistry()
	}
	return cfg.GetImageName()
}

// Status 返回当前运行状态
func (d *BinaryDriver) Status() types.RuntimeStatus {
	return d.status
}

// Install 安装二进制（全新安装或覆盖安装）
// 调用前需确保 Config 中 BaseImage/Registry/Version/Edition 已填充
func (d *BinaryDriver) Install() error {
	cfg := d.Config
	installDir := filepath.Dir(cfg.BinaryPath)

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("create install directory failed: %w", err)
	}
	// 二进制数据目录：DataPath/data/
	if err := os.MkdirAll(filepath.Join(cfg.DataPath, "data"), 0755); err != nil {
		return fmt.Errorf("create data path failed: %w", err)
	}

	// 下载到临时目录
	slog.Info("Install", "pull", cfg.GetImageName())
	if err := d.pullFiles(defaultExtractTargets); err != nil {
		return err
	}

	// 下载完成后停止进程
	d.processStop()

	// 覆盖/复制 binary
	tempDir := downloadPath()
	if d.status.Exists {
		// 覆盖安装：用 go-update
		stagingBin := filepath.Join(tempDir, "dpanel")
		binFile, err := os.Open(stagingBin)
		if err != nil {
			return fmt.Errorf("open staging binary failed: %w", err)
		}
		defer binFile.Close()
		slog.Info("Upgrade", "apply", cfg.BinaryPath)
		if err := update.Apply(binFile, update.Options{TargetPath: cfg.BinaryPath}); err != nil {
			return fmt.Errorf("apply binary update failed: %w", err)
		}
	} else {
		// 全新安装：直接复制
		slog.Info("Install", "copy", installDir)
		if err := copyFile(filepath.Join(tempDir, "dpanel"), cfg.BinaryPath, 0755); err != nil {
			return fmt.Errorf("copy binary failed: %w", err)
		}
	}

	// config.yaml：不存在才复制
	configDst := filepath.Join(installDir, "config.yaml")
	if _, err := os.Stat(configDst); os.IsNotExist(err) {
		if err := copyFile(filepath.Join(tempDir, "config.yaml"), configDst, 0644); err != nil {
			return fmt.Errorf("copy config failed: %w", err)
		}
	}

	os.RemoveAll(tempDir)

	if err := writeEnv(cfg); err != nil {
		return err
	}

	if err := d.processStart(); err != nil {
		return fmt.Errorf("start binary failed: %w", err)
	}

	d.status.Exists = true
	d.status.Running = true
	return nil
}

// Upgrade 升级二进制：先解析镜像（可能只有 --name），再走安装流程
func (d *BinaryDriver) Upgrade() error {
	_ = d.ResolveImage()
	return d.Install()
}

// Uninstall 卸载二进制
func (d *BinaryDriver) Uninstall() error {
	cfg := d.Config

	d.processStop()

	installPath := cfg.BinaryPath
	slog.Info("Uninstall", "remove", installPath)
	if err := os.Remove(installPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove binary failed: %w", err)
	}

	// 清理 .env
	_ = os.Remove(envPath(cfg))

	if cfg.UninstallRemoveData && cfg.DataPath != "" {
		slog.Info("Uninstall", "remove_data", cfg.DataPath)
		if err := os.RemoveAll(cfg.DataPath); err != nil {
			return fmt.Errorf("remove data path failed: %w", err)
		}
	}

	slog.Info("Uninstall Done")
	return nil
}

// Backup 二进制安装无备份操作
func (d *BinaryDriver) Backup() error {
	return nil
}

// Start 启动进程
func (d *BinaryDriver) Start() error {
	return d.processStart()
}

// Stop 停止进程
func (d *BinaryDriver) Stop() error {
	return d.processStop()
}

// ========== 私有方法 ==========

// pullFiles 从 OCI 镜像提取文件到临时目录
func (d *BinaryDriver) pullFiles(targets []types.ExtractTarget) error {
	// 准备临时目录
	tempDir := downloadPath()
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("create temp directory failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ref, err := name.ParseReference(
		d.Config.GetImageName(),
		name.WithDefaultRegistry("index.docker.io"),
		name.WithDefaultTag("latest"),
	)
	if err != nil {
		return fmt.Errorf("parse image reference failed: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("pull image failed: %w", err)
	}

	fs := mutate.Extract(img)
	defer fs.Close()

	// 包装 reader 追踪提取进度
	var pfs io.Reader = fs
	if d.ProgressFunc != nil {
		pfs = &progressReader{r: fs, fn: d.ProgressFunc}
	}

	targetMap := make(map[string]*types.ExtractTarget, len(targets))
	for i := range targets {
		targetMap[pathpkg.Clean("/"+targets[i].ImagePath)] = &targets[i]
	}

	extracted := make(map[string]bool, len(targets))

	reader := tar.NewReader(pfs)
	for {
		header, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read image filesystem failed: %w", err)
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}

		cleanPath := pathpkg.Clean("/" + header.Name)
		t, ok := targetMap[cleanPath]
		if !ok {
			continue
		}

		outPath := filepath.Join(downloadPath(), t.Name)
		outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, t.Mode)
		if err != nil {
			return fmt.Errorf("create file %s failed: %w", outPath, err)
		}
		if _, err := io.Copy(outFile, reader); err != nil {
			outFile.Close()
			return fmt.Errorf("extract %s failed: %w", t.Name, err)
		}
		outFile.Close()

		extracted[cleanPath] = true
		if len(extracted) == len(targets) {
			break
		}
	}

	for _, t := range targets {
		key := pathpkg.Clean("/" + t.ImagePath)
		if _, ok := extracted[key]; !ok {
			return fmt.Errorf("%s not found in image %s", t.ImagePath, d.Config.GetImageName())
		}
	}

	// 进度完成，固定最后一行
	if d.ProgressDone != nil {
		d.ProgressDone()
	}

	return nil
}

// buildCmdEnv 从安装目录 .env 读取环境变量，构造子进程环境
func buildCmdEnv(binaryPath string) ([]string, error) {
	env, err := ReadEnv(filepath.Join(filepath.Dir(binaryPath), ".env"))
	if err != nil {
		return nil, err
	}
	result := os.Environ()
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result, nil
}

func (d *BinaryDriver) processStart() error {
	installPath, _ := filepath.Abs(d.Config.BinaryPath)
	configYaml := filepath.Join(filepath.Dir(installPath), "config.yaml")

	cmd := exec.Command(installPath, "server:start", "-f", configYaml)
	cmd.SysProcAttr = sysProcAttr()

	cmdEnv, err := buildCmdEnv(d.Config.BinaryPath)
	if err != nil {
		return fmt.Errorf("read env failed: %w", err)
	}
	cmd.Env = cmdEnv

	slog.Info("Install", "start", installPath)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start process failed: %w", err)
	}

	// 等待 1 秒检查进程是否存活
	time.Sleep(1 * time.Second)
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("process exited immediately")
	}

	slog.Info("Started", "pid", cmd.Process.Pid)

	// PID 写入 .env
	ePath := envPath(d.Config)
	envData, _ := ReadEnv(ePath)
	if envData == nil {
		envData = make(map[string]string)
	}
	envData["PID"] = strconv.Itoa(cmd.Process.Pid)
	_ = WriteEnv(ePath, envData)

	return nil
}

// findProcessesByName 按进程名查找所有匹配的进程
func findProcessesByName(name string) ([]*process.Process, error) {
	all, err := process.Processes()
	if err != nil {
		return nil, err
	}
	var matched []*process.Process
	for _, p := range all {
		pName, err := p.Name()
		if err != nil {
			continue
		}
		if pName == name {
			matched = append(matched, p)
		}
	}
	return matched, nil
}

func (d *BinaryDriver) processStop() error {
	procs, err := findProcessesByName("dpanel-" + d.Config.Name)
	if err != nil || len(procs) == 0 {
		slog.Info("Stop", "status", "not running")
		return nil
	}

	var pidStrs []string
	for _, p := range procs {
		pidStrs = append(pidStrs, strconv.Itoa(int(p.Pid)))
	}
	slog.Info("Stop", "pid", strings.Join(pidStrs, ","))

	for _, p := range procs {
		p.SendSignal(syscall.SIGTERM)
	}

	// 等待进程退出（最多 10 秒）
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		procs, _ = findProcessesByName("dpanel-" + d.Config.Name)
		if len(procs) == 0 {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 超时后 SIGKILL
	for _, p := range procs {
		slog.Warn("Stop", "kill", int(p.Pid))
		p.SendSignal(syscall.SIGKILL)
	}

	return nil
}
