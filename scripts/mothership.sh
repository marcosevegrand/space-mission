#!/bin/bash

# Define binary path
BIN="./bin/mothership/mothership"

# Check if binary exists
if [ ! -f "$BIN" ]; then
    echo "❌ Mothership binary not found. Run 'make mothership' first."
    exit 1
fi

echo "🛸 Starting Mothership on Localhost..."
# Ports: TS=9001, ML=9002, API=9003
$BIN -ts-addr="127.0.0.1:9001" -ml-addr="127.0.0.1:9002" -api-addr="127.0.0.1:9003"
