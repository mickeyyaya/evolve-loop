#!/usr/bin/env bash
# ACS predicate 050 — cycle 62
# Verifies that scripts/cli_adapters/gemini.sh has the full NATIVE-mode block
# (not just the 6-line stub), so the capability flag
# gemini.capabilities.json:non_interactive_prompt=true is backed by working code.
#
# AC-ID: cycle-62-050
# Description: gemini-native-block-shipped
# Evidence: presence of NATIVE block markers, --probe smoke, mutation anti-tautology
# Author: builder (manual fix, Step 2 of plan)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: plan Step 2 (B0)
#
# metadata:
#   id: 050-gemini-native-block-shipped
#   cycle: 62
#   task: gemini-native-restore
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
GEM="$REPO_ROOT/scripts/cli_adapters/gemini.sh"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

# ── AC1: NATIVE block has the argv builder array variable ─────────────────────
# Use grep -q (exit-status only) to avoid grep -c text-vs-exit-code confusion.
if grep -q '_g_argv' "$GEM" 2>/dev/null; then
    echo "GREEN AC1: gemini.sh has _g_argv array builder"
else
    echo "RED AC1: gemini.sh missing _g_argv — NATIVE block absent (stub only)"
    rc=1
fi

# ── AC2: NATIVE block translates output to claude envelope ────────────────────
# This marker distinguishes the full block from the stub (stub doesn't translate).
if grep -q '_claude_envelope' "$GEM" 2>/dev/null; then
    echo "GREEN AC2: gemini.sh translates output to claude envelope"
else
    echo "RED AC2: gemini.sh missing _claude_envelope translation — NATIVE block is stub-only"
    rc=1
fi

# ── AC3: gemini.sh --probe smoke exits 0 ──────────────────────────────────────
if bash "$GEM" --probe >/dev/null 2>&1; then
    echo "GREEN AC3: gemini.sh --probe exits 0 (adapter loads cleanly)"
else
    probe_rc=$?
    echo "RED AC3: gemini.sh --probe exited $probe_rc"
    rc=1
fi

# ── AC4 (anti-tautology): mutation test — stub copy must FAIL the AC1/AC2 ─────
# Build a mutant: copy gemini.sh and revert the NATIVE block to the 6-line stub.
MUTANT="$TMP/gemini-mutant.sh"
awk '
    /if \[ "\$_GEMINI_NATIVE_CAP" = "true" \]; then/ { skip=1 }
    skip && /^fi$/ {
        # Reset: emit the 6-line stub
        print "if [ \"$_GEMINI_NATIVE_CAP\" = \"true\" ]; then"
        print "    _GEMINI_BIN=$(detect_gemini_native 2>/dev/null) || _GEMINI_BIN=\"\""
        print "    if [ -n \"$_GEMINI_BIN\" ] && [ -x \"$_GEMINI_BIN\" ] && [ -n \"${PROMPT_FILE:-}\" ]; then"
        print "        echo \"[gemini-adapter] NATIVE mode: invoking gemini binary directly (cli_resolution=native)\" >&2"
        print "        exec \"$_GEMINI_BIN\" < \"$PROMPT_FILE\""
        print "    fi"
        print "fi"
        skip=0
        next
    }
    !skip { print }
' "$GEM" > "$MUTANT"

# Re-run AC1 against the mutant — must NOT find _g_argv
if ! grep -q '_g_argv' "$MUTANT" 2>/dev/null; then
    echo "GREEN AC4 (anti-tautology): stub mutant correctly lacks _g_argv (predicate would catch a regression)"
else
    echo "RED AC4 (anti-tautology): stub mutant still has _g_argv — predicate is tautological"
    rc=1
fi

exit "$rc"
