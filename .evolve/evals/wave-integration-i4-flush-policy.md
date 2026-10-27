---
score_cap:
  - criterion: "Integration package test suite passes with zero regressions"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/integration/..."
  - criterion: "Flush decider handles each D7 trigger (wave ends, batch reaches cap, max_age_minutes, unblocks queued work)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'Test.*Flush.*' ./internal/integration/..."
  - criterion: "A risky deliverable flushes alone per D9"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'Test.*Risky.*' ./internal/integration/..."
  - criterion: "The adaptive cap halves on red, grows on green, and stays inside bounds 1 to 4 with default 4 per D8"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'Test.*Cap.*' ./internal/integration/..."
  - criterion: "Negative case: a batch below cap without trigger conditions does not flush"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'Test.*(NoFlush|Negative|BelowCap).*' ./internal/integration/..."
  - criterion: "Edge cases: empty batch and boundary cap clamping behave correctly"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'Test.*(Boundary|Empty|Clamp).*' ./internal/integration/..."
  - criterion: "Code carries zero comments per hard rule 10 and has no placeholder text"
    max_if_missing: 5
    evidence: "test -d go/internal/integration && ! grep -E '^\\s*//' go/internal/integration/*.go && ! grep -q -i -E 'TODO|FIXME|lorem ipsum' go/internal/integration/*.go"
---

# Eval: Wave integration I4 flush policy and adaptive cap (cycle 2037)

> Evaluates component I4 of plan docs/plans/wave-integration-2026-10.md (decisions D7, D8, and D9, ADR-0147). Verifies pure flush policy decider functions and adaptive cap adaptation in package go/internal/integration with unit tests covering each trigger, isolated risky deliverable flush, adaptive cap halving and growth within bounds [1, 4], negative no-flush conditions, and edge boundaries.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| suite-pass | `go test -count=1 ./internal/integration/...` exits 0 | 7/10 | package unit test suite execution |
| d7-triggers | All D7 flush triggers (wave end, cap reached, max age 45m, unblocks queued work) verified | 7/10 | targeted flush test execution |
| risky-alone | Risky deliverables flush alone per D9 | 6/10 | targeted risky deliverable flush test execution |
| adaptive-cap | Cap scaling (halve on confirmed red, grow on green, bounds [1, 4], default 4) verified | 6/10 | targeted cap adaptation test execution |
| negative-case | Under-cap batch without trigger condition does not flush | 6/10 | negative no-flush test execution |
| edge-cases | Empty batch and boundary clamping at min/max cap verified | 5/10 | boundary and empty batch test execution |
| clean-code | No comments added per hard rule 10 and no placeholders | 5/10 | comment audit and placeholder check |

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 ./internal/integration/...`
- `[code]` `cd go && go test -count=1 -run "Test.*Flush.*" ./internal/integration/...`
- `[code]` `cd go && go test -count=1 -run "Test.*Risky.*" ./internal/integration/...`
- `[code]` `cd go && go test -count=1 -run "Test.*Cap.*" ./internal/integration/...`
- `[code]` `cd go && go test -count=1 -run "Test.*(NoFlush|Negative|BelowCap).*" ./internal/integration/...`
- `[code]` `cd go && go test -count=1 -run "Test.*(Boundary|Empty|Clamp).*" ./internal/integration/...`
- `[code]` `test -d go/internal/integration && ! grep -E '^\s*//' go/internal/integration/*.go && ! grep -q -i -E 'TODO|FIXME|lorem ipsum' go/internal/integration/*.go`
- `[code]` `test -s go/internal/integration/flush.go && [ "$(cat go/internal/integration/flush.go)" != "" ]`

## Acceptance Criteria (code-graded)

### AC1: package integration suite passes cleanly [code]
```bash
cd go && go test -count=1 ./internal/integration/...
```
Expected: exit 0

### AC2: each D7 trigger flushes batch once [code]
```bash
cd go && go test -count=1 -run "Test.*Flush.*" ./internal/integration/...
```
Expected: exit 0

### AC3: risky deliverable flushes alone [code]
```bash
cd go && go test -count=1 -run "Test.*Risky.*" ./internal/integration/...
```
Expected: exit 0

### AC4: adaptive cap halves on red and grows on green within [1, 4] with default 4 [code]
```bash
cd go && go test -count=1 -run "Test.*Cap.*" ./internal/integration/...
```
Expected: exit 0

### AC5 (negative): under-cap batch without trigger does not flush [code]
```bash
cd go && go test -count=1 -run "Test.*(NoFlush|Negative|BelowCap).*" ./internal/integration/...
```
Expected: exit 0

### AC6 (edge): empty batch and cap boundaries [1, 4] handled cleanly [code]
```bash
cd go && go test -count=1 -run "Test.*(Boundary|Empty|Clamp).*" ./internal/integration/...
```
Expected: exit 0

### AC7 (negative): no comments in code and no placeholder text [code]
```bash
test -d go/internal/integration && ! grep -E '^\s*//' go/internal/integration/*.go && ! grep -q -i -E 'TODO|FIXME|lorem ipsum' go/internal/integration/*.go
```
Expected: exit 0

### AC8 (edge): flush implementation file is created and non-empty [code]
```bash
test -s go/internal/integration/flush.go && [ "$(cat go/internal/integration/flush.go)" != "" ]
```
Expected: exit 0
