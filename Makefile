# ==============================================================================
# SPACE MISSION BUILD SYSTEM
# ==============================================================================

# Variables for output directories
BIN_DIR := bin
GC_BIN := $(BIN_DIR)/groundcontrol
MS_BIN := $(BIN_DIR)/mothership
ROVER_BIN := $(BIN_DIR)/rover

# Variables for source directories
GC_CMD := ./cmd/groundcontrol
MS_CMD := ./cmd/mothership
ROVER_CMD := ./cmd/rover
GC_UI_SRC := ./ui/groundcontrol

# .PHONY tells Make that these targets are not actual files
.PHONY: all clean help groundcontrol mothership rover ui-groundcontrol

# Default target: Build everything
all: clean groundcontrol mothership rover

# ==============================================================================
# 1. GROUND CONTROL (Backend + Frontend)
# ==============================================================================

# The main entry point for Ground Control
groundcontrol: ui-groundcontrol
	@echo "🚀 Building Ground Control Binary..."
	@mkdir -p $(GC_BIN)
	# Copy the built UI assets to the binary's folder
	@mkdir -p $(GC_BIN)/dist
	@cp -r $(GC_UI_SRC)/dist/* $(GC_BIN)/dist/
	# Build the Go binary
	go build -o $(GC_BIN)/groundcontrol $(GC_CMD)
	@echo "✅ Ground Control built successfully!"

# The React Frontend
ui-groundcontrol:
	@echo "🎨 Building Ground Control UI..."
	# Install dependencies only if node_modules is missing (saves time)
	@[ -d "$(GC_UI_SRC)/node_modules" ] || (cd $(GC_UI_SRC) && npm install)
	# Run the Vite build
	cd $(GC_UI_SRC) && npm run build

# ==============================================================================
# 2. MOTHERSHIP
# ==============================================================================

mothership:
	@echo "🛸 Building Mothership..."
	@mkdir -p $(MS_BIN)
	# Build the Go binary
	go build -o $(MS_BIN)/mothership $(MS_CMD)
	@echo "✅ Mothership built successfully!"

# ==============================================================================
# 3. ROVER
# ==============================================================================

rover:
	@echo "🤖 Building Rover..."
	@mkdir -p $(ROVER_BIN)
	# Build the Go binary
	go build -o $(ROVER_BIN)/rover $(ROVER_CMD)
	@echo "✅ Rover built successfully!"

# ==============================================================================
# UTILITIES
# ==============================================================================

clean:
	@echo "🧹 Cleaning up build artifacts..."
	rm -rf $(BIN_DIR)
	# Optional: Clean UI build artifacts if you want a fresh start
	# rm -rf $(GC_UI_SRC)/dist
	# rm -rf $(GC_UI_SRC)/node_modules

help:
	@echo "Available commands:"
	@echo "  make all           - Clean and build everything"
	@echo "  make groundcontrol - Build Ground Control (Go + React)"
	@echo "  make mothership    - Build Mothership"
	@echo "  make rover         - Build Rover"
	@echo "  make clean         - Remove bin/ directory"
