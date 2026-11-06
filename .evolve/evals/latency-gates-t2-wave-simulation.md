---
score_cap:
  - criterion: "TestWaveSimulation exists in go/internal/fleet and passes over 200 seeds, covering both the Graham-bound and the serial-bound populations and the Resupply arrivals"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -v -run '^TestWaveSimulation$' ./internal/fleet | grep -Eq 'wave-simulation seeds=200 graham=[1-9][0-9]* serial=[1-9][0-9]* arrivals=[1-9][0-9]* checks=[0-9]+'"
  - criterion: "The 200-seed simulation runs in under 2 seconds"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestWaveSimulation$' ./internal/fleet | grep -Eq -- '--- PASS: TestWaveSimulation \\(([01])\\.[0-9]+s\\)'"
  - criterion: "The simulation runs the real admission: the test binary of go/internal/fleet contains internal/admission, and some but not all seeds set PoolConfig.Next"
    max_if_missing: 5
    evidence: "cd go && go list -deps -test ./internal/fleet | sed 's/ \\[.*//' | grep -Fxq github.com/mickeyyaya/evolve-loop/go/internal/admission && go test -count=1 -v -run '^TestWaveSimulation$' ./internal/fleet | grep -Eq 'wave-simulation .*admission=[1-9][0-9]*'"
  - criterion: "The acceptance predicates for the simulation, including the mutant kills for the pool, the admission and the file rule, pass"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2194"
---

# Eval: Latency gates T2 wave simulation

> Pins the seeded wave simulation of docs/plans/latency-gates-2026-10.md section 3.1. It drives the real fleet.RunPool and the real admission with a virtual clock, and checks no idle slot beside an admissible todo, the live count at or under the target, the Graham bound on conflict-free seeds with Window nil, and Cmax at or under the sum of lane times on conflicted seeds. Source: cycle 2177, ADR-0144.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| simulation-runs | 200 seeds, both bound populations, arrivals | 4/10 | `go test -v -run ^TestWaveSimulation$ ./internal/fleet` |
| speed | under 2 seconds | 6/10 | the `--- PASS` duration |
| real-admission | the test binary holds internal/admission and the summary line reports admission=k | 5/10 | `go list -deps -test ./internal/fleet` |
| mutant-kills | the simulation fails, and prints the seed, when the pool, the admission or the file rule is broken | 5/10 | `go test -tags acs ./acs/cycle2194` |
