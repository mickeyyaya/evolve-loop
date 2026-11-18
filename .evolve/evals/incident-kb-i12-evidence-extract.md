---
score_cap:
  - criterion: "The extract keeps each error with 5 lines of context and no more"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2502_001_|TestC2502_008_' ./acs/cycle2502"
  - criterion: "Repeated lines merge into one template with a count and a first and last example"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2502_002_' ./acs/cycle2502"
  - criterion: "The extract text stays under the budget and a tight budget drops the tail before an error"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2502_003_' ./acs/cycle2502"
  - criterion: "The result records the whole-source raw digest and the kept ranges, and redacts secrets in the text"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2502_004_|TestC2502_009_' ./acs/cycle2502"
  - criterion: "The same input gives the same bytes and a changed input does not"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2502_005_' ./acs/cycle2502"
  - criterion: "Pointer mode keeps only the digest and the size, and an unknown mode is refused"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2502_006_|TestC2502_007_' ./acs/cycle2502"
  - criterion: "internal/evidencex is enrolled in the repo-wide apicover gate and names every exported symbol"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2502_010_' ./acs/cycle2502"
---

# Eval: internal/evidencex fixed extract pipeline (I12)

> Pins the I12 acceptance of the incident knowledge base plan (D15, D16). A 2,000-line
> fixture log with 3 planted errors is driven through `evidencex.Extract` in both modes.
> Source incident: cycle 2502 fleet lane; the plan lives on origin/console/incident-knowledge-base
> and is not on main, so the acceptance line and D16 are the spec.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| context | Each error keeps 5 lines before and after, clamped at the edges | 4/10 | `go test -tags acs -run 'TestC2502_001_\|TestC2502_008_' ./acs/cycle2502` |
| templates | Repeats merge into a template with a count and first and last example | 4/10 | `go test -tags acs -run TestC2502_002_ ./acs/cycle2502` |
| budget | Text fits the budget and errors outrank the tail | 4/10 | `go test -tags acs -run TestC2502_003_ ./acs/cycle2502` |
| digest | Raw whole-source digest and kept ranges, secrets redacted | 4/10 | `go test -tags acs -run 'TestC2502_004_\|TestC2502_009_' ./acs/cycle2502` |
| determinism | Same input gives the same bytes | 4/10 | `go test -tags acs -run TestC2502_005_ ./acs/cycle2502` |
| modes | Pointer keeps the digest only, unknown mode refused | 5/10 | `go test -tags acs -run 'TestC2502_006_\|TestC2502_007_' ./acs/cycle2502` |
| graduation | apicover enrolment and named exports | 6/10 | `go test -tags acs -run TestC2502_010_ ./acs/cycle2502` |
