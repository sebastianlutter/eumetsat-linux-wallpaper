package wallpaper

import "testing"

func TestResolveSessionPrefersHyprlandProcessOverStaleDesktopVars(t *testing.T) {
	cfg := DefaultConfig()
	inputs := DetectionInputs{
		BaseEnv: map[string]string{
			"XDG_CURRENT_DESKTOP": "Hyprland",
			"DESKTOP_SESSION":     "gnome",
			"WAYLAND_DISPLAY":     "wayland-1",
			"DISPLAY":             ":0",
		},
		UserManagerEnv: map[string]string{
			"HYPRLAND_INSTANCE_SIGNATURE": "sig",
			"WAYLAND_DISPLAY":             "wayland-1",
		},
		Processes: []ProcessInfo{
			{PID: 10, Comm: "gnome-shell", Env: map[string]string{"DISPLAY": ":0"}},
			{PID: 11, Comm: "Hyprland", Env: map[string]string{"HYPRLAND_INSTANCE_SIGNATURE": "sig", "WAYLAND_DISPLAY": "wayland-1"}},
		},
	}

	session := resolveSession(cfg, inputs)
	if session.WindowManager != "hyprland" {
		t.Fatalf("WindowManager = %q, want hyprland", session.WindowManager)
	}
	if session.SessionType != "wayland" {
		t.Fatalf("SessionType = %q, want wayland", session.SessionType)
	}
	if session.Source != "process" && session.Source != "environment" {
		t.Fatalf("Source = %q, want process/environment", session.Source)
	}
	if session.Environment["HYPRLAND_INSTANCE_SIGNATURE"] != "sig" {
		t.Fatalf("missing HYPRLAND_INSTANCE_SIGNATURE in merged environment")
	}
}

func TestDetectAvailableBackendsForHyprland(t *testing.T) {
	session := SessionInfo{
		WindowManager: "hyprland",
		SessionType:   "wayland",
	}
	processes := []ProcessInfo{{Comm: "hyprpaper"}}
	tools := map[string]string{
		"hyprctl":     "/usr/bin/hyprctl",
		"swaybg":      "/usr/bin/swaybg",
		"swww":        "/usr/bin/swww",
		"swww-daemon": "/usr/bin/swww-daemon",
	}
	backends := detectAvailableBackends(session, processes, tools)
	expected := []string{"hyprpaper", "swww", "swaybg"}
	for _, want := range expected {
		if !containsString(backends, want) {
			t.Fatalf("backends %v missing %q", backends, want)
		}
	}
}
