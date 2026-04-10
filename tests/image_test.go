package tests

import (
	"testing"

	"github.com/dpanel-dev/installer/internal/config"
	"github.com/dpanel-dev/installer/internal/types"
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
