---
score_cap:
  - criterion: "The dead-red predicate packages go/acs/cycle1257 and go/acs/cycle1259 no longer resolve in the Go toolchain (directory gone / no Go files), and no go/acs source grades the phantom GoLaneSelection machinery"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1697_001_DeadRedPackagesNoLongerResolve$' ./acs/cycle1697 | grep -q -- '--- PASS: TestC1697_001_DeadRedPackagesNoLongerResolve '"
  - criterion: "Neither dead-red predicate file is tracked by git (deleted from the shipped tree, not merely from disk)"
    max_if_missing: 8
    evidence: "test -z \"$(git ls-files go/acs/cycle1257 go/acs/cycle1259)\""
  - criterion: "F5 of docs/operations/batch-integrity-review-2026-08-04.md is extended in place: exactly one F5 heading, F6 still next, Issue/Gap/Solution intact, and a Closure block citing both deleted paths, cycle 1697 and the skipped_count outcome"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1697_002_F5ExtendedInPlaceWithClosure$' ./acs/cycle1697 | grep -q -- '--- PASS: TestC1697_002_F5ExtendedInPlaceWithClosure '"
---

# Eval: remove the dead-red go/acs/cycle1257 + cycle1259 predicate corpus and close F5

> Inbox item `dead-red-acs-corpus-cleanup` (cycle 1697; issue/gap/solution in
> `docs/operations/batch-integrity-review-2026-08-04.md` F5). Commit `fcdd466e`
> shipped `go/acs/cycle1257/predicates_test.go` and
> `go/acs/cycle1259/predicates_test.go` (~550 lines) from cycles whose audits
> FAILed. They grade an abandoned acssuite-internal selection design whose
> unit tests never existed at any commit. At base `f341bc89` they were red by
> construction: `go test -tags acs` gave 5 FAIL in cycle1257 and 2 FAIL in
> cycle1259, and neither package produced a SKIP. This eval pins their
> removal and the F5 closure record.
>
> Premise correction measured in cycle 1697: the inbox fix said to "verify the
> EGPS skipped_count drops". It cannot drop. EGPS
> (`acssuite.goLanePatterns`, `go/internal/acssuite/acssuite.go:379-395`) never
> runs a historical cycle dir, and these packages FAILed instead of SKIPping.
> So no skipped_count grader exists. The F5 closure must record that outcome
> truthfully.
>
> The cycle-scoped change-set predicate `TestC1697_003_…` (no protected
> surface touched, no over-deletion) is NOT a grader here. It reads the lane's
> diff against its fork point, so it is only meaningful during cycle 1697's
> audit.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| removed (toolchain) | both dead packages no longer resolve; no phantom references in go/acs | 8/10 | `go test -tags acs -run '^TestC1697_001_' ./acs/cycle1697` |
| removed (tracking) | neither dead file is tracked in the shipped tree | 8/10 | `git ls-files go/acs/cycle1257 go/acs/cycle1259` is empty |
| F5 closure (docs) | F5 extended in place with closure evidence, not duplicated | 6/10 | `go test -tags acs -run '^TestC1697_002_' ./acs/cycle1697` |
