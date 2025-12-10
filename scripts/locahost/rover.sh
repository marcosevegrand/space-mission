#!/bin/bash

# Define binary path
BIN="./bin/rover/rover"

# Check if binary exists
if [ ! -f "$BIN" ]; then
    echo "❌ Rover binary not found. Run 'make rover' first."
    exit 1
fi

# Check for argument (Number of Rovers)
if [[ ! "$1" =~ ^[0-9]+$ ]] || [ "$1" -lt 1 ]; then
    echo "❌ Erro: Por favor forneça o número de rovers a instanciar."
    echo "   Uso: $0 <quantidade>"
    echo "   Exemplo: $0 5"
    exit 1
fi

ROVER_COUNT=$1
MOTHERSHIP_IP="127.0.0.1"
BASE_PORT=9010

# Function to kill all child processes on exit
cleanup() {
    echo ""
    echo "🛑 Stopping all rovers..."
    kill 0
}
trap cleanup EXIT

echo "🤖 Initializing Rover Fleet with $ROVER_COUNT units..."

# Loop to start N rovers
for (( i=1; i<=ROVER_COUNT; i++ ))
do
    # Calculate unique port for this rover (9011, 9012, etc.)
    CURRENT_PORT=$((BASE_PORT + i))

    $BIN -id=$i \
        -m-ts-addr="$MOTHERSHIP_IP:9001" \
        -m-ml-addr="$MOTHERSHIP_IP:9002" \
        -r-ml-addr="$MOTHERSHIP_IP:$CURRENT_PORT" &

    echo "   -> Rover $i started (UDP: $CURRENT_PORT)"
done

echo "✅ All systems go. Press Ctrl+C to stop the fleet."
wait
