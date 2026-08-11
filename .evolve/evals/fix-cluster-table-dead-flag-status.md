---
score_cap:
  - criterion: "10 dead flags no longer appear as ACTIVE or DEPRECATED in the hand-maintained cluster tables of control-flags.md"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle354/... -run 'TestC354_001|TestC354_002|TestC354_003'"
  - criterion: "At least 5 of the 10 dead flags show DEAD (uppercase) in the hand-maintained cluster tables"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle354/... -run TestC354_004_DeadFlagsShowDeadInClusterTable"
  - criterion: "evolve flags check exits 0 (Generated Flag Index in sync with flagregistry)"
    max_if_missing: 5
    evidence: "cd go && make build && ./bin/evolve flags check"
---

# Eval: Fix stale ACTIVE/DEPRECATED status in control-flags.md cluster tables

> Pins the correctness of the hand-maintained cluster tables in
> `docs/architecture/control-flags.md` — specifically that the 10 flags the
> flagregistry marks `StatusDead` are shown as DEAD (not ACTIVE or DEPRECATED)
> in the hand-maintained annotation section (before the `## Generated Flag Index`
> marker). The registry is the SSOT; these cluster table rows drifted when the
> v12 bash-retirement wave correctly marked the flags dead in the registry but
> left the hand-maintained rows unchanged (cycle-354, scout finding F1).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| negative-absence | Core Infra flags (RESOLVE_ROOTS_LOADED, FAILURE_CLASSIFICATIONS_LOADED) not ACTIVE | 7/10 | `go test -tags acs ./acs/cycle354/... -run TestC354_001` |
| negative-absence | Platform/CLI Hybrid 5 flags not ACTIVE | 7/10 | `go test -tags acs ./acs/cycle354/... -run TestC354_002` |
| negative-absence | STRICT_FAILURES not DEPRECATED | 7/10 | `go test -tags acs ./acs/cycle354/... -run TestC354_003` |
| positive-presence | ≥5 flags show DEAD (uppercase) in hand-maintained section | 6/10 | `go test -tags acs ./acs/cycle354/... -run TestC354_004` |
| generated-index-sync | evolve flags check exits 0 | 5/10 | `cd go && make build && ./bin/evolve flags check` |
