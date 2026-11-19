---
score_cap:
  - criterion: "A fence-reverted patch stays readable at the path the agent run WARN prints"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run TestC2538_001 ./acs/cycle2538"
  - criterion: "git apply of the kept patch restores the reverted writes on the dispatched tree"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2538_002 ./acs/cycle2538"
  - criterion: "A run that reverts nothing leaves no patch and removes its workspace"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2538_003 ./acs/cycle2538"
---

# Eval: evolve agent run keeps the fence-reverted patch

> Pins the contract that `evolve agent run` leaves the `fence-reverted.patch` named by its own WARN line after the run returns. Source incident: cycle 2538. The run removed the workspace that held the patch, so a builder's reverted work was lost.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| patch-survives | The patch exists at the printed path after the run | 3/10 | `go test -tags acs -run TestC2538_001 ./acs/cycle2538` |
| patch-applies | `git apply` restores the reverted write | 4/10 | `go test -tags acs -run TestC2538_002 ./acs/cycle2538` |
| clean-run | No patch and no workspace when nothing was reverted | 6/10 | `go test -tags acs -run TestC2538_003 ./acs/cycle2538` |
