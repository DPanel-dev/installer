package core

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
	"github.com/joho/godotenv"
)

// ReadEnv 读取 .env 文件
func ReadEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env, err := godotenv.Parse(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse .env failed: %w", err)
	}
	return env, nil
}

// WriteEnv 写入 .env 文件
func WriteEnv(path string, env map[string]string) error {
	content, err := godotenv.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal env failed: %w", err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0644); err != nil {
		return fmt.Errorf("write .env failed: %w", err)
	}
	return nil
}

// copyFile 复制文件，保持权限
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// detectRegistry 自动检测可用的镜像仓库，返回最优 registry
func detectRegistry() string {
	slog.Info("Registry", "detecting", "...")

	hubLatency := config.TestRegistryLatency(types.RegistryDockerHub)
	aliLatency := config.TestRegistryLatency(types.RegistryAliYun)

	registry := types.RegistryUnavailable
	if aliLatency > 0 {
		registry = types.RegistryAliYun
	}
	if hubLatency > 0 && (aliLatency <= 0 || hubLatency <= aliLatency) {
		registry = types.RegistryDockerHub
	}

	slog.Info("Registry", "selected", registry, "hub_ms", hubLatency, "aliyun_ms", aliLatency)
	return registry
}

// envPath 返回安装目录下的 .env 路径
func envPath(cfg *config.Config) string {
	return filepath.Join(filepath.Dir(cfg.BinaryPath), ".env")
}

// defaultEnvPath 返回安装程序目录下的 default.env 路径
func defaultEnvPath() string {
	execPath, _ := os.Executable()
	return filepath.Join(filepath.Dir(execPath), "default.env")
}

// writeEnv 合并写入环境变量到 .env 文件
func writeEnv(cfg *config.Config) error {
	// 读取 default.env 作为基础
	dPath := defaultEnvPath()
	env, _ := ReadEnv(dPath)
	if env == nil {
		env = make(map[string]string)
	}

	// 用 Config 覆盖安装器管理的 key
	// 二进制模式：实际数据目录 = DataPath/data/，容器模式：DataPath 本身就是挂载目录
	storagePath := cfg.DataPath
	if cfg.InstallType == types.InstallTypeBinary {
		storagePath = filepath.Join(cfg.DataPath, "data")
	}
	absStoragePath, _ := filepath.Abs(storagePath)
	env["DP_SYSTEM_STORAGE_LOCAL_PATH"] = absStoragePath
	env["STORAGE_LOCAL_PATH"] = absStoragePath // 兼容旧版
	env["APP_SERVER_HOST"] = cfg.ServerHost
	env["APP_SERVER_PORT"] = strconv.Itoa(cfg.ServerPort)

	if cfg.HTTPProxy != "" {
		env["HTTP_PROXY"] = cfg.HTTPProxy
		env["HTTPS_PROXY"] = cfg.HTTPProxy
	}
	if cfg.DNS != "" {
		env["DP_DNS"] = cfg.DNS
	}

	// beta 版自动开启 debug 日志
	if cfg.Version == types.VersionBE {
		env["DP_LOG_CONSOLE_LEVEL"] = "debug"
		env["DP_LOG_FILE_LEVEL"] = "debug"
	}

	// 写入安装目录 .env
	if err := WriteEnv(envPath(cfg), env); err != nil {
		return err
	}

	// 同步写入安装程序目录 default.env
	return WriteEnv(dPath, env)
}

// buildInstallEnv 构建安装器管理的环境变量
func buildInstallEnv(cfg *config.Config) map[string]string {
	storagePath := cfg.DataPath
	if cfg.InstallType == types.InstallTypeBinary {
		storagePath = filepath.Join(cfg.DataPath, "data")
	}
	absStoragePath, _ := filepath.Abs(storagePath)
	env := map[string]string{
		"APP_NAME":                     cfg.Name,
		"DP_SYSTEM_STORAGE_LOCAL_PATH": absStoragePath,
		"STORAGE_LOCAL_PATH":           absStoragePath,
		"APP_SERVER_HOST":              cfg.ServerHost,
		"APP_SERVER_PORT":              strconv.Itoa(cfg.ServerPort),
	}
	if cfg.HTTPProxy != "" {
		env["HTTP_PROXY"] = cfg.HTTPProxy
		env["HTTPS_PROXY"] = cfg.HTTPProxy
	}
	if cfg.DNS != "" {
		env["DP_DNS"] = cfg.DNS
	}
	if cfg.Version == types.VersionBE {
		env["DP_LOG_CONSOLE_LEVEL"] = "debug"
		env["DP_LOG_FILE_LEVEL"] = "debug"
	}
	return env
}
