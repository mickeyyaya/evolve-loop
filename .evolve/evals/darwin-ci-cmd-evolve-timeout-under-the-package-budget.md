---
score_cap:
  - criterion: "testlatency -timeout-margin reports the margin of the slowest package and flags LOW under 20%"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2215_00[1-6]' ./acs/cycle2215"
  - criterion: "the Makefile prints the darwin go test timeout it applies"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2215_007 ./acs/cycle2215"
  - criterion: "the darwin CI job writes the margin to the job summary from the Makefile timeout"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2215_008 ./acs/cycle2215"
---

# Eval: macOS CI reports the cmd/evolve margin against the go test timeout

> Pins the margin report added for the macOS job. The job timed out once in cmd/evolve on main (inbox item darwin-ci-cmd-evolve-timeout-under-the-package-budget, cycle 2215). The go test timeout, not the step timeout, killed the package, and no output showed how close it ran.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| margin-section | slowest package margin reported, LOW under 20% | 6/10 | `go test -tags acs -run 'TestC2215_00[1-6]'` |
| one-home | Makefile prints the applied darwin timeout | 6/10 | `go test -tags acs -run TestC2215_007` |
| job-wiring | go.yml passes the Makefile value to testlatency | 7/10 | `go test -tags acs -run TestC2215_008` |
