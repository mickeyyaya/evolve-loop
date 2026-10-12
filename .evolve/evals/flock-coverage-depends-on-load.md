---
score_cap:
  - criterion: "lockwithin.go line 50 (hand-off wait after a lost abandon swap) is covered in every isolated run of the deadline tests"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1857_001 ./acs/cycle1857"
  - criterion: "the full flock suite leaves no uncovered lockwithin.go block"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1857_002 ./acs/cycle1857"
---

# Eval: flock-coverage-depends-on-load

> Pins that `internal/adapters/flock` reaches 100% coverage by a barrier, not by a scheduling race. Source incident: cycle-1857 inbox item flock-coverage-depends-on-load (cover-strict 99.1% under a loaded lane floor).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| isolated-runs | line 50 covered in 40 of 40 runs | 7/10 | `go test -tags acs -run TestC1857_001 ./acs/cycle1857` |
| full-suite | no zero-count lockwithin.go block | 7/10 | `go test -tags acs -run TestC1857_002 ./acs/cycle1857` |
