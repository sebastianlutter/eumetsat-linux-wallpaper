# EUMETSAT Linux wallpaper

Python script to create an awesome wallpaper of our earth, as seen by EUMETSAT satellites above Europe.

## Fork notice

This repository is a fork of https://github.com/qchenevier/eumetsat-osx-wallpaper, adapted for Linux use with i3wm or GNOME in mind.

### Changes from upstream

- Switched to pip/venv tooling and Linux-only usage (removed conda and macOS scripts).
- Added a CLI with arguments for output directory, wallpaper setter (feh or GNOME), and TLS options.
- Added optional output fitting to a target resolution or auto-detection via xrandr with letterboxing.
- Updated `run.sh` to bootstrap a `venv/` environment and install dependencies automatically.
- Removed bundled sample images and the mask helper script to keep the repo lean.

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


### Create cron job

Open you crontab file:
```
crontab -e
```

And add this line to your crontab file (don't forget to replace `PATH_TO_YOUR_FOLDER`):
```
* * * * * cd PATH_TO_YOUR_FOLDER/eumetsat-osx-wallpaper && /bin/bash -lc 'source .venv/bin/activate && ./run.sh --output-dir ./output' >/tmp/stdout.log 2>/tmp/stderr.log
```

You'll be able to check the logs using:
```
cat /tmp/std*.log
```

## Notes

- The generated PNG is saved as `earth_YYYY-MM-DD_HH-MM-SS.png` in the output directory.
