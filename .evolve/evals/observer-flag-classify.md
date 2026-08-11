---
score_cap:
  - criterion: "Four Observer flags (STALL_S, POLL_S, NUDGE_S, NUDGE_BODY) are StatusActive in the flagregistry"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/flagregistry/... -run TestLookup_SpotChecks"
  - criterion: "Generated flag index is in sync with the updated registry"
    max_if_missing: 6
    evidence: "cd go && ./bin/evolve flags check"
  - criterion: "EVOLVE_OBSERVER_NUDGE_S default in runtime-reference.md is 300 (not the stale 0)"
    max_if_missing: 6
    evidence: "grep 'EVOLVE_OBSERVER_NUDGE_S' docs/operations/runtime-reference.md | grep -q '300'"
  - criterion: "EVOLVE_OBSERVER_STALL_S is NOT StatusInternal in registry_table.go (negative gaming guard)"
    max_if_missing: 7
    evidence: "! grep '\"EVOLVE_OBSERVER_STALL_S\", Status: StatusInternal' go/internal/flagregistry/registry_table.go"
---

# Eval: observer-flag-classify

> Pins the Observer cluster flag reclassification introduced in cycle-353.
> The 2026-06-11 inventory sweep left EVOLVE_OBSERVER_STALL_S, POLL_S, NUDGE_S,
> and NUDGE_BODY as StatusInternal placeholders, even though they are
> operator-configurable dials (ADR-0030). This eval prevents future cycles
> from regressing those flags back to Internal or leaving the NUDGE_S default
> wrong in runtime-reference.md. Source incident: cycle-352 audit-FAIL on
> gofmt+FileNotContains; cycle-353 closes the Observer cluster classification gap.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| behavioral-registry | 4 flags StatusActive — go test gate | 7/10 | `go test ./internal/flagregistry/... -run TestLookup_SpotChecks` |
| flags-index-sync | evolve flags check exits 0 | 6/10 | `./bin/evolve flags check` |
| doc-correctness | NUDGE_S default is 300 in runtime-reference | 6/10 | `grep EVOLVE_OBSERVER_NUDGE_S runtime-reference.md \| grep 300` |
| negative-guard | STALL_S NOT StatusInternal | 7/10 | `! grep STALL_S.*StatusInternal registry_table.go` |
