package wallpaper

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Main(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		args = []string{"run"}
	}
	switch args[0] {
	case "run":
		if err := runSubcommand(ctx, args[1:], stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "detect":
		if err := detectSubcommand(ctx, args[1:], stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "install":
		if err := installSubcommand(ctx, args[1:], stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "service":
		if err := serviceSubcommand(ctx, args[1:], stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "help", "-h", "--help":
		printUsage(stdout)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
		printUsage(stderr)
		return 1
	}
	return 0
}

func runSubcommand(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := LoadConfig(extractConfigPath(args, defaultConfigPath()))
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := cfg.ConfigPath
	autoResolution := cfg.AutoResolution
	noAutoResolution := false
	fs.StringVar(&configPath, "config", cfg.ConfigPath, "config file path")
	fs.StringVar(&cfg.ImageDir, "image-dir", cfg.ImageDir, "directory for timestamped wallpapers")
	fs.StringVar(&cfg.CurrentLink, "current-link", cfg.CurrentLink, "stable symlink to the current wallpaper")
	fs.StringVar(&cfg.WindowManager, "window-manager", cfg.WindowManager, "window manager override")
	fs.StringVar(&cfg.SessionType, "session-type", cfg.SessionType, "session type override")
	fs.StringVar(&cfg.WallpaperBackend, "wallpaper-backend", cfg.WallpaperBackend, "wallpaper backend override")
	fs.StringVar(&cfg.WallpaperCommand, "wallpaper-command", cfg.WallpaperCommand, "custom wallpaper command")
	fs.StringVar(&cfg.Resolution, "resolution", cfg.Resolution, "render to WIDTHxHEIGHT")
	fs.BoolVar(&autoResolution, "auto-resolution", cfg.AutoResolution, "auto-detect the current resolution")
	fs.BoolVar(&noAutoResolution, "no-auto-resolution", false, "disable auto resolution detection")
	fs.IntVar(&cfg.OffsetY, "offset-y", cfg.OffsetY, "vertical offset for rendered wallpaper")
	fs.StringVar(&cfg.APIURL, "api-url", cfg.APIURL, "metadata API URL")
	fs.DurationVar(&cfg.HTTPTimeout, "http-timeout", cfg.HTTPTimeout, "HTTP timeout")
	fs.BoolVar(&cfg.InsecureTLS, "insecure-tls", cfg.InsecureTLS, "disable TLS verification")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level: error, info, debug")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.ConfigPath = configPath
	cfg.AutoResolution = autoResolution && !noAutoResolution
	cfg.ImageDir = expandPath(cfg.ImageDir)
	cfg.CurrentLink = expandPath(cfg.CurrentLink)
	if shouldDeriveCurrentLink(fs, cfg.CurrentLink) {
		cfg.CurrentLink = filepath.Join(cfg.ImageDir, "current.png")
	}
	logger := NewLogger(cfg.LogLevel, stdout, stderr)
	session, err := DetectSession(ctx, cfg, logger)
	if err != nil {
		return err
	}
	selected, err := resolveImage(ctx, cfg, session, logger)
	if err != nil {
		return err
	}
	if err := rotateCache(cfg.ImageDir, time.Now(), logger); err != nil {
		return err
	}
	return applyWallpaper(ctx, cfg, session, selected, logger)
}

func detectSubcommand(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := LoadConfig(extractConfigPath(args, defaultConfigPath()))
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.ConfigPath, "config", cfg.ConfigPath, "config file path")
	fs.StringVar(&cfg.WindowManager, "window-manager", cfg.WindowManager, "window manager override")
	fs.StringVar(&cfg.SessionType, "session-type", cfg.SessionType, "session type override")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.ImageDir = expandPath(cfg.ImageDir)
	cfg.CurrentLink = expandPath(cfg.CurrentLink)
	if shouldDeriveCurrentLink(fs, cfg.CurrentLink) {
		cfg.CurrentLink = filepath.Join(cfg.ImageDir, "current.png")
	}
	logger := NewLogger(cfg.LogLevel, stdout, stderr)
	session, err := DetectSession(ctx, cfg, logger)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "window_manager=%s\n", emptyFallback(session.WindowManager, "unknown"))
	fmt.Fprintf(stdout, "session_type=%s\n", emptyFallback(session.SessionType, "unknown"))
	fmt.Fprintf(stdout, "source=%s\n", session.Source)
	fmt.Fprintf(stdout, "desktop=%s\n", emptyFallback(session.DesktopName, "unknown"))
	fmt.Fprintf(stdout, "available_backends=%s\n", strings.Join(session.AvailableBackends, ","))
	toolNames := make([]string, 0, len(session.AvailableTools))
	for name := range session.AvailableTools {
		toolNames = append(toolNames, name)
	}
	sort.Strings(toolNames)
	fmt.Fprintf(stdout, "available_tools=%s\n", strings.Join(toolNames, ","))
	fmt.Fprintf(stdout, "current_link=%s\n", expandPath(cfg.CurrentLink))
	return nil
}

func installSubcommand(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := LoadConfig(extractConfigPath(args, defaultConfigPath()))
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	opts := InstallOptions{
		ConfigPath: cfg.ConfigPath,
		OnCalendar: cfg.OnCalendar,
	}
	fs.StringVar(&opts.ConfigPath, "config", cfg.ConfigPath, "config file path")
	fs.StringVar(&cfg.ImageDir, "image-dir", cfg.ImageDir, "directory for timestamped wallpapers")
	fs.StringVar(&cfg.CurrentLink, "current-link", cfg.CurrentLink, "stable symlink to the current wallpaper")
	fs.StringVar(&opts.BinaryPath, "binary-path", "", "explicit install path for the binary")
	fs.StringVar(&opts.SourceBinary, "source-binary", "", "source binary to install")
	fs.StringVar(&opts.OnCalendar, "on-calendar", cfg.OnCalendar, "systemd OnCalendar expression")
	fs.BoolVar(&opts.ForceSystem, "force-system", false, "install to /usr/local/bin via sudo")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.ImageDir = expandPath(cfg.ImageDir)
	cfg.CurrentLink = expandPath(cfg.CurrentLink)
	if shouldDeriveCurrentLink(fs, cfg.CurrentLink) {
		cfg.CurrentLink = filepath.Join(cfg.ImageDir, "current.png")
	}
	logger := NewLogger(cfg.LogLevel, stdout, stderr)
	return installBinaryAndUnits(ctx, opts, cfg, logger, os.Stdin)
}

func serviceSubcommand(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	logger := NewLogger("info", stdout, stderr)
	return serviceAction(ctx, action, logger)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: eumetsat-wallpaper <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  run      Download/select an image and set the wallpaper")
	fmt.Fprintln(w, "  detect   Print detected session, tools, and backend candidates")
	fmt.Fprintln(w, "  install  Install the binary, config, and user systemd units")
	fmt.Fprintln(w, "  service  Run helper actions: status, list, start, logs")
}

func shouldDeriveCurrentLink(fs *flag.FlagSet, currentLink string) bool {
	if flagWasSet(fs, "current-link") {
		return strings.TrimSpace(currentLink) == ""
	}
	defaultLink := DefaultConfig().CurrentLink
	return strings.TrimSpace(currentLink) == "" || currentLink == defaultLink
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}
