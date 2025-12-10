#!/bin/bash

# Define binary path
BIN="./bin/groundcontrol/groundcontrol"
DIST="./bin/groundcontrol/dist"

# Check if binary exists
if [ ! -f "$BIN" ]; then
    echo "❌ Ground Control binary not found. Run 'make groundcontrol' first."
    exit 1
fi

# Check if dist folder exists
if [ ! -d "$DIST" ]; then
    echo "❌ UI assets not found. Run 'make groundcontrol' first."
    exit 1
fi

# Open Firefox in the background after a short delay
(sleep 2 && firefox http://localhost:3000) &

echo "🚀 Starting Ground Control UI..."
# Connects to Mothership API at localhost:9003
$BIN -port=":3000" -api-addr="http://127.0.0.1:9003" -dist="$DIST"
