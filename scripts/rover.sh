#!/bin/bash

# Define binary path
BIN="./bin/rover/rover"

# Check if binary exists
if [ ! -f "$BIN" ]; then
    echo "❌ Rover binary not found. Run 'make rover' first."
    exit 1
fi

MOTHERSHIP_IP="127.0.0.1"

# Function to kill all child processes on exit
cleanup() {
    echo "🛑 Stopping all rovers..."
    kill 0
}
trap cleanup EXIT

echo "🤖 Initializing Rover Fleet..."

# Rover 1 (ID: 1, Port: 9011)
$BIN -id=1 \
    -m-ts-addr="$MOTHERSHIP_IP:9001" \
    -m-ml-addr="$MOTHERSHIP_IP:9002" \
    -r-ml-addr="$MOTHERSHIP_IP:9011" &
echo "   -> Rover 1 started (UDP: 9011)"

# Rover 2 (ID: 2, Port: 9012)
$BIN -id=2 \
    -m-ts-addr="$MOTHERSHIP_IP:9001" \
    -m-ml-addr="$MOTHERSHIP_IP:9002" \
    -r-ml-addr="$MOTHERSHIP_IP:9012" &
echo "   -> Rover 2 started (UDP: 9012)"

# Rover 3 (ID: 3, Port: 9013)
$BIN -id=3 \
    -m-ts-addr="$MOTHERSHIP_IP:9001" \
    -m-ml-addr="$MOTHERSHIP_IP:9002" \
    -r-ml-addr="$MOTHERSHIP_IP:9013" &
echo "   -> Rover 3 started (UDP: 9013)"

# Rover 4 (ID: 4, Port: 9014)
$BIN -id=4 \
    -m-ts-addr="$MOTHERSHIP_IP:9001" \
    -m-ml-addr="$MOTHERSHIP_IP:9002" \
    -r-ml-addr="$MOTHERSHIP_IP:9014" &
echo "   -> Rover 4 started (UDP: 9014)"

# Rover 5 (ID: 5, Port: 9015)
$BIN -id=5 \
    -m-ts-addr="$MOTHERSHIP_IP:9001" \
    -m-ml-addr="$MOTHERSHIP_IP:9002" \
    -r-ml-addr="$MOTHERSHIP_IP:9015" &
echo "   -> Rover 5 started (UDP: 9015)"

echo "✅ All systems go. Press Ctrl+C to stop the fleet."
wait
