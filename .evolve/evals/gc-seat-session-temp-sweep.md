---
score_cap:
  - criterion: "ReapSeatSessions removes the folder of a removed worktree over the age limit and counts its bytes"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2213_001_|TestC2213_006_' ./acs/cycle2213"
  - criterion: "A folder of a live worktree or a live session is kept, proven with real directories"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2213_002_|TestC2213_004_|TestC2213_005_' ./acs/cycle2213"
  - criterion: "evolve gc runs the sweep from the gc policy root and TTL and reports the space released"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestRunGC_.*SeatSession' ./cmd/evolve"
  - criterion: "The dry-run CLI preview counts only the removed worktree folder and takes its age limit from gc.temp_ttl_hours"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2213_008_|TestC2213_009_' ./acs/cycle2213"
---

# Eval: evolve gc sweeps the seat session folders under the Claude temp root

> Pins the contract of `gc.ReapSeatSessions` and its `evolve gc` step. On 2026-11-06 the Claude temp root held 46.7 GB in 1013 folders and no gc scope read it. The sweep removes only a folder of a removed worktree that is over the age limit and has no fresh file inside. Source incident: wave 158, where the disk reached 135 MB free.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| removal-and-bytes | A stale removed-worktree folder is removed and its bytes counted | 5/10 | `go test -tags acs -run TestC2213_001_` |
| liveness | A live worktree or live session folder is kept | 4/10 | `go test -tags acs -run TestC2213_002_` |
| wiring | `evolve gc` calls the sweep and prints the space released | 5/10 | `go test -run 'TestRunGC_.*SeatSession' ./cmd/evolve` |
| policy-ttl | The age limit comes from `gc.temp_ttl_hours` | 6/10 | `go test -tags acs -run TestC2213_009_` |
