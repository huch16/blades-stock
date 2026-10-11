#!/bin/bash
# Network Chaos Experiments for Blades-Stock
# Requires: Linux with tc (traffic control) and chaosblade (optional)
# Run with sudo

set -euo pipefail

INTERFACE="${1:-eth0}"  # Default interface
TARGET_IP="${2:-}"      # Optional target IP (e.g., API endpoint)
DURATION="${3:-30}"     # Duration in seconds

echo "=== Network Chaos Experiment ==="
echo "Interface: $INTERFACE"
echo "Target IP: ${TARGET_IP:-any}"
echo "Duration: ${DURATION}s"
echo ""

# Function to cleanup on exit
cleanup() {
    echo "Cleaning up tc rules..."
    sudo tc qdisc del dev "$INTERFACE" root 2>/dev/null || true
    sudo tc qdisc del dev "$INTERFACE" ingress 2>/dev/null || true
    echo "Done."
}
trap cleanup EXIT

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root (sudo)" 
   exit 1
fi

# Experiment 1: Add 200ms latency + 10% packet loss
echo "Experiment 1: 200ms latency + 10% packet loss"
sudo tc qdisc add dev "$INTERFACE" root netem delay 200ms 50ms loss 10%
sleep "$DURATION"
cleanup

# Experiment 2: 500ms latency (simulate slow API)
echo "Experiment 2: 500ms latency (slow API)"
sudo tc qdisc add dev "$INTERFACE" root netem delay 500ms 100ms
sleep "$DURATION"
cleanup

# Experiment 3: 50% packet loss (unstable network)
echo "Experiment 3: 50% packet loss"
sudo tc qdisc add dev "$INTERFACE" root netem loss 50%
sleep "$DURATION"
cleanup

# Experiment 4: Bandwidth limit 1Mbps
echo "Experiment 4: Bandwidth limit 1Mbps"
sudo tc qdisc add dev "$INTERFACE" root tbf rate 1mbit burst 32kbit latency 400ms
sleep "$DURATION"
cleanup

# Experiment 5: If target IP provided, apply to specific destination
if [[ -n "$TARGET_IP" ]]; then
    echo "Experiment 5: Targeted chaos to $TARGET_IP"
    sudo tc qdisc add dev "$INTERFACE" root handle 1: prio
    sudo tc filter add dev "$INTERFACE" protocol ip parent 1: prio 1 u32 match ip dst "$TARGET_IP" flowid 1:1
    sudo tc qdisc add dev "$INTERFACE" parent 1:1 handle 10: netem delay 300ms loss 20%
    sleep "$DURATION"
    cleanup
fi

echo "=== All experiments completed ==="
