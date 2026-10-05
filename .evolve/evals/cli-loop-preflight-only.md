---
score_cap:
  - criterion: "evolve loop --preflight-only runs the real readiness gate, persists .evolve/loop-preflight.json, exits 1 naming the blocking check, and writes no cycle-state, state, ledger or run lease"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_001' ./acs/cycle1791"
  - criterion: "--preflight-only exits 0 with READY and every check's verdict when no check halts, and 1 naming every halting check otherwise, leaving cycle-state.json untouched"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_002' ./acs/cycle1791"
  - criterion: "--preflight-only with --skip-preflight, --dry-run or --detach exits 10 as mutually exclusive and runs nothing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_003' ./acs/cycle1791"
  - criterion: "runtime-reference.md's Launch step documents a runnable evolve loop --preflight-only command"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_004' ./acs/cycle1791"
  - criterion: "the stubbed-gate unit contract passes: all-pass exits 0 READY honouring --skip-preflight-boot without reaching the launch path, a halt exits 1 naming each blocking check, the boundary re-exec handoff is left for the loop, conflicting modes exit 10"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1791_005' ./acs/cycle1791"
---

# Eval: evolve loop --preflight-only

> Pins `evolve loop --preflight-only`, cycle 1791: the loop's pre-batch readiness gate (`internal/looppreflight`) runs on its own, prints each check's verdict and dispatches nothing. Exit 0 means ready and exit 1 names the blocking check; a real launch's gate halt keeps exit 2. Before this change the gate ran only inside a real launch, so a failing check was found with the batch already started (inbox item cli-loop-preflight-only, 2026-09-30 CLI inventory). The flag must also leave the boundary re-exec handoff alone and refuse combinations that would check nothing or dispatch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| real-gate-halt | real gate runs, halt exits 1 naming disk-space, no run state | 7/10 | `go test -tags acs -run TestC1791_001 ./acs/cycle1791` |
| verdict-consistency | exit code agrees with the persisted verdict; READY on pass | 6/10 | `go test -tags acs -run TestC1791_002 ./acs/cycle1791` |
| mode-conflicts | --skip-preflight / --dry-run / --detach exit 10 | 7/10 | `go test -tags acs -run TestC1791_003 ./acs/cycle1791` |
| launch-step-doc | the documented command runs the gate | 5/10 | `go test -tags acs -run TestC1791_004 ./acs/cycle1791` |
| unit-contract | stubbed-gate pass/halt/handoff/conflict tests pass | 6/10 | `go test -tags acs -run TestC1791_005 ./acs/cycle1791` |
