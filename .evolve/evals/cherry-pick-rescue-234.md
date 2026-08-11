# Eval: cherry-pick-rescue-234

**Slug:** `cherry-pick-rescue-234`
**Goal:** Cherry-pick `39e6b92` from `rescue/cycle-234-build` onto the cycle worktree, delivering phase-boundary checkpoints, cycle-level bridge-failure handling, and ship-closure idempotency.

---

## Acceptance Criteria

### AC1 — Go test suite green [code]
```bash
cd go && go test ./internal/... ./cmd/... -count=1 2>&1 | tail -5
# expect: no FAIL lines
```
**Negative case:** If any test FAILs, the cherry-pick introduced a regression.

### AC2 — phase-boundary-checkpoint new reason constant present [code]
```bash
grep -r "ReasonPhaseComplete" go/internal/checkpoint/checkpoint.go
# expect: at least one match
```

### AC3 — ErrCycleLevelFailure type defined [code]
```bash
grep -r "ErrCycleLevelFailure" go/internal/core/errors.go
# expect: at least one match
```

### AC4 — discardBinaryChurn present in ship/gitops.go [code]
```bash
grep -n "func discardBinaryChurn" go/internal/phases/ship/gitops.go
# expect: one match
```

### AC5 — ACS scripts present for this cycle [code]
```bash
ls acs/cycle-234/
# expect: 001-phase-boundary-checkpoint.sh 002-cycle-level-bridge-failure.sh 003-ship-closure-idempotency.sh 004-regression-trees-green.sh
```

### AC6 — Ship closure idempotency test present [code]
```bash
ls go/internal/phases/ship/closure_idempotency_test.go
# expect: file exists (exit 0)
```

### AC7 — PhaseBoundaryCheckpointer wired in checkpoint init [code]
```bash
grep -n "PhaseBoundaryCheckpointer" go/internal/checkpoint/checkpoint.go
# expect: at least one match (the init() hook)
```

### AC8 (edge) — cherry-pick does NOT introduce detached-HEAD state [code]
```bash
git branch --show-current
# expect: non-empty (not detached HEAD)
```

### AC9 (negative) — cycle-level failure type does NOT inherit batch-fatal errors [code]
```bash
grep -A8 "func wrapCycleLevelError" go/internal/core/orchestrator.go | grep "ErrPhaseGateFailed\|ErrLedgerChainBroken\|ErrLockHeld"
# expect: at least one match (these errors must NOT be wrapped into cycle-level)
```
