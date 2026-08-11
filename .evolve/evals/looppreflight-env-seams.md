# Eval: looppreflight-env-seams

## Goal
Make `defaultTmuxSessions` and `defaultDiskFreeBytes` deterministically testable by converting them to package-level `var` seams (matching the `execVersion` pattern in `versioninventory.go`). Then exercise all reachable branches via injected fakes. Also improve `saveVersionCache` and `PrettyJSON` partial coverage. Target: ≥ 96.5% looppreflight coverage; safe floor **95.5%** (≥ 1.0% headroom required, ≥ 1.5% preferred — TDD engineer MUST measure before pinning).

## Background

Cycle-309 FAIL root cause: looppreflight coverage floor pinned at 98.0% with only 0.1% headroom. Under the EVOLVE_SANDBOX binding run, env-sensitive functions (`defaultTmuxSessions`, `defaultDiskFreeBytes`) execute different branches depending on sandbox/host state, shifting coverage below the floor. The fix is two-part: (1) make those functions injectable so ALL branches are provably covered in every run, (2) set the floor with ≥ 1.5% headroom below the measured figure.

## Acceptance Criteria

### AC1 — `defaultTmuxSessions` converted to var and both branches tested [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -run TestDefaultTmuxSessions -v -count=1 2>&1 | \
  grep -qE "^--- PASS: TestDefaultTmuxSessions" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`; test must cover both the success path (inject a seam returning `["ses1"]`) AND the error path (inject a seam returning a non-nil error) by directly overriding the `defaultTmuxSessions` var in `t.Cleanup`.

### AC2 — Error branch of `defaultTmuxSessions` covered [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -run TestDefaultTmuxSessions_Error -v -count=1 2>&1 | \
  grep -qE "^--- PASS" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`; specifically tests that injecting an error results in `nil, non-nil error` return.

### AC3 — `saveVersionCache` covers marshal-error and rename paths [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -run TestSaveVersionCache -v -count=1 2>&1 | \
  grep -qE "^--- PASS" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`; tests: (a) happy path — written map is loadable back, (b) write to unwritable dir returns error.

### AC4 — Coverage ≥ 95.5% and floor headroom ≥ 1.0% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -count=1 -coverprofile=/tmp/lp310.out 2>/dev/null && \
  pct=$(go tool cover -func=/tmp/lp310.out | grep "^total:" | awk '{gsub(/%/,""); print $3}') && \
  python3 -c "import sys; p=float('$pct'); sys.exit(0 if p >= 95.5 else 1)" && echo "PASS: $pct%" || echo "FAIL: $pct% (< 95.5%)"
```
Expected: `PASS` with printed coverage ≥ 95.5%.

**CRITICAL (cycle-309 lesson):** The TDD engineer MUST run this check BEFORE writing the ACS predicate and set the floor at `(measured - 1.5%)` or lower. The floor is a ratchet on demonstrated reality, not an aspiration.

### AC5 — All existing looppreflight tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -count=1 -short 2>&1 | \
  grep -E "^ok|^FAIL" | grep -q "^ok" && echo "PASS" || echo "FAIL"
```
Expected: `PASS`; zero regressions.

### AC6 — Negative: var seam overridden in one test does not affect another [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/looppreflight/... -run "TestDefaultTmuxSessions|TestDefaultTmuxSessions_Error" \
  -count=3 -v 2>&1 | grep -cE "^--- PASS" | awk '{exit ($1 < 6)}'
```
Expected: exit 0 (both tests pass in 3 repeated runs — seam restore is clean).
