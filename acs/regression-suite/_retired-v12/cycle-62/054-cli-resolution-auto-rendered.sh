#!/usr/bin/env bash
# ACS predicate 054 — cycle 62
# Verifies render-cli-resolution.sh produces a CLI Resolution markdown section
# byte-stable from ledger entries.
#
# AC-ID: cycle-62-054
# Description: cli-resolution-auto-rendered
# Evidence: 4 ACs — section structure, ledger fidelity, anti-tautology, empty cycle
# Author: builder (manual fix, Step 6 of plan)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: plan Step 6 (B6)
#
# metadata:
#   id: 054-cli-resolution-auto-rendered
#   cycle: 62
#   task: cli-resolution-renderer
#   severity: LOW

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RENDER="$REPO_ROOT/scripts/observability/render-cli-resolution.sh"
LEDGER="$REPO_ROOT/.evolve/ledger.jsonl"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

if [ ! -x "$RENDER" ]; then
    echo "RED PRE: render-cli-resolution.sh missing or not executable at $RENDER"
    exit 1
fi

# ── AC1: rendered output has expected H2 + table header ───────────────────────
OUTPUT=$(bash "$RENDER" 61 2>/dev/null)
if echo "$OUTPUT" | grep -qE '^## CLI Resolution' && \
   echo "$OUTPUT" | grep -qE 'Phase \| Actual CLI \| Actual Model \| Source'; then
    echo "GREEN AC1: output has '## CLI Resolution' H2 and table header"
else
    echo "RED AC1: output missing required structure (H2 or table header)"
    rc=1
fi

# ── AC2: cycle 61 row for scout shows gemini → cli_resolution=llm_config ─────
# This is the load-bearing fidelity check: the gemini routing happened, and
# the renderer must surface it.
if echo "$OUTPUT" | grep -qE '\| scout \| gemini \|.*\| llm_config \|'; then
    echo "GREEN AC2: cycle 61 scout row correctly reports gemini + llm_config source"
else
    echo "RED AC2: cycle 61 scout row missing or mis-rendered"
    echo "  output:"
    echo "$OUTPUT" | grep -E '\| scout' | head -3 | sed 's/^/    /'
    rc=1
fi

# ── AC3: anti-tautology — empty cycle produces placeholder, not data ──────────
EMPTY_OUTPUT=$(bash "$RENDER" 99999 2>/dev/null)
if echo "$EMPTY_OUTPUT" | grep -qE 'no agent_subprocess ledger entries for cycle 99999'; then
    echo "GREEN AC3 (anti-tautology): empty cycle correctly produces placeholder"
else
    echo "RED AC3 (anti-tautology): empty cycle did NOT produce placeholder"
    echo "  got: $(echo "$EMPTY_OUTPUT" | tail -3)"
    rc=1
fi

# ── AC4: missing ledger handled gracefully ────────────────────────────────────
NOLEDGER_OUTPUT=$(EVOLVE_LEDGER=/dev/null bash "$RENDER" 61 2>/dev/null || true)
# /dev/null counts as missing file or empty file depending on path resolution;
# either way the script should produce SOME placeholder (not crash).
if echo "$NOLEDGER_OUTPUT" | grep -qE '## CLI Resolution'; then
    echo "GREEN AC4: missing/empty ledger handled gracefully (no crash, placeholder emitted)"
else
    echo "RED AC4: missing ledger caused output to lack H2 — renderer crashed"
    rc=1
fi

exit "$rc"
