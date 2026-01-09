import argparse
import shutil
import subprocess
from pathlib import Path

import numpy as np
import requests


def _rasterio_data_to_image(array):
    return array.swapaxes(0, 2).swapaxes(0, 1)


def download_image(image_url, verify=True, timeout=30):
    from rasterio.io import MemoryFile

    response_image = requests.get(image_url, verify=verify, timeout=timeout)
    response_image.raise_for_status()
    with MemoryFile(response_image.content) as memfile:
        with memfile.open() as dataset:
            data_array = dataset.read()
    return _rasterio_data_to_image(data_array)


def open_image_file(image_file):
    from PIL import Image

    return np.array(Image.open(image_file))


def crop_bottom_of_image(image_array, n_pixels):
    return image_array[:-n_pixels, :, :]


def add_black_margin(image_array, n_pixels):
    pad_width = ((n_pixels, n_pixels), (n_pixels, n_pixels), (0, 0))
    return np.pad(image_array, pad_width, constant_values=0)


def color_mixing(image_array, mix=0.3):
    new_image_array = image_array.copy()
    genuine_norm = (new_image_array**2).sum(axis=2)

    red = new_image_array[:, :, 0]
    green = new_image_array[:, :, 1]
    blue = new_image_array[:, :, 2]

    new_image_array[:, :, 0] = (1 - mix) * red + mix * green
    new_image_array[:, :, 1] = (1 - 1.5 * mix) * green + mix * red + 0.5 * mix * blue
    new_image_array[:, :, 2] = blue

    norm = (new_image_array**2).sum(axis=2)

    return np.nan_to_num(
        new_image_array * np.dstack(3 * [genuine_norm]) / np.dstack(3 * [norm]), 0
    ).clip(max=1)


def saturate(x, factor=2):
    return np.tanh((x - 0.5) * factor) / np.tanh(0.5 * factor) / 2 + 0.5


def fetch_latest_image_url(verify=True, timeout=30):
    response = requests.get(
        "https://meteosat-url.appspot.com/msg", verify=verify, timeout=timeout
    )
    response.raise_for_status()
    payload = response.json()
    image_url = payload["url"]
    image_date = payload["date"].replace(":", "-").replace(" ", "_")
    return image_url, image_date


def generate_earth_image(output_dir, verify=True):
    bottom_crop = 79
    black_margin = 50
    color_mixing_factor = 0.3
    saturation_factor = 1.75

    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    image_url, image_date = fetch_latest_image_url(verify=verify)
    image_file = output_dir / f"earth_{image_date}.png"

    mask_image_array = open_image_file(Path(__file__).parent / "mask_raw.png")[:, :, :3]
    image_array = download_image(image_url, verify=verify)

    image_array = (
        (image_array * (1 - mask_image_array / 255)).astype("uint8").clip(min=0) / 255
    )
    image_array = crop_bottom_of_image(image_array, n_pixels=bottom_crop)
    image_array = add_black_margin(image_array, n_pixels=black_margin)

    image_array = color_mixing(image_array, mix=color_mixing_factor)
    image_array = saturate(image_array, factor=saturation_factor)

    image_array_int = (image_array * 255).astype(np.uint8)

    from PIL import Image

    Image.fromarray(image_array_int).save(image_file)
    return image_file


def _parse_resolution(value):
    try:
        width_str, height_str = value.lower().split("x", 1)
        width = int(width_str)
        height = int(height_str)
    except ValueError as exc:
        raise ValueError("Resolution must be in the form WIDTHxHEIGHT.") from exc
    if width <= 0 or height <= 0:
        raise ValueError("Resolution must use positive integers.")
    return width, height


def _detect_resolution():
    if not shutil.which("xrandr"):
        raise RuntimeError("xrandr not found; install it or pass --resolution.")

    result = subprocess.run(
        ["xrandr", "--current"],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    for line in result.stdout.splitlines():
        if "*" in line:
            current = line.strip().split()[0]
            return _parse_resolution(current)
    raise RuntimeError("Unable to detect current resolution from xrandr output.")


def _fit_image_to_resolution(image_file, target_width, target_height):
    from PIL import Image

    image = Image.open(image_file)
    image_ratio = image.width / image.height
    target_ratio = target_width / target_height

    if image_ratio > target_ratio:
        new_width = target_width
        new_height = max(1, int(round(target_width / image_ratio)))
    else:
        new_height = target_height
        new_width = max(1, int(round(target_height * image_ratio)))

    resized = image.resize((new_width, new_height), Image.Resampling.LANCZOS)
    canvas = Image.new("RGB", (target_width, target_height), (0, 0, 0))
    offset_x = (target_width - new_width) // 2
    offset_y = (target_height - new_height) // 2
    canvas.paste(resized, (offset_x, offset_y))
    canvas.save(image_file)


def set_feh_wallpaper(image_file):
    image_path = Path(image_file).resolve()
    if not image_path.exists():
        raise FileNotFoundError(f"Wallpaper image not found: {image_path}")

    if not shutil.which("feh"):
        raise RuntimeError("feh not found; install it or set the wallpaper manually.")

    subprocess.run(
        [
            "feh",
            "--bg-fill",
            str(image_path),
        ],
        check=True,
    )


def set_gnome_wallpaper(image_file):
    image_path = Path(image_file).resolve()
    if not image_path.exists():
        raise FileNotFoundError(f"Wallpaper image not found: {image_path}")

    if not shutil.which("gsettings"):
        raise RuntimeError("gsettings not found; install GNOME or set the wallpaper manually.")

    uri = f"file://{image_path}"
    subprocess.run(
        [
            "gsettings",
            "set",
            "org.gnome.desktop.background",
            "picture-uri",
            uri,
        ],
        check=True,
    )
    subprocess.run(
        [
            "gsettings",
            "set",
            "org.gnome.desktop.background",
            "picture-uri-dark",
            uri,
        ],
        check=True,
    )


def parse_args():
    parser = argparse.ArgumentParser(
        description="Download the latest EUMETSAT image and generate a wallpaper."
    )
    parser.add_argument(
        "--output-dir",
        default=".",
        help="Directory to write the generated PNG (default: current directory).",
    )
    parser.add_argument(
        "--set-wallpaper",
        choices=["feh", "gnome", "none"],
        default="none",
        help="Optionally set the wallpaper using feh or gnome (default: none).",
    )
    parser.add_argument(
        "--resolution",
        help="Target resolution in WIDTHxHEIGHT to fit the output image.",
    )
    parser.add_argument(
        "--auto-resolution",
        action="store_true",
        help="Detect the target resolution using xrandr.",
    )
    parser.add_argument(
        "--insecure",
        action="store_true",
        help="Disable TLS verification for the EUMETSAT request.",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    image_file = generate_earth_image(args.output_dir, verify=not args.insecure)
    if args.auto_resolution and args.resolution:
        raise ValueError("Use either --resolution or --auto-resolution, not both.")
    if args.auto_resolution:
        width, height = _detect_resolution()
        _fit_image_to_resolution(image_file, width, height)
    elif args.resolution:
        width, height = _parse_resolution(args.resolution)
        _fit_image_to_resolution(image_file, width, height)
    print(f"Saved wallpaper to {image_file}")

    if args.set_wallpaper == "feh":
        set_feh_wallpaper(image_file)
        print("feh wallpaper updated.")
    elif args.set_wallpaper == "gnome":
        set_gnome_wallpaper(image_file)
        print("GNOME wallpaper updated.")


if __name__ == "__main__":
    main()
