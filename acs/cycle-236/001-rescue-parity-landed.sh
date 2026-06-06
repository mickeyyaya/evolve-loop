#!/usr/bin/env bash
# ACS — cycle 236 / AC1: rescue/cycle-235-audited feature commits landed with
# zero content drift on touched paths.
#
# Classification: BEHAVIORAL — invokes git (the system under test for a landing
# operation) and asserts on observable repo state: commit subjects in history,
# tree-content parity vs the rescue tip, and git tracking of new files. This is
# the AC as written in intent.md ("diff between cherry-picked result and 6f2e1af
# tree is empty for the touched paths").
#
# gofmt-modulo parity: go/internal/core/orchestrator_phaseboundary_test.go at
# 6f2e1af has an import-order gofmt -s nit (the cycle-233 failure mode that
# 81d2c2f had to clean up). AC3 demands gofmt -s CI parity, so .go files are
# compared after gofmt -s normalization of BOTH sides — permitting exactly the
# gofmt fix and nothing else. Non-go files must match byte-for-byte.
#
# NOTE: cycle-scoped predicate — references rescue-branch SHAs, which are valid
# while rescue/cycle-235-audited exists. The permanent regression pin lives in
# .evolve/evals/cherry-pick-rescue-235.md with SHA-free evidence.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel)
cd "$TOP"

RESCUE_TIP=6f2e1af6cd8549095f84b8e8511377ebf4d9c352
SCAFFOLD=568085ca505e7dde998e03c739cd185bcdffa2c8
fail=0

# 0. Rescue objects must be reachable for parity comparison.
if ! git cat-file -e "$RESCUE_TIP" 2>/dev/null; then
  echo "RED: rescue tip $RESCUE_TIP not in object db — cannot verify parity" >&2
  exit 1
fi

# 1. Both feature-commit subjects present in HEAD history (cherry-pick preserves
#    subjects). Pipe-free `git log --grep` on purpose: `git log | grep -q` under
#    `set -o pipefail` SIGPIPEs git log (exit 141) on an EARLY match → false RED
#    on present subjects, and silently swallows the inverted scaffold check.
if [ -z "$(git log --fixed-strings --grep='feat: failure supervision tree and ship idempotency' --format=%h)" ]; then
  echo "RED: e565834 subject (failure supervision tree and ship idempotency) not in git log" >&2
  fail=1
fi
if [ -z "$(git log --fixed-strings --grep='feat: implement phase-agnostic binary churn recovery in audit' --format=%h)" ]; then
  echo "RED: 6f2e1af subject (phase-agnostic binary churn recovery) not in git log" >&2
  fail=1
fi

# 2. NEGATIVE (scout BA2): the cycle-235 scaffold commit must NOT be cherry-picked.
if [ -n "$(git log --fixed-strings --grep='chore: scaffold cycle-235 tests' --format=%h)" ]; then
  echo "RED: scaffold 568085c was cherry-picked — BA2 violation (skip the scaffold)" >&2
  fail=1
fi
# NEGATIVE: cycle-235 run-scaffolding must not land (cycle-scoped, superseded).
if git ls-files --error-unmatch acs/cycle-235 >/dev/null 2>&1; then
  echo "RED: acs/cycle-235/ run-scaffolding landed — must be skipped" >&2
  fail=1
fi
if git ls-files --error-unmatch .evolve/evals/cherry-pick-rescue-234.md >/dev/null 2>&1; then
  echo "RED: superseded eval cherry-pick-rescue-234.md landed — must be skipped" >&2
  fail=1
fi

# 3. Content parity vs 6f2e1af tree on touched paths.
#    Path set = feature-commit diff (e565834 + 6f2e1af, i.e. SCAFFOLD..RESCUE_TIP)
#    PLUS two feature-integrity files that live only in the skipped scaffold
#    commit but belong to the features being landed (the 6f2e1af unit test and
#    the audit-leak eval — without them the feature lands orphaned):
PATHS="$(git diff --name-only "$SCAFFOLD" "$RESCUE_TIP")
go/internal/core/orchestrator_auditleak_test.go
.evolve/evals/audit-phase-leak-recover.md"

for f in $PATHS; do
  case "$f" in
    *.go)
      if ! cmp -s <(git show "$RESCUE_TIP:$f" | gofmt -s) <(cat "$f" 2>/dev/null | gofmt -s); then
        echo "RED: content drift (beyond gofmt -s) in $f vs $RESCUE_TIP" >&2
        fail=1
      fi
      ;;
    *)
      if ! git diff --quiet "$RESCUE_TIP" HEAD -- "$f"; then
        echo "RED: content drift in $f vs $RESCUE_TIP (or file missing)" >&2
        fail=1
      fi
      ;;
  esac
done

# 4. Dual-check (cycle-92/93 rule): new non-go artifacts must be ON DISK and
#    GIT-TRACKED — [ -f ] alone passes for gitignored worktree files that ship
#    would silently drop (.evolve/evals/ is gitignore-shadowed; needs add -f).
for f in \
  .evolve/evals/ship-closure-idempotency.md \
  .evolve/evals/cycle-level-bridge-failure.md \
  .evolve/evals/phase-boundary-checkpoint.md \
  .evolve/evals/audit-phase-leak-recover.md \
  acs/cycle-234/001-phase-boundary-checkpoint.sh \
  acs/cycle-234/002-cycle-level-bridge-failure.sh \
  acs/cycle-234/003-ship-closure-idempotency.sh \
  acs/cycle-234/004-regression-trees-green.sh
do
  if [ ! -f "$f" ]; then
    echo "RED: $f missing on disk" >&2
    fail=1
    continue
  fi
  if ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    echo "RED: $f untracked — gitignore shadow? needs git add -f" >&2
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "GREEN: rescue feature commits landed, scaffold skipped, zero drift (modulo gofmt -s) on all touched paths" >&2
exit 0
