package wallpaper

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIURL         = "https://meteosat-url.appspot.com/msg"
	defaultServiceName    = "eumetsat-wallpaper.service"
	defaultTimerName      = "eumetsat-wallpaper.timer"
	defaultOnCalendar     = "*:0/15"
	defaultConfigFileName = "eumetsat-wallpaper.conf"
)

type Config struct {
	ConfigPath       string
	ImageDir         string
	CurrentLink      string
	WindowManager    string
	SessionType      string
	WallpaperBackend string
	WallpaperCommand string
	Resolution       string
	AutoResolution   bool
	OffsetY          int
	APIURL           string
	HTTPTimeout      time.Duration
	InsecureTLS      bool
	LogLevel         string
	OnCalendar       string
}

func DefaultConfig() Config {
	home := userHomeDir()
	imageDir := filepath.Join(home, "Bilder", "eumetsat-wallpaper")
	return Config{
		ConfigPath:       defaultConfigPath(),
		ImageDir:         imageDir,
		CurrentLink:      filepath.Join(imageDir, "current.png"),
		WallpaperBackend: "auto",
		AutoResolution:   true,
		APIURL:           defaultAPIURL,
		HTTPTimeout:      30 * time.Second,
		LogLevel:         "info",
		OnCalendar:       defaultOnCalendar,
	}
}

func defaultConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(userHomeDir(), ".config")
	}
	return filepath.Join(configDir, defaultConfigFileName)
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return home
	}
	if home = os.Getenv("HOME"); home != "" {
		return home
	}
	return "."
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = cfg.ConfigPath
	}
	cfg.ConfigPath = path
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	sawImageDir := false
	sawCurrentLink := false
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return cfg, fmt.Errorf("%s:%d: invalid config line", path, lineNo)
		}
		key = strings.ToUpper(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		value = trimMatchingQuotes(value)
		if key == "IMAGE_DIR" {
			sawImageDir = true
		}
		if key == "CURRENT_LINK" {
			sawCurrentLink = true
		}
		if err := applyConfigValue(&cfg, key, value); err != nil {
			return cfg, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}
	if sawImageDir && !sawCurrentLink {
		cfg.CurrentLink = filepath.Join(cfg.ImageDir, "current.png")
	}
	return cfg, nil
}

func trimMatchingQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func applyConfigValue(cfg *Config, key, value string) error {
	switch key {
	case "IMAGE_DIR":
		cfg.ImageDir = expandPath(value)
	case "CURRENT_LINK":
		cfg.CurrentLink = expandPath(value)
	case "WINDOW_MANAGER":
		cfg.WindowManager = normalizeWindowManager(value)
	case "SESSION_TYPE":
		cfg.SessionType = normalizeSessionType(value)
	case "WALLPAPER_BACKEND":
		cfg.WallpaperBackend = normalizeBackend(value)
	case "WALLPAPER_COMMAND":
		cfg.WallpaperCommand = value
	case "RESOLUTION":
		cfg.Resolution = value
	case "AUTO_RESOLUTION":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("AUTO_RESOLUTION: %w", err)
		}
		cfg.AutoResolution = parsed
	case "OFFSET_Y":
		offset, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("OFFSET_Y: %w", err)
		}
		cfg.OffsetY = offset
	case "API_URL":
		cfg.APIURL = value
	case "HTTP_TIMEOUT":
		d, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("HTTP_TIMEOUT: %w", err)
		}
		cfg.HTTPTimeout = d
	case "INSECURE_TLS":
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("INSECURE_TLS: %w", err)
		}
		cfg.InsecureTLS = parsed
	case "LOG_LEVEL":
		cfg.LogLevel = normalizeLogLevel(value)
	case "ON_CALENDAR":
		cfg.OnCalendar = value
	default:
	}
	return nil
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		return userHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(userHomeDir(), path[2:])
	}
	return path
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", value)
	}
}

func parseDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("empty duration")
	}
	if digitsOnly(value) {
		seconds, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		return time.Duration(seconds) * time.Second, nil
	}
	return time.ParseDuration(value)
}

func digitsOnly(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func WriteDefaultConfig(path string, cfg Config) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := cfg.RenderConfigFile()
	return os.WriteFile(path, []byte(content), 0o644)
}

func (cfg Config) RenderConfigFile() string {
	return strings.TrimSpace(fmt.Sprintf(`
# EUMETSAT wallpaper configuration
IMAGE_DIR=%s
CURRENT_LINK=%s
WINDOW_MANAGER=%s
SESSION_TYPE=%s
WALLPAPER_BACKEND=%s
WALLPAPER_COMMAND=%s
RESOLUTION=%s
AUTO_RESOLUTION=%t
OFFSET_Y=%d
API_URL=%s
HTTP_TIMEOUT=%s
INSECURE_TLS=%t
LOG_LEVEL=%s
ON_CALENDAR=%s
`, cfg.ImageDir, cfg.CurrentLink, cfg.WindowManager, cfg.SessionType, cfg.WallpaperBackend, cfg.WallpaperCommand, cfg.Resolution, cfg.AutoResolution, cfg.OffsetY, cfg.APIURL, cfg.HTTPTimeout, cfg.InsecureTLS, cfg.LogLevel, cfg.OnCalendar)) + "\n"
}

func normalizeWindowManager(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, "hypr"):
		return "hyprland"
	case strings.Contains(value, "sway"):
		return "sway"
	case strings.Contains(value, "gnome"):
		return "gnome"
	case strings.Contains(value, "i3"):
		return "i3"
	default:
		return value
	}
}

func normalizeSessionType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "wayland", "x11":
		return value
	default:
		return value
	}
}

func normalizeBackend(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "auto", "custom", "gnome", "hyprpaper", "swww", "swaymsg", "swaybg", "feh", "xwallpaper", "nitrogen", "none":
		return value
	default:
		return value
	}
}

func normalizeLogLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return "debug"
	case "error":
		return "error"
	default:
		return "info"
	}
}
