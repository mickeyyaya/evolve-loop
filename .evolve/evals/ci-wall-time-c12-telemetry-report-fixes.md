---
score_cap:
  - criterion: "a package window at 60% busy is not cpu-saturation"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2100_001 ./acs/cycle2100"
  - criterion: "a binary with under 0.5 s of CPU is wait"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2100_002 ./acs/cycle2100"
  - criterion: "the runner CPU count is 3 for macOS and 4 for ubuntu in the saturation rule"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2100_003 ./acs/cycle2100"
  - criterion: "a package with a wall above twice its floor is contention"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC2100_004 ./acs/cycle2100"
  - criterion: "the report changes no input file and its JSON keeps every current field"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2100_005 ./acs/cycle2100"
---

# Eval: CI wall time C12, the telemetry report limit fixes

> Pins the C12 rules of docs/architecture/ci-telemetry-and-critical-path.md section 13 through citelemetry.Run, the function behind evolve ci telemetry. Source incident: the first instrumented CI run (39031338517) gave wrong limit values for 303 ubuntu packages and 184 macOS packages. Cycle 2100.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| window | 60% busy window is not saturation | 6/10 | `go test -tags acs -run TestC2100_001 ./acs/cycle2100` |
| tiny-cpu | under 0.5 s CPU is wait | 7/10 | `go test -tags acs -run TestC2100_002 ./acs/cycle2100` |
| cpu-count | macOS 3, ubuntu 4 | 7/10 | `go test -tags acs -run TestC2100_003 ./acs/cycle2100` |
| contention | wall above 2x floor | 8/10 | `go test -tags acs -run TestC2100_004 ./acs/cycle2100` |
| preservation | inputs untouched, JSON fields kept | 7/10 | `go test -tags acs -run TestC2100_005 ./acs/cycle2100` |
