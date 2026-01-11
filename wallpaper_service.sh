#!/bin/bash

# Configuration
SERVICE_NAME="eumetsat-wallpaper.service"
TIMER_NAME="eumetsat-wallpaper.timer"

# Function to display help
show_help() {
    echo "Usage: $0 {status|list|start|logs}"
    echo ""
    echo "Actions:"
    echo "  status  - Check if the timer and service are active"
    echo "  list    - Show when the wallpaper will next be downloaded"
    echo "  start   - Run the download script manually right now"
    echo "  logs    - View the output/errors from the last runs"
}

case "$1" in
    status)
        echo "--- Timer Status ---"
        systemctl --user status "$TIMER_NAME"
        echo -e "\n--- Service Status ---"
        systemctl --user status "$SERVICE_NAME"
        ;;
    list)
        echo "--- Scheduled Runs ---"
        systemctl --user list-timers --all | grep "$TIMER_NAME"
        ;;
    start)
        echo "Triggering wallpaper download immediately..."
        systemctl --user start "$SERVICE_NAME"
        ;;
    logs)
        echo "--- Recent Logs ---"
        journalctl --user -u "$SERVICE_NAME" -n 20 --no-pager
        ;;
    *)
        show_help
        exit 1
        ;;
esac
