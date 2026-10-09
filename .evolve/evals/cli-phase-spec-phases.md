---
score_cap:
  - criterion: "evolve phase and evolve compose resolve spec and catalog phases through phasecmd.ResolveRunner"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1844_00[1267]' ./acs/cycle1844"
  - criterion: "an unknown phase name exits 10 listing both the built-in set and the spec set"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1844_00[458]' ./acs/cycle1844"
  - criterion: "the catalog partition keeps built-in and spec names disjoint and sorted"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1844_003 ./acs/cycle1844"
  - criterion: "compose still refuses ship without --ship-anyway when a spec phase is in the sequence"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1844_009 ./acs/cycle1844"
  - criterion: "runtime-reference.md documents spec phase execution through phase and compose"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1844_010 ./acs/cycle1844"
---

# Eval: evolve phase and evolve compose run spec phases

> Pins the contract that `evolve phase <name>` and `evolve compose --phases` resolve a name against the compiled registry first and the merged spec catalog second, and that a name in neither exits 10 with both sets listed. Source incident: an operator could not rerun `plan-review` or `spec-verify` outside a cycle; `evolve phase` and `evolve compose` rejected every spec phase as unknown (cycle 1844, inbox `cli-phase-spec-phases`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| resolver-reachability | phase and compose reach ResolveRunner for spec names | 4/10 | `go test -tags acs -run 'TestC1844_00[1267]' ./acs/cycle1844` |
| dual-set-error | unknown name exits 10 with both sets | 6/10 | `go test -tags acs -run 'TestC1844_00[458]' ./acs/cycle1844` |
| disjoint-partition | built-in and spec sets disjoint and sorted | 6/10 | `go test -tags acs -run TestC1844_003 ./acs/cycle1844` |
| ship-guard | ship refused without --ship-anyway | 7/10 | `go test -tags acs -run TestC1844_009 ./acs/cycle1844` |
| docs | runtime-reference documents the verbs | 8/10 | `go test -tags acs -run TestC1844_010 ./acs/cycle1844` |
