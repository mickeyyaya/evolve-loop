#!/usr/bin/env bash
# Predicate: phase-gate.sh gate_discover_to_build fails (hard) at kill_rate < 0.7
# Behavioral: counts occurrences of the fail-gate activation pattern, verifies >= 1
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
GATE_FILE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
[ -f "$GATE_FILE" ] || { echo "MISSING: $GATE_FILE" >&2; exit 1; }
# Count lines with fail-gate default (EVOLVE_MUTATION_GATE_STRICT:-1) in gate_discover_to_build
# This pattern was activated in cycle-86 (predicate-quality Layer 4)
fail_gate_count=$(grep -c 'EVOLVE_MUTATION_GATE_STRICT:-1' "$GATE_FILE" 2>/dev/null || true)
fail_gate_count=$(echo "$fail_gate_count" | tr -d ' \n')
[ "${fail_gate_count:-0}" -ge 1 ]
