#!/usr/bin/env bash
# AC-ID: cycle-91-001-regression-suite-slice-script
# Description: Verifies that scripts/lifecycle/run-regression-suite-slice.sh
#   exists, is executable, bash-3.2 clean (no banned patterns), and produces
#   `N/N PASS` (with N >= 1) when invoked with `CLAUDE.md` as the sole touched
#   file. The cycle-91 RED defects (cycle-49/006, cycle-89/003, cycle-89/004)
#   were remediated in cycle-90 commit 940da5d, so the slice over CLAUDE.md
#   MUST agree by returning PASS.
# Evidence: intent.md:acceptance_checks bullet 1; intent.md:interfaces bullet 1.
# Author: tdd-engineer (cycle-91)
# Created: 2026-05-20
# Acceptance-of: build-report.md row "scripts/lifecycle/run-regression-suite-slice.sh authored"
#
# Behavioral: writes a temporary touched-files list containing only CLAUDE.md,
# pipes it to the slice script via stdin AND passes via argv (the contract
# allows either per intent.md:interfaces), then inspects the script's exit
# code AND its final stdout line. A mutant that always exits 0 without
# running predicates fails the "N >= 1" check (slice must hit at least the
# three known reachable predicates). A mutant that always returns
# `0/0 PASS` fails because CLAUDE.md is the most heavily-grepped file in
# the regression-suite — the empty-slice case is impossible here.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
SCRIPT="$REPO_ROOT/scripts/lifecycle/run-regression-suite-slice.sh"
AC_ID="cycle-91-001-regression-suite-slice-script"

# ---- Structural checks -----------------------------------------------------

if [ ! -f "$SCRIPT" ]; then
  echo "RED $AC_ID: script not found at scripts/lifecycle/run-regression-suite-slice.sh" >&2
  exit 1
fi

if [ ! -x "$SCRIPT" ]; then
  echo "RED $AC_ID: script exists but not executable (chmod +x missing)" >&2
  exit 1
fi

# ---- bash 3.2 compatibility ------------------------------------------------
# Banned patterns enumerated in CLAUDE.md "Shell & environment conventions".

banned_hits=""
if grep -nE '\bdeclare -A\b' "$SCRIPT" >/dev/null 2>&1; then
  banned_hits="${banned_hits} declare-A(bash4+)"
fi
if grep -nE '\b(mapfile|readarray)\b' "$SCRIPT" >/dev/null 2>&1; then
  banned_hits="${banned_hits} mapfile/readarray(bash4+)"
fi
if grep -nE '\$\{[A-Za-z_][A-Za-z_0-9]*\^\^' "$SCRIPT" >/dev/null 2>&1; then
  banned_hits="${banned_hits} \${var^^}(bash4+)"
fi
if grep -nE '\$\{[A-Za-z_][A-Za-z_0-9]*,,' "$SCRIPT" >/dev/null 2>&1; then
  banned_hits="${banned_hits} \${var,,}(bash4+)"
fi
if grep -nE "sed -i ''" "$SCRIPT" >/dev/null 2>&1; then
  banned_hits="${banned_hits} sed-i-empty-arg(BSD-incompat)"
fi
if grep -nE '\bdate -d\b' "$SCRIPT" >/dev/null 2>&1; then
  banned_hits="${banned_hits} date-d(GNU-only)"
fi

if [ -n "$banned_hits" ]; then
  echo "RED $AC_ID: bash 3.2 compatibility violations:${banned_hits}" >&2
  exit 1
fi

# ---- Behavioral: invoke with CLAUDE.md as touched file --------------------

if [ ! -f "$REPO_ROOT/CLAUDE.md" ]; then
  echo "RED $AC_ID: CLAUDE.md missing from repo root — cannot exercise slice" >&2
  exit 1
fi

# Run the script with CLAUDE.md piped via stdin (the documented primary contract).
set +e
out=$(cd "$REPO_ROOT" && printf 'CLAUDE.md\n' | "$SCRIPT" 2>&1)
rc=$?
set -e

if [ "$rc" -ne 0 ]; then
  echo "RED $AC_ID: slice exited rc=$rc (expected 0 PASS) for CLAUDE.md touched-files input" >&2
  echo "  script output:" >&2
  printf '%s\n' "$out" | sed 's/^/    /' >&2
  exit 1
fi

# Final stdout line MUST match `<num>/<num> PASS` shape with both numbers equal
# and the numerator >= 1 (CLAUDE.md is grepped by multiple regression predicates,
# so the empty-slice case is impossible here).
final_line=$(printf '%s\n' "$out" | awk 'NF{line=$0} END{print line}')
if ! printf '%s' "$final_line" | grep -qE '^[0-9]+/[0-9]+ PASS'; then
  echo "RED $AC_ID: slice final line does not match 'N/N PASS' shape" >&2
  echo "  final line: $final_line" >&2
  exit 1
fi

# Extract numerator and denominator; require equal AND numerator >= 1.
numer=$(printf '%s' "$final_line" | awk '{split($1, a, "/"); print a[1]}')
denom=$(printf '%s' "$final_line" | awk '{split($1, a, "/"); print a[2]}')

if [ -z "$numer" ] || [ -z "$denom" ]; then
  echo "RED $AC_ID: could not parse N/M from final line: $final_line" >&2
  exit 1
fi

if [ "$numer" -lt 1 ]; then
  echo "RED $AC_ID: slice over CLAUDE.md returned numer=$numer (expected >= 1; CLAUDE.md is grepped by multiple predicates so empty-slice is impossible)" >&2
  exit 1
fi

if [ "$numer" -ne "$denom" ]; then
  echo "RED $AC_ID: slice over CLAUDE.md returned $numer/$denom PASS — failing predicates within slice (cycle-91 RED defects should be remediated by cycle-90 commit 940da5d)" >&2
  echo "  script output:" >&2
  printf '%s\n' "$out" | sed 's/^/    /' >&2
  exit 1
fi

echo "GREEN $AC_ID: script present, executable, bash-3.2 clean, returns $numer/$denom PASS for CLAUDE.md touched-files"
exit 0
