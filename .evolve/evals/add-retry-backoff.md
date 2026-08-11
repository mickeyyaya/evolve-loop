# Eval: add-retry-backoff

## Metadata
- slug: add-retry-backoff
- cycle: 180
- task: Add configurable exponential backoff between phase retry attempts (GAP 2 from self-healing-gaps.md)

## Context

The phase retry loop in `go/internal/core/orchestrator.go` retries immediately on transient
bridge errors and ArtifactTimeout (exit 81) without any inter-attempt sleep. Under load
or when a transient failure reflects a resource contention issue, immediate retry can
collide with the same condition. GAP 2 in `docs/architecture/self-healing-gaps.md` lists
"no backoff" as an open item. This task closes it by sleeping between retry attempts.

The backoff should be controlled by `EVOLVE_RETRY_BACKOFF_BASE_S` (default 5 seconds),
which is added to `CLAUDE.md`'s env-var table.

## Acceptance Criteria

### AC-1: Orchestrator imports `time` and sleeps between retry attempts [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -n "Sleep\|backoff\|EVOLVE_RETRY_BACKOFF_BASE_S" "$REPO_ROOT/go/internal/core/orchestrator.go" \
  | grep -v "^.*//.*Sleep" \
  || { echo "RED: no Sleep or backoff in orchestrator.go"; exit 1; }
echo "GREEN: backoff sleep present in orchestrator.go"
```

### AC-2: EVOLVE_RETRY_BACKOFF_BASE_S documented in CLAUDE.md [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q "EVOLVE_RETRY_BACKOFF_BASE_S" "$REPO_ROOT/CLAUDE.md" \
  || { echo "RED: EVOLVE_RETRY_BACKOFF_BASE_S not in CLAUDE.md env-var table"; exit 1; }
echo "GREEN: EVOLVE_RETRY_BACKOFF_BASE_S documented"
```

### AC-3: Backoff is not applied on the first attempt (only between retries) [code]
The sleep must only occur when attempt > 1, not before the first launch.
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
# A test with name containing "Backoff" must exist in the core package.
cd "$REPO_ROOT/go" && go test ./internal/core/... -run "Backoff" -v -count=1 -timeout 60s 2>&1 \
  | grep -E "--- PASS.*Backoff|--- FAIL.*Backoff" \
  | grep -v FAIL \
  | grep -q PASS \
  || { echo "RED: no passing Backoff test found in ./internal/core/..."; exit 1; }
echo "GREEN: Backoff test passes"
```

### AC-4: EVOLVE_RETRY_BACKOFF_BASE_S=0 disables backoff (test must verify) [code]
Setting the env var to 0 must produce zero sleep, allowing tests to run without delays.
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go" && go test ./internal/core/... -run "BackoffDisabled\|Backoff.*Zero\|Backoff.*NoSleep" \
  -v -count=1 -timeout 60s 2>&1 \
  | grep -E "--- PASS|--- FAIL" \
  | grep -v FAIL \
  | grep -q PASS \
  || { echo "RED: no passing BackoffDisabled/Zero/NoSleep test found"; exit 1; }
echo "GREEN: Zero-backoff test passes"
```

### AC-5: All core package tests still pass (no regression) [code]
```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go" && go test ./internal/core/... -count=1 -timeout 120s 2>&1 \
  | tail -5 \
  | grep -q "^ok" \
  || { echo "RED: ./internal/core/... tests failed"; exit 1; }
echo "GREEN: ./internal/core/... all pass"
```
