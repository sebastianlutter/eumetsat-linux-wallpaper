package wallpaper

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type commandResult struct {
	Stdout string
	Stderr string
}

func runCommand(ctx context.Context, logger *Logger, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergedEnvSlice(env)
	if logger != nil {
		logger.Infof("running command: %s", quoteCommand(name, args...))
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if logger != nil {
			logger.Errorf("%s failed: %v", quoteCommand(name, args...), err)
			if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
				logger.Errorf("%s", trimmed)
			}
		}
		return fmt.Errorf("%s: %w", quoteCommand(name, args...), err)
	}
	if logger != nil && len(bytes.TrimSpace(output)) > 0 {
		logger.Debugf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func commandOutput(ctx context.Context, env map[string]string, name string, args ...string) (commandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergedEnvSlice(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, err
}

func mergedEnvSlice(env map[string]string) []string {
	current := envMap(os.Environ())
	for key, value := range env {
		current[key] = value
	}
	keys := make([]string, 0, len(current))
	for key := range current {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	output := make([]string, 0, len(keys))
	for _, key := range keys {
		output = append(output, key+"="+current[key])
	}
	return output
}

func envMap(items []string) map[string]string {
	result := make(map[string]string, len(items))
	for _, item := range items {
		key, value, found := strings.Cut(item, "=")
		if found {
			result[key] = value
		}
	}
	return result
}

func mergeMaps(base map[string]string, overlays ...map[string]string) map[string]string {
	result := make(map[string]string, len(base))
	for key, value := range base {
		result[key] = value
	}
	for _, overlay := range overlays {
		for key, value := range overlay {
			if value == "" {
				continue
			}
			result[key] = value
		}
	}
	return result
}

func quoteCommand(name string, args ...string) string {
	all := make([]string, 0, len(args)+1)
	all = append(all, name)
	all = append(all, args...)
	quoted := make([]string, 0, len(all))
	for _, item := range all {
		quoted = append(quoted, shellQuote(item))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$`!&;()<>|*?[]{}") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func resolvePath(path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func parseTimestamp(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
}

func parseImageTimestampFromName(name string) (time.Time, bool) {
	base := filepath.Base(name)
	if !strings.HasPrefix(base, "earth_") || !strings.HasSuffix(base, ".png") {
		return time.Time{}, false
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(base, "earth_"), ".png")
	parsed, err := time.ParseInLocation("2006-01-02_15-04-05", stamp, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func extractConfigPath(args []string, fallback string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--config" && i+1 < len(args) {
			return expandPath(args[i+1])
		}
		if strings.HasPrefix(arg, "--config=") {
			return expandPath(strings.TrimPrefix(arg, "--config="))
		}
	}
	return fallback
}

func isTTYFile(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func parsePositiveInt(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be positive: %d", n)
	}
	return n, nil
}
