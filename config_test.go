package wallpaper

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigDerivesCurrentLinkFromImageDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "eumetsat-wallpaper.conf")
	content := "IMAGE_DIR=~/Pictures/wallpapers\nAUTO_RESOLUTION=false\nHTTP_TIMEOUT=45\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	expectedDir := filepath.Join(home, "Pictures", "wallpapers")
	if cfg.ImageDir != expectedDir {
		t.Fatalf("ImageDir = %q, want %q", cfg.ImageDir, expectedDir)
	}
	if cfg.CurrentLink != filepath.Join(expectedDir, "current.png") {
		t.Fatalf("CurrentLink = %q", cfg.CurrentLink)
	}
	if cfg.AutoResolution {
		t.Fatalf("AutoResolution = true, want false")
	}
	if cfg.HTTPTimeout != 45*time.Second {
		t.Fatalf("HTTPTimeout = %v, want 45s", cfg.HTTPTimeout)
	}
}

func TestApplyConfigValueDurationSyntax(t *testing.T) {
	cfg := DefaultConfig()
	if err := applyConfigValue(&cfg, "HTTP_TIMEOUT", "1m30s"); err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPTimeout != 90*time.Second {
		t.Fatalf("HTTPTimeout = %v, want 90s", cfg.HTTPTimeout)
	}
}
