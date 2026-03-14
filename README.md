# EUMETSAT Linux Wallpaper

`eumetsat-wallpaper` is a Go CLI for Linux that downloads current EUMETSAT satellite imagery, converts it into a wallpaper-friendly PNG, keeps a useful local history, and applies the current image to the active desktop session.

The implementation is built specifically for user-level `systemd` timers. Instead of assuming the right graphical environment is already present in the service environment, it detects the running session and chooses the best available wallpaper backend at runtime.

> Disclaimer: this project has been vibe coded. Treat it as useful automation, but still verify the behavior in your own desktop and systemd environment before relying on it blindly.

Example generated wallpaper:

![EUMETSAT example wallpaper](earth_2026-01-11_07-00-00.png)

## Overview

At each run the tool:

1. Loads configuration from `~/.config/eumetsat-wallpaper.conf` plus any CLI overrides.
2. Detects the active desktop session and window manager.
3. Fetches the latest EUMETSAT metadata.
4. Downloads the latest image if it is not already cached.
5. Applies the built-in mask and image processing pipeline.
6. Optionally renders a fitted monitor-sized variant.
7. Updates a stable `current.png` symlink.
8. Sets the wallpaper using the best available backend.
9. Rotates the image cache according to the retention policy.

If the internet is unavailable, the tool reuses the newest cached image instead of failing immediately.

## Supported Environments

The current target sessions are:

- GNOME
- Hyprland
- Sway
- i3 on X11

Automatic backend selection currently works like this:

- `gnome` -> `gsettings`
- `hyprland` -> `hyprpaper` if it is already running, otherwise `swww` if installed, otherwise managed `swaybg`
- `sway` -> `swaymsg`, fallback to managed `swaybg`
- `i3` or generic X11 fallback -> `feh`, then `xwallpaper`, then `nitrogen`

For custom setups you can bypass auto-selection with:

- `WALLPAPER_BACKEND=custom`
- `WALLPAPER_COMMAND=...`

Custom command placeholders:

- `{image}` -> the selected timestamped source image
- `{current}` -> the stable current wallpaper path, usually `current.png`

## Defaults

The default installation is a user install.

- Config: `~/.config/eumetsat-wallpaper.conf`
- Image directory: `~/Bilder/eumetsat-wallpaper`
- Stable current image: `~/Bilder/eumetsat-wallpaper/current.png`
- Timer cadence: every 15 minutes via `OnCalendar=*:0/15`

The cache layout is split intentionally:

- canonical timestamped images live directly in `IMAGE_DIR`
- rendered monitor-sized variants live in `IMAGE_DIR/.rendered`
- `CURRENT_LINK` is a symlink to the currently selected image

## Requirements

Required to build or install from this repository:

- Go 1.26 or newer
- Linux with `systemd --user`

Optional runtime tools, depending on your session:

- `gsettings` for GNOME
- `hyprctl` and optionally `hyprpaper` for Hyprland
- `swww` for Hyprland if you want that backend
- `swaymsg` or `swaybg` for Sway
- `feh`, `xwallpaper`, or `nitrogen` for i3/X11
- `loginctl`, `systemctl`, `systemd-run`, `xrandr`, `xdpyinfo`, `xwininfo`, `wlr-randr`, and `gdbus` improve detection and resolution handling

## Installation

### Quick Install

From the repository root:

```bash
./install.sh
```

`install.sh` builds a temporary binary and delegates to the Go installer. The installer:

1. chooses an install location for the binary
2. writes the config file if it does not already exist
3. writes the user systemd service and timer units
4. stops old timer, service, and managed wallpaper helper units
5. reloads the user daemon and clears stale failure state
6. enables and starts the timer
7. starts the service once immediately so the new install is exercised right away

The installer prefers these user-level binary targets in order:

1. `$XDG_BIN_HOME`, but only if it is already on `PATH`
2. writable user-owned directories already on `PATH`
3. `systemd-path user-binaries`, but only if that directory is on `PATH`
4. `~/.local/bin`, but only if it is on `PATH`

If no suitable user target is available on `PATH`, the installer prompts to install into `/usr/local/bin` with `sudo` instead of silently choosing a location you cannot call from the terminal.

### Build First, Install Second

If you prefer to build explicitly:

```bash
go build -o ./dist/eumetsat-wallpaper ./cmd/eumetsat-wallpaper
./dist/eumetsat-wallpaper install
```

Useful installer options:

```bash
./dist/eumetsat-wallpaper install --on-calendar "hourly"
./dist/eumetsat-wallpaper install --image-dir "$HOME/Pictures/eumetsat"
./dist/eumetsat-wallpaper install --current-link "$HOME/Pictures/eumetsat/current.png"
./dist/eumetsat-wallpaper install --config "$HOME/.config/eumetsat-wallpaper.conf"
./dist/eumetsat-wallpaper install --binary-path "$HOME/.local/bin/eumetsat-wallpaper"
./dist/eumetsat-wallpaper install --force-system
```

### Reinstall Or Upgrade

If you updated the repository or rebuilt the binary, run the installer again:

```bash
./install.sh
```

This refreshes the installed binary and rewrites the generated systemd units. Your existing config file is kept as-is because the installer only creates it when it does not exist yet.

## Uninstall

The repository now includes a dedicated uninstall wrapper:

```bash
./uninstall.sh
```

It builds a temporary helper binary and removes:

- the user timer
- the user service
- transient managed wallpaper units started by this project
- the installed `eumetsat-wallpaper` binary

By default, uninstall keeps your config file and cached images.

If the installed binary is in a protected location such as `/usr/local/bin`, the uninstaller prompts for `sudo` before removing it.

### Remove Everything Including User Data

```bash
./uninstall.sh --purge
```

`--purge` additionally removes:

- `~/.config/eumetsat-wallpaper.conf` or your custom config path
- the configured image cache directory, usually `~/Bilder/eumetsat-wallpaper`

### Uninstall Via The Installed Binary

If the binary is available on your `PATH`, you can run:

```bash
eumetsat-wallpaper uninstall
```

or:

```bash
eumetsat-wallpaper uninstall --purge
```

`./uninstall.sh` is the safer default because it does not depend on the installed binary being reachable from your current shell.

If you used a custom install path or config path and the generated service unit is no longer present, you can still override discovery explicitly:

```bash
./uninstall.sh --binary-path /custom/bin/eumetsat-wallpaper --config /custom/config/eumetsat-wallpaper.conf
```

## Usage

### Development Wrapper

From the repository checkout:

```bash
./run.sh
```

`run.sh` is only a development wrapper around:

```bash
go run ./cmd/eumetsat-wallpaper run
```

### Installed Binary

After installation:

```bash
eumetsat-wallpaper run
```

### Main Commands

```bash
eumetsat-wallpaper run
eumetsat-wallpaper detect
eumetsat-wallpaper install
eumetsat-wallpaper service status
eumetsat-wallpaper service list
eumetsat-wallpaper service start
eumetsat-wallpaper service logs
```

### Common Examples

Run once but do not change the wallpaper:

```bash
eumetsat-wallpaper run --wallpaper-backend none
```

Use an explicit output resolution:

```bash
eumetsat-wallpaper run --resolution 1920x1200
```

Disable automatic resolution detection:

```bash
eumetsat-wallpaper run --no-auto-resolution
```

Use a custom image directory:

```bash
eumetsat-wallpaper run \
  --image-dir "$HOME/Pictures/eumetsat" \
  --current-link "$HOME/Pictures/eumetsat/current.png"
```

Use a custom wallpaper command:

```bash
eumetsat-wallpaper run \
  --wallpaper-backend custom \
  --wallpaper-command 'my-wallpaper-setter --file {current}'
```

Inspect what the tool detects:

```bash
eumetsat-wallpaper detect
```

Check timer logs:

```bash
eumetsat-wallpaper service logs
```

## Session Detection

Session detection is tuned for `systemd --user` timer execution.

The tool currently detects the active session in this order:

1. explicit CLI or config overrides
2. `loginctl` session data when available
3. running compositor or window-manager processes
4. environment variables as a fallback

The runtime tries to reconstruct the environment needed by wallpaper setter commands, including values such as:

- `DISPLAY`
- `WAYLAND_DISPLAY`
- `XDG_RUNTIME_DIR`
- `DBUS_SESSION_BUS_ADDRESS`
- `HYPRLAND_INSTANCE_SIGNATURE`
- `SWAYSOCK`
- `XDG_CURRENT_DESKTOP`
- `XDG_SESSION_DESKTOP`
- `DESKTOP_SESSION`

This is the main reason it behaves more reliably from a timer than the earlier shell-based approach.

## Resolution Handling

Resolution handling works like this:

- if `RESOLUTION` or `--resolution` is set, that exact size is used
- otherwise, if auto-resolution is enabled, the tool tries to detect the active output size
- if detection fails, the canonical image is still used

Current detector priority depends on session type and WM, and can include:

- `hyprctl monitors -j`
- `swaymsg -t get_outputs -r`
- GNOME Mutter via `gdbus`
- `wlr-randr`
- `xrandr`
- `xdpyinfo`
- `xwininfo`

Rendered output is letterboxed on a black background so the satellite image is not stretched.

## Cache And Retention

Canonical images are stored as:

```text
earth_YYYY-MM-DD_HH-MM-SS.png
```

Retention policy:

- keep every image from the last 24 hours
- for images older than 24 hours and newer than 30 days, keep the newest image per ISO week
- for images older than 30 days, keep the newest image per month without limit

If the latest remote timestamp already exists locally, the tool reuses it instead of regenerating the image.

If the network is down or metadata fetch fails, the tool selects the newest cached image using:

1. the parsed filename timestamp
2. file modification time if the filename does not parse

## Configuration

Configuration uses simple `KEY=VALUE` syntax in:

```bash
~/.config/eumetsat-wallpaper.conf
```

Lines prefixed with `export` are also accepted.

Precedence is:

```text
CLI flags > config file > built-in defaults
```

### Example Config

```ini
# EUMETSAT wallpaper configuration
IMAGE_DIR=/home/myself/Bilder/eumetsat-wallpaper
CURRENT_LINK=/home/myself/Bilder/eumetsat-wallpaper/current.png
WINDOW_MANAGER=
SESSION_TYPE=
WALLPAPER_BACKEND=auto
WALLPAPER_COMMAND=
RESOLUTION=
AUTO_RESOLUTION=true
OFFSET_Y=0
API_URL=https://meteosat-url.appspot.com/msg
HTTP_TIMEOUT=30s
INSECURE_TLS=false
LOG_LEVEL=info
ON_CALENDAR=*:0/15
```

### Config Reference

| Key | Default | Description |
| --- | --- | --- |
| `IMAGE_DIR` | `~/Bilder/eumetsat-wallpaper` | Directory for canonical timestamped wallpapers. |
| `CURRENT_LINK` | `IMAGE_DIR/current.png` | Stable symlink to the currently selected wallpaper. |
| `WINDOW_MANAGER` | auto | Override WM detection. Common values are `gnome`, `hyprland`, `sway`, `i3`. |
| `SESSION_TYPE` | auto | Override session type detection, usually `wayland` or `x11`. |
| `WALLPAPER_BACKEND` | `auto` | Force a backend such as `gnome`, `hyprpaper`, `swww`, `swaymsg`, `swaybg`, `feh`, `xwallpaper`, `nitrogen`, `custom`, or `none`. |
| `WALLPAPER_COMMAND` | empty | Custom command used when `WALLPAPER_BACKEND=custom`. |
| `RESOLUTION` | empty | Explicit output resolution in `WIDTHxHEIGHT` format. |
| `AUTO_RESOLUTION` | `true` | Whether the tool should auto-detect output resolution. |
| `OFFSET_Y` | `0` | Vertical offset applied to the fitted image. Negative values move the image up. |
| `API_URL` | `https://meteosat-url.appspot.com/msg` | Metadata endpoint that returns the latest image URL and timestamp. |
| `HTTP_TIMEOUT` | `30s` | HTTP timeout for metadata and image downloads. Numeric values are interpreted as seconds. |
| `INSECURE_TLS` | `false` | Disables TLS verification. Only use it deliberately. |
| `LOG_LEVEL` | `info` | Log level: `error`, `info`, or `debug`. |
| `ON_CALENDAR` | `*:0/15` | Schedule used by `install` when generating the timer unit. |

## Logging

The tool prints concise logs by default. Typical messages include:

- detected session and detection source
- whether an image was reused, downloaded, or loaded from cache
- resolution detection success or fallback
- the exact wallpaper command that is executed

For more detail:

```bash
eumetsat-wallpaper run --log-level debug
```

To inspect service logs directly:

```bash
journalctl --user -u eumetsat-wallpaper.service -n 100 --no-pager
```

## Systemd Operation

Installed unit names:

- `eumetsat-wallpaper.service`
- `eumetsat-wallpaper.timer`

Useful commands:

```bash
systemctl --user status eumetsat-wallpaper.timer
systemctl --user status eumetsat-wallpaper.service
systemctl --user list-timers --all
systemctl --user start eumetsat-wallpaper.service
journalctl --user -u eumetsat-wallpaper.service -n 100 --no-pager
```

The repository also contains example unit files:

- `eumetsat-wallpaper.service`
- `eumetsat-wallpaper.timer`

Those are examples only. The installer writes fresh units with the exact binary path and config path chosen during installation.

If you want user timers to keep running after logout, you may also need lingering enabled:

```bash
sudo loginctl enable-linger "$USER"
```

## Troubleshooting

### The Timer Runs But The Wallpaper Does Not Change

Start with:

```bash
eumetsat-wallpaper detect
```

Check:

- `window_manager`
- `session_type`
- `available_backends`
- `available_tools`

If auto mode does not find a suitable backend, set one explicitly in config or on the CLI.

### Hyprland Is Detected But `hyprpaper` Is Not Used

That is expected unless `hyprpaper` is already running. In auto mode the tool deliberately does not start `hyprpaper`. It falls back to:

1. `swww`
2. `swaybg`

### The Image Downloads But Does Not Match My Monitor Size

Try an explicit resolution:

```bash
eumetsat-wallpaper run --resolution 1920x1200
```

Or install the resolution detection tools that are appropriate for your environment.

### I Only Want To Populate The Cache

Use:

```bash
eumetsat-wallpaper run --wallpaper-backend none
```

### I Want To Run The Same Action As The Timer

Use:

```bash
eumetsat-wallpaper run --log-level debug
```

## Development

Common development commands:

```bash
go test ./...
go build ./cmd/eumetsat-wallpaper
go run ./cmd/eumetsat-wallpaper detect
go run ./cmd/eumetsat-wallpaper run --wallpaper-backend none
```

The Go implementation is the active runtime and the source of truth for behavior.
