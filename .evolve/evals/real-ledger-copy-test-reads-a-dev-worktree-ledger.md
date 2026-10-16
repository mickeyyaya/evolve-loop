---
score_cap:
  - criterion: "A short ledger (2 lines) makes TestSeal_RealLedgerCopy skip naming path and line count"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1892_001 ./acs/cycle1892"
  - criterion: "A ledger of exactly 100 lines skips rather than failing the 101-line tail assertion"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1892_002 ./acs/cycle1892"
  - criterion: "A long ledger (150 lines) still seals and passes without skipping"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1892_003 ./acs/cycle1892"
---

# Eval: TestSeal_RealLedgerCopy must not read a short dev-worktree ledger as a failure

> Pins the skip contract of `TestSeal_RealLedgerCopy` in `go/internal/adapters/ledger/seal_test.go`. In a console dev worktree the first candidate ledger holds only a few lines, `Seal(100)` truncates nothing, and the test failed with `real-copy live tail = 2 lines, want 101`. Source: inbox item `real-ledger-copy-test-reads-a-dev-worktree-ledger`, cycle 1892.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| short-skips | 2-line ledger skips with path and count | 7/10 | `go test -tags acs -run TestC1892_001 ./acs/cycle1892` |
| boundary | 100-line ledger skips | 6/10 | `go test -tags acs -run TestC1892_002 ./acs/cycle1892` |
| long-seals | 150-line ledger seals and passes | 7/10 | `go test -tags acs -run TestC1892_003 ./acs/cycle1892` |
