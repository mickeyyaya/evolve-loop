#!/usr/bin/env bash
#
# run-claude-tmux-probe.sh — one-shot probe of the claude-tmux prototype adapter.
#
# Synthesizes a minimal profile + prompt with an embedded challenge token,
# takes snapshots before/after, invokes the adapter, verifies artifact and
# token, runs the billing-mode comparison. Prints an overall GO / NO-GO.
#
# Usage:
#   bash scripts/utility/run-claude-tmux-probe.sh [MODEL]
#
# MODEL defaults to "haiku" (cheapest). Use "sonnet" or "opus" to test other tiers.
#
# Exit codes:
#   0  GO     — billing PASS, artifact OK, adapter rc=0
#   1  NO-GO  — any one of the above failed
#

set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ADAPTER="$REPO/scripts/cli_adapters/claude-tmux.sh"
VERIFIER="$REPO/scripts/utility/verify-subscription-billing.sh"

MODEL="${1:-haiku}"

# Sanity checks
[ -x "$ADAPTER" ]  || { echo "[probe] ERROR: adapter not executable: $ADAPTER"  >&2; exit 1; }
[ -x "$VERIFIER" ] || { echo "[probe] ERROR: verifier not executable: $VERIFIER" >&2; exit 1; }

# Preflight: cost-leak env check
if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
    echo "[probe] ABORT: ANTHROPIC_API_KEY is set (would invalidate billing test)" >&2
    echo "         unset ANTHROPIC_API_KEY and re-run" >&2
    exit 1
fi
if [ -n "${ANTHROPIC_BASE_URL:-}" ] || [ -n "${EVOLVE_ANTHROPIC_BASE_URL:-}" ]; then
    echo "[probe] ABORT: proxy endpoint env set (would invalidate billing test)" >&2
    exit 1
fi

# Probe workspace
WS="$REPO/.evolve/tmp/tmux-probe-$$-$(date +%s)"
mkdir -p "$WS/snaps"
echo "[probe] workspace: $WS"

# Synthesize a minimal profile JSON
PROFILE="$WS/profile.json"
cat > "$PROFILE" <<EOF
{
  "name": "probe",
  "role": "probe",
  "cli": "claude-tmux",
  "model_tier_default": "$MODEL",
  "allowed_tools": ["Write", "Read"],
  "disallowed_tools": [],
  "add_dir": [],
  "extra_flags": [],
  "max_budget_usd": 0.10,
  "max_turns": 1,
  "permission_mode": "bypassPermissions",
  "schema_filter_enabled": false,
  "challenge_token_required": true
}
EOF

# Synthesize a challenge token
TOKEN=$(openssl rand -hex 8)
echo "[probe] challenge_token=$TOKEN"

# Artifact and logs
ARTIFACT="$WS/probe-artifact.md"
STDOUT_LOG="$WS/stdout.log"
STDERR_LOG="$WS/stderr.log"

# Synthesize a prompt that asks the agent to write the artifact with the token
PROMPT="$WS/prompt.txt"
cat > "$PROMPT" <<EOF
You are running as the tmux-billing-probe subagent for evolve-loop.

This is a billing-verification test. Your only job is to write a small file to disk and exit.

MANDATORY OUTPUT CONTRACT:
- Use your Write tool to create the file at this EXACT path:
  $ARTIFACT
- The file must contain exactly these two lines (no extra text):

<!-- challenge-token: $TOKEN -->
PROTOTYPE OK 12345

That is the entire task. Do not call any other tools. Do not explain. Just Write the file and stop.
EOF

# --- Snapshot BEFORE ---------------------------------------------------------
echo "[probe] taking BEFORE snapshot..."
SNAP_BEFORE=$(bash "$VERIFIER" snapshot "$WS/snaps" "before")
echo "[probe] BEFORE = $SNAP_BEFORE"

# --- Invoke the adapter ------------------------------------------------------
echo "[probe] invoking claude-tmux.sh ..."
echo ""
EVOLVE_TMUX_PROTOTYPE_ALLOW_BYPASS=1 \
PROFILE_PATH="$PROFILE" \
RESOLVED_MODEL="$MODEL" \
PROMPT_FILE="$PROMPT" \
CYCLE="0" \
WORKSPACE_PATH="$WS" \
STDOUT_LOG="$STDOUT_LOG" \
STDERR_LOG="$STDERR_LOG" \
ARTIFACT_PATH="$ARTIFACT" \
AGENT="probe" \
bash "$ADAPTER"
RC=$?
echo ""
echo "[probe] adapter rc=$RC"

# --- Snapshot AFTER ----------------------------------------------------------
echo "[probe] taking AFTER snapshot..."
SNAP_AFTER=$(bash "$VERIFIER" snapshot "$WS/snaps" "after")
echo "[probe] AFTER = $SNAP_AFTER"

# --- Verify artifact + token -------------------------------------------------
if [ -f "$ARTIFACT" ] && grep -q "$TOKEN" "$ARTIFACT"; then
    echo "[probe] ARTIFACT OK (token present): $ARTIFACT"
    echo "[probe] artifact contents:"
    sed 's/^/  | /' "$ARTIFACT"
    ARTIFACT_OK=1
else
    echo "[probe] ARTIFACT FAIL (missing or token absent at $ARTIFACT)"
    if [ -f "$ARTIFACT" ]; then
        echo "[probe] file exists but token missing; contents:"
        sed 's/^/  | /' "$ARTIFACT"
    fi
    ARTIFACT_OK=0
fi

# --- Compare billing snapshots -----------------------------------------------
echo ""
echo "[probe] running billing comparison..."
bash "$VERIFIER" compare "$SNAP_BEFORE" "$SNAP_AFTER"
BILLING_RC=$?

# --- Overall verdict ---------------------------------------------------------
echo ""
echo "================ PROBE SUMMARY ================"
echo "  workspace:        $WS"
echo "  model used:       $MODEL"
echo "  adapter rc:       $RC          (0=ok, 2=safety-gate, 3=env-leak, 80=REPL-timeout, 81=artifact-timeout)"
echo "  artifact written: $ARTIFACT_OK (1=ok, 0=missing/no-token)"
echo "  billing rc:       $BILLING_RC  (0=PASS, 1=FAIL, 2=INCONCLUSIVE)"

if [ "$RC" = "0" ] && [ "$ARTIFACT_OK" = "1" ] && [ "$BILLING_RC" = "0" ]; then
    echo "  OVERALL:          GO"
    echo ""
    echo "  Next step: take a manual screenshot of console.anthropic.com BEFORE and AFTER"
    echo "             to confirm the subscription quota actually decremented and API credits"
    echo "             stayed unchanged. The automated verifier can only see local file state."
    exit 0
else
    echo "  OVERALL:          NO-GO"
    echo ""
    echo "  See logs in $WS for diagnostic detail. Document findings in"
    echo "  docs/research/tmux-claude-driver-prototype.md before re-attempting."
    exit 1
fi
