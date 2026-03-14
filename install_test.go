package wallpaper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChooseBinaryDestinationOnlyUsesUserPathEntries(t *testing.T) {
	home := t.TempDir()
	xdgBin := filepath.Join(home, ".local", "bin")
	userBin := filepath.Join(home, "bin")
	if err := os.MkdirAll(xdgBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("XDG_BIN_HOME", xdgBin)
	t.Setenv("PATH", userBin+string(os.PathListSeparator)+"/usr/local/bin"+string(os.PathListSeparator)+"/usr/bin")

	destination, err := chooseBinaryDestination()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(userBin, "eumetsat-wallpaper")
	if destination != want {
		t.Fatalf("destination = %q, want %q", destination, want)
	}
}

func TestChooseBinaryDestinationFailsWhenNoUserPathEntryExists(t *testing.T) {
	home := t.TempDir()
	xdgBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(xdgBin, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("XDG_BIN_HOME", xdgBin)
	t.Setenv("PATH", "/usr/local/bin"+string(os.PathListSeparator)+"/usr/bin")

	destination, err := chooseBinaryDestination()
	if err == nil {
		t.Fatalf("expected an error, got destination %q", destination)
	}
}

func TestReadInstalledUnitInfoExtractsBinaryAndConfigPath(t *testing.T) {
	servicePath := filepath.Join(t.TempDir(), defaultServiceName)
	binaryPath := "/tmp/eumetsat bin/eumetsat-wallpaper"
	configPath := "/tmp/eumetsat config/eumetsat-wallpaper.conf"
	content := renderServiceUnit(binaryPath, configPath)
	if err := os.WriteFile(servicePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := readInstalledUnitInfo(servicePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.BinaryPath != binaryPath {
		t.Fatalf("BinaryPath = %q, want %q", info.BinaryPath, binaryPath)
	}
	if info.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", info.ConfigPath, configPath)
	}
}
