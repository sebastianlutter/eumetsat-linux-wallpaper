package wallpaper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type ProcessInfo struct {
	PID  int
	Comm string
	Args string
	Env  map[string]string
}

type LoginctlSession struct {
	ID      string
	Name    string
	Type    string
	Desktop string
	Active  bool
	State   string
	Class   string
	Leader  int
}

type DetectionInputs struct {
	BaseEnv        map[string]string
	UserManagerEnv map[string]string
	LoginSessions  []LoginctlSession
	Processes      []ProcessInfo
	Tools          map[string]string
}

type SessionInfo struct {
	WindowManager     string
	SessionType       string
	Source            string
	DesktopName       string
	Environment       map[string]string
	AvailableTools    map[string]string
	AvailableBackends []string
	ProcessNames      []string
}

func DetectSession(ctx context.Context, cfg Config, logger *Logger) (SessionInfo, error) {
	baseEnv := envMap(os.Environ())
	inputs := DetectionInputs{
		BaseEnv:        baseEnv,
		UserManagerEnv: readSystemdUserEnvironment(ctx, logger),
		LoginSessions:  readLoginctlSessions(ctx, logger),
		Processes:      readProcessList(ctx, logger),
		Tools:          detectTools(),
	}
	session := resolveSession(cfg, inputs)
	session.AvailableTools = inputs.Tools
	session.AvailableBackends = detectAvailableBackends(session, inputs.Processes, inputs.Tools)
	session.ProcessNames = uniqueProcessNames(inputs.Processes)
	logger.Infof("detected session: wm=%s session=%s source=%s", emptyFallback(session.WindowManager, "unknown"), emptyFallback(session.SessionType, "unknown"), emptyFallback(session.Source, "fallback"))
	logger.Debugf("detected process list: %s", strings.Join(session.ProcessNames, ", "))
	return session, nil
}

func resolveSession(cfg Config, inputs DetectionInputs) SessionInfo {
	session := SessionInfo{
		Environment: mergeMaps(inputs.BaseEnv, inputs.UserManagerEnv),
	}

	if cfg.WindowManager != "" {
		session.WindowManager = normalizeWindowManager(cfg.WindowManager)
		session.Source = "config"
	}
	if cfg.SessionType != "" {
		session.SessionType = normalizeSessionType(cfg.SessionType)
		if session.Source == "" {
			session.Source = "config"
		}
	}

	activeLogin := pickActiveLoginctlSession(inputs.LoginSessions)
	if activeLogin.ID != "" {
		if session.WindowManager == "" {
			session.WindowManager = normalizeWindowManager(strings.TrimSpace(nonEmpty(activeLogin.Desktop, activeLogin.Name)))
			if session.WindowManager != "" {
				session.Source = "loginctl"
			}
		}
		if session.SessionType == "" {
			session.SessionType = normalizeSessionType(activeLogin.Type)
		}
		if activeLogin.Leader > 0 {
			if env, err := readProcEnviron(activeLogin.Leader); err == nil {
				session.Environment = mergeMaps(session.Environment, env)
			}
		}
	}

	matchedProcess := pickWMProcess(inputs.Processes, session.Environment)
	if matchedProcess.PID > 0 {
		session.Environment = mergeMaps(session.Environment, matchedProcess.Env)
		if session.WindowManager == "" {
			session.WindowManager = processToWindowManager(matchedProcess.Comm, matchedProcess.Args)
			if session.WindowManager != "" {
				session.Source = "process"
			}
		}
		if session.SessionType == "" {
			session.SessionType = defaultSessionTypeForWM(session.WindowManager)
		}
	}

	if session.WindowManager == "" {
		session.WindowManager = inferWindowManagerFromEnv(session.Environment)
		if session.WindowManager != "" {
			session.Source = "environment"
		}
	}
	if session.SessionType == "" {
		session.SessionType = inferSessionType(session.Environment, session.WindowManager)
		if session.SessionType != "" && session.Source == "" {
			session.Source = "environment"
		}
	}

	session.DesktopName = nonEmpty(
		session.Environment["XDG_CURRENT_DESKTOP"],
		session.Environment["XDG_SESSION_DESKTOP"],
		session.Environment["DESKTOP_SESSION"],
		session.WindowManager,
	)
	if session.Source == "" {
		session.Source = "fallback"
	}
	return session
}

func readSystemdUserEnvironment(ctx context.Context, logger *Logger) map[string]string {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	result, err := commandOutput(ctx, nil, "systemctl", "--user", "show-environment")
	if err != nil {
		if logger != nil {
			logger.Debugf("systemctl --user show-environment unavailable: %v", err)
		}
		return nil
	}
	return parseKeyValueLines(result.Stdout)
}

func readLoginctlSessions(ctx context.Context, logger *Logger) []LoginctlSession {
	if _, err := exec.LookPath("loginctl"); err != nil {
		return nil
	}
	list, err := commandOutput(ctx, nil, "loginctl", "list-sessions", "--no-legend")
	if err != nil {
		if logger != nil {
			logger.Debugf("loginctl list-sessions unavailable: %v", err)
		}
		return nil
	}
	userName := os.Getenv("USER")
	var sessions []LoginctlSession
	for _, line := range strings.Split(list.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		id := fields[0]
		show, err := commandOutput(ctx, nil, "loginctl", "show-session", id, "-p", "Name", "-p", "Type", "-p", "Desktop", "-p", "Active", "-p", "State", "-p", "Class", "-p", "Leader")
		if err != nil {
			continue
		}
		values := parseKeyValueLines(show.Stdout)
		if name := values["Name"]; userName != "" && name != "" && name != userName {
			continue
		}
		leader, _ := strconv.Atoi(values["Leader"])
		sessions = append(sessions, LoginctlSession{
			ID:      id,
			Name:    values["Name"],
			Type:    normalizeSessionType(values["Type"]),
			Desktop: normalizeWindowManager(values["Desktop"]),
			Active:  strings.EqualFold(values["Active"], "yes"),
			State:   strings.ToLower(values["State"]),
			Class:   strings.ToLower(values["Class"]),
			Leader:  leader,
		})
	}
	return sessions
}

func readProcessList(ctx context.Context, logger *Logger) []ProcessInfo {
	psPath, err := exec.LookPath("ps")
	if err != nil {
		return nil
	}
	uid := strconv.Itoa(os.Getuid())
	result, err := commandOutput(ctx, nil, psPath, "-u", uid, "-o", "pid=,comm=,args=")
	if err != nil {
		if logger != nil {
			logger.Debugf("process scan unavailable: %v", err)
		}
		return nil
	}
	var processes []ProcessInfo
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		comm := fields[1]
		args := strings.Join(fields[2:], " ")
		env, _ := readProcEnviron(pid)
		processes = append(processes, ProcessInfo{
			PID:  pid,
			Comm: comm,
			Args: args,
			Env:  env,
		})
	}
	return processes
}

func readProcEnviron(pid int) (map[string]string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, err
	}
	items := strings.Split(string(data), "\x00")
	return envMap(items), nil
}

func detectTools() map[string]string {
	tools := []string{
		"feh",
		"gdbus",
		"gsettings",
		"hyprctl",
		"hyprpaper",
		"journalctl",
		"loginctl",
		"nitrogen",
		"pkill",
		"swww",
		"swww-daemon",
		"swaybg",
		"swaymsg",
		"systemctl",
		"systemd-path",
		"systemd-run",
		"xdpyinfo",
		"xrandr",
		"xwallpaper",
		"xwininfo",
		"wlr-randr",
	}
	result := make(map[string]string, len(tools))
	for _, tool := range tools {
		if path, err := exec.LookPath(tool); err == nil {
			result[tool] = path
		}
	}
	return result
}

func parseKeyValueLines(raw string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		result[key] = value
	}
	return result
}

func pickActiveLoginctlSession(sessions []LoginctlSession) LoginctlSession {
	for _, session := range sessions {
		if session.Active && session.Class == "user" {
			return session
		}
	}
	for _, session := range sessions {
		if session.Class == "user" && (session.State == "active" || session.State == "online") {
			return session
		}
	}
	return LoginctlSession{}
}

func pickWMProcess(processes []ProcessInfo, env map[string]string) ProcessInfo {
	preferred := normalizeWindowManager(nonEmpty(
		env["XDG_CURRENT_DESKTOP"],
		env["XDG_SESSION_DESKTOP"],
		env["DESKTOP_SESSION"],
	))
	bestScore := -1
	var best ProcessInfo
	for _, process := range processes {
		wm := processToWindowManager(process.Comm, process.Args)
		if wm == "" {
			continue
		}
		score := 10
		if wm == preferred && preferred != "" {
			score += 10
		}
		if process.Env["WAYLAND_DISPLAY"] != "" {
			score++
		}
		if process.Env["DISPLAY"] != "" {
			score++
		}
		if wm == "hyprland" && process.Env["HYPRLAND_INSTANCE_SIGNATURE"] != "" {
			score += 5
		}
		if score > bestScore {
			bestScore = score
			best = process
		}
	}
	return best
}

func processToWindowManager(comm, args string) string {
	target := strings.ToLower(strings.TrimSpace(comm + " " + args))
	switch {
	case strings.Contains(target, "hyprland"):
		return "hyprland"
	case strings.Contains(target, "gnome-shell"):
		return "gnome"
	case strings.Contains(target, "sway"):
		return "sway"
	case strings.Contains(target, "i3"):
		return "i3"
	default:
		return ""
	}
}

func inferWindowManagerFromEnv(env map[string]string) string {
	combined := strings.ToLower(strings.Join([]string{
		env["XDG_CURRENT_DESKTOP"],
		env["XDG_SESSION_DESKTOP"],
		env["DESKTOP_SESSION"],
		env["HYPRLAND_INSTANCE_SIGNATURE"],
		env["SWAYSOCK"],
	}, " "))
	switch {
	case strings.Contains(combined, "hypr"):
		return "hyprland"
	case strings.Contains(combined, "sway"):
		return "sway"
	case strings.Contains(combined, "gnome"):
		return "gnome"
	case strings.Contains(combined, "i3"):
		return "i3"
	default:
		return ""
	}
}

func inferSessionType(env map[string]string, wm string) string {
	if value := normalizeSessionType(env["XDG_SESSION_TYPE"]); value != "" {
		return value
	}
	if env["WAYLAND_DISPLAY"] != "" {
		return "wayland"
	}
	if env["DISPLAY"] != "" {
		return "x11"
	}
	return defaultSessionTypeForWM(wm)
}

func defaultSessionTypeForWM(wm string) string {
	switch normalizeWindowManager(wm) {
	case "hyprland", "sway":
		return "wayland"
	case "i3":
		return "x11"
	default:
		return ""
	}
}

func detectAvailableBackends(session SessionInfo, processes []ProcessInfo, tools map[string]string) []string {
	var backends []string
	if session.WindowManager == "gnome" && tools["gsettings"] != "" {
		backends = append(backends, "gnome")
	}
	if session.WindowManager == "hyprland" && tools["hyprctl"] != "" && processRunning(processes, "hyprpaper") {
		backends = append(backends, "hyprpaper")
	}
	if session.WindowManager == "hyprland" && tools["swww"] != "" {
		backends = append(backends, "swww")
	}
	if session.WindowManager == "hyprland" && tools["swaybg"] != "" {
		backends = append(backends, "swaybg")
	}
	if session.WindowManager == "sway" && tools["swaymsg"] != "" {
		backends = append(backends, "swaymsg")
	}
	if session.WindowManager == "sway" && tools["swaybg"] != "" {
		backends = append(backends, "swaybg")
	}
	if session.SessionType == "x11" || session.WindowManager == "i3" {
		for _, backend := range []string{"feh", "xwallpaper", "nitrogen"} {
			if tools[backend] != "" {
				backends = append(backends, backend)
			}
		}
	}
	return uniqueStrings(backends)
}

func processRunning(processes []ProcessInfo, target string) bool {
	for _, process := range processes {
		if strings.EqualFold(process.Comm, target) {
			return true
		}
	}
	return false
}

func uniqueProcessNames(processes []ProcessInfo) []string {
	var names []string
	for _, process := range processes {
		if process.Comm == "" {
			continue
		}
		names = append(names, process.Comm)
	}
	return uniqueStrings(names)
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
