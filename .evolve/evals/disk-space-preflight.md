---
score_cap:
  - criterion: "boot preflight halts below the floor on injected statfs, naming `evolve gc`"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestRun_DiskSpace_' ./internal/looppreflight"
  - criterion: "wave boundary stops the run before launching below the floor"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestDispatchFleetIteration_Disk|TestDefaultLoopPreflight_DiskFloor' ./cmd/evolve"
  - criterion: "floor comes from policy.json preflight.min_free_gib with a default, documented in runtime-reference.md"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestPreflightConfig_MinFreeGiB ./internal/policy && grep -q min_free_gib ../docs/operations/runtime-reference.md"
---

# Eval: disk-space preflight

> Wave 27 (2026-09-28) went 0/2 on 143 MiB free; the system failure read as lane code FAILs. A disk-space check halts boot and each wave boundary (ADR-0072 system halt).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| boot | halt below floor | 7/10 | `go test -run TestRun_DiskSpace_ ./internal/looppreflight` |
| boundary | wave stop | 7/10 | `go test -run TestDispatchFleetIteration_Disk ./cmd/evolve` |
| config | policy floor + docs | 6/10 | `go test -run TestPreflightConfig_MinFreeGiB ./internal/policy` |
