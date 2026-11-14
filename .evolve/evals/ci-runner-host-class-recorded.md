---
score_cap:
  - criterion: "The sampler header holds the CPU count and the memory size of the host"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2379_001_|TestC2379_002_' ./acs/cycle2379"
  - criterion: "The job summary prints the host class beside the package wall report, and says not recorded when the header is absent"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2379_010_|TestC2379_011_|TestC2379_012_|TestC2379_013_' ./acs/cycle2379"
  - criterion: "Every wall budget step of the workflow passes the sampler file to testlatency"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2379_014_' ./acs/cycle2379"
  - criterion: "The package notes name the host class sources and pass the STE lint"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2379_020_' ./acs/cycle2379"
---

# Eval: The runner sampler records the host class

> Pins the host class record of `hostsampler` and its print in the job summary. The macOS host class (3 cores or 5 cores) is 41% of the bridge wall variance, and no log printed the core count. Source incident: cycle 2360, `docs/research/ci-wall-variance-2026-11.md` fix R2.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| header-from-real-host | `hostsampler` writes one host record first, with the values of `hw.ncpu` and `hw.memsize` or `nproc` | 6/10 | `go test -tags acs -run TestC2379_001_ ./acs/cycle2379` |
| summary-line | `testlatency -samples` prints one Host class line from the header | 6/10 | `go test -tags acs -run TestC2379_010_ ./acs/cycle2379` |
| workflow-wiring | each `testlatency -budget` step passes `-samples` | 6/10 | `go test -tags acs -run TestC2379_014_ ./acs/cycle2379` |
| docs | both package notes name the sources and pass `ste-lint --strict` | 8/10 | `go test -tags acs -run TestC2379_020_ ./acs/cycle2379` |
