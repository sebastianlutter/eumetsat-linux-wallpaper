package wallpaper

import (
	"reflect"
	"testing"
)

func TestResolutionDetectorOrderForHyprland(t *testing.T) {
	session := SessionInfo{WindowManager: "hyprland", SessionType: "wayland"}
	got := detectorNames(resolutionDetectorsFor(session))
	wantPrefix := []string{"hyprctl", "gdbus", "wlr-randr"}
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("detector prefix = %v, want %v", got[:len(wantPrefix)], wantPrefix)
	}
}
