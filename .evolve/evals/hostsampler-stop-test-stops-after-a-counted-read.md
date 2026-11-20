---
score_cap:
  - criterion: "the stop test has no time.Sleep and starts no goroutine"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2574 -run 'TestC2574_00[12]_'"
  - criterion: "the reader sends the stop after a counted read"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2574 -run TestC2574_003_"
  - criterion: "the stop test passes 50 times under the race detector"
    max_if_missing: 7
    evidence: "cd go && go test -race -count=50 ./cmd/hostsampler/ -run '^TestRun_StopSignalEndsRunAndClosesFile$'"
  - criterion: "the sleep-sync baseline row of the stop test is retired and the ratchet is green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2574 -run 'TestC2574_00[67]_'"
---

# Eval: hostsampler stop test stops after a counted read

> The stop test slept 80ms before it sent SIGTERM, so a loaded runner could stop
> the run before the first sample. The stop must come from the reader after a counted read.
> Source: inbox item hostsampler-stop-test-stops-after-a-counted-read (cycle 2574).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-sleep | No time.Sleep or goroutine in the stop test | 6/10 | `TestC2574_001`, `TestC2574_002` |
| counted-read | Reader sends the stop after a counted read | 6/10 | `TestC2574_003` |
| race-stress | 50 passes under -race | 7/10 | `go test -race -count=50` |
| ratchet | Baseline row retired, ratchet green | 7/10 | `TestC2574_006`, `TestC2574_007` |
