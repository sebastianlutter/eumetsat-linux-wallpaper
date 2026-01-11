VENV_DIR="venv"

if [[ -d "$VENV_DIR" ]]; then
    echo "Using existing virtual environment: $VENV_DIR"
else
    echo "Creating virtual environment: $VENV_DIR"
    python3 -m venv "$VENV_DIR"
fi

source "$VENV_DIR/bin/activate"

REQS_MARKER="$VENV_DIR/.requirements_installed"
if [[ ! -f "$REQS_MARKER" ]]; then
    echo "Installing requirements from requirements.txt"
    pip install -r requirements.txt
    touch "$REQS_MARKER"
fi

check_internet() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsS --max-time 3 https://www.google.com >/dev/null 2>&1
    else
        ping -c 1 -W 2 1.1.1.1 >/dev/null 2>&1
    fi
}

attempt=1
max_attempts=3
while ! check_internet; do
    if [[ $attempt -ge $max_attempts ]]; then
        echo "Error: No internet connection after ${max_attempts} attempts."
        exit 1
    fi
    echo "No internet yet. Retrying in 2 seconds... ($attempt/$max_attempts)"
    sleep 2
    attempt=$((attempt + 1))
done

python3 download_earth_from_eumetsat.py --resolution=3200x2000 "$@"

# 4. Find the most recently created .png file
# 'ls -t' sorts by modification time (newest first)
LATEST_IMAGE=$(ls -t *.png 2>/dev/null | head -n 1)

if [[ -n "$LATEST_IMAGE" ]]; then
    IMAGE_PATH="$(pwd)/$LATEST_IMAGE"
    echo "Setting wallpaper: $IMAGE_PATH"
    # --bg-fill scales the image to fit the screen
    feh --bg-fill "$IMAGE_PATH"
else
    echo "Error: No .png wallpaper found in $(pwd)"
    exit 1
fi
