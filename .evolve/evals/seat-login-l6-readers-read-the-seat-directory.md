---
score_cap:
  - criterion: "A seat started with SeatTokenFile writes under the seat root, and the spend reader and the post-mortem reader find its transcript and usage there"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSeatReadersJourney_' ./internal/bridge"
  - criterion: "evolve gc removes only the stale seat directory folder of a removed worktree and spares a live seat, the token holder and the settings"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2236_00[2-5]' ./acs/cycle2236"
  - criterion: "The plan row L6 states that the readers are built and no sentence of the plan says the readers are missing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2236_006' ./acs/cycle2236"
  - criterion: "The evolve auth usage text does not tell the operator that set must wait"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2236_007' ./acs/cycle2236"
---

# Eval: Seat login L6, the readers read the seat directory

> Pins the last gate before the operator runs `evolve auth seat-token set`. The spend reader and both post-mortem readers already look in the stored root and the seat root through `tokenusage.ClaudeTranscriptRoots` (PR 1375). This eval adds the journey proof for a seat started with `--seat-token-file`, the gc sweep of the seat directory, and the plan and usage wording. Source incident: the logout of 2026-11-09 (cycle 2232 router, three audit seats), recorded in the inbox item `seat-login-l6-readers-read-the-seat-directory`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| journey | Seat token file to both readers, under the seat root | 6/10 | `go test -run '^TestSeatReadersJourney_' ./internal/bridge` |
| owned state | gc sweeps stale seat folders, spares live seats and the holder | 6/10 | `go test -tags acs -run 'TestC2236_00[2-5]' ./acs/cycle2236` |
| plan | Row L6 says built, no stale sentence | 7/10 | `go test -tags acs -run TestC2236_006 ./acs/cycle2236` |
| usage | No wait wording in the auth usage | 8/10 | `go test -tags acs -run TestC2236_007 ./acs/cycle2236` |
