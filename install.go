package wallpaper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type InstallOptions struct {
	ConfigPath   string
	BinaryPath   string
	SourceBinary string
	OnCalendar   string
	ForceSystem  bool
}

type UninstallOptions struct {
	ConfigPath string
	BinaryPath string
	Purge      bool
}

func installBinaryAndUnits(ctx context.Context, opts InstallOptions, cfg Config, logger *Logger, stdin io.Reader) error {
	if opts.SourceBinary == "" {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		opts.SourceBinary = executable
	}
	destination := opts.BinaryPath
	installedWithSudo := false
	if destination == "" && opts.ForceSystem {
		destination = "/usr/local/bin/eumetsat-wallpaper"
	}
	if destination == "" {
		path, err := chooseBinaryDestination()
		if err == nil {
			destination = path
		}
	}
	if destination == "" {
		if prompted, err := maybeInstallWithSudo(opts.SourceBinary, logger, stdin); err != nil {
			return err
		} else if prompted {
			destination = "/usr/local/bin/eumetsat-wallpaper"
			installedWithSudo = true
		} else {
			return fmt.Errorf("no writable user binary directory found on PATH; use --binary-path or rerun with --force-system")
		}
	}
	if !installedWithSudo {
		if err := copyBinary(opts.SourceBinary, destination); err != nil {
			if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "permission denied") {
				prompted, sudoErr := maybeInstallWithSudo(opts.SourceBinary, logger, stdin)
				if sudoErr != nil {
					return sudoErr
				}
				if prompted {
					destination = "/usr/local/bin/eumetsat-wallpaper"
				} else {
					return err
				}
			} else {
				return err
			}
		}
	}

	cfg.ConfigPath = opts.ConfigPath
	cfg.OnCalendar = opts.OnCalendar
	if err := WriteDefaultConfig(cfg.ConfigPath, cfg); err != nil {
		return err
	}
	if err := installUserUnits(destination, cfg); err != nil {
		return err
	}
	if err := restartInstalledUnits(ctx, logger); err != nil {
		return fmt.Errorf("installed files but failed to restart user units: %w", err)
	}
	logger.Infof("installed binary: %s", destination)
	logger.Infof("installed config: %s", cfg.ConfigPath)
	logger.Infof("reloaded and started units: %s, %s", defaultServiceName, defaultTimerName)
	logger.Infof("enabled timer: %s (%s)", defaultTimerName, cfg.OnCalendar)
	return nil
}

func chooseBinaryDestination() (string, error) {
	uid := os.Getuid()
	candidates := candidateBinaryDirs(os.Getenv("PATH"))
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if err := ensureDir(dir); err != nil {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if ok && int(stat.Uid) != uid {
			continue
		}
		if info.Mode().Perm()&0o200 == 0 {
			continue
		}
		return filepath.Join(dir, "eumetsat-wallpaper"), nil
	}
	return "", fmt.Errorf("no writable user binary directory found on PATH")
}

func candidateBinaryDirs(pathEnv string) []string {
	home := userHomeDir()
	var candidates []string
	if value := os.Getenv("XDG_BIN_HOME"); value != "" {
		dir := expandPath(value)
		if pathContainsDir(pathEnv, dir) {
			candidates = append(candidates, dir)
		}
	}
	for _, entry := range pathEntries(pathEnv) {
		entry = strings.TrimSpace(entry)
		if entry == "" || !filepath.IsAbs(entry) {
			continue
		}
		if !strings.HasPrefix(entry, home) {
			continue
		}
		candidates = append(candidates, entry)
	}
	if path, err := exec.LookPath("systemd-path"); err == nil {
		_ = path
		if result, err := commandOutput(context.Background(), nil, "systemd-path", "user-binaries"); err == nil {
			dir := strings.TrimSpace(result.Stdout)
			if pathContainsDir(pathEnv, dir) {
				candidates = append(candidates, dir)
			}
		}
	}
	localBin := filepath.Join(home, ".local", "bin")
	if pathContainsDir(pathEnv, localBin) {
		candidates = append(candidates, localBin)
	}
	return uniqueStrings(candidates)
}

func pathEntries(pathEnv string) []string {
	entries := filepath.SplitList(pathEnv)
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		result = append(result, filepath.Clean(expandPath(entry)))
	}
	return uniqueStrings(result)
}

func pathContainsDir(pathEnv, dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	target := filepath.Clean(expandPath(dir))
	for _, entry := range pathEntries(pathEnv) {
		if entry == target {
			return true
		}
	}
	return false
}

func copyBinary(sourcePath, destinationPath string) error {
	if err := ensureDir(filepath.Dir(destinationPath)); err != nil {
		return err
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	tmpPath := destinationPath + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmpFile, sourceFile); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, destinationPath)
}

func maybeInstallWithSudo(sourceBinary string, logger *Logger, stdin io.Reader) (bool, error) {
	if !isTTYFile(os.Stdout) || !isTTYFile(os.Stdin) {
		return false, nil
	}
	reader := bufio.NewReader(stdin)
	fmt.Fprint(os.Stdout, "No writable user bin directory on PATH was found. Install to /usr/local/bin with sudo? [y/N] ")
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		return false, nil
	}
	logger.Infof("installing with sudo to /usr/local/bin/eumetsat-wallpaper")
	cmd := exec.Command("sudo", "install", "-m", "0755", sourceBinary, "/usr/local/bin/eumetsat-wallpaper")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return true, cmd.Run()
}

func installUserUnits(binaryPath string, cfg Config) error {
	userDir := userSystemdDir()
	if err := ensureDir(userDir); err != nil {
		return err
	}
	servicePath := filepath.Join(userDir, defaultServiceName)
	timerPath := filepath.Join(userDir, defaultTimerName)
	if err := os.WriteFile(servicePath, []byte(renderServiceUnit(binaryPath, cfg.ConfigPath)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(timerPath, []byte(renderTimerUnit(cfg.OnCalendar)), 0o644); err != nil {
		return err
	}
	return nil
}

func restartInstalledUnits(ctx context.Context, logger *Logger) error {
	stopInstalledUnits(ctx, logger)
	if err := runCommand(ctx, logger, nil, "systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	runCommandBestEffort(ctx, logger, nil, "systemctl", "--user", "reset-failed", defaultServiceName, defaultTimerName, swaybgUnit, swwwUnit)
	if err := runCommand(ctx, logger, nil, "systemctl", "--user", "enable", "--now", defaultTimerName); err != nil {
		return err
	}
	logger.Infof("starting immediate wallpaper refresh via %s", defaultServiceName)
	if err := runCommand(ctx, logger, nil, "systemctl", "--user", "start", defaultServiceName); err != nil {
		return err
	}
	logger.Infof("immediate wallpaper refresh finished")
	return nil
}

func renderServiceUnit(binaryPath, configPath string) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=EUMETSAT Linux wallpaper updater
After=graphical-session.target
Wants=graphical-session.target

[Service]
Type=oneshot
ExecStart=%s run --config %s
WorkingDirectory=%%h
Environment=PATH=%%h/.local/bin:%%h/bin:/usr/local/bin:/usr/bin

[Install]
WantedBy=default.target
`, systemdQuote(resolvePath(binaryPath)), systemdQuote(resolvePath(configPath)))) + "\n"
}

func renderTimerUnit(onCalendar string) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=Run EUMETSAT Linux wallpaper updater on schedule

[Timer]
OnCalendar=%s
Persistent=true
Unit=%s

[Install]
WantedBy=timers.target
`, onCalendar, defaultServiceName)) + "\n"
}

func userSystemdDir() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(userHomeDir(), ".config")
	}
	return filepath.Join(configDir, "systemd", "user")
}

func serviceAction(ctx context.Context, action string, logger *Logger) error {
	switch action {
	case "status":
		if err := runCommand(ctx, logger, nil, "systemctl", "--user", "status", defaultTimerName); err != nil {
			return err
		}
		return runCommand(ctx, logger, nil, "systemctl", "--user", "status", defaultServiceName)
	case "list":
		return runCommand(ctx, logger, nil, "systemctl", "--user", "list-timers", "--all", defaultTimerName)
	case "start":
		return runCommand(ctx, logger, nil, "systemctl", "--user", "start", "--no-block", defaultServiceName)
	case "logs":
		return runCommand(ctx, logger, nil, "journalctl", "--user", "-u", defaultServiceName, "-n", "50", "--no-pager")
	default:
		return fmt.Errorf("unknown service action %q", action)
	}
}

func systemdQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
