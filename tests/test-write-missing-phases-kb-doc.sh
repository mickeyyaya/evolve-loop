#!/usr/bin/env bash
# tests/test-write-missing-phases-kb-doc.sh
#
# TDD RED suite for cycle-214 Task 3: write-missing-phases-kb-doc.
# A pure documentation deliverable — there is no "system under test" to invoke,
# so verification is content/presence assertions on the markdown file (the
# legitimate config/doc-check category). MUST FAIL at RED (file absent).
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
DOC="$ROOT/knowledge-base/research/missing-development-phases-2026-06-03.md"
REL="knowledge-base/research/missing-development-phases-2026-06-03.md"

PASS=0; FAIL=0
ok() { echo "PASS: $1"; PASS=$((PASS+1)); }
no() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

# --- AC3.1: file exists (+ git-tracked so it survives ship) -----------------
if [ -f "$DOC" ]; then ok "KB doc exists on disk"
else no "KB doc exists on disk"; fi

if git -C "$ROOT" ls-files --error-unmatch "$REL" >/dev/null 2>&1; then
  ok "KB doc is git-tracked"
else no "KB doc is git-tracked (untracked may be dropped at ship)"; fi

# --- AC3.5: >500 bytes of content -------------------------------------------
bytes=$([ -f "$DOC" ] && wc -c < "$DOC" | tr -d ' ' || echo 0)
if [ "${bytes:-0}" -gt 500 ]; then ok "KB doc > 500 bytes (got $bytes)"
else no "KB doc > 500 bytes (got ${bytes:-0})"; fi

# --- AC3.2: required structural sections ------------------------------------
# Match liberally (case-insensitive, heading or prose) to test intent not exact
# heading text. Each required theme must appear.
need_section() {
  local label="$1" pat="$2"
  if [ -f "$DOC" ] && grep -qiE "$pat" "$DOC"; then ok "section present: $label"
  else no "section present: $label"; fi
}
need_section "Research Findings" 'research findings'
need_section "Missing Phases"    'missing (development )?phases'
need_section "Phase Design Guide" 'phase design|how to (write|author).*phase\.json|design guide'

# --- AC3.3: covers the four named phases ------------------------------------
need_phase() {
  if [ -f "$DOC" ] && grep -qiE "$1" "$DOC"; then ok "covers $1"
  else no "covers $1"; fi
}
need_phase 'security-scan'
need_phase 'dependency-audit'
need_phase 'performance-bench'
need_phase 'post-ship-monitor'

# --- AC3.4 (positive half): affirms the optional-only safety rule -----------
# The strict negative ("does NOT recommend mandatory") is a judgment call
# dispositioned manual+checklist for the Auditor; here we positively assert the
# doc states user phases must be optional / the spine stays protected.
if [ -f "$DOC" ] && grep -qiE 'optional' "$DOC"; then
  ok "affirms optional-only user-phase safety rule"
else no "affirms optional-only user-phase safety rule"; fi

echo ""; echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
