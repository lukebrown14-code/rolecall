#!/bin/sh
# Render the TUI's main screens and pop-ups to text files, for design review.
# Usage: scripts/snap.sh [out-dir] [WIDTHxHEIGHT]...   (default: a temp dir, 120x36 80x24)
set -eu
cd "$(dirname "$0")/.."
out=${1:-$(mktemp -d)}
[ $# -gt 0 ] && shift
sizes=${*:-120x36 80x24}
for size in $sizes; do
  ROLECALL_SNAP_DIR="$out" ROLECALL_SNAP_SIZE="$size" go test ./internal/ui/tests -run '^TestSnapshots$' -count=1 >/dev/null
done
echo "$out"
