# Eval: clihealth-zero-coverage-tests

## Task
Add tests covering `Benchable`, `NewBenchEntry`, `firstLine`, `truncateRunes`, and the uncovered
branches of `NewStore`, `Load`, and `write` in `go/internal/clihealth/clihealth.go`.

## Acceptance Criteria

### AC1 — Zero-coverage functions are covered [code]
```bash
cd go && go test ./internal/clihealth/... -coverprofile=/tmp/clihealth-ac1.out -count=1 2>&1
go tool cover -func=/tmp/clihealth-ac1.out | grep -E "^github.com/mickeyyaya/evolve-loop/go/internal/clihealth/clihealth.go:(Benchable|NewBenchEntry|firstLine|truncateRunes)"
```
Expected: all four functions show coverage > 0%.

### AC2 — Package coverage floor ≥ 85% [code]
```bash
cd go && go test ./internal/clihealth/... -cover -count=1 2>&1 | grep "coverage:"
```
Expected output contains `coverage: 8[5-9]\.` or `coverage: 9[0-9]\.` or `coverage: 100\.`.

### AC3 — All existing tests still pass [code]
```bash
cd go && go test ./internal/clihealth/... -count=1 2>&1 | tail -3
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/clihealth` with no FAIL lines.

### AC4 — Negative: non-benchable patterns return false [code]
```bash
cd go && go test ./internal/clihealth/... -run TestBenchable -v -count=1 2>&1
```
Expected: `--- PASS: TestBenchable` — test covers both `rate_limit → true` and a non-benchable pattern → false.

### AC5 — NewBenchEntry composes strike-scaled cooldown without reset hint [code]
```bash
cd go && go test ./internal/clihealth/... -run TestNewBenchEntry -v -count=1 2>&1
```
Expected: `--- PASS: TestNewBenchEntry` — test verifies that with no reset-hint in pane text, `BenchedUntil` equals `now + CooldownForStrikes(strikes)`.

### AC6 — Edge: truncateRunes handles multibyte (non-ASCII) runes without panic [code]
```bash
cd go && go test ./internal/clihealth/... -run TestTruncateRunesMultibyte -v -count=1 2>&1
```
Expected: `--- PASS: TestTruncateRunesMultibyte` — pane text with `■` (U+25A0, 3-byte UTF-8) truncates cleanly.
