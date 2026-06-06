#!/usr/bin/env bash
# tests/test-fix-posthoc-artifact-paths.sh — cycle-242 task 2 contract.
#
# Audit MEDIUM finding (cycle 241): POSTHOC templates reference
# builder-usage.json / builder-timing.json, but the runner actually emits
# build-usage.json / phase-timing.json. Templates must use the real names.
#
# Content-parity form only (per cycle-242 goal): assertions inspect file
# CONTENT in the working tree, never commit presence.
set -uo pipefail

TOP=$(git rev-parse --show-toplevel 2>/dev/null) || TOP=.
REF="$TOP/agents/evolve-builder-reference.md"
SCHEMA="$TOP/docs/architecture/posthoc-schema.md"

PASS=0; FAIL=0

ok()   { echo "PASS: $1"; PASS=$((PASS+1)); }
bad()  { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

count_in() { # count_in <pattern> <file>
  grep -c "$1" "$2" 2>/dev/null || true
}

# --- Negative assertions: the WRONG names must be gone (AC as written:
# grep -c 'builder-usage\.json' over both files returns 0) -----------------
for f in "$REF" "$SCHEMA"; do
  n=$(count_in 'builder-usage\.json' "$f")
  if [ "${n:-0}" -eq 0 ]; then
    ok "zero builder-usage.json references in ${f##*/}"
  else
    bad "${f##*/} still references builder-usage.json (${n}x)"
  fi
done

# builder-timing.json must also be absent (pre-existing GREEN at baseline —
# kept as a regression guard; scout step 7 says fix it wherever present).
for f in "$REF" "$SCHEMA"; do
  n=$(count_in 'builder-timing\.json' "$f")
  if [ "${n:-0}" -eq 0 ]; then
    ok "zero builder-timing.json references in ${f##*/}"
  else
    bad "${f##*/} still references builder-timing.json (${n}x)"
  fi
done

# --- Positive assertions: the CORRECT name must replace it (anti-deletion
# guard — wiping the POSTHOC lines would also zero the grep above) ---------
if grep -q 'build-usage\.json' "$REF"; then
  ok "evolve-builder-reference.md references build-usage.json"
else
  bad "evolve-builder-reference.md does not mention build-usage.json — line deleted instead of corrected?"
fi

if grep -q 'build-usage\.json' "$SCHEMA"; then
  ok "posthoc-schema.md references build-usage.json"
else
  bad "posthoc-schema.md does not mention build-usage.json — line deleted instead of corrected?"
fi

# POSTHOC sentinel machinery itself must survive the edit.
if grep -q 'POSTHOC: jq' "$REF"; then
  ok "POSTHOC jq sentinel still present in evolve-builder-reference.md"
else
  bad "POSTHOC jq sentinel removed from evolve-builder-reference.md"
fi

if grep -q 'POSTHOC: jq' "$SCHEMA"; then
  ok "POSTHOC jq sentinel still present in posthoc-schema.md"
else
  bad "POSTHOC jq sentinel removed from posthoc-schema.md"
fi

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
