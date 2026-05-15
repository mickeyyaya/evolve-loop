#!/usr/bin/env bash
# ACS predicate 055 — cycle 62
# Verifies memo profile no longer permits Bash(cat:*), Bash(tail:*), Bash(head:*)
# — closes the shell-redirect-escape path observed in cycle 61 where memo
# wrote files at project root via `cat ... > memo_context.txt`.
#
# AC-ID: cycle-62-055
# Description: memo-no-shell-redirect-tools
# Evidence: 3 ACs — allowlist absence + persona update + Read present (substitution safe)
# Author: builder (manual fix, Step 7 of plan)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: plan Step 7 (B4)
#
# metadata:
#   id: 055-memo-no-shell-redirect-tools
#   cycle: 62
#   task: memo-profile-lockdown
#   severity: MEDIUM

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
MEMO_PROFILE="$REPO_ROOT/.evolve/profiles/memo.json"
MEMO_PERSONA="$REPO_ROOT/agents/evolve-memo.md"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

if [ ! -f "$MEMO_PROFILE" ]; then
    echo "RED PRE: memo profile not found at $MEMO_PROFILE"
    exit 1
fi
if [ ! -f "$MEMO_PERSONA" ]; then
    echo "RED PRE: memo persona not found at $MEMO_PERSONA"
    exit 1
fi

# ── AC1: profile allowlist contains no Bash(cat:*), Bash(tail:*), Bash(head:*) ─
forbidden=("cat" "tail" "head")
forbidden_count=0
for tool in "${forbidden[@]}"; do
    matches=$(jq -r --arg t "$tool" '.allowed_tools | map(select(test("^Bash\\(\($t):")))' "$MEMO_PROFILE" 2>/dev/null | jq length 2>/dev/null || echo 0)
    if [ "$matches" -gt 0 ]; then
        echo "RED AC1: memo allowlist still permits Bash($tool:*) — $matches occurrence(s)"
        forbidden_count=$((forbidden_count + matches))
        rc=1
    fi
done
if [ "$forbidden_count" = "0" ]; then
    echo "GREEN AC1: memo allowlist has no Bash(cat:*), Bash(tail:*), Bash(head:*)"
fi

# ── AC2: Read remains in allowlist (memo can still inspect files) ─────────────
if jq -e '.allowed_tools | any(. == "Read")' "$MEMO_PROFILE" >/dev/null 2>&1; then
    echo "GREEN AC2: Read tool remains in memo allowlist (substitution available)"
else
    echo "RED AC2: Read tool missing from memo allowlist — substitution path broken"
    rc=1
fi

# ── AC3 (anti-tautology): predicate would FAIL if Bash(cat:*) was added back ──
MUTANT=$(jq '.allowed_tools += ["Bash(cat:*)"]' "$MEMO_PROFILE" 2>/dev/null)
mutant_matches=$(echo "$MUTANT" | jq -r '.allowed_tools | map(select(test("^Bash\\(cat:")))' 2>/dev/null | jq length 2>/dev/null || echo 0)
if [ "$mutant_matches" -gt 0 ]; then
    echo "GREEN AC3 (anti-tautology): mutant profile (with Bash(cat:*) re-added) would trip AC1"
else
    echo "RED AC3 (anti-tautology): mutant lacks Bash(cat:*) — anti-tautology test is broken"
    rc=1
fi

# ── AC4: persona references Read (not cat) for file inspection ────────────────
if grep -qiE 'use\s+Read|prefer\s+Read|Read.*not.*cat|not.*cat.*Read' "$MEMO_PERSONA" 2>/dev/null; then
    echo "GREEN AC4: memo persona advises using Read (not cat)"
else
    echo "RED AC4: memo persona doesn't reference 'use Read (not cat)' guidance"
    rc=1
fi

exit "$rc"
