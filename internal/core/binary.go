package core

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	pathpkg "path"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	update "github.com/inconshreveable/go-update"
)

func (e *Engine) backupBinary() error {
	if !e.Config.UpgradeBackup {
		return nil
	}

	installPath := e.binaryInstallPath()
	if _, err := os.Stat(installPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat installed binary failed: %w", err)
	}

	backupPath := fmt.Sprintf("%s.bak.%s", installPath, time.Now().Format("20060102150405"))
	if err := os.MkdirAll(filepath.Dir(backupPath), 0755); err != nil {
		return fmt.Errorf("create backup directory failed: %w", err)
	}
	if err := copyFile(installPath, backupPath, 0755); err != nil {
		return fmt.Errorf("backup binary failed: %w", err)
	}

	slog.Info("Binary backup created", "source", installPath, "backup", backupPath)
	return nil
}

func (e *Engine) installBinary() error {
	if e.Config.OS == "windows" {
		return fmt.Errorf("binary install does not support windows")
	}

	if err := os.MkdirAll(e.Config.DataPath, 0755); err != nil {
		return fmt.Errorf("create data path failed: %w", err)
	}

	installPath := e.binaryInstallPath()
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("create binary directory failed: %w", err)
	}

	slog.Info("Pulling binary from OCI image", "image", e.Config.GetImageName(), "path", installPath)

	tmpFile, err := os.CreateTemp(filepath.Dir(installPath), ".dpanel-install-*")
	if err != nil {
		return fmt.Errorf("create temp file failed: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	mode, err := e.pullBinary(tmpFile)
	if err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file failed: %w", err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("chmod temp file failed: %w", err)
	}
	if err := os.Rename(tmpPath, installPath); err != nil {
		return fmt.Errorf("install binary failed: %w", err)
	}
	if err := os.Chmod(installPath, mode); err != nil {
		return fmt.Errorf("chmod installed binary failed: %w", err)
	}

	slog.Info("Binary installed", "path", installPath)
	return nil
}

func (e *Engine) upgradeBinary() error {
	if e.Config.OS == "windows" {
		return fmt.Errorf("binary upgrade does not support windows")
	}

	installPath := e.binaryInstallPath()

	tmpFile, err := os.CreateTemp(filepath.Dir(installPath), ".dpanel-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp file failed: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	mode, err := e.pullBinary(tmpFile)
	if err != nil {
		return err
	}
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek temp file failed: %w", err)
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("chmod temp file failed: %w", err)
	}
	if err := update.Apply(tmpFile, update.Options{TargetPath: installPath}); err != nil {
		return fmt.Errorf("apply binary update failed: %w", err)
	}
	if err := os.Chmod(installPath, mode); err != nil {
		return fmt.Errorf("chmod updated binary failed: %w", err)
	}

	slog.Info("Binary upgraded", "path", installPath)
	return nil
}

func (e *Engine) uninstallBinary() error {
	installPath := e.binaryInstallPath()
	if err := os.Remove(installPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove binary failed: %w", err)
	}

	if e.Config.UninstallRemoveData && e.Config.DataPath != "" {
		if err := os.RemoveAll(e.Config.DataPath); err != nil {
			return fmt.Errorf("remove data path failed: %w", err)
		}
	}

	return nil
}

func (e *Engine) binaryInstallPath() string {
	if e.Config.OS == "windows" {
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		return filepath.Join(programFiles, "DPanel", "dpanel.exe")
	}
	return "/usr/local/bin/dpanel"
}

func (e *Engine) pullBinary(dst *os.File) (os.FileMode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ref, err := name.ParseReference(
		e.Config.GetImageName(),
		name.WithDefaultRegistry("index.docker.io"),
		name.WithDefaultTag("latest"),
	)
	if err != nil {
		return 0, fmt.Errorf("parse image reference failed: %w", err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		return 0, fmt.Errorf("pull image failed: %w", err)
	}

	fs := mutate.Extract(img)
	defer fs.Close()

	reader := tar.NewReader(fs)
	targetPath := "/app/server/dpanel"
	bestMode := os.FileMode(0755)

	for {
		header, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, fmt.Errorf("read image filesystem failed: %w", err)
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}

		if pathpkg.Clean("/"+header.Name) != targetPath {
			continue
		}

		if err := dst.Truncate(0); err != nil {
			return 0, fmt.Errorf("truncate temp file failed: %w", err)
		}
		if _, err := dst.Seek(0, io.SeekStart); err != nil {
			return 0, fmt.Errorf("seek temp file failed: %w", err)
		}
		if _, err := io.Copy(dst, reader); err != nil {
			return 0, fmt.Errorf("extract binary failed: %w", err)
		}

		if header.Mode&0111 != 0 {
			bestMode = os.FileMode(header.Mode & 0777)
		}
		if _, err := dst.Seek(0, io.SeekStart); err != nil {
			return 0, fmt.Errorf("seek temp file failed: %w", err)
		}
		return bestMode, nil
	}

	return 0, fmt.Errorf("binary %s not found in image %s", targetPath, e.Config.GetImageName())
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
