package wallpaper

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func resolveTargetResolution(ctx context.Context, cfg Config, session SessionInfo, logger *Logger) (int, int, error) {
	if cfg.Resolution != "" {
		return parseResolution(cfg.Resolution)
	}
	if !cfg.AutoResolution {
		return 0, 0, fmt.Errorf("auto resolution disabled")
	}
	candidates := resolutionDetectorsFor(session)
	logger.Debugf("resolution detectors: %s", strings.Join(detectorNames(candidates), ", "))
	var lastErr error
	for _, detector := range candidates {
		width, height, err := detector.Run(ctx, session.Environment)
		if err == nil {
			logger.Infof("detected resolution via %s: %dx%d", detector.Name, width, height)
			return width, height, nil
		}
		lastErr = err
		logger.Debugf("resolution detector %s failed: %v", detector.Name, err)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no detector candidates")
	}
	return 0, 0, lastErr
}

func parseResolution(value string) (int, int, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid resolution %q", value)
	}
	width, err := parsePositiveInt(parts[0])
	if err != nil {
		return 0, 0, err
	}
	height, err := parsePositiveInt(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

type resolutionDetector struct {
	Name string
	Run  func(context.Context, map[string]string) (int, int, error)
}

func resolutionDetectorsFor(session SessionInfo) []resolutionDetector {
	var detectors []resolutionDetector
	switch session.WindowManager {
	case "hyprland":
		detectors = append(detectors, detectorHyprctl(), detectorGDBus(), detectorWLRRandr())
	case "sway":
		detectors = append(detectors, detectorSwaymsg(), detectorWLRRandr())
	case "gnome":
		detectors = append(detectors, detectorGDBus())
	}
	if session.SessionType == "wayland" {
		detectors = append(detectors, detectorHyprctl(), detectorSwaymsg(), detectorWLRRandr(), detectorGDBus())
	}
	detectors = append(detectors, detectorXRandr(), detectorXDpyinfo(), detectorXWinInfo())
	return uniqueDetectors(detectors)
}

func uniqueDetectors(items []resolutionDetector) []resolutionDetector {
	seen := make(map[string]bool, len(items))
	var result []resolutionDetector
	for _, item := range items {
		if item.Name == "" || seen[item.Name] {
			continue
		}
		seen[item.Name] = true
		result = append(result, item)
	}
	return result
}

func detectorNames(items []resolutionDetector) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func detectorHyprctl() resolutionDetector {
	return resolutionDetector{
		Name: "hyprctl",
		Run: func(ctx context.Context, env map[string]string) (int, int, error) {
			result, err := commandOutput(ctx, env, "hyprctl", "monitors", "-j")
			if err != nil {
				return 0, 0, err
			}
			var outputs []struct {
				Name    string `json:"name"`
				Focused bool   `json:"focused"`
				Width   int    `json:"width"`
				Height  int    `json:"height"`
			}
			if err := json.Unmarshal([]byte(result.Stdout), &outputs); err != nil {
				return 0, 0, err
			}
			if len(outputs) == 0 {
				return 0, 0, fmt.Errorf("no monitors reported")
			}
			best := outputs[0]
			for _, output := range outputs[1:] {
				if output.Focused || output.Width*output.Height > best.Width*best.Height {
					best = output
				}
			}
			return best.Width, best.Height, nil
		},
	}
}

func detectorSwaymsg() resolutionDetector {
	return resolutionDetector{
		Name: "swaymsg",
		Run: func(ctx context.Context, env map[string]string) (int, int, error) {
			result, err := commandOutput(ctx, env, "swaymsg", "-t", "get_outputs", "-r")
			if err != nil {
				return 0, 0, err
			}
			var outputs []struct {
				Active      bool `json:"active"`
				Focused     bool `json:"focused"`
				CurrentMode struct {
					Width  int `json:"width"`
					Height int `json:"height"`
				} `json:"current_mode"`
			}
			if err := json.Unmarshal([]byte(result.Stdout), &outputs); err != nil {
				return 0, 0, err
			}
			var best struct {
				Focused bool
				Width   int
				Height  int
			}
			found := false
			for _, output := range outputs {
				if !output.Active || output.CurrentMode.Width == 0 || output.CurrentMode.Height == 0 {
					continue
				}
				if !found || output.Focused || output.CurrentMode.Width*output.CurrentMode.Height > best.Width*best.Height {
					best = struct {
						Focused bool
						Width   int
						Height  int
					}{
						Focused: output.Focused,
						Width:   output.CurrentMode.Width,
						Height:  output.CurrentMode.Height,
					}
					found = true
				}
			}
			if !found {
				return 0, 0, fmt.Errorf("no active outputs")
			}
			return best.Width, best.Height, nil
		},
	}
}

func detectorGDBus() resolutionDetector {
	return resolutionDetector{
		Name: "gdbus",
		Run: func(ctx context.Context, env map[string]string) (int, int, error) {
			result, err := commandOutput(ctx, env, "gdbus", "call", "--session", "--dest", "org.gnome.Mutter.DisplayConfig", "--object-path", "/org/gnome/Mutter/DisplayConfig", "--method", "org.gnome.Mutter.DisplayConfig.GetCurrentState")
			if err != nil {
				return 0, 0, err
			}
			re := regexp.MustCompile(`(\d{3,5})x(\d{3,5})`)
			matches := re.FindAllStringSubmatch(result.Stdout, -1)
			if len(matches) == 0 {
				return 0, 0, fmt.Errorf("no display config sizes found")
			}
			bestArea := 0
			bestW, bestH := 0, 0
			for _, match := range matches {
				width, _ := strconv.Atoi(match[1])
				height, _ := strconv.Atoi(match[2])
				if width*height > bestArea {
					bestArea = width * height
					bestW, bestH = width, height
				}
			}
			return bestW, bestH, nil
		},
	}
}

func detectorXRandr() resolutionDetector {
	return resolutionDetector{
		Name: "xrandr",
		Run: func(ctx context.Context, env map[string]string) (int, int, error) {
			result, err := commandOutput(ctx, env, "xrandr", "--current")
			if err != nil {
				return 0, 0, err
			}
			re := regexp.MustCompile(`^\s+(\d+)x(\d+)\s+.*\*`)
			for _, line := range strings.Split(result.Stdout, "\n") {
				match := re.FindStringSubmatch(line)
				if len(match) == 3 {
					width, _ := strconv.Atoi(match[1])
					height, _ := strconv.Atoi(match[2])
					return width, height, nil
				}
			}
			return 0, 0, fmt.Errorf("no active xrandr mode found")
		},
	}
}

func detectorXDpyinfo() resolutionDetector {
	return resolutionDetector{
		Name: "xdpyinfo",
		Run: func(ctx context.Context, env map[string]string) (int, int, error) {
			result, err := commandOutput(ctx, env, "xdpyinfo")
			if err != nil {
				return 0, 0, err
			}
			re := regexp.MustCompile(`dimensions:\s+(\d+)x(\d+)`)
			match := re.FindStringSubmatch(result.Stdout)
			if len(match) != 3 {
				return 0, 0, fmt.Errorf("no xdpyinfo dimensions found")
			}
			width, _ := strconv.Atoi(match[1])
			height, _ := strconv.Atoi(match[2])
			return width, height, nil
		},
	}
}

func detectorXWinInfo() resolutionDetector {
	return resolutionDetector{
		Name: "xwininfo",
		Run: func(ctx context.Context, env map[string]string) (int, int, error) {
			result, err := commandOutput(ctx, env, "xwininfo", "-root")
			if err != nil {
				return 0, 0, err
			}
			widthMatch := regexp.MustCompile(`Width:\s+(\d+)`).FindStringSubmatch(result.Stdout)
			heightMatch := regexp.MustCompile(`Height:\s+(\d+)`).FindStringSubmatch(result.Stdout)
			if len(widthMatch) != 2 || len(heightMatch) != 2 {
				return 0, 0, fmt.Errorf("no xwininfo dimensions found")
			}
			width, _ := strconv.Atoi(widthMatch[1])
			height, _ := strconv.Atoi(heightMatch[1])
			return width, height, nil
		},
	}
}

func detectorWLRRandr() resolutionDetector {
	return resolutionDetector{
		Name: "wlr-randr",
		Run: func(ctx context.Context, env map[string]string) (int, int, error) {
			result, err := commandOutput(ctx, env, "wlr-randr")
			if err != nil {
				return 0, 0, err
			}
			re := regexp.MustCompile(`(\d+)x(\d+)`)
			bestArea := 0
			bestW, bestH := 0, 0
			for _, line := range strings.Split(result.Stdout, "\n") {
				if !(strings.Contains(line, "current") || strings.Contains(line, "*")) {
					continue
				}
				match := re.FindStringSubmatch(line)
				if len(match) != 3 {
					continue
				}
				width, _ := strconv.Atoi(match[1])
				height, _ := strconv.Atoi(match[2])
				if width*height > bestArea {
					bestArea = width * height
					bestW, bestH = width, height
				}
			}
			if bestArea == 0 {
				return 0, 0, fmt.Errorf("no current wlr-randr mode found")
			}
			return bestW, bestH, nil
		},
	}
}
