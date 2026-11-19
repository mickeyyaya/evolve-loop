---
score_cap:
  - criterion: "The enforce control survives a peer that enters Run 2s late"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2524_001 ./acs/cycle2524"
  - criterion: "The shadow test uses no time.Sleep and a rendezvous bound of at least 5s"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2524_002 ./acs/cycle2524"
  - criterion: "A concurrent shadow dispatch still turns the serial test red"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2524_003 ./acs/cycle2524"
  - criterion: "The serial and not-off tests stay green 50 times under -race at GOMAXPROCS 1 and 2"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2524_004 ./acs/cycle2524"
  - criterion: "The change touches no Go source beside the shadow test file"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2524_005 ./acs/cycle2524"
---

# Eval: shadow-batch overlap test proves overlap with a rendezvous

> Pins the fix for the sleep-timed overlap detector in `evaluate_batch_shadow_test.go`, which flaked the boundary train under load (cycle 2492).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| slow-scheduler | late peer does not fail the control | 6/10 | `go test -tags acs -run TestC2524_001 ./acs/cycle2524` |
| no-sleep | no time.Sleep, bound at least 5s | 6/10 | `go test -tags acs -run TestC2524_002 ./acs/cycle2524` |
| mutant-red | concurrent shadow turns the test red | 7/10 | `go test -tags acs -run TestC2524_003 ./acs/cycle2524` |
| repeat-green | 50 runs under -race at GOMAXPROCS 1 and 2 | 6/10 | `go test -tags acs -run TestC2524_004 ./acs/cycle2524` |
| scope | no Go source beside the test file | 5/10 | `go test -tags acs -run TestC2524_005 ./acs/cycle2524` |
