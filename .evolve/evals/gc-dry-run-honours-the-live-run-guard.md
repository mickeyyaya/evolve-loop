---
score_cap:
  - criterion: "gc --dry-run with a fresh lease prints the live lease line and no session, socket or cycle-process sweep line"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2219_001 ./acs/cycle2219"
  - criterion: "gc --dry-run --json reports the live run directories and no process sweep line"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2219_002 ./acs/cycle2219"
  - criterion: "gc --dry-run without a live lease keeps the full process sweep preview"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2219_003 ./acs/cycle2219"
  - criterion: "gc --dry-run takes no gc lease and writes nothing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2219_004 ./acs/cycle2219"
  - criterion: "runGC keeps 100% statement coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2219_005 ./acs/cycle2219"
---

# Eval: gc --dry-run honours the live-run guard

> Pins the rule that `evolve gc --dry-run` previews the same sweeps as a real run. While a run holds a fresh lease, a real run skips the sweeps that stop processes. The preview must skip them too and must print the live lease line. Source incident: wave 153, cycle 2219 inbox item `gc-dry-run-honours-the-live-run-guard`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| live-preview | Live lease hides process sweeps and prints the notice | 7/10 | `go test -tags acs -run TestC2219_001 ./acs/cycle2219` |
| live-json | JSON lists live runs | 6/10 | `go test -tags acs -run TestC2219_002 ./acs/cycle2219` |
| no-lease-preserved | Preview unchanged without a live lease | 6/10 | `go test -tags acs -run TestC2219_003 ./acs/cycle2219` |
| no-write | No gc lock, no write | 7/10 | `go test -tags acs -run TestC2219_004 ./acs/cycle2219` |
| coverage | runGC at 100% | 6/10 | `go test -tags acs -run TestC2219_005 ./acs/cycle2219` |
