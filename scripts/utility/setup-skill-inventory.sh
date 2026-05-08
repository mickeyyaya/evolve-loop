#!/usr/bin/env bash
#
# setup-skill-inventory.sh — Thin wrapper around setup_skill_inventory.py.
#
# Called from Phase 0 (CALIBRATE) to populate .evolve/skill-inventory.json
# with a deterministic, filesystem-backed map of every installed skill.
# Replaces LLM-side parsing of the system-reminder skill listing (see
# skills/evolve-loop/phases.md § "Skill Inventory").
#
# Usage:
#   bash scripts/utility/setup-skill-inventory.sh [--force] [--quiet]
#
# Exit codes:
#   0 = inventory written (or cache hit, still fresh)
#   1 = python3 missing or scan failed
#
# This script is safe to run outside a cycle. It does NOT touch state.json
# (phase-gate.sh tamper-detection stays intact). It writes exactly one file:
# .evolve/skill-inventory.json.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PY_SCRIPT="$SCRIPT_DIR/setup_skill_inventory.py"

if ! command -v python3 >/dev/null 2>&1; then
    echo "[setup-skill-inventory] FATAL: python3 not found in PATH" >&2
    exit 1
fi

if [ ! -f "$PY_SCRIPT" ]; then
    echo "[setup-skill-inventory] FATAL: $PY_SCRIPT missing" >&2
    exit 1
fi

mkdir -p "$PROJECT_ROOT/.evolve"

# Forward all flags (--force, --quiet) to the python script.
exec python3 "$PY_SCRIPT" --project-root "$PROJECT_ROOT" "$@"
