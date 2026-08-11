# Eval: clihealth-coverage-boost

## Goal
Raise `internal/clihealth` coverage from 76.2% to the committed floor of ≥ 88% (95% aspirational) by adding tests for the four zero-coverage functions (`Benchable`, `NewBenchEntry`, `firstLine`, `truncateRunes`) and the partially-covered `write` (63.6%) and `NewStore` (66.7%) paths.

## Acceptance Criteria

### AC1 — Benchable happy + negative [code]
```bash
cd go && go test ./internal/clihealth/... -run "TestBenchable" -v -count=1
```
Expected: `--- PASS: TestBenchable` (tests both `rate_limit` → true and another pattern → false)

### AC2 — NewBenchEntry round-trip [code]
```bash
cd go && go test ./internal/clihealth/... -run "TestNewBenchEntry" -v -count=1
```
Expected: `--- PASS: TestNewBenchEntry` (strikes incremented, evidence truncated to 160 runes, BenchedUntil > BenchedAt)

### AC3 — firstLine extracts first line [code]
```bash
cd go && go test ./internal/clihealth/... -run "TestFirstLine" -v -count=1
```
Expected: `--- PASS: TestFirstLine` (multi-line string → first line only; single-line → unchanged)

### AC4 — truncateRunes handles multibyte [code]
```bash
cd go && go test ./internal/clihealth/... -run "TestTruncateRunes" -v -count=1
```
Expected: `--- PASS: TestTruncateRunes` (unicode string truncated at rune boundary, not byte boundary; short string unchanged)

### AC5 — Coverage ≥ 88% (committed floor) [code]
```bash
cd go && go test ./internal/clihealth/... -count=1 -coverprofile=/tmp/clihealth301.out && go tool cover -func=/tmp/clihealth301.out | grep "^total:" | awk '{print $3}' | awk -F'%' '{exit ($1 < 88)}'
```
Expected: exit 0 (coverage ≥ 88% — cycle-310 committed floor; matches `go/acs/cycle310` C310_005)

### AC6 — No regression in clihealth canary tests [code]
```bash
cd go && go test ./cmd/evolve/ -run "TestCanary" -count=1 -v 2>&1 | grep -c "^--- PASS:" | awk '{exit ($1 < 4)}'
```
Expected: exit 0 (at least 4 canary tests still pass)

## Negative Cases

### NC1 — Benchable rejects unknown pattern [code]
```bash
cd go && go test ./internal/clihealth/... -run "TestBenchable_Unknown" -v -count=1
```
Expected: `--- PASS` (assert `Benchable("trust_prompt") == false`)

### NC2 — truncateRunes does not over-truncate [code]
```bash
cd go && go test ./internal/clihealth/... -run "TestTruncateRunes_Short" -v -count=1
```
Expected: `--- PASS` (string shorter than limit returned unchanged)

## Grader Notes
- `[code]` graders run from repo root; `cd go` changes to the Go module dir.
- AC5 awk exits 0 when coverage passes, non-zero when it fails — correct passing behavior for the eval runner.
