#!/usr/bin/env bash
# ACS predicate 007 — cycle 53
# When a capability is missing (supports.*=false), subagent-run.sh emits
# exactly one parseable WARN line per missing cap AND writes a dispatch plan
# JSON (EVOLVE_DISPATCH_PLAN_LOG) with capability_warns array matching
# the expected format.
#
# AC-ID: cycle-53-007
# Description: structured WARN lines emitted per degraded capability;
#   dispatch plan JSON records all WARNs in machine-readable form
# Evidence: scripts/dispatch/subagent-run.sh
# Author: builder (evolve-builder)
# Created: 2026-05-14T00:00:00Z
# Acceptance-of: build-report.md AC-2
#
# metadata:
#   id: 007-degraded-mode-emits-structured-warn
#   cycle: 53
#   task: capability-matrix-files
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

if [ ! -f "$SUBAGENT_RUN" ]; then
    echo "RED: subagent-run.sh not found at $SUBAGENT_RUN"
    exit 1
fi

# ── Setup ─────────────────────────────────────────────────────────────────────
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# llm_config routing scout→gemini (has budget_cap_native=false AND permission_scoping=false)
TMP_CONFIG="$TMP_DIR/llm_config.json"
cat > "$TMP_CONFIG" <<'EOJSON'
{"schema_version":1,"phases":{"scout":{"cli":"gemini","model":"gemini-3-pro-preview"}}}
EOJSON

# Sentinel adapter — just exits 0
SENTINEL_DIR="$TMP_DIR/adapters"
mkdir -p "$SENTINEL_DIR"
cat > "$SENTINEL_DIR/gemini.sh" <<'EOADA'
#!/usr/bin/env bash
exit 0
EOADA
chmod +x "$SENTINEL_DIR/gemini.sh"

PLAN_LOG="$TMP_DIR/dispatch-plan.json"

# Run --validate-profile with EVOLVE_DISPATCH_PLAN_LOG
stderr_out=$(EVOLVE_TESTING=1 \
  EVOLVE_GEMINI_CLAUDE_PATH="" \
  EVOLVE_LLM_CONFIG_PATH="$TMP_CONFIG" \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$SENTINEL_DIR" \
  EVOLVE_DISPATCH_PLAN_LOG="$PLAN_LOG" \
  bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 >/dev/null) || true

rc=0

# ── AC1: dispatch plan file exists ───────────────────────────────────────────
if [ ! -f "$PLAN_LOG" ]; then
    echo "RED AC1: EVOLVE_DISPATCH_PLAN_LOG not written (EGPS dispatch plan feature missing)"
    rc=1
else
    echo "GREEN AC1: dispatch plan log written at $PLAN_LOG"
fi

# ── AC2: plan JSON is valid and has capability_warns array ───────────────────
if [ -f "$PLAN_LOG" ] && command -v jq >/dev/null 2>&1; then
    if ! jq empty "$PLAN_LOG" 2>/dev/null; then
        echo "RED AC2: dispatch plan log is not valid JSON"
        rc=1
    else
        warn_count=$(jq '.capability_warns | length' "$PLAN_LOG" 2>/dev/null || echo "0")
        if [ "$warn_count" -eq 0 ]; then
            echo "RED AC2: capability_warns array is empty — expected at least budget_cap_native warn"
            echo "  plan JSON: $(cat "$PLAN_LOG")"
            rc=1
        else
            echo "GREEN AC2: capability_warns has $warn_count entries"
        fi
    fi
fi

# ── AC3: each warn entry matches format regex ─────────────────────────────────
if [ -f "$PLAN_LOG" ] && command -v jq >/dev/null 2>&1 && jq empty "$PLAN_LOG" 2>/dev/null; then
    warn_count=$(jq '.capability_warns | length' "$PLAN_LOG" 2>/dev/null || echo "0")
    format_ok=1
    i=0
    while [ "$i" -lt "$warn_count" ]; do
        entry=$(jq -r ".capability_warns[$i]" "$PLAN_LOG" 2>/dev/null || echo "")
        # Format: "cli=<name> missing=<cap_name> substitute=<sub>"
        if ! printf '%s' "$entry" | grep -qE '^cli=[a-z]+ missing=[a-z_]+ substitute=[a-z_]+$'; then
            echo "RED AC3: warn entry[$i] does not match format: '$entry'"
            format_ok=0
            rc=1
        fi
        i=$((i + 1))
    done
    [ "$format_ok" = "1" ] && echo "GREEN AC3: all warn entries match required format"
fi

# ── AC4: WARN lines in stderr match capability_warns one-to-one ──────────────
if [ -f "$PLAN_LOG" ] && command -v jq >/dev/null 2>&1 && jq empty "$PLAN_LOG" 2>/dev/null; then
    warn_count=$(jq '.capability_warns | length' "$PLAN_LOG" 2>/dev/null || echo "0")
    stderr_warn_count=$(printf '%s\n' "$stderr_out" | grep -c "\[adapter-cap\] WARN" || echo "0")
    if [ "$warn_count" != "$stderr_warn_count" ]; then
        echo "RED AC4: dispatch plan has $warn_count warns but stderr has $stderr_warn_count WARN lines"
        rc=1
    else
        echo "GREEN AC4: warn count matches between plan ($warn_count) and stderr ($stderr_warn_count)"
    fi
fi

# ── AC5 (anti-tautology): no warns written for claude (full capabilities) ────
TMP_CONFIG2="$TMP_DIR/llm_config_claude.json"
cat > "$TMP_CONFIG2" <<'EOJSON'
{"schema_version":1,"phases":{"scout":{"cli":"claude","model":"claude-sonnet-4-6"}}}
EOJSON
cat > "$SENTINEL_DIR/claude.sh" <<'EOADA'
#!/usr/bin/env bash
exit 0
EOADA
chmod +x "$SENTINEL_DIR/claude.sh"
PLAN_LOG2="$TMP_DIR/dispatch-plan-claude.json"

EVOLVE_TESTING=1 \
  EVOLVE_LLM_CONFIG_PATH="$TMP_CONFIG2" \
  EVOLVE_ADAPTERS_DIR_OVERRIDE="$SENTINEL_DIR" \
  EVOLVE_DISPATCH_PLAN_LOG="$PLAN_LOG2" \
  bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 >/dev/null || true

if [ -f "$PLAN_LOG2" ] && command -v jq >/dev/null 2>&1 && jq empty "$PLAN_LOG2" 2>/dev/null; then
    claude_warn_count=$(jq '.capability_warns | length' "$PLAN_LOG2" 2>/dev/null || echo "0")
    if [ "$claude_warn_count" -gt 0 ]; then
        echo "RED AC5 (anti-tautology): claude dispatch plan has $claude_warn_count warns — should be 0"
        rc=1
    else
        echo "GREEN AC5: claude dispatch plan has 0 warns (anti-tautology passed)"
    fi
fi

exit "$rc"
