---
score_cap:
  - criterion: "evolve doctor versions --json lists each detected CLI's installed version"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC1865_001 ./acs/cycle1865"
  - criterion: "--latest flags a lagging CLI with exit 1 and a failed lookup prints unavailable with exit 0"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestDoctorVersions_Latest' ./internal/cli/opscmd"
  - criterion: "runtime-reference documents the verb"
    max_if_missing: 8
    evidence: "grep -qF 'evolve doctor versions [--latest] [--json]' docs/operations/runtime-reference.md"
---

# Eval: evolve doctor versions

> Pins the operator verb `evolve doctor versions [--latest] [--json]` from inbox item cli-doctor-versions (cycle 1865).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| subprocess-json | installed versions listed | 4/10 | `go test -tags acs -run TestC1865_001 ./acs/cycle1865` |
| injected-release | lag exit 1, unavailable exit 0 | 6/10 | `go test -run '^TestDoctorVersions_Latest' ./internal/cli/opscmd` |
| docs | verb documented | 8/10 | `grep -qF` |
