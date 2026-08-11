---
score_cap:
  - criterion: "countFieldDuplicates helper function declared and package compiles"
    max_if_missing: 8
    evidence: "grep -q 'func countFieldDuplicates' go/internal/cyclehealth/cyclehealth.go && (cd go && go build ./internal/cyclehealth/...)"
  - criterion: "checkLedgerDuplicates delegates to countFieldDuplicates (no inline seen map)"
    max_if_missing: 7
    evidence: "result=$(awk '/^func checkLedgerDuplicates\\(/,/^\\}/' go/internal/cyclehealth/cyclehealth.go) && echo \"$result\" | grep -q countFieldDuplicates"
  - criterion: "checkChallengeTokens delegates to countFieldDuplicates (no inline seen map)"
    max_if_missing: 7
    evidence: "result=$(awk '/^func checkChallengeTokens\\(/,/^\\}/' go/internal/cyclehealth/cyclehealth.go) && echo \"$result\" | grep -q countFieldDuplicates"
  - criterion: "all cyclehealth tests pass under -race"
    max_if_missing: 9
    evidence: "(cd go && go test -race ./internal/cyclehealth/...)"
  - criterion: "empty-field entries produce no anomaly (TestCountFieldDuplicates_EmptyField passes)"
    max_if_missing: 6
    evidence: "(cd go && go test -race -run TestCountFieldDuplicates_EmptyField ./internal/cyclehealth/...)"
  - criterion: "package coverage >= 93.3%"
    max_if_missing: 5
    evidence: "(cd go && go test -cover ./internal/cyclehealth/... 2>&1 | grep -oE '[0-9]+\\.[0-9]+' | head -1 | awk '{exit ($1+0 < 93.3)}')"
---

# Eval: Extract countFieldDuplicates helper in cyclehealth

> Pins the refactoring contract introduced in cycle 137: `checkLedgerDuplicates`
> and `checkChallengeTokens` in `go/internal/cyclehealth/cyclehealth.go` must
> delegate the duplicate-detection frequency-map loop to a shared helper
> `countFieldDuplicates` rather than implementing it inline. The helper must
> correctly skip empty-field entries and ignore cross-cycle entries.
>
> Source incident: cycle 137 identified ~20 lines of verbatim-duplicated logic
> across the two signal implementations — a maintenance hazard and the canonical
> DRY violation the TDD engineer's test suite now pins.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| func-declared | helper declared + compiles | 8/10 | `grep + go build` |
| caller-delegates-1 | checkLedgerDuplicates delegates | 7/10 | awk body inspection |
| caller-delegates-2 | checkChallengeTokens delegates | 7/10 | awk body inspection |
| tests-pass-race | full suite green under -race | 9/10 | `go test -race` |
| empty-field-edge | empty field → 0 anomalies | 6/10 | targeted test run |
| coverage-floor | coverage ≥ 93.3% | 5/10 | `go test -cover` |
