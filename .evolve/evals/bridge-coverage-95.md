# Eval: bridge-coverage-95

## Task
Cover the remaining uncovered paths in `bridge` package to reach total ≥95% statement coverage: `BootSmokeTest` branches (65% → ≥95%), `channel/enablement.go:ResolveStage` (0% → 100%).

## Criteria

### C1 — BootSmokeTest coverage ≥ 90% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestBootSmoke -count=1 -short -coverprofile=/tmp/bootsmoke-cov.out 2>&1 | tail -3 && \
  go tool cover -func=/tmp/bootsmoke-cov.out | grep 'bootsmoke.go:.*BootSmokeTest'
```
Expected: coverage ≥ 90.0%.

### C2 — ResolveStage covered [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/channel/... -run TestResolveStage -v -count=1 2>&1 | grep -E 'PASS|FAIL|ResolveStage'
```
Expected: `PASS` with `TestResolveStage` visible.

### C3 — Total bridge coverage ≥ 95% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -coverprofile=/tmp/bridge-full-cov.out 2>&1 | tail -3 && \
  pct=$(go tool cover -func=/tmp/bridge-full-cov.out | grep '^total' | awk '{print $3}' | tr -d '%') && \
  echo "Total coverage: ${pct}%" && \
  python3 -c "exit(0 if float('$pct') >= 95.0 else 1)"
```
Expected: exit 0 (coverage ≥ 95.0%).

### C4 — Negative case: BootSmokeTest rejects non-tmux driver [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestBootSmokeRejectsHeadless -v -count=1 2>&1 | grep -E 'PASS|FAIL|ExitBadFlags'
```
Expected: `PASS` and `ExitBadFlags` visible (non-tmux driver returns error code).

### C5 — Negative case: BootSmokeTest rejects unknown driver name [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestBootSmokeUnknownDriver -v -count=1 2>&1 | grep -E 'PASS|FAIL|ExitBadFlags'
```
Expected: `PASS`.

### C6 — ResolveStage returns valid stage values [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/channel/... -run TestResolveStage -v -count=1 2>&1 | grep -E 'off|shadow|enforce|PASS|FAIL'
```
Expected: `PASS`; output shows distinct stage values being tested.
