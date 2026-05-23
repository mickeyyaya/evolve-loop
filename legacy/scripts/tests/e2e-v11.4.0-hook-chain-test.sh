#!/usr/bin/env bash
#
# e2e-v11.4.0-hook-chain-test.sh — Live e2e for the v11.4.0 hook chain.
#
# Spawns a real Claude subprocess via agent-bridge and verifies the full
# chain that v11.4.0 wires together:
#
#   spawned Claude tool call
#       → PreToolUse hook fires
#           → legacy/scripts/guards/evolve-guard-dispatch.sh
#               → probes binary (#16 hardened-shim)
#               → execs `evolve guard <name>` (the rebuilt Go binary)
#                   → guard logic decides Allow/Deny
#                   → appendGuardsLog writes `[ts] [tag] ALLOW|DENY` line
#                     to .evolve/guards.log (byte-equivalent to bash)
#
# Workspace is isolated to a tmpdir: the spawned session's
# CLAUDE_PROJECT_DIR resolves to that tmpdir, so guards.log writes
# there — production .evolve/guards.log is untouched.
#
# Cost: ~$0.05 (Haiku, single tool call, ~50 tokens prompt). Gated on
# EVOLVE_E2E_LIVE_LLM=1 so CI / unit-test runs skip cleanly.
#
# Usage:
#   EVOLVE_E2E_LIVE_LLM=1 bash legacy/scripts/tests/e2e-v11.4.0-hook-chain-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BRIDGE="$REPO_ROOT/tools/agent-bridge/bin/bridge"
EVOLVE_BIN="$REPO_ROOT/go/bin/evolve"

PASS=0; FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Skip gates: env opt-in, binaries present, no conflicting auth state.

if [ "${EVOLVE_E2E_LIVE_LLM:-0}" != "1" ]; then
    echo "SKIP: EVOLVE_E2E_LIVE_LLM!=1 — set to 1 to run live LLM e2e (~\$0.05/run)"
    exit 0
fi

if ! command -v claude >/dev/null 2>&1; then
    echo "SKIP: claude binary not on PATH"
    exit 0
fi

if [ ! -x "$BRIDGE" ]; then
    echo "SKIP: bridge binary not at $BRIDGE"
    exit 0
fi

if [ ! -x "$EVOLVE_BIN" ]; then
    echo "SKIP: evolve binary not at $EVOLVE_BIN — run 'cd go && go build -o bin/evolve ./cmd/evolve'"
    exit 0
fi

if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
    echo "SKIP: ANTHROPIC_API_KEY is set — bridge refuses to run with API-key path active"
    exit 0
fi

# Isolated workspace. Spawned claude session's CLAUDE_PROJECT_DIR will
# resolve here, so its hooks write guards.log into $WS/.evolve/, not
# production $REPO_ROOT/.evolve/.
WS="$(mktemp -d "${TMPDIR:-/tmp}/evolve-e2e-XXXXXX")"
trap 'rm -rf "$WS"' EXIT

mkdir -p "$WS/.claude" "$WS/.evolve"

# Mirror the project's settings.json into the test workspace with
# $CLAUDE_PROJECT_DIR pre-substituted to the real $REPO_ROOT so the
# hook commands resolve to the real shim + scripts. (When claude sets
# CLAUDE_PROJECT_DIR=$WS in the env, the shim's own resolve logic
# still uses that as $repo_root — which is what we want: guards.log
# is then written under $WS/.evolve/.)
sed "s|\\\$CLAUDE_PROJECT_DIR|$REPO_ROOT|g" \
    "$REPO_ROOT/.claude/settings.json" >"$WS/.claude/settings.json"

cat >"$WS/profile.json" <<'JSON'
{
  "name": "v11.4.0-hook-verify",
  "model": "haiku",
  "allowed_tools": ["Bash"],
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
JSON

# Unique marker so we can confirm the spawned session actually ran the
# requested tool call (vs. asking us a question, refusing, etc.).
MARKER="hookchain-$(openssl rand -hex 4 2>/dev/null || date +%s)"

cat >"$WS/prompt.txt" <<PROMPT
You are a probe agent verifying a guard hook chain. Do EXACTLY this:

1. Use your Bash tool to run: echo "$MARKER"
2. After bash returns, stop. Do nothing else.

No narration. No further tool calls. No questions.
PROMPT

header "Phase 1: spawn claude via bridge (haiku, single Bash call)"
# Export EVOLVE_GO_BIN so the dispatch shim uses the explicit binary
# path rather than relying on $repo_root/go/bin/evolve resolution
# (which would fail since $WS doesn't contain go/bin/).
(
    cd "$WS"
    export EVOLVE_GO_BIN="$EVOLVE_BIN"
    "$BRIDGE" launch \
        --cli=claude-p \
        --profile="$WS/profile.json" \
        --model=haiku \
        --prompt-file="$WS/prompt.txt" \
        --workspace="$WS" \
        --stdout-log="$WS/claude-stdout.log" \
        --stderr-log="$WS/claude-stderr.log" \
        --artifact="$WS/artifact.md" \
        >"$WS/bridge-stdout.log" 2>"$WS/bridge-stderr.log"
)
bridge_rc=$?

if [ "$bridge_rc" -ne 0 ]; then
    echo "  bridge stdout:"
    sed 's/^/    /' "$WS/bridge-stdout.log" 2>/dev/null | head -20
    echo "  bridge stderr:"
    sed 's/^/    /' "$WS/bridge-stderr.log" 2>/dev/null | head -20
    echo "  claude stdout:"
    sed 's/^/    /' "$WS/claude-stdout.log" 2>/dev/null | head -20
    echo "  claude stderr:"
    sed 's/^/    /' "$WS/claude-stderr.log" 2>/dev/null | head -20
    fail_ "bridge launch rc=$bridge_rc"
else
    pass "bridge launch rc=0"
fi

header "Phase 2: spawned session executed exactly one Bash tool call"
# Note: claude-p stdout is the assistant's final text reply, NOT tool
# outputs — so the echo's $MARKER stdout won't appear there. The hard
# evidence is guards.log: a Bash call fires 4 hooks (ship-gate,
# phase-gate-pre, research-quota-gate, doc-deletion-guard), so exactly
# 4 audit lines = exactly 1 Bash tool call took the allow path.
WS_GUARDS_LOG="$WS/.evolve/guards.log"
if [ -f "$WS_GUARDS_LOG" ]; then
    audit_lines=$(wc -l <"$WS_GUARDS_LOG" | tr -d ' ')
    if [ "$audit_lines" -eq 4 ]; then
        pass "exactly 4 audit lines (1 Bash call × 4 Bash-matching hooks)"
    elif [ "$audit_lines" -gt 4 ]; then
        pass "$audit_lines audit lines (more than 1 Bash call — agent may have retried; acceptable)"
    else
        fail_ "only $audit_lines audit lines; expected ≥4 (agent didn't make a Bash call)"
    fi
else
    fail_ "no $WS_GUARDS_LOG — hook chain didn't fire"
fi

header "Phase 3: dispatch shim wrote to isolated guards.log"
WS_GUARDS_LOG="$WS/.evolve/guards.log"
if [ ! -f "$WS_GUARDS_LOG" ]; then
    fail_ "guards.log not present at $WS_GUARDS_LOG (hook chain didn't fire?)"
else
    line_count=$(wc -l <"$WS_GUARDS_LOG")
    if [ "$line_count" -lt 1 ]; then
        fail_ "guards.log empty"
    else
        pass "guards.log has $line_count audit lines"
    fi
fi

header "Phase 4: expected guard tags present (each hook fired at least once)"
# A Bash tool call should fire all four Bash-matching hooks. The Edit
# hook (role-gate) is on a different matcher, so doesn't fire here.
for tag in ship-gate phase-gate-pre research-quota-gate doc-deletion-guard; do
    if [ -f "$WS_GUARDS_LOG" ] && grep -q "\[$tag\] " "$WS_GUARDS_LOG"; then
        pass "[$tag] entry present"
    else
        fail_ "[$tag] entry missing"
    fi
done

header "Phase 5: audit-line format byte-equivalent to bash spec"
# Format: `[YYYY-MM-DDTHH:MM:SSZ] [tag] ALLOW` (DENY adds ": reason").
# Validate the first matching line shape.
if [ -f "$WS_GUARDS_LOG" ]; then
    first_line="$(head -1 "$WS_GUARDS_LOG")"
    if [[ "$first_line" =~ ^\[[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z\]\ \[[a-z-]+\]\ (ALLOW|DENY) ]]; then
        pass "audit line format matches spec: $first_line"
    else
        fail_ "audit line format malformed: $first_line"
    fi
fi

header "Phase 6: no unexpected DENYs (clean allow path for echo)"
if [ -f "$WS_GUARDS_LOG" ]; then
    denies="$(grep DENY "$WS_GUARDS_LOG" || true)"
    if [ -z "$denies" ]; then
        pass "no DENY entries (benign Bash call took allow path)"
    else
        echo "  unexpected DENY entries:"
        echo "$denies" | sed 's/^/    /'
        fail_ "$(echo "$denies" | wc -l | tr -d ' ') unexpected DENY entries"
    fi
fi

echo
echo "==================================================="
echo "e2e-v11.4.0-hook-chain-test.sh: $PASS PASS, $FAIL FAIL"
echo "==================================================="
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
