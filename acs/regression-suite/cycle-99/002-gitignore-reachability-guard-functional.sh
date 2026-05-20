#!/usr/bin/env bash
# AC-ID: cycle-99-002-gitignore-reachability-guard-functional
# Description: Cycle-99 ships scripts/guards/gitignore-reachability-check.sh — a pre-Builder-handoff gate that exits 0 on git-tracked paths and non-zero with a path-naming diagnostic on .gitignore'd paths, preventing the cycle-92 silent-deliverable-drop failure mode.
# Evidence: scripts/guards/gitignore-reachability-check.sh ; subprocess invocation on positive (CLAUDE.md) and negative (synthesized .DS_Store) test fixtures
# Author: tdd-engineer (cycle-99)
# Created: 2026-05-20T14:00:00Z
# Acceptance-of: cycle-99/scout-report.md §3 T2 ; cycle-99/triage-decision.md scope[T2]
# AC-source: cycle-99/scout-report.md T2 ; triage-decision.md scope[T2]
# Behavioral predicate (TRUE behavioral — invokes the system under test):
#   Cycle-99 must add a pre-Builder-handoff gate at
#   scripts/guards/gitignore-reachability-check.sh that, given one or more
#   deliverable paths as arguments:
#     (a) exits 0 when ALL paths are reachable (not gitignored), and
#     (b) exits non-zero when ANY path is gitignored, naming the offending
#         path on stderr.
#   The gate prevents the cycle-92 failure mode where a deliverable matched
#   .gitignore and was silently dropped at ship.
#
# RED until Builder writes the guard script; GREEN once the guard exists,
# is executable, git-tracked, and behaves correctly on both inputs.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (guard present, tracked, exec, correctly distinguishes
#              tracked vs gitignored inputs)
#   1 = RED   (missing, untracked, non-exec, or wrong exit behavior)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

GUARD="scripts/guards/gitignore-reachability-check.sh"

fail_count=0

# (1) Dual-existence check.
if [ ! -f "$GUARD" ]; then
  echo "RED: $GUARD missing on disk" >&2
  exit 1  # Hard fail — no further checks possible.
fi
if ! git ls-files --error-unmatch "$GUARD" >/dev/null 2>&1; then
  echo "RED: $GUARD is not git-tracked" >&2
  fail_count=$(( fail_count + 1 ))
fi
if [ ! -x "$GUARD" ]; then
  echo "RED: $GUARD is not executable (chmod +x missing)" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (2) Behavioral — positive case: pass a known git-tracked, non-ignored path.
#     CLAUDE.md is git-tracked and not in .gitignore. Guard must exit 0.
#     We invoke via `bash` to bypass the missing-exec-bit issue if any, and
#     we treat exit codes 0 or success identically.
positive_path="CLAUDE.md"
if [ ! -f "$positive_path" ]; then
  echo "RED: test fixture $positive_path missing — cannot exercise guard" >&2
  fail_count=$(( fail_count + 1 ))
else
  if bash "$GUARD" "$positive_path" >/dev/null 2>&1; then
    : # expected — exit 0 on reachable path
  else
    echo "RED: $GUARD exited non-zero on git-tracked non-ignored path ($positive_path)" >&2
    fail_count=$(( fail_count + 1 ))
  fi
fi

# (3) Behavioral — negative case: pass a path that IS gitignored.
#     `.DS_Store` is explicitly listed at the top of repo .gitignore.
#     We create a temp .DS_Store under a unique sentinel dir, run the
#     guard, then clean up. Guard must exit non-zero.
sentinel_dir=".evolve/runs/cycle-99/_acs-002-fixture-$$"
sentinel_path="$sentinel_dir/.DS_Store"

mkdir -p "$sentinel_dir" 2>/dev/null
: > "$sentinel_path" 2>/dev/null

# Confirm the fixture path is actually ignored by the repo's .gitignore.
if ! git check-ignore -q "$sentinel_path" 2>/dev/null; then
  # Fixture path is unexpectedly reachable — fall back to a path we KNOW
  # is ignored by inspecting .gitignore for `.DS_Store`. If even that is
  # absent, skip the negative case rather than fail spuriously.
  echo "RED: fixture path $sentinel_path is not gitignored by repo's .gitignore (test fixture invalid)" >&2
  fail_count=$(( fail_count + 1 ))
else
  if bash "$GUARD" "$sentinel_path" >/dev/null 2>&1; then
    echo "RED: $GUARD exited 0 on gitignored path ($sentinel_path) — guard does not detect ignored deliverables" >&2
    fail_count=$(( fail_count + 1 ))
  fi
  # Capture stderr on a fresh invocation so we can assert the offending
  # path appears in the diagnostic (operators need to know WHICH path).
  guard_stderr=$(bash "$GUARD" "$sentinel_path" 2>&1 >/dev/null || true)
  case "$guard_stderr" in
    *"$sentinel_path"*) : ;;
    *)
      echo "RED: $GUARD diagnostic does not name the offending path; got: $guard_stderr" >&2
      fail_count=$(( fail_count + 1 ))
      ;;
  esac
fi

# Cleanup fixture regardless of pass/fail.
rm -rf "$sentinel_dir" 2>/dev/null || true

if [ "$fail_count" -ne 0 ]; then
  echo "RED: gitignore-reachability guard does not satisfy functional contract ($fail_count issue[s])" >&2
  exit 1
fi

echo "GREEN: $GUARD exists, tracked, executable; exits 0 on reachable input and non-zero with diagnostic on gitignored input"
exit 0
