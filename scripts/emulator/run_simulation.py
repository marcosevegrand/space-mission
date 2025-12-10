#!/usr/bin/env python3
import glob
import os
import signal
import subprocess
import sys
import time

# ==============================================================================
# CONFIGURATION
# ==============================================================================

LOG_FILENAME = "simulation.log"

# Node names must match the filenames found in /tmp/pycore.*/ (e.g. "mothership", "ground")
NODES = {
    "ground": {"id": 1, "name": "ground"},
    "mothership": {"id": 2, "name": "mothership"},
    "rover1": {"id": 9, "name": "rover1"},
    "rover2": {"id": 10, "name": "rover2"},
    "rover3": {"id": 11, "name": "rover3"},
}

# Network Configuration
MOTHERSHIP_TS_PORT = 9001
MOTHERSHIP_ML_PORT = 9002
MOTHERSHIP_API_PORT = 9003
MOTHERSHIP_API_IP = "10.0.0.21"
MOTHERSHIP_TS_IP = "10.0.1.20"
MOTHERSHIP_TS_IP = "10.0.1.20"

# Binary Paths (Relative to project root)
BIN_MOTHERSHIP = "./bin/mothership/mothership"
BIN_ROVER = "./bin/rover/rover"
BIN_GC = "./bin/groundcontrol/groundcontrol"
DIST_GC = "./bin/groundcontrol/dist"

# Global log file handle
log_file = None

# ==============================================================================
# UTILITIES
# ==============================================================================


def log(message):
    """Prints to console and writes to the log file."""
    print(message)
    if log_file:
        timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
        log_file.write(f"[{timestamp}] [SCRIPT] {message}\n")
        log_file.flush()


def check_root():
    if os.geteuid() != 0:
        log("❌ Error: This script must be run as root (sudo).")
        sys.exit(1)


def is_daemon_running():
    try:
        subprocess.check_output(["pgrep", "-x", "core-daemon"])
        return True
    except subprocess.CalledProcessError:
        return False


def start_daemon():
    log("🔄 core-daemon not found. Starting it...")
    try:
        # Log daemon output to the main log file as well
        subprocess.Popen(["core-daemon"], stdout=log_file, stderr=subprocess.STDOUT)
        time.sleep(2)
        if is_daemon_running():
            log("✅ core-daemon started.")
        else:
            log("❌ Failed to start core-daemon.")
            sys.exit(1)
    except Exception as e:
        log(f"❌ Error starting daemon: {e}")
        sys.exit(1)


def get_latest_session_dir():
    sessions = glob.glob("/tmp/pycore.*")
    if not sessions:
        return None
    return max(sessions, key=os.path.getmtime)


def run_vcmd(session_dir, node_name, command, log_name, input_command=None):
    ctl_file = os.path.join(session_dir, node_name)

    if not os.path.exists(ctl_file):
        log(f"❌ Control file not found: {ctl_file}")
        return None

    project_root = os.getcwd()
    wrapped_cmd = f"cd {project_root} && {command}"
    full_cmd = ["vcmd", "-c", ctl_file, "--", "bash", "-c", wrapped_cmd]

    log(f"   🚀 Launching {log_name} inside node '{node_name}'...")

    try:
        # Redirect stdout/stderr to the shared log file
        stdin_val = subprocess.PIPE if input_command else subprocess.DEVNULL

        proc = subprocess.Popen(
            full_cmd,
            stdout=log_file,  # Direct node output to simulation.log
            stderr=subprocess.STDOUT,  # Merge stderr into stdout
            stdin=stdin_val,
            cwd=project_root,
        )

        if input_command:
            proc.stdin.write((input_command + "\n").encode("utf-8"))
            proc.stdin.flush()
            log(f"      ↳ CLI Command injected: '{input_command}'")

        return proc
    except Exception as e:
        log(f"   ❌ Failed to launch {log_name}: {e}")
        return None


# ==============================================================================
# MAIN SCRIPT
# ==============================================================================


def main():
    global log_file

    # Open the master log file
    try:
        log_file = open(LOG_FILENAME, "w")
        log(f"📝 Logging started. Output directed to {LOG_FILENAME}")
    except IOError as e:
        print(f"❌ Failed to create log file: {e}")
        sys.exit(1)

    check_root()

    # 1. Daemon Check
    if not is_daemon_running():
        start_daemon()
    else:
        log("✅ core-daemon is running.")

    # 2. GUI Start
    log("🎨 Opening CORE GUI...")
    try:
        subprocess.Popen(
            ["core-gui"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
        )
    except FileNotFoundError:
        try:
            subprocess.Popen(
                ["core-pygui"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
            )
        except:
            log("⚠️ Could not launch GUI automatically.")

    # 3. User Interaction
    print("\n" + "=" * 60)
    print("👉 ACTION REQUIRED:")
    print("1. In CORE GUI, load: coreemu/topology.xml")
    print("2. Click the green 'Start' button.")
    print("3. Wait for green nodes.")
    print("=" * 60)
    input("\nPRESS ENTER HERE ONCE THE SIMULATION IS RUNNING...")

    # 4. Session Detection
    session_dir = get_latest_session_dir()
    if not session_dir:
        log("❌ No active CORE session found.")
        sys.exit(1)

    log(f"✅ Detected active session: {session_dir}")

    procs = []

    # --- PHASE 1: GROUND CONTROL ---
    log("\n--- PHASE 1: GROUND CONTROL ---")
    node_gc = NODES["ground"]
    cmd_gc = (
        f"{BIN_GC} "
        f"-port=:3000 "
        f"-api-addr={MOTHERSHIP_API_IP}:{MOTHERSHIP_API_PORT} "
        f"-dist={DIST_GC}"
    )
    p_gc = run_vcmd(session_dir, node_gc["name"], cmd_gc, "ground_control")
    if p_gc:
        procs.append(p_gc)

    # Open Firefox INSIDE Ground Node
    log("🌍 Opening Firefox inside 'ground' node (http://localhost:3000)...")
    display_val = os.environ.get("DISPLAY", ":0")
    cmd_firefox = f"DISPLAY={display_val} firefox http://localhost:3000"
    p_ff = run_vcmd(session_dir, node_gc["name"], cmd_firefox, "firefox_ground")
    if p_ff:
        procs.append(p_ff)

    log("⏳ Waiting 5s...")
    time.sleep(5)

    # --- PHASE 2: MOTHERSHIP ---
    log("\n--- PHASE 2: MOTHERSHIP ---")
    node_ms = NODES["mothership"]
    cmd_ms = (
        f"{BIN_MOTHERSHIP} "
        f"-ts-addr=:{MOTHERSHIP_TS_PORT} "
        f"-ml-addr=:{MOTHERSHIP_ML_PORT} "
        f"-api-addr=:{MOTHERSHIP_API_PORT}"
    )

    # Run Mothership and inject the load command
    p_ms = run_vcmd(
        session_dir,
        node_ms["name"],
        cmd_ms,
        "mothership",
        input_command="load assets/missions/file1.txt",
    )
    if p_ms:
        procs.append(p_ms)

    log("⏳ Waiting 5s for Mothership to initialize...")
    time.sleep(5)

    # --- PHASE 3: ROVERS ---
    log("\n--- PHASE 3: ROVERS ---")
    for key in ["rover1", "rover2", "rover3"]:
        node = NODES[key]
        r_id = node["id"] - 8
        cmd_rover = (
            f"{BIN_ROVER} "
            f"-id={r_id} "
            f"-m-ts-addr={MOTHERSHIP_TS_IP}:{MOTHERSHIP_TS_PORT} "
            f"-m-ml-addr={MOTHERSHIP_TS_IP}:{MOTHERSHIP_ML_PORT} "
            f"-r-ml-addr=:9000"
        )
        p_r = run_vcmd(session_dir, node["name"], cmd_rover, key)
        if p_r:
            procs.append(p_r)
        time.sleep(1)

    log("\n✅ System Operational.")
    log("⌨️  Press Ctrl+C to stop binaries.")

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        log("\n🛑 Stopping processes...")
        for p in procs:
            p.terminate()
        if log_file:
            log_file.close()
        print("Done.")


if __name__ == "__main__":
    main()
