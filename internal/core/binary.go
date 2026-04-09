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

	"github.com/dpanel-dev/installer/internal/types"
)

// extractTarget 定义从 OCI 镜像提取文件的规则
type extractTarget struct {
	ImagePath string      // OCI 镜像内路径
	Name      string      // 本地文件名
	Mode      os.FileMode // 文件权限
}

// 默认提取目标
var defaultExtractTargets = []extractTarget{
	{ImagePath: "/app/server/dpanel", Name: "dpanel", Mode: 0755},
	{ImagePath: "/app/server/config.yaml", Name: "config.yaml", Mode: 0644},
}

// envPath 返回安装目录下的 .env 路径（进程运行时使用）
func (e *Engine) envPath() string {
	return filepath.Join(filepath.Dir(e.Config.BinaryPath), ".env")
}

// defaultEnvPath 返回安装程序目录下的 default.env 路径（用户可编辑的默认值）
func (e *Engine) defaultEnvPath() string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), "default.env")
}

// downloadPath 返回安装程序目录下的临时目录
func (e *Engine) downloadPath() string {
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

// pullFiles 从 OCI 镜像提取文件到临时目录
func (e *Engine) pullFiles(targets []extractTarget) error {
	// 准备临时目录
	tempDir := e.downloadPath()
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("create temp directory failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ref, err := name.ParseReference(
		e.Config.GetImageName(),
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
	if e.ProgressFunc != nil {
		pfs = &progressReader{r: fs, fn: e.ProgressFunc}
	}

	targetMap := make(map[string]*extractTarget, len(targets))
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

		outPath := filepath.Join(e.downloadPath(), t.Name)
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
			return fmt.Errorf("%s not found in image %s", t.ImagePath, e.Config.GetImageName())
		}
	}

	// 进度完成，固定最后一行
	if e.ProgressDone != nil {
		e.ProgressDone()
	}

	return nil
}

// ========== 安装/升级/卸载 ==========

func (e *Engine) installBinary() error {
	installDir := filepath.Dir(e.Config.BinaryPath)

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("create install directory failed: %w", err)
	}
	if err := os.MkdirAll(e.Config.DataPath, 0755); err != nil {
		return fmt.Errorf("create data path failed: %w", err)
	}

	// 下载到临时目录
	slog.Info("Install", "pull", e.Config.GetImageName())
	if err := e.pullFiles(defaultExtractTargets); err != nil {
		return err
	}

	// 下载完成后停止残余进程（目录被删除但进程未停止的情况）
	e.processStop()

	// 从临时目录复制到安装目录
	tempDir := e.downloadPath()
	slog.Info("Install", "copy", installDir)
	if err := copyFile(filepath.Join(tempDir, "dpanel"), e.Config.BinaryPath, 0755); err != nil {
		return fmt.Errorf("copy binary failed: %w", err)
	}

	// config.yaml：不存在才复制
	configDst := filepath.Join(installDir, "config.yaml")
	if _, err := os.Stat(configDst); os.IsNotExist(err) {
		if err := copyFile(filepath.Join(tempDir, "config.yaml"), configDst, 0644); err != nil {
			return fmt.Errorf("copy config failed: %w", err)
		}
	}

	os.RemoveAll(tempDir)

	if err := e.writeEnv(); err != nil {
		return err
	}

	if err := e.processStart(); err != nil {
		return fmt.Errorf("start binary failed: %w", err)
	}

	return nil
}

func (e *Engine) upgradeBinary() error {
	installDir := filepath.Dir(e.Config.BinaryPath)

	// 先拉取（服务仍在运行，不受影响）
	slog.Info("Upgrade", "pull", e.Config.GetImageName())
	if err := e.pullFiles(defaultExtractTargets); err != nil {
		return err
	}

	// 拉取成功后再停止
	if err := e.processStop(); err != nil {
		return err
	}

	// go-update：从临时目录读取新版本覆盖安装目录的二进制
	tempDir := e.downloadPath()
	stagingBin := filepath.Join(tempDir, "dpanel")
	binFile, err := os.Open(stagingBin)
	if err != nil {
		return fmt.Errorf("open staging binary failed: %w", err)
	}
	defer binFile.Close()

	slog.Info("Upgrade", "apply", e.Config.BinaryPath)
	if err := update.Apply(binFile, update.Options{TargetPath: e.Config.BinaryPath}); err != nil {
		return fmt.Errorf("apply binary update failed: %w", err)
	}
	if err := os.Chmod(e.Config.BinaryPath, 0755); err != nil {
		return fmt.Errorf("chmod binary failed: %w", err)
	}

	// config.yaml：不存在才复制（升级一般已存在，跳过）
	configDst := filepath.Join(installDir, "config.yaml")
	if _, err := os.Stat(configDst); os.IsNotExist(err) {
		copyFile(filepath.Join(tempDir, "config.yaml"), configDst, 0644)
	}

	os.RemoveAll(tempDir)

	if err := e.writeEnv(); err != nil {
		return err
	}

	if err := e.processStart(); err != nil {
		return fmt.Errorf("start binary failed: %w", err)
	}

	return nil
}

func (e *Engine) uninstallBinary() error {
	if err := e.processStop(); err != nil {
		return err
	}

	installPath := e.Config.BinaryPath
	slog.Info("Uninstall", "remove", installPath)
	if err := os.Remove(installPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove binary failed: %w", err)
	}

	// 清理 .env
	_ = os.Remove(e.envPath())

	if e.Config.UninstallRemoveData && e.Config.DataPath != "" {
		slog.Info("Uninstall", "remove_data", e.Config.DataPath)
		if err := os.RemoveAll(e.Config.DataPath); err != nil {
			return fmt.Errorf("remove data path failed: %w", err)
		}
	}

	slog.Info("Uninstall Done")
	return nil
}

// ========== 进程管理 ==========

// writeEnv 合并写入环境变量：
// 1. 读取安装程序目录的 default.env（用户自定义默认值）
// 2. 用 Config 的值覆盖
// 3. 写入安装目录 .env（进程运行时） + 安装程序目录 default.env（用户可编辑）
func (e *Engine) writeEnv() error {
	// 1. 读取 default.env 作为基础
	defaultPath := e.defaultEnvPath()
	env, _ := ReadEnv(defaultPath)
	if env == nil {
		env = make(map[string]string)
	}

	// 2. 用 Config 覆盖安装器管理的 key
	dataPath, _ := filepath.Abs(e.Config.DataPath)
	env["DP_SYSTEM_STORAGE_LOCAL_PATH"] = dataPath
	env["STORAGE_LOCAL_PATH"] = dataPath // 兼容旧版
	env["APP_SERVER_HOST"] = e.Config.ServerHost
	env["APP_SERVER_PORT"] = strconv.Itoa(e.Config.ServerPort)

	if e.Config.HTTPProxy != "" {
		env["HTTP_PROXY"] = e.Config.HTTPProxy
		env["HTTPS_PROXY"] = e.Config.HTTPProxy
	}
	if e.Config.DNS != "" {
		env["DP_DNS"] = e.Config.DNS
	}

	// beta 版自动开启 debug 日志
	if e.Config.Version == types.VersionBE {
		env["DP_LOG_CONSOLE_LEVEL"] = "debug"
		env["DP_LOG_FILE_LEVEL"] = "debug"
	}

	// 3. 写入安装目录 .env
	if err := WriteEnv(e.envPath(), env); err != nil {
		return err
	}

	// 4. 同步写入安装程序目录 default.env（首次生成，后续更新）
	return WriteEnv(defaultPath, env)
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

func (e *Engine) processStart() error {
	installPath, _ := filepath.Abs(e.Config.BinaryPath)
	configYaml := filepath.Join(filepath.Dir(installPath), "config.yaml")

	cmd := exec.Command(installPath, "server:start", "-f", configYaml)
	cmd.SysProcAttr = sysProcAttr()

	cmdEnv, err := buildCmdEnv(e.Config.BinaryPath)
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
	return nil
}

// findProcessesByPath 查找匹配二进制路径的所有进程
func findProcessesByPath(binaryPath string) ([]*process.Process, error) {
	absPath, _ := filepath.Abs(binaryPath)
	all, err := process.Processes()
	if err != nil {
		return nil, err
	}
	var matched []*process.Process
	for _, p := range all {
		// 优先用 Exe 匹配
		if exe, err := p.Exe(); err == nil && exe == absPath {
			matched = append(matched, p)
			continue
		}
		// 兜底：用 Cmdline 匹配
		if cmdline, err := p.Cmdline(); err == nil && strings.HasPrefix(cmdline, absPath+" ") {
			matched = append(matched, p)
		}
	}
	return matched, nil
}

func (e *Engine) processStop() error {
	procs, err := findProcessesByPath(e.Config.BinaryPath)
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
		procs, _ = findProcessesByPath(e.Config.BinaryPath)
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
