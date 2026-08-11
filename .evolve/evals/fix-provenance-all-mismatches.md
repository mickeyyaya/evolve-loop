# Eval: fix-provenance-all-mismatches

## Task
Fix `CheckProvenance` in `go/internal/phasecoherence/provenance.go` to report ALL mismatches (both `tree_sha` AND `inputs_digest`) when both fields are wrong simultaneously.

## Acceptance Criteria

### [code] Amplification test passes — both-fields violation fires
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
out=$(cd "$WORKTREE/go" && go test -race -count=1 \
  -run 'TestCheckProvenance_BothTreeSHAAndInputsDigestMismatch' \
  ./internal/phasecoherence/ 2>&1)
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: TestCheckProvenance_BothTreeSHAAndInputsDigestMismatch failed (exit $rc)" >&2
  echo "$out" | tail -15 >&2
  exit 1
fi
echo "GREEN: both-fields amplification test passes" >&2
exit 0
```

### [code] Full phasecoherence package — no regression
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
out=$(cd "$WORKTREE/go" && go test -race -count=1 ./internal/phasecoherence/... 2>&1)
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: phasecoherence package regression (exit $rc)" >&2
  echo "$out" | tail -15 >&2
  exit 1
fi
echo "GREEN: full phasecoherence suite passes" >&2
exit 0
```

### [code] Negative — single-field mismatch still returns exactly one violation
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
out=$(cd "$WORKTREE/go" && go test -race -count=1 \
  -run 'TestProvenanceGate_TamperedPhase_ReturnsViolation|TestProvenanceGate_WrongCycle_ReturnsViolation' \
  ./internal/phasecoherence/ 2>&1)
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: single-field mismatch regression (exit $rc)" >&2
  echo "$out" | tail -10 >&2
  exit 1
fi
echo "GREEN: single-field mismatch tests still pass" >&2
exit 0
```

### [code] Edge — valid header with all fields matching returns zero violations
```bash
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
out=$(cd "$WORKTREE/go" && go test -race -count=1 \
  -run 'TestProvenanceGate_ValidHeader_NoViolation' \
  ./internal/phasecoherence/ 2>&1)
rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: valid-header test regressed (exit $rc)" >&2
  echo "$out" | tail -10 >&2
  exit 1
fi
echo "GREEN: valid-header zero-violation test passes" >&2
exit 0
```
