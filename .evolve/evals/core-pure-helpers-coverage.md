# Eval: core-pure-helpers-coverage

## Task
Add unit tests in `go/internal/core` covering pure helper functions currently at 0-66% statement coverage:
`isScoutEvalMaterialization`, `withinRoot`, `NewShipError` (odd-trailing-key edge), `ShipError.Error()` (nil receiver), `ShipError.DebugString()` (multi-key ordering), `intFromAny`/`floatFromAny` (unknown-type fallback).

## Criteria

### C1 — isScoutEvalMaterialization fully covered [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestIsScoutEvalMaterialization -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS` — test runs and passes.

### C2 — withinRoot edge cases covered (empty root + traversal attempt) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestWithinRoot -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`.

### C3 — ShipError nil receiver and DebugString multi-key [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestShipError -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`.

### C4 — intFromAny / floatFromAny unknown-type fallback [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestFromAny -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`.

### C5 — Negative: isScoutEvalMaterialization rejects non-scout phase [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestIsScoutEvalMaterialization -v -count=1 2>&1 | grep -E "non-scout|build.*false|PASS"
```
Expected: `PASS` (the test verifies false is returned for non-scout phases).

### C6 — Overall core package coverage ≥ 79% after changes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -count=1 -coverprofile=/tmp/core-pure-cov.out 2>&1 | tail -3 && \
  pct=$(go tool cover -func=/tmp/core-pure-cov.out | grep '^total' | awk '{print $3}' | tr -d '%') && \
  echo "Total coverage: ${pct}%" && \
  python3 -c "exit(0 if float('${pct}') >= 79.0 else 1)"
```
Expected: exit 0 (coverage ≥ 79.0%).
