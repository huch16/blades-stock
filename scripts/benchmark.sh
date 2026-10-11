#!/bin/bash
# Performance baseline script for Blades-Stock
# Usage: ./scripts/benchmark.sh

set -euo pipefail

BINARY="./blades-stock"
DATA_DIR="./data"
LOG_DIR="./logs"

echo "=== Blades-Stock Performance Baseline ==="
echo "Binary: $BINARY"
echo "Date: $(date)"
echo ""

# Check if binary exists
if [[ ! -f "$BINARY" ]]; then
    echo "Binary not found. Building..."
    go build -tags "desktop,production" -ldflags "-w -s" -o "$BINARY" .
fi

# 1. Startup time
echo "1. Measuring startup time..."
startup_times=()
for i in {1..5}; do
    start=$(date +%s%3N)
    timeout 5s "$BINARY" --version >/dev/null 2>&1 || true
    end=$(date +%s%3N)
    startup_times+=($((end - start)))
done
avg_startup=$(printf '%s\n' "${startup_times[@]}" | awk '{sum+=$1} END {print sum/NR}')
echo "   Average startup time: ${avg_startup}ms"

# 2. Database query performance (requires running instance)
echo ""
echo "2. Database query baseline (requires running server)..."
echo "   Skipping - requires running instance"

# 3. Memory usage
echo ""
echo "3. Memory usage (static analysis)..."
go tool nm -size "$BINARY" | head -20

# 4. Binary size
echo ""
echo "4. Binary size:"
ls -lh "$BINARY"

# 5. Build time
echo ""
echo "5. Build time measurement..."
start=$(date +%s%3N)
go build -tags "desktop,production" -ldflags "-w -s" -o "$BINARY.tmp" . 2>/dev/null
end=$(date +%s%3N)
build_time=$((end - start))
rm -f "$BINARY.tmp"
echo "   Build time: ${build_time}ms"

echo ""
echo "=== Baseline Complete ==="
echo "Record these values for regression tracking:"
echo "  startup_avg_ms: $avg_startup"
echo "  build_time_ms: $build_time"
