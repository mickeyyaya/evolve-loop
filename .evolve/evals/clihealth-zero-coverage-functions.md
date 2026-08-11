# Eval: clihealth-zero-coverage-functions

## Goal
Add tests for the four zero-coverage functions in `go/internal/clihealth/clihealth.go`:
`Benchable`, `NewBenchEntry`, `firstLine` (private, exercised via NewBenchEntry),
`truncateRunes` (private, exercised via NewBenchEntry). Measured baseline: 76.2%.
Floor target: 88%.

## Code Graders

### AC1: Tests compile and pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test -count=1 ./internal/clihealth/...
```
Expected: exit 0

### AC2: Coverage meets or exceeds floor 88% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -count=1 -coverprofile=/tmp/clihealth-c314.out ./internal/clihealth/... && \
  pct=$(go tool cover -func=/tmp/clihealth-c314.out | awk '/^total:/{gsub(/%/,"",$3); print $3}') && \
  echo "coverage: ${pct}%" && \
  awk "BEGIN{exit ($pct < 88.0)}"
```
Expected: exit 0, coverage line printed ≥ 88.0%

### AC3: Benchable — negative case: non-rate-limit pattern rejected [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -count=1 -run TestBenchable ./internal/clihealth/...
```
Expected: exit 0

### AC4: NewBenchEntry — strike accumulation and truncation tested [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -count=1 -run TestNewBenchEntry ./internal/clihealth/...
```
Expected: exit 0

### AC5: Full regression — no existing tests regress [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test -count=1 ./internal/clihealth/...
```
Expected: exit 0, all tests PASS

## Negative Cases (eval gaming guard)

### AC6: Benchable returns false for "auth_recheck" [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -count=1 -run TestBenchable_NonBenchablePatterns ./internal/clihealth/...
```
Expected: exit 0

### AC7: truncateRunes boundary — exactly n runes is NOT truncated [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test -count=1 -run TestTruncateRunesBoundary ./internal/clihealth/...
```
Expected: exit 0

## Thresholds
- All checks: pass@1 = 1.0
