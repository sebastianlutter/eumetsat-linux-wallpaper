package wallpaper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type installedUnitInfo struct {
	BinaryPath string
	ConfigPath string
}

func uninstallBinaryAndUnits(ctx context.Context, opts UninstallOptions, cfg Config, logger *Logger, stdin io.Reader) error {
	servicePath := filepath.Join(userSystemdDir(), defaultServiceName)
	installed, err := readInstalledUnitInfo(servicePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if strings.TrimSpace(opts.ConfigPath) == "" && installed.ConfigPath != "" {
		cfg.ConfigPath = installed.ConfigPath
		loaded, loadErr := LoadConfig(installed.ConfigPath)
		if loadErr == nil {
			cfg = loaded
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			return loadErr
		}
	} else if strings.TrimSpace(opts.ConfigPath) != "" {
		cfg.ConfigPath = expandPath(opts.ConfigPath)
		loaded, loadErr := LoadConfig(cfg.ConfigPath)
		if loadErr == nil {
			cfg = loaded
		} else if !errors.Is(loadErr, os.ErrNotExist) {
			return loadErr
		}
	}

	binaryPath := strings.TrimSpace(opts.BinaryPath)
	if binaryPath == "" {
		binaryPath = installed.BinaryPath
	}
	if binaryPath == "" {
		binaryPath = findInstalledBinaryPath()
	}
	binaryPath = expandPath(binaryPath)

	stopInstalledUnits(ctx, logger)

	for _, path := range []string{
		filepath.Join(userSystemdDir(), defaultServiceName),
		filepath.Join(userSystemdDir(), defaultTimerName),
	} {
		if err := removePath(path, logger, stdin); err != nil {
			return err
		}
	}

	runCommandBestEffort(ctx, logger, nil, "systemctl", "--user", "daemon-reload")
	runCommandBestEffort(ctx, logger, nil, "systemctl", "--user", "reset-failed", defaultServiceName, defaultTimerName)

	if binaryPath != "" {
		if err := removePath(binaryPath, logger, stdin); err != nil {
			return err
		}
		logger.Infof("removed binary: %s", binaryPath)
	} else {
		logger.Infof("no installed binary path found")
	}

	if opts.Purge {
		if err := purgeUserData(cfg, logger, stdin); err != nil {
			return err
		}
	}

	logger.Infof("uninstall completed")
	return nil
}

func readInstalledUnitInfo(servicePath string) (installedUnitInfo, error) {
	content, err := os.ReadFile(servicePath)
	if err != nil {
		return installedUnitInfo{}, err
	}
	var info installedUnitInfo
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ExecStart=") {
			continue
		}
		fields, err := splitQuotedFields(strings.TrimPrefix(line, "ExecStart="))
		if err != nil {
			return installedUnitInfo{}, err
		}
		if len(fields) == 0 {
			continue
		}
		info.BinaryPath = fields[0]
		for i := 1; i < len(fields); i++ {
			if fields[i] == "--config" && i+1 < len(fields) {
				info.ConfigPath = fields[i+1]
				break
			}
			if strings.HasPrefix(fields[i], "--config=") {
				info.ConfigPath = strings.TrimPrefix(fields[i], "--config=")
				break
			}
		}
		return info, nil
	}
	return info, fmt.Errorf("no ExecStart found in %s", servicePath)
}

func splitQuotedFields(value string) ([]string, error) {
	var fields []string
	var current strings.Builder
	var quote byte
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		fields = append(fields, current.String())
		current.Reset()
	}

	for i := 0; i < len(value); i++ {
		ch := value[i]
		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		switch ch {
		case '\\':
			if quote == '\'' {
				current.WriteByte(ch)
				continue
			}
			escaped = true
		case '"', '\'':
			if quote == 0 {
				quote = ch
				continue
			}
			if quote == ch {
				quote = 0
				continue
			}
			current.WriteByte(ch)
		case ' ', '\t':
			if quote != 0 {
				current.WriteByte(ch)
				continue
			}
			flush()
		default:
			current.WriteByte(ch)
		}
	}
	if escaped {
		current.WriteByte('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in %q", value)
	}
	flush()
	return fields, nil
}

func stopInstalledUnits(ctx context.Context, logger *Logger) {
	runCommandBestEffort(ctx, logger, nil, "systemctl", "--user", "disable", "--now", defaultTimerName)
	runCommandBestEffort(ctx, logger, nil, "systemctl", "--user", "stop", defaultServiceName)
	runCommandBestEffort(ctx, logger, nil, "systemctl", "--user", "stop", swaybgUnit)
	runCommandBestEffort(ctx, logger, nil, "systemctl", "--user", "stop", swwwUnit)
}

func runCommandBestEffort(ctx context.Context, logger *Logger, env map[string]string, name string, args ...string) {
	if logger != nil {
		logger.Infof("running command: %s", quoteCommand(name, args...))
	}
	result, err := commandOutput(ctx, env, name, args...)
	if err != nil {
		if logger != nil {
			output := strings.TrimSpace(strings.TrimSpace(result.Stdout) + "\n" + strings.TrimSpace(result.Stderr))
			if output != "" {
				logger.Debugf("%s", output)
			}
		}
		return
	}
	if logger != nil && strings.TrimSpace(result.Stdout) != "" {
		logger.Debugf("%s", strings.TrimSpace(result.Stdout))
	}
}

func removePath(path string, logger *Logger, stdin io.Reader) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	path = expandPath(path)
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		prompted, sudoErr := maybeRemoveWithSudo(path, logger, stdin)
		if sudoErr != nil {
			return sudoErr
		}
		if prompted {
			return nil
		}
	}
	return err
}

func purgeUserData(cfg Config, logger *Logger, stdin io.Reader) error {
	for _, path := range []string{cfg.ConfigPath, cfg.ImageDir} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		path = expandPath(path)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
					prompted, sudoErr := maybeRemoveWithSudo(path, logger, stdin)
					if sudoErr != nil {
						return sudoErr
					}
					if prompted {
						logger.Infof("removed data directory: %s", path)
						continue
					}
				}
				return err
			}
			logger.Infof("removed data directory: %s", path)
			continue
		}
		if err := removePath(path, logger, stdin); err != nil {
			return err
		}
		logger.Infof("removed file: %s", path)
	}
	return nil
}

func maybeRemoveWithSudo(target string, logger *Logger, stdin io.Reader) (bool, error) {
	if !isTTYFile(os.Stdout) || !isTTYFile(os.Stdin) {
		return false, nil
	}
	reader := bufio.NewReader(stdin)
	fmt.Fprintf(os.Stdout, "Removing %s requires sudo. Continue? [y/N] ", target)
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return false, nil
	}
	logger.Infof("removing with sudo: %s", target)
	cmd := exec.Command("sudo", "rm", "-rf", target)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return true, cmd.Run()
}

func findInstalledBinaryPath() string {
	for _, candidate := range installedBinaryCandidates() {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		return candidate
	}
	return ""
}

func installedBinaryCandidates() []string {
	home := userHomeDir()
	var candidates []string
	if value := os.Getenv("XDG_BIN_HOME"); value != "" {
		candidates = append(candidates, filepath.Join(expandPath(value), "eumetsat-wallpaper"))
	}
	for _, dir := range pathEntries(os.Getenv("PATH")) {
		if strings.HasPrefix(dir, home) {
			candidates = append(candidates, filepath.Join(dir, "eumetsat-wallpaper"))
		}
	}
	if path, err := exec.LookPath("systemd-path"); err == nil {
		_ = path
		if result, err := commandOutput(context.Background(), nil, "systemd-path", "user-binaries"); err == nil {
			dir := strings.TrimSpace(result.Stdout)
			if dir != "" {
				candidates = append(candidates, filepath.Join(dir, "eumetsat-wallpaper"))
			}
		}
	}
	candidates = append(candidates,
		filepath.Join(home, ".local", "bin", "eumetsat-wallpaper"),
		"/usr/local/bin/eumetsat-wallpaper",
	)
	return uniqueStrings(candidates)
}
