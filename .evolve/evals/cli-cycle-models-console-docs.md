---
score_cap:
  - criterion: "runtime-reference.md documents every flag and exit code of evolve cycle models"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2122_004 ./acs/cycle2122"
  - criterion: "the documented bullet passes ste-lint --strict and sits beside the models performance bullet"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2122_005 ./acs/cycle2122"
  - criterion: "evolve cycle models prints unreported for a record without a served model and exits 1 or 10 on bad input"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2122_003 ./acs/cycle2122"
  - criterion: "the out-of-scope pr refresh integration test is unchanged from base"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2122_006 ./acs/cycle2122"
  - criterion: "the research index loses no row against base"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2122_007 ./acs/cycle2122"
  - criterion: "the build explanation names exactly the files the diff changes"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2122_008 ./acs/cycle2122"
---

# Eval: evolve cycle models operator docs

> Pins the operator-facing entry for `evolve cycle models --cycle N [--json]` in
> docs/operations/runtime-reference.md. The command and its served-model recording
> shipped before cycle 2122; only the operator entry was missing.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| docs-flags-exit-codes | Bullet names each real flag and measured exit code | 6/10 | `go test -tags acs -run TestC2122_004 ./acs/cycle2122` |
| docs-ste-placement | Bullet passes strict STE lint, beside models performance | 5/10 | `go test -tags acs -run TestC2122_005 ./acs/cycle2122` |
| unreported-behavior | Missing served model prints unreported; bad input fails loudly | 7/10 | `go test -tags acs -run TestC2122_003 ./acs/cycle2122` |
| scope-fence-pr-refresh | The pr refresh integration test is not weakened (cycle 2122 audit H1) | 4/10 | `go test -tags acs -run TestC2122_006 ./acs/cycle2122` |
| scope-fence-index | No research index row is deleted (cycle 2122 audit H2) | 4/10 | `go test -tags acs -run TestC2122_007 ./acs/cycle2122` |
| narrative-fidelity | Changed Areas matches the diff (cycle 2122 audit) | 5/10 | `go test -tags acs -run TestC2122_008 ./acs/cycle2122` |
