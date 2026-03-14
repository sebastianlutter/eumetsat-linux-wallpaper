package wallpaper

import "testing"

func TestSelectWallpaperBackendAuto(t *testing.T) {
	cfg := DefaultConfig()
	session := SessionInfo{
		WindowManager:     "sway",
		SessionType:       "wayland",
		AvailableBackends: []string{"swaymsg", "swaybg"},
	}
	backend, err := selectWallpaperBackend(cfg, session)
	if err != nil {
		t.Fatal(err)
	}
	if backend != "swaymsg" {
		t.Fatalf("backend = %q, want swaymsg", backend)
	}
}

func TestExpandWallpaperCommand(t *testing.T) {
	command := expandWallpaperCommand("setter --image {image} --current {current}", SelectedImage{
		SourcePath:    "/tmp/some path/image.png",
		WallpaperPath: "/tmp/current.png",
	})
	want := "setter --image '/tmp/some path/image.png' --current /tmp/current.png"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
}
