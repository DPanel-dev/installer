package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// extractTarget 定义从 OCI 镜像提取文件的规则
type extractTarget struct {
	ImagePath string      // OCI 镜像内路径，如 "/app/server/dpanel"
	Name      string      // 本地文件名，如 "dpanel"、"config.yaml"
	Mode      os.FileMode // 文件权限，如 0755、0644
	Action    func(tmpPath, finalPath string, mode os.FileMode) error
}

// overwriteAction 覆盖：chmod + rename -new → final
func overwriteAction(tmpPath, finalPath string, mode os.FileMode) error {
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, finalPath)
}

// skipIfExistsAction 跳过：不存在则 rename，存在则保留 -new
func skipIfExistsAction(tmpPath, finalPath string, mode os.FileMode) error {
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		return os.Rename(tmpPath, finalPath)
	}
	return nil
}

// keepNewAction 只 chmod，保留 -new 给调用方处理
func keepNewAction(tmpPath, _ string, mode os.FileMode) error {
	return os.Chmod(tmpPath, mode)
}

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
