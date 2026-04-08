package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
