#!/usr/bin/env bash
# AC-ID: cycle-94-005-trust-kernel-regression-guard
# AC-source: cycle-94/intent.md acceptance_check #6
# Behavioral predicate: cycle-93 trust-kernel regression suite must
# remain 5/5 PASS after the cycle-94 diffs land. P1 and L2 both touch
# scripts/dispatch/subagent-run.sh — the trust-kernel-adjacent script
# that role-gate, ship-gate, and the ledger-hash-chain depend on. This
# predicate is the regression wrapper.
#
# Rationale: per cycle-93 hardening (pre-merge tree-SHA verify +
# commit-SHA self-attestation), the trust kernel binds Builder's
# attested HEAD to the ship-gate verification step. Any structural
# change to subagent-run.sh that breaks ledger emission, challenge-
# token routing, or the SHA chain would silently corrupt the audit
# binding. Re-running the cycle-93 5/5 suite is the canonical guard.
#
# RED if the upstream suite reports anything other than "5/5 PASS";
# GREEN when full pass.
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}}"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

SUITE="tests/test-cycle-93-trust-kernel.sh"
if [ ! -x "$SUITE" ] && [ ! -f "$SUITE" ]; then
  echo "RED: $SUITE not found" >&2
  exit 1
fi

# Run the upstream suite and capture summary line.
out=$(bash "$SUITE" 2>&1)
rc=$?

# Suite prints `N/N PASS` on success (line 38 of the harness).
if [ "$rc" -ne 0 ]; then
  echo "RED: trust-kernel regression suite exited rc=$rc" >&2
  echo "$out" >&2
  exit 1
fi

if ! echo "$out" | grep -Eq '^[0-9]+/[0-9]+[[:space:]]+PASS$'; then
  echo "RED: trust-kernel summary line missing or malformed" >&2
  echo "$out" >&2
  exit 1
fi

# Require 5/5 specifically — partial pass is a regression.
if ! echo "$out" | grep -Eq '^5/5[[:space:]]+PASS$'; then
  echo "RED: trust-kernel regression suite did not report 5/5 PASS" >&2
  echo "$out" >&2
  exit 1
fi

echo "GREEN: trust-kernel suite 5/5 PASS — no regression on subagent-run.sh diffs"
exit 0
