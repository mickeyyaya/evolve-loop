---
score_cap:
  - criterion: "A selector maps a changed doc to the Go tests that read it, and its package tests pass"
    max_if_missing: 5
    evidence: "test -d go/internal/docstests && cd go && go test -count=1 ./internal/docstests/"
  - criterion: "The docs-only route runs the selected tests in a required job and the CI required job waits for it"
    max_if_missing: 6
    evidence: "grep -q '^  docs-tests:' .github/workflows/required.yml && grep -q 'needs.docs-tests.result' .github/workflows/required.yml"
  - criterion: "The ci workflow contract tests still pass with the docs-tests job in the graph"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/ciparity/ && go test -count=1 -tags integration -run 'TestRequiredResult_|TestRouting' ./internal/ciparity/"
  - criterion: "The docs-tests step resolves its range on a push event, not only on a pull request"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2610_017|TestC2610_019|TestC2610_020' ./acs/cycle2610"
  - criterion: "The evolve ci docs-tests verb is documented in the runtime reference"
    max_if_missing: 8
    evidence: "grep -q 'evolve ci docs-tests' docs/operations/runtime-reference.md"
---

# Eval: A docs-only change runs the Go tests that read the changed docs

> Pins the contract of the docs-tests CI job. A docs-only PR must run the Go tests that read the changed docs and must keep skipping the full Go suite. Source incident: main CI run 45059893021 went red on `TestRemovalDesignAmp_TheR9RowLinksOnlyToAnExistingDesign` after the docs batch #1695 landed #1671, because every Go job was skipped in the PR. Cycle 2610 materialized the criteria as predicates in `go/acs/cycle2610/predicates_test.go`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| selector | a changed doc selects the tests that read it | 5/10 | `go test ./internal/docstests/` |
| job graph | docs-tests runs on the docs-only route and gates the required result | 6/10 | `grep` of `required.yml` |
| contract tests | the ciparity workflow tests match the new graph | 7/10 | `go test ./internal/ciparity/` |
| push route | the docs-tests step gets a valid base and head on a push event (audit H1) | 6/10 | `go test -tags acs -run TestC2610_017` |
| documentation | the verb is in the runtime reference | 8/10 | `grep` of `runtime-reference.md` |
