---
score_cap:
  - criterion: "An unknown pair name in the handoff block is refused at decode"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestHandoffPolicy_UnknownPairRefusedAtDecode$' ./internal/policy"
  - criterion: "With no handoff block every pair resolves to stage shadow"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestHandoffPolicy_AnAbsentBlockResolvesEveryPairToShadow$' ./internal/policy"
  - criterion: "A stage other than shadow or enforce is refused"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestHandoffPolicy_InvalidStageRefused$' ./internal/policy"
  - criterion: "The handoff block is documented in docs/operations/runtime-reference.md"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRuntimeReference_DocumentsTheHandoffBlock$' ./internal/policy"
---

# Eval: Phase handoff H6, the handoff policy block

> Pins component H6 of ADR-0141: the strict-decoded `handoff` block in `.evolve/policy.json` with the five pairs, a stage for each pair and a compiled default of `shadow`. Source incident: cycle 1950 audit-fail, the motivation for the phase-handoff contract.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unknown-pair | An unknown pair is refused at decode | 3/10 | `go test -run TestHandoffPolicy_UnknownPairRefusedAtDecode` |
| default-shadow | An absent block resolves every pair to shadow | 4/10 | `go test -run TestHandoffPolicy_AnAbsentBlockResolvesEveryPairToShadow` |
| stage-vocabulary | Only shadow and enforce are stages | 4/10 | `go test -run TestHandoffPolicy_InvalidStageRefused` |
| docs | The block is in runtime-reference.md | 6/10 | `go test -run TestRuntimeReference_DocumentsTheHandoffBlock` |
