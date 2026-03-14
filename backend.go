package wallpaper

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	swaybgUnit = "eumetsat-wallpaper-swaybg"
	swwwUnit   = "eumetsat-wallpaper-swww"
)

func applyWallpaper(ctx context.Context, cfg Config, session SessionInfo, image SelectedImage, logger *Logger) error {
	backend, err := selectWallpaperBackend(cfg, session)
	if err != nil {
		if normalizeBackend(cfg.WallpaperBackend) == "auto" && session.WindowManager == "" && session.SessionType == "" {
			logger.Infof("no active graphical session detected; wallpaper update skipped")
			return nil
		}
		return err
	}
	if backend == "none" {
		logger.Infof("wallpaper application disabled")
		return nil
	}
	switch backend {
	case "custom":
		return runCustomWallpaperCommand(ctx, cfg, session, image, logger)
	case "gnome":
		uri := "file://" + resolvePath(image.WallpaperPath)
		if err := runCommand(ctx, logger, session.Environment, "gsettings", "set", "org.gnome.desktop.background", "picture-uri", uri); err != nil {
			return err
		}
		return runCommand(ctx, logger, session.Environment, "gsettings", "set", "org.gnome.desktop.background", "picture-uri-dark", uri)
	case "hyprpaper":
		return setHyprpaperWallpaper(ctx, session, image, logger)
	case "swww":
		if err := ensureManagedCommand(ctx, logger, session.Environment, swwwUnit, "swww-daemon"); err != nil {
			return err
		}
		return runCommand(ctx, logger, session.Environment, "swww", "img", resolvePath(image.WallpaperPath), "--resize", "fit", "--transition-type", "none")
	case "swaymsg":
		return runCommand(ctx, logger, session.Environment, "swaymsg", "output", "*", "bg", resolvePath(image.WallpaperPath), "fill")
	case "swaybg":
		if _, hasPkill := session.AvailableTools["pkill"]; hasPkill {
			_ = runCommand(ctx, logger, session.Environment, "pkill", "swaybg")
		}
		return ensureManagedCommand(ctx, logger, session.Environment, swaybgUnit, "swaybg", "-m", "fill", "-i", resolvePath(image.WallpaperPath))
	case "feh":
		return runCommand(ctx, logger, session.Environment, "feh", "--bg-fill", resolvePath(image.WallpaperPath))
	case "xwallpaper":
		return runCommand(ctx, logger, session.Environment, "xwallpaper", "--zoom", resolvePath(image.WallpaperPath))
	case "nitrogen":
		return runCommand(ctx, logger, session.Environment, "nitrogen", "--set-zoom-fill", "--save", resolvePath(image.WallpaperPath))
	default:
		return fmt.Errorf("unsupported wallpaper backend %q", backend)
	}
}

func selectWallpaperBackend(cfg Config, session SessionInfo) (string, error) {
	override := normalizeBackend(cfg.WallpaperBackend)
	if override == "" {
		override = "auto"
	}
	if override == "custom" {
		if strings.TrimSpace(cfg.WallpaperCommand) == "" {
			return "", fmt.Errorf("WALLPAPER_COMMAND is required for custom backend")
		}
		return "custom", nil
	}
	if override != "auto" {
		if override == "none" {
			return "none", nil
		}
		if !containsString(session.AvailableBackends, override) && session.AvailableTools[override] == "" && override != "swaybg" && override != "swww" && override != "hyprpaper" {
			return "", fmt.Errorf("requested backend %q is not available", override)
		}
		return override, nil
	}
	if session.WindowManager == "gnome" && session.AvailableTools["gsettings"] != "" {
		return "gnome", nil
	}
	if session.WindowManager == "hyprland" {
		for _, candidate := range []string{"hyprpaper", "swww", "swaybg"} {
			if containsString(session.AvailableBackends, candidate) {
				return candidate, nil
			}
		}
	}
	if session.WindowManager == "sway" {
		for _, candidate := range []string{"swaymsg", "swaybg"} {
			if containsString(session.AvailableBackends, candidate) {
				return candidate, nil
			}
		}
	}
	if session.WindowManager == "i3" || session.SessionType == "x11" {
		for _, candidate := range []string{"feh", "xwallpaper", "nitrogen"} {
			if containsString(session.AvailableBackends, candidate) {
				return candidate, nil
			}
		}
	}
	if strings.TrimSpace(cfg.WallpaperCommand) != "" {
		return "custom", nil
	}
	return "", fmt.Errorf("no supported wallpaper backend found for wm=%s session=%s", session.WindowManager, session.SessionType)
}

func setHyprpaperWallpaper(ctx context.Context, session SessionInfo, image SelectedImage, logger *Logger) error {
	result, err := commandOutput(ctx, session.Environment, "hyprctl", "monitors", "-j")
	if err != nil {
		return err
	}
	var monitors []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &monitors); err != nil {
		return err
	}
	if len(monitors) == 0 {
		return fmt.Errorf("hyprctl reported no monitors")
	}
	for _, monitor := range monitors {
		if err := runCommand(ctx, logger, session.Environment, "hyprctl", "hyprpaper", "wallpaper", fmt.Sprintf("%s,%s,fill", monitor.Name, resolvePath(image.WallpaperPath))); err != nil {
			return err
		}
	}
	return nil
}

func runCustomWallpaperCommand(ctx context.Context, cfg Config, session SessionInfo, image SelectedImage, logger *Logger) error {
	command := expandWallpaperCommand(cfg.WallpaperCommand, image)
	return runCommand(ctx, logger, session.Environment, "/bin/sh", "-c", command)
}

func expandWallpaperCommand(template string, image SelectedImage) string {
	command := strings.ReplaceAll(template, "{image}", shellQuote(resolvePath(image.SourcePath)))
	command = strings.ReplaceAll(command, "{current}", shellQuote(resolvePath(image.WallpaperPath)))
	return command
}

func ensureManagedCommand(ctx context.Context, logger *Logger, env map[string]string, unit string, name string, args ...string) error {
	if _, ok := detectTools()["systemd-run"]; ok {
		if _, hasSystemctl := detectTools()["systemctl"]; hasSystemctl {
			_ = runCommand(ctx, logger, env, "systemctl", "--user", "stop", unit)
		}
		systemdArgs := []string{"--user", "--unit", unit, "--collect", "--property=Type=simple", name}
		systemdArgs = append(systemdArgs, args...)
		return runCommand(ctx, logger, env, "systemd-run", systemdArgs...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergedEnvSlice(env)
	if logger != nil {
		logger.Infof("running command: %s", quoteCommand(name, args...))
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func selectedWallpaperPath(image SelectedImage) string {
	return filepath.Clean(resolvePath(image.WallpaperPath))
}
