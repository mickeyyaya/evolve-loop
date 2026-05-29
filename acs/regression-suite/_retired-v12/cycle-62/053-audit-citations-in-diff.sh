#!/usr/bin/env bash
# ACS predicate 053 — cycle 62
# Verifies audit-citation-check.sh correctly distinguishes in-scope vs
# out-of-scope citations in audit-report.md.
#
# AC-ID: cycle-62-053
# Description: audit-citations-in-diff
# Evidence: 4 ACs — out-of-scope RED, in-scope GREEN, no-line skipped, anti-tautology
# Author: builder (manual fix, Step 5 of plan)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: plan Step 5 (B2)
#
# metadata:
#   id: 053-audit-citations-in-diff
#   cycle: 62
#   task: audit-citation-check
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
CHECK="$REPO_ROOT/scripts/lifecycle/audit-citation-check.sh"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

if [ ! -x "$CHECK" ]; then
    echo "RED PRE: audit-citation-check.sh missing or not executable at $CHECK"
    exit 1
fi

# ── AC1: out-of-scope citation → RED ──────────────────────────────────────────
OOS="$TMP/out-of-scope.md"
cat > "$OOS" << 'EOF'
# Audit Report — Test

## Evidence
Verified `scripts/cli_adapters/gemini.sh:206` shows the NATIVE-mode echo line.
EOF

if bash "$CHECK" "$OOS" --diff-files "scripts/dispatch/some-other.sh,docs/foo.md" >/dev/null 2>&1; then
    echo "RED AC1: check passed for out-of-scope citation (false negative)"
    rc=1
else
    echo "GREEN AC1: check correctly RED'd a citation outside diff scope"
fi

# ── AC2: in-scope citation → GREEN ────────────────────────────────────────────
INSCOPE="$TMP/in-scope.md"
cat > "$INSCOPE" << 'EOF'
# Audit Report — Test

## Evidence
Verified `scripts/cli_adapters/gemini.sh:206` is correct.
EOF

if bash "$CHECK" "$INSCOPE" --diff-files "scripts/cli_adapters/gemini.sh,docs/foo.md" >/dev/null 2>&1; then
    echo "GREEN AC2: check passed for in-scope citation"
else
    echo "RED AC2: check RED'd an in-scope citation (false positive)"
    rc=1
fi

# ── AC3: no path:line citations → GREEN (vacuous) ─────────────────────────────
NOCITES="$TMP/no-cites.md"
cat > "$NOCITES" << 'EOF'
# Audit Report — Test

## Evidence
General reference to scripts/foo.sh without a specific line cited.
The file `bar.sh` is mentioned by name but no :line evidence given.
EOF

if bash "$CHECK" "$NOCITES" --diff-files "anything,here" >/dev/null 2>&1; then
    echo "GREEN AC3: audit without path:line citations passes vacuously"
else
    echo "RED AC3: check RED'd an audit with no specific citations — over-strict"
    rc=1
fi

# ── AC4 (anti-tautology): regression replay against cycle-61 audit-report ────
# Cycle 61's audit cited scripts/cli_adapters/gemini.sh:206 but the file was
# NOT in cycle 61's diff (4160750). With diff-files matching cycle 61's actual
# committed file set, the check MUST exit 1.
C61_AUDIT="$REPO_ROOT/.evolve/runs/cycle-61/audit-report.md"
if [ -f "$C61_AUDIT" ]; then
    # Cycle 61's actual committed files (from git show 4160750 --name-only):
    C61_DIFF="acs/cycle-61/043-gemini-native-mode.sh,acs/regression-suite/cycle-60/040-e2e-mixed-cli-cycle.sh,acs/regression-suite/cycle-60/042-legacy-no-llm-config-cycle-completes.sh,scripts/cli_adapters/gemini.capabilities.json,scripts/dispatch/subagent-run.sh,scripts/tests/gemini-adapter-test.sh"
    if bash "$CHECK" "$C61_AUDIT" --diff-files "$C61_DIFF" >/dev/null 2>&1; then
        echo "RED AC4 (regression replay): cycle 61 audit passed against its real diff scope — check would NOT have caught the regression"
        rc=1
    else
        echo "GREEN AC4 (regression replay): cycle 61 audit correctly RED'd against its real diff scope (gemini.sh:206 not in commit)"
    fi
else
    echo "SKIP AC4: cycle-61 audit-report not present (workspace cleaned)"
fi

exit "$rc"
