#!/usr/bin/env bash
# ACS predicate 010 — cycle 54
# role-gate.sh enforces write permissions based on cycle phase, not CLI identity.
# When phase=research, writes to .evolve/state.json are DENIED (exit 2).
# When phase=learn, the same write is ALLOWED (exit 0).
# Architectural guarantee: trust-kernel enforcement is structurally CLI-independent;
# the same denial fires whether llm_config routes Scout to claude, gemini, or codex.
#
# AC-ID: cycle-54-010
# Description: role-gate enforces write permissions by phase, regardless of CLI
# Evidence: scripts/guards/role-gate.sh allow_for_phase()
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-4
#
# metadata:
#   id: 010-trust-kernel-cli-independent
#   cycle: 54
#   task: trust-kernel-cli-independence
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
ROLE_GATE="$REPO_ROOT/scripts/guards/role-gate.sh"

if [ ! -f "$ROLE_GATE" ]; then
    echo "RED: role-gate.sh not found at $ROLE_GATE"
    exit 1
fi

# ── Setup ─────────────────────────────────────────────────────────────────────
# The worktree lives under /var/folders on macOS, which is in role-gate.sh's
# always-safe directory list. To test phase enforcement correctly, we create a
# symlinked scripts/guards/ tree under $HOME (not always-safe), so role-gate.sh
# resolves its REPO_ROOT to a non-always-safe path. Pattern from role-gate-test.sh Test 22.
TMP_HOME_ROOT="$HOME/.evolve-predicate-010-test-$$"
mkdir -p "$TMP_HOME_ROOT/scripts/guards"
mkdir -p "$TMP_HOME_ROOT/.evolve/runs/cycle-test-54"
ln -sf "$ROLE_GATE" "$TMP_HOME_ROOT/scripts/guards/role-gate.sh"
# Symlinked invocation: BASH_SOURCE[0] resolves to the symlink path,
# so REPO_ROOT inside role-gate.sh = $TMP_HOME_ROOT (not the worktree root).
ROLE_GATE_VIA_SYMLINK="$TMP_HOME_ROOT/scripts/guards/role-gate.sh"

trap 'rm -rf "$TMP_HOME_ROOT"' EXIT

TMP_WS="$TMP_HOME_ROOT/.evolve/runs/cycle-test-54"
STATE_JSON="$TMP_HOME_ROOT/.evolve/state.json"

rc=0

# ── AC1: phase=research → state.json write DENIED (exit 2) ───────────────────
# Simulates a Scout agent (running on any CLI) attempting to write state.json
# — forbidden during research phase per role-gate allow_for_phase().
TMP_CS_RESEARCH="$TMP_HOME_ROOT/cycle-state-research.json"
cat > "$TMP_CS_RESEARCH" <<EOJSON
{
  "cycle_id": "54",
  "phase": "research",
  "workspace_path": "$TMP_WS"
}
EOJSON

PAYLOAD='{"tool_name":"Write","tool_input":{"file_path":"'"$STATE_JSON"'"}}'

gate_rc=0
printf '%s' "$PAYLOAD" | \
  EVOLVE_CYCLE_STATE_FILE="$TMP_CS_RESEARCH" \
  bash "$ROLE_GATE_VIA_SYMLINK" 2>/dev/null || gate_rc=$?

if [ "$gate_rc" -eq 2 ]; then
    echo "GREEN AC1: role-gate exits 2 (DENY) for state.json write during research phase"
else
    echo "RED AC1: expected exit 2, got $gate_rc — state.json write not denied in research phase"
    rc=1
fi

# ── AC2 (anti-tautology): phase=learn → state.json write ALLOWED (exit 0) ────
# Confirms the DENY is phase-specific: learn phase explicitly allows state.json.
TMP_CS_LEARN="$TMP_HOME_ROOT/cycle-state-learn.json"
cat > "$TMP_CS_LEARN" <<EOJSON
{
  "cycle_id": "54",
  "phase": "learn",
  "workspace_path": "$TMP_WS"
}
EOJSON

gate_rc2=0
printf '%s' "$PAYLOAD" | \
  EVOLVE_CYCLE_STATE_FILE="$TMP_CS_LEARN" \
  bash "$ROLE_GATE_VIA_SYMLINK" 2>/dev/null || gate_rc2=$?

if [ "$gate_rc2" -eq 0 ]; then
    echo "GREEN AC2: role-gate exits 0 (ALLOW) for state.json write during learn phase (anti-tautology passed)"
else
    echo "RED AC2: expected exit 0 for learn phase, got $gate_rc2 (anti-tautology failed)"
    rc=1
fi

exit "$rc"
