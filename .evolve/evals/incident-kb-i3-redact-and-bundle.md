---
score_cap:
  - criterion: "A changed bundle file fails issue check"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2496_001_' ./acs/cycle2496"
  - criterion: "A token in a source is redacted in the bundle copy and the manifest keeps the raw digest"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2496_002_|TestC2496_006_' ./acs/cycle2496"
  - criterion: "An over-budget bundle keeps each error template and turns the oldest repeat source into a pointer entry"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2496_003_|TestC2496_004_' ./acs/cycle2496"
  - criterion: "The entropy rule passes a valid bundle and fails a random 40-character token"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2496_005_|TestC2496_007_' ./acs/cycle2496"
  - criterion: "internal/redact is a leaf that panetrust imports and the repo-wide apicover gate enrols"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2496_008_|TestC2496_009_' ./acs/cycle2496"
---

# Eval: internal/redact leaf and the evidence bundle (I3)

> Pins the four clauses of the I3 acceptance of the incident knowledge base plan
> (docs/plans/incident-knowledge-base-2026-11.md, D3, D13, D16). The bundle is driven
> through the real `evolve issue evidence add` and `evolve issue check` entry points.
> Source incident: cycle 2496 scout and spec-verify planned no work because the plan
> was not on main; the plan is on origin/console/incident-knowledge-base.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tamper | A changed bundle file fails issue check | 4/10 | `go test -tags acs -run TestC2496_001_ ./acs/cycle2496` |
| redaction | A token is redacted, raw digest kept | 4/10 | `go test -tags acs -run 'TestC2496_002_\|TestC2496_006_' ./acs/cycle2496` |
| budget | Pointer entry for the oldest repeat source, templates kept | 4/10 | `go test -tags acs -run 'TestC2496_003_\|TestC2496_004_' ./acs/cycle2496` |
| entropy | Valid bundle passes, random 40-character token fails | 5/10 | `go test -tags acs -run 'TestC2496_005_\|TestC2496_007_' ./acs/cycle2496` |
| leaf | redact is a leaf, panetrust imports it, apicover enrols it | 6/10 | `go test -tags acs -run 'TestC2496_008_\|TestC2496_009_' ./acs/cycle2496` |
