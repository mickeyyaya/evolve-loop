---
score_cap:
  - criterion: "The cycle-start hook runs the cache trim before the disk check, and a trim failure only warns"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2542_001' ./acs/cycle2542"
  - criterion: "At 9% free the guard prints a WARN line and dispatch goes on; at 20% it is silent"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2542_002' ./acs/cycle2542"
  - criterion: "At 4% free the guard pauses dispatch and at 6% it resumes, and a cancelled context ends the pause"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2542_003' ./acs/cycle2542"
  - criterion: "A failed or zero-total free-space read prints a WARN line and does not pause"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2542_004' ./acs/cycle2542"
  - criterion: "Production code in cmd/evolve calls looppreflight.RunCycleStart from LaunchFn and from the sequential cycle path"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2542_006' ./acs/cycle2542"
---

# Eval: Cycle-start cache trim and percent-free disk guard (practice M4)

> Pins the host self-heal of plan model-capability-practice component M4: the build cache trim runs at each cycle start, and a percent-free disk guard warns at 10% free and pauses new dispatches at 5% free until space returns. The guard reads free space through an injected seam, so tests use no real disk. Source incident: cycle 2542 inbox item model-capability-practice-m4-host-self-heal.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| trim-at-start | Trim runs once, first, and a failure does not stop dispatch | 6/10 | `go test -tags acs -run TestC2542_001 ./acs/cycle2542` |
| warn-band | 9% free warns and dispatch goes on | 7/10 | `go test -tags acs -run TestC2542_002 ./acs/cycle2542` |
| pause-resume | 4% pauses, 6% resumes, cancel ends the pause | 7/10 | `go test -tags acs -run TestC2542_003 ./acs/cycle2542` |
| read-failure | A failed read warns and does not pause | 7/10 | `go test -tags acs -run TestC2542_004 ./acs/cycle2542` |
| caller-proof | Production callers reach RunCycleStart | 6/10 | `go test -tags acs -run TestC2542_006 ./acs/cycle2542` |
