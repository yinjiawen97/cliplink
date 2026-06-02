#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="$PROJECT_DIR/cliplink"
PLIST_SRC="$SCRIPT_DIR/com.cliplink.plist"
PLIST_DST="$HOME/Library/LaunchAgents/com.cliplink.plist"

if [ ! -f "$BINARY" ]; then
    echo "Error: binary not found at $BINARY"
    echo "Run 'make build' in the project directory first."
    exit 1
fi

# Unload existing if present
launchctl unload "$PLIST_DST" 2>/dev/null || true

# Write plist with actual binary path
sed "s|CLIPLINK_BINARY_PATH|$BINARY|g" "$PLIST_SRC" > "$PLIST_DST"

launchctl load "$PLIST_DST"
echo "cliplink autostart installed. Logs: tail -f /tmp/cliplink.log"
