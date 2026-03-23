#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_SCRIPT="$SCRIPT_DIR/evolve-status.py"

if [ ! -f "$PYTHON_SCRIPT" ]; then
    echo "Error: evolve-status.py not found at $PYTHON_SCRIPT"
    exit 1
fi

python3 "$PYTHON_SCRIPT" "$@"
