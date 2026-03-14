# AGENTS.md

This file explains how to work on `eumetsat-linux-wallpaper` safely and effectively.

It is intended for future coding agents and developers who need repository-specific guidance beyond generic coding instructions.

## Project Purpose

This project provides a Linux wallpaper updater that:

- downloads current EUMETSAT satellite imagery
- converts the remote image into a wallpaper-friendly PNG
- caches timestamped history locally
- rotates that cache according to retention rules
- detects the active desktop session reliably from a `systemd --user` timer
- applies the wallpaper using the best available backend for the running session

The current product is a Go CLI. The older Python file is legacy and is not the source of truth for runtime behavior.

## Source Of Truth

Treat the Go implementation as authoritative:

- `cmd/eumetsat-wallpaper/main.go`
- `app.go`
- `config.go`
- `detect.go`
- `fetch.go`
- `image.go`
- `resolution.go`
- `cache.go`
- `backend.go`
- `install.go`
- `uninstall.go`

Support or legacy files:

- `download_earth_from_eumetsat.py`
  - legacy implementation
  - may be useful for historical comparison
  - should not be treated as current behavior documentation
- `run.sh`
  - development wrapper around `go run`
- `install.sh`
  - bootstrap installer that builds a temporary binary and delegates to the Go installer
- `uninstall.sh`
  - bootstrap uninstaller that builds a temporary binary and delegates to the Go uninstaller
- `wallpaper_service.sh`
  - helper wrapper for the Go `service` subcommand
- `eumetsat-wallpaper.service` and `eumetsat-wallpaper.timer`
  - example unit files only
  - the actual installer generates fresh units with absolute paths

## High-Level Runtime Flow

The `run` subcommand currently does this:

1. Load config from `~/.config/eumetsat-wallpaper.conf` or an explicit `--config` path.
2. Apply CLI overrides on top of the loaded config.
3. Detect the active session and usable wallpaper backend.
4. Fetch metadata from `API_URL`.
5. If the newest timestamp already exists locally, reuse it.
6. Otherwise download the remote JPEG and convert it into the canonical PNG.
7. If resolution fitting is enabled, render a fitted variant into `.rendered/`.
8. Update `CURRENT_LINK` as a symlink to the selected image.
9. Rotate the cache.
10. Apply the wallpaper unless the backend is `none`.

Important behavior:

- There is no separate network preflight. The tool directly tries the metadata request.
- When the network request fails, it falls back to the newest cached image.
- If no graphical session is detected and backend mode is still `auto`, wallpaper application should be skipped gracefully after logging the reason.

## Key Invariants

These invariants are important. Preserve them unless you are intentionally redesigning behavior.

### Canonical Images Must Stay Canonical

Timestamped images in `IMAGE_DIR` are the canonical cache:

- pattern: `earth_YYYY-MM-DD_HH-MM-SS.png`
- they are the history source for reuse and retention logic
- they should not be overwritten when the monitor size changes

If a monitor-sized output is needed, render it separately into:

- `IMAGE_DIR/.rendered/`

### `current.png` Is The Stable Consumer Path

`CURRENT_LINK` is meant to be stable for:

- wallpaper tools
- users
- any third-party automation that wants the current image

It should remain a symlink to the current selected image or rendered variant.

### Systemd User Timers Are A First-Class Use Case

Do not assume the service inherits a complete shell environment.

Session detection must remain robust when run from:

- `systemd --user`
- an interactive terminal
- a partially populated user manager environment

Avoid changes that rely only on the current shell’s `DISPLAY`, `WAYLAND_DISPLAY`, or `XDG_CURRENT_DESKTOP` unless they are clearly just fallbacks.

### Logging Should Stay Slim But Useful

Default `info` logging should stay concise.

Good log content:

- what session and WM were detected
- which backend was selected
- whether the image was reused, downloaded, or loaded from cache
- which resolution detector succeeded
- the exact wallpaper command being executed

Avoid noisy default logs.

## Session Detection Rules

Detection order matters:

1. explicit config or CLI overrides
2. `loginctl` session data when available
3. running compositor or WM processes
4. environment variable fallback

Current target WMs:

- `hyprland`
- `sway`
- `gnome`
- `i3`

Current target session types:

- `wayland`
- `x11`

Important file:

- `detect.go`

Current implementation details:

- reads `systemctl --user show-environment` when available
- scans current-user processes via `ps`
- reads `/proc/<pid>/environ` for candidate compositor and WM processes when possible
- merges environment fragments so setter commands get the best available session environment

When changing detection logic:

- prefer additive improvements over wholesale replacement
- preserve operation when `loginctl` is unavailable
- preserve fallback behavior when `/proc/<pid>/environ` cannot be read

## Wallpaper Backend Rules

Current auto-selection priority:

### GNOME

- `gsettings`

### Hyprland

- `hyprpaper` only if `hyprpaper` is already running
- otherwise `swww` if installed
- otherwise managed `swaybg`

That conservative behavior is intentional. Do not silently auto-start `hyprpaper` in auto mode unless the design is revisited deliberately.

### Sway

- `swaymsg`
- fallback managed `swaybg`

### X11 / i3

- `feh`
- `xwallpaper`
- `nitrogen`

Important file:

- `backend.go`

When adding a backend:

1. update detection
2. update auto-selection priority
3. add or update tests
4. update `README.md`
5. update this file if the design changed materially

## Managed Daemon Behavior

Some setters are daemon-like and need to outlive the oneshot service:

- `swaybg`
- `swww-daemon`

The current design prefers transient user units created through `systemd-run --user`.

Preserve this behavior unless there is a strong reason not to. A oneshot timer service should not depend on child processes surviving only by accident.

Current transient unit names:

- `eumetsat-wallpaper-swaybg`
- `eumetsat-wallpaper-swww`

## Image Pipeline Rules

Important files:

- `fetch.go`
- `image.go`

Current image flow:

- fetch metadata from `API_URL`
- download the remote JPEG
- decode it with the Go standard library
- apply the embedded `mask_raw.png`
- crop the bottom strip
- add a black margin
- apply color mixing and saturation adjustment
- save a PNG

Important design choice:

- `mask_raw.png` is embedded into the Go binary via `go:embed`

Do not reintroduce a runtime dependency on the repository checkout for the installed tool.

## Resolution Handling

Important file:

- `resolution.go`

Current behavior:

- `RESOLUTION` overrides everything
- otherwise `AUTO_RESOLUTION=true` enables detector-based sizing
- if detection fails, the canonical image is still usable

Current detectors may include:

- `hyprctl`
- `swaymsg`
- `gdbus`
- `wlr-randr`
- `xrandr`
- `xdpyinfo`
- `xwininfo`

If you change detector order:

- keep WM-specific detectors ahead of generic fallbacks
- keep X11 tools available as lower-priority fallbacks
- add tests for the ordering change

## Cache And Retention Rules

Important file:

- `cache.go`

Current retention policy:

- keep all images from the last 24 hours
- for images older than 24 hours and newer than 30 days, keep the newest image per ISO week
- for images older than 30 days, keep the newest image per month with no limit

Important invariants:

- never treat `current.png` as a candidate cache image
- canonical timestamped images are the only input to retention
- `.rendered/` is transient output, not history

When adjusting retention:

- update tests
- update `README.md`
- think through offline fallback behavior

## Installer And Systemd Rules

Important file:

- `install.go`

Current installer behavior:

- prefers user-level installation
- writes the config file only if it does not already exist
- always regenerates the systemd units
- stops old timer, service, and managed helper units before reloading
- resets stale failed state and starts the service once after install
- uses absolute paths in generated units

User install target priority:

1. `XDG_BIN_HOME` if it is already on `PATH`
2. writable user-owned directories already on `PATH`
3. `systemd-path user-binaries` if it is on `PATH`
4. `~/.local/bin` if it is on `PATH`

System install fallback:

- `/usr/local/bin/eumetsat-wallpaper`

Important installer rule:

- do not silently install into a user-local directory that is not on the user’s interactive `PATH`
- if no suitable user dir on `PATH` exists, prefer prompting for sudo and using `/usr/local/bin`

Do not hardcode repository paths into generated units.

The installed service must continue to work even if the git checkout is moved or deleted.

## Uninstall Behavior

Important files:

- `uninstall.go`
- `uninstall.sh`

Current uninstall behavior:

- the CLI has an `uninstall` subcommand
- `uninstall.sh` is the preferred repo-level entry point because it does not depend on the installed binary being on `PATH`
- uninstall removes generated user units and the installed binary
- uninstall preserves config and cached images unless `--purge` is passed
- uninstall discovers the installed binary and config path from the generated service unit before falling back to common locations

Preserve that service-unit discovery path unless the install state model is redesigned deliberately.

## CLI And Config Precedence

Important files:

- `app.go`
- `config.go`

Config format is intentionally simple:

- `KEY=VALUE`
- `export KEY=VALUE` is also accepted

Precedence is:

- CLI flags
- config file
- defaults

If you add a new config key:

1. add it to `Config`
2. add parsing in `applyConfigValue`
3. add it to `RenderConfigFile`
4. expose it on the relevant CLI subcommand if appropriate
5. add tests
6. update `README.md`

## Testing Expectations

Always run:

```bash
go test ./...
```

Usually also run:

```bash
go build ./cmd/eumetsat-wallpaper
```

Useful smoke commands:

```bash
go run ./cmd/eumetsat-wallpaper detect
go run ./cmd/eumetsat-wallpaper run --wallpaper-backend none
go run ./cmd/eumetsat-wallpaper service status
```

Tests should cover, whenever relevant:

- config parsing
- config precedence
- session detection
- backend selection
- cache rotation
- timestamp parsing
- resolution selection or detector ordering
- custom command expansion
- image processing behavior when relevant

When possible:

- avoid network in tests
- avoid depending on the real desktop environment
- use fake commands in a temp `PATH` for integration-style tests

## Documentation Expectations

If you change user-facing behavior, update:

- `README.md`
- `AGENTS.md` when architecture or workflow guidance changed materially

The README is for users.
This file is for implementers.

Do not let them drift apart.

## Known Limits And Non-Goals

Current intentional limits:

- one wallpaper image is applied across all monitors
- `hyprpaper` is not auto-started in auto mode
- the legacy Python implementation is not maintained as a feature-equal fallback

If you change any of these, update both docs and tests.

## Practical Advice For Future Changes

When working on this repo:

- prefer stdlib-only Go unless a dependency clearly earns its place
- do not reintroduce Python runtime dependencies for the main product path
- do not rely on a shell login environment for correctness
- preserve stable, minimal logs
- keep install behavior user-first
- keep offline fallback working

If a bug report says “works in terminal, fails from systemd timer”, start by inspecting:

- session detection
- environment reconstruction
- backend selection
- generated unit contents
- whether a daemon-style backend needs `systemd-run --user`
