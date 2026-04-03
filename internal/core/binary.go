package core

import (
	"archive/tar"
	"bufio"
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
)

// 默认提取规则
var defaultExtractTargets = []extractTarget{
	{ImagePath: "/app/server/dpanel", Name: "dpanel", Mode: 0755, Action: overwriteAction},
	{ImagePath: "/app/server/config.yaml", Name: "config.yaml", Mode: 0644, Action: skipIfExistsAction},
}

// pullFiles 从 OCI 镜像提取文件，统一先写到 {Name}-new，再调 Action 处理
func (e *Engine) pullFiles(targets []extractTarget) error {
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

	dir := filepath.Dir(e.Config.BinaryPath)

	// 构建 imagePath → target 索引
	targetMap := make(map[string]*extractTarget, len(targets))
	for i := range targets {
		targetMap[pathpkg.Clean("/"+targets[i].ImagePath)] = &targets[i]
	}

	extracted := make(map[string]bool, len(targets))

	reader := tar.NewReader(fs)
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

		// 提取到 {Name}-new
		tmpPath := filepath.Join(dir, t.Name+"-new")
		tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("create temp file %s failed: %w", tmpPath, err)
		}
		if _, err := io.Copy(tmpFile, reader); err != nil {
			tmpFile.Close()
			return fmt.Errorf("extract %s failed: %w", t.Name, err)
		}
		tmpFile.Close()

		// 调用 Action 处理
		finalPath := filepath.Join(dir, t.Name)
		if t.Action != nil {
			if err := t.Action(tmpPath, finalPath, t.Mode); err != nil {
				return fmt.Errorf("process %s failed: %w", t.Name, err)
			}
		}

		extracted[cleanPath] = true
		if len(extracted) == len(targets) {
			break
		}
	}

	// 检查是否所有目标都找到
	for _, t := range targets {
		key := pathpkg.Clean("/" + t.ImagePath)
		if _, ok := extracted[key]; !ok {
			return fmt.Errorf("%s not found in image %s", t.ImagePath, e.Config.GetImageName())
		}
	}

	return nil
}

// ========== 安装/升级/卸载 ==========

func (e *Engine) installBinary() error {
	if err := e.processStop(); err != nil {
		return err
	}

	if err := os.MkdirAll(e.Config.DataPath, 0755); err != nil {
		return fmt.Errorf("create data path failed: %w", err)
	}

	installPath := e.Config.BinaryPath
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("create binary directory failed: %w", err)
	}

	slog.Info("Pulling files from OCI image", "image", e.Config.GetImageName(), "path", installPath)

	if err := e.pullFiles(defaultExtractTargets); err != nil {
		return err
	}

	slog.Info("Binary installed", "path", installPath)

	// 写 .env（processStart 会更新 PID）
	if err := e.writeEnv(); err != nil {
		return err
	}

	if err := e.processStart(); err != nil {
		return fmt.Errorf("start binary failed: %w", err)
	}

	return nil
}

func (e *Engine) upgradeBinary() error {
	if err := e.processStop(); err != nil {
		return err
	}

	installPath := e.Config.BinaryPath
	dir := filepath.Dir(installPath)

	slog.Info("Pulling files from OCI image", "image", e.Config.GetImageName(), "path", installPath)

	// 升级时 binary 用 keepNewAction 保留 -new，config 复用默认逻辑
	targets := []extractTarget{
		{ImagePath: "/app/server/dpanel", Name: "dpanel", Mode: 0755, Action: keepNewAction},
		{ImagePath: "/app/server/config.yaml", Name: "config.yaml", Mode: 0644, Action: skipIfExistsAction},
	}

	if err := e.pullFiles(targets); err != nil {
		return err
	}

	// go-update 从 dpanel-new 读取
	binNewPath := filepath.Join(dir, "dpanel-new")
	binNewFile, err := os.Open(binNewPath)
	if err != nil {
		return fmt.Errorf("open %s failed: %w", binNewPath, err)
	}
	defer binNewFile.Close()

	if err := update.Apply(binNewFile, update.Options{TargetPath: installPath}); err != nil {
		return fmt.Errorf("apply binary update failed: %w", err)
	}
	_ = os.Remove(binNewPath)
	if err := os.Chmod(installPath, 0755); err != nil {
		return fmt.Errorf("chmod updated binary failed: %w", err)
	}

	// 更新 .env 并重启
	if err := e.writeEnv(); err != nil {
		return err
	}

	slog.Info("Binary upgraded", "path", installPath)

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
	if err := os.Remove(installPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove binary failed: %w", err)
	}

	// 清理 .env
	_ = os.Remove(filepath.Join(filepath.Dir(installPath), ".env"))

	if e.Config.UninstallRemoveData && e.Config.DataPath != "" {
		if err := os.RemoveAll(e.Config.DataPath); err != nil {
			return fmt.Errorf("remove data path failed: %w", err)
		}
	}

	slog.Info("Binary uninstalled", "path", installPath)
	return nil
}

// ========== 进程管理 ==========

// writeEnv 从 config 构造环境变量并写入 .env
func (e *Engine) writeEnv() error {
	dataPath, _ := filepath.Abs(e.Config.DataPath)
	env := map[string]string{
		"STORAGE_LOCAL_PATH": dataPath,
		"APP_SERVER_PORT":    strconv.Itoa(e.Config.Port),
	}
	if e.Config.HTTPProxy != "" {
		env["HTTP_PROXY"] = e.Config.HTTPProxy
		env["HTTPS_PROXY"] = e.Config.HTTPProxy
	}
	if e.Config.DNS != "" {
		env["DP_DNS"] = e.Config.DNS
	}
	return WriteEnv(filepath.Join(filepath.Dir(e.Config.BinaryPath), ".env"), env)
}

// buildCmdEnv 从 .env 读取环境变量，构造子进程环境
func buildCmdEnv(binaryPath string) ([]string, error) {
	env, err := ReadEnv(filepath.Join(filepath.Dir(binaryPath), ".env"))
	if err != nil {
		return nil, err
	}
	result := os.Environ()
	for k, v := range env {
		if k == "PID" {
			continue
		}
		result = append(result, k+"="+v)
	}
	return result, nil
}

func (e *Engine) processStart() error {
	installPath, _ := filepath.Abs(e.Config.BinaryPath)
	configYaml := filepath.Join(filepath.Dir(installPath), "config.yaml")

	cmd := exec.Command(installPath, "server:start", "-f", configYaml)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 从 .env 读取环境变量
	cmdEnv, err := buildCmdEnv(e.Config.BinaryPath)
	if err != nil {
		return fmt.Errorf("read env failed: %w", err)
	}
	cmd.Env = cmdEnv

	slog.Info("Starting binary", "cmd", cmd.String(), "path", installPath)

	// 通过 pipe 捕获输出并用 slog 记录
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe failed: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start process failed: %w", err)
	}

	// 后台读取输出并通过 slog 记录
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			slog.Info("dpanel", "pid", cmd.Process.Pid, "stream", "stdout", "msg", scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.Info("dpanel", "pid", cmd.Process.Pid, "stream", "stderr", "msg", scanner.Text())
		}
	}()

	// 等待 1 秒检查进程是否存活
	time.Sleep(1 * time.Second)
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		exitErr := cmd.Wait()
		return fmt.Errorf("process exited immediately: %v", exitErr)
	}

	// 写 PID 到 .env
	envPath := filepath.Join(filepath.Dir(e.Config.BinaryPath), ".env")
	pidEnv, err := ReadEnv(envPath)
	if err != nil {
		cmd.Process.Signal(syscall.SIGTERM)
		return fmt.Errorf("read .env failed: %w", err)
	}
	pidEnv["PID"] = strconv.Itoa(cmd.Process.Pid)
	if err := WriteEnv(envPath, pidEnv); err != nil {
		cmd.Process.Signal(syscall.SIGTERM)
		return fmt.Errorf("write pid to .env failed: %w", err)
	}

	slog.Info("Binary started", "pid", cmd.Process.Pid)
	return nil
}

func (e *Engine) processStop() error {
	envPath := filepath.Join(filepath.Dir(e.Config.BinaryPath), ".env")
	env, err := ReadEnv(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read .env failed: %w", err)
	}

	pidStr, ok := env["PID"]
	if !ok || pidStr == "" {
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
	if err != nil {
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	// 检查进程是否存活
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		slog.Info("Stale PID cleaned", "pid", pid)
		return nil
	}

	// 进程存活，发送 SIGTERM
	slog.Info("Stopping running process", "pid", pid)
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to process %d failed: %w", pid, err)
	}

	// 等待进程退出（最多 10 秒）
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 检查是否仍在运行
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		slog.Warn("Process did not exit, sending SIGKILL", "pid", pid)
		proc.Signal(syscall.SIGKILL)
	}

	// 清除 .env 中的 PID
	delete(env, "PID")
	_ = WriteEnv(envPath, env)

	slog.Info("Process stopped", "pid", pid)
	return nil
}
