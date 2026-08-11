# Eval: gc-error-path-coverage

## Goal
Cover 5 specific uncovered blocks in `go/internal/gc` (95.0% → 98%+):
- `discover.go:81-83` — missing `runs/` dir yields empty list
- `gc.go:125-127` — Plan rejects empty/relative EvolveDir
- `gc.go:130-132` — Plan with nil Now defaults to wall clock
- `gc.go:204-205` — `dirEntriesOlderThan` filter-rejects-entry path
- `discover.go:93-94` — already tested via symlink (verify no regression)

## Acceptance Criteria

### AC1: Missing runs/ yields empty list [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/gc/... -run TestDiscover_MissingRunsDirYieldsEmpty -v -count=1 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC2: Plan rejects non-absolute EvolveDir [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/gc/... -run TestPlan_RequiresAbsoluteEvolveDir -v -count=1 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC3: Plan with nil Now does not panic [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/gc/... -run TestPlan_NilNowUsesWallClock -v -count=1 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC4: dirEntriesOlderThan filter-reject path covered [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/gc/... -run TestDirEntriesOlderThan_FilterRejectsEntry -v -count=1 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC5: Overall coverage reaches 98%+ [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/gc/... -count=1 -coverprofile=/tmp/gc_cov_300.out 2>&1 | \
  grep "coverage:" | grep -E "9[89]\." && echo "PASS" || echo "FAIL: coverage below 98%"
```
Expected: `PASS` (coverage ≥ 98%)

### AC6: No regression — all gc tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/gc/... -count=1 2>&1 | \
  grep -E "^ok|^FAIL" | grep -q "^ok" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`

### AC7: Negative — Plan with relative dir returns error (not panic) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/gc/... -run TestPlan_RequiresAbsoluteEvolveDir -v -count=1 2>&1 | \
  grep -q "EvolveDir must be absolute" || \
  go test ./internal/gc/... -run TestPlan_RequiresAbsoluteEvolveDir -v -count=1 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```
Expected: `PASS` (test verifies error message, test itself passes)
