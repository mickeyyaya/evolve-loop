# Eval: core-pure-helpers-coverage

## Task
Add unit tests in `go/internal/core` covering pure helper functions currently at 0-66% statement coverage:
`isScoutEvalMaterialization`, `withinRoot`, `NewShipError` (odd-trailing-key edge), `ShipError.Error()` (nil receiver), `ShipError.DebugString()` (multi-key ordering), `intFromAny`/`floatFromAny` (unknown-type fallback).

## Criteria

### C1 — the main-tree ownership Specification is fully covered [code]
`isScoutEvalMaterialization` was deleted on 2026-10-07 (the scout's eval home is its workspace; incident `docs/incidents/2026-10-06-cycle-1811-cross-lane-eval-relocation.md`). `mainTreeOwnership` replaced it.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestMainTreeOwnership_ -v -count=1 2>&1 | grep -E "PASS|FAIL"
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

### C5 — Negative: a path another owner holds is never this lane's [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run 'TestMainTreeOwnership_(ForeignOwner|HeldBySibling)' -v -count=1 2>&1 | grep -E "sibling|PASS"
```
Expected: `PASS` (the table rows for sibling evals, predicate packages, change records and mints are refused).

### C6 — Overall core package coverage ≥ 79% after changes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -count=1 -coverprofile=/tmp/core-pure-cov.out 2>&1 | tail -3 && \
  pct=$(go tool cover -func=/tmp/core-pure-cov.out | grep '^total' | awk '{print $3}' | tr -d '%') && \
  echo "Total coverage: ${pct}%" && \
  python3 -c "exit(0 if float('${pct}') >= 79.0 else 1)"
```
Expected: exit 0 (coverage ≥ 79.0%).
