---
score_cap:
  - criterion: "The signal-code generator writes ASD-STE100 text"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2536_001 ./acs/cycle2536"
  - criterion: "The committed signal-code region has no STE finding outside rows defined in protected files"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2536_002 ./acs/cycle2536"
  - criterion: "The flag-index generator and committed region have no STE finding outside rows defined in protected files"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2536_00[34]' ./acs/cycle2536"
  - criterion: "Each generator check verb reports no drift"
    max_if_missing: 7
    evidence: "cd go && go run ./cmd/evolve signals codes check && go run ./cmd/evolve flags check && go run ./cmd/evolve skills check && go run ./cmd/evolve docs index check"
  - criterion: "The README version row keeps its | vX.Y | prefix"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/versionbump"
---

# Eval: The doc generators write ASD-STE100 text (STE S11)

> Pins the generated text of the signal-code table and the flag index to the ASD-STE100 house rules. The lint skips marked regions, so the predicates remove the markers before they lint. The rows whose text lives in a protected file go to a console follow-up, because a lane builder cannot change that file. Source: cycle 2536, plan docs/plans/ste100-docs-rewrite-2026-10.md batch S11.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| signal-codes | the generated table has no STE finding | 6/10 | `go test -tags acs -run TestC2536_001 ./acs/cycle2536` |
| flag-index | the generated index has no STE finding | 6/10 | `go test -tags acs -run TestC2536_003 ./acs/cycle2536` |
| no-drift | the four check verbs exit 0 | 7/10 | the four `check` verbs |
| readme-prefix | the row keeps `| vX.Y |` | 7/10 | `go test ./internal/versionbump` |
