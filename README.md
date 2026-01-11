# EUMETSAT Linux wallpaper

Python script to create an awesome wallpaper of our earth, as seen by EUMETSAT satellites above Europe.

Example image:

![EUMETSAT example wallpaper](earth_2026-01-11_07-00-00.png)

## Fork notice

This repository is a fork of https://github.com/qchenevier/eumetsat-osx-wallpaper, adapted for Linux use with i3wm or GNOME in mind.

### Changes from upstream

- Switched to pip/venv tooling and Linux-only usage (removed conda and macOS scripts).
- Added a CLI with arguments for output directory, wallpaper setter (feh or GNOME), and TLS options.
- Added optional output fitting to a target resolution or auto-detection via xrandr with letterboxing.
- Updated `run.sh` to bootstrap a `venv/` environment and install dependencies automatically.
- Removed the mask helper script to keep the repo lean.

## Prerequisites

Linux only. These instructions target Arch Linux and use pip with a virtual environment.

## Script installation

### Create a virtual environment

```bash
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
```

### Run the script

```bash
./run.sh
```

### Install as a systemd user timer

```bash
./install.sh
```

`install.sh` finds the absolute path of this repo, replaces `CHANGE_ME` in the unit
files with that path, copies the units into `~/.config/systemd/user`, reloads the
user systemd daemon, and enables the hourly timer.

The timer (`eumetsat-wallpaper.timer`) runs every hour and triggers the service
(`eumetsat-wallpaper.service`), which executes `run.sh` inside the repo and sets
the wallpaper.

You can manage the units with:

```bash
./wallpaper_service.sh status
./wallpaper_service.sh list
./wallpaper_service.sh start
./wallpaper_service.sh logs
```

Optional: write to a specific folder and set the wallpaper via feh (i3wm):

```bash
./run.sh --output-dir ./output --set-wallpaper feh
```

Optional: set GNOME wallpaper:

```bash
./run.sh --output-dir ./output --set-wallpaper gnome
```

Optional: fit the image to a target resolution without stretching:

```bash
./run.sh --output-dir ./output --resolution 1920x1080
```

Optional: auto-detect resolution with xrandr:

```bash
./run.sh --output-dir ./output --auto-resolution
```

## Notes

- The generated PNG is saved as `earth_YYYY-MM-DD_HH-MM-SS.png` in the output directory.
- `wallpaper_service.sh` is a small helper for `systemctl --user` commands to check
  status, list the next scheduled run, trigger a manual run, or view recent logs.
