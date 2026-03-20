#!/usr/bin/env bash
# status.sh - Print evolve-loop state summary

STATE_FILE=".evolve/state.json"

if [ ! -f "$STATE_FILE" ]; then
    echo "No .evolve/state.json found."
    exit 0
fi

python3 -c "
import json
import sys

try:
    with open('$STATE_FILE', 'r') as f:
        data = json.load(f)
        cycle = data.get('lastCycleNumber', 'Unknown')
        mastery = data.get('mastery', {}).get('level', 'Unknown')
        print(f'Evolve-Loop Status:')
        print(f'  Cycle: {cycle}')
        print(f'  Mastery: {mastery}')
except Exception as e:
    print(f'Error reading state: {e}')
    sys.exit(1)
"
