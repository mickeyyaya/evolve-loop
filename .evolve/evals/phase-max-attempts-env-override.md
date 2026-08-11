---
title: phase-max-attempts-env-override
cycle: 176
score_cap: 1.0
---

# Eval: EVOLVE_PHASE_MAX_ATTEMPTS env override

## Context

`phaseMaxAttempts` is a hardcoded `const = 2` in `orchestrator.go`. Operators running
multi-CLI or slow-compile phases cannot widen the retry budget without source changes.
`resolvePhaseMaxAttempts(env map[string]string) int` reads `EVOLVE_PHASE_MAX_ATTEMPTS`
from the cycle env snapshot, clamped to [1, 5], defaulting to 2.

## Acceptance Criteria

### AC-1: default returns 2 when env var is unset [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go"
T="TestPhaseMaxAttempts_Default"
go test -list "^${T}$" ./internal/core/... 2>/dev/null | grep -qx "$T" \
  || { echo "RED: $T not discoverable"; exit 1; }
go test -run "^${T}$" ./internal/core/... -count=1 -timeout 30s \
  || { echo "RED: $T failed"; exit 1; }
echo "GREEN: $T passed"
```

### AC-2: env override reads EVOLVE_PHASE_MAX_ATTEMPTS and clamps to [1,5] [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go"
T="TestPhaseMaxAttempts_EnvOverride"
go test -list "^${T}$" ./internal/core/... 2>/dev/null | grep -qx "$T" \
  || { echo "RED: $T not discoverable"; exit 1; }
go test -run "^${T}$" ./internal/core/... -count=1 -timeout 30s \
  || { echo "RED: $T failed"; exit 1; }
echo "GREEN: $T passed"
```

### AC-3: out-of-range values clamp to [1,5] boundary [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go"
T="TestPhaseMaxAttempts_OutOfRange"
go test -list "^${T}$" ./internal/core/... 2>/dev/null | grep -qx "$T" \
  || { echo "RED: $T not discoverable"; exit 1; }
go test -run "^${T}$" ./internal/core/... -count=1 -timeout 30s \
  || { echo "RED: $T failed"; exit 1; }
echo "GREEN: $T passed"
```

### AC-4: non-transient errors not retried beyond first attempt regardless of max [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go"
T="TestPhaseMaxAttempts_NonTransient_NoExtraAttempt"
go test -list "^${T}$" ./internal/core/... 2>/dev/null | grep -qx "$T" \
  || { echo "RED: $T not discoverable"; exit 1; }
go test -run "^${T}$" ./internal/core/... -count=1 -timeout 30s \
  || { echo "RED: $T failed"; exit 1; }
echo "GREEN: $T passed"
```

### AC-5: EVOLVE_PHASE_MAX_ATTEMPTS documented in CLAUDE.md env-var table [code]

```bash
# acs-predicate: config-check — doc-presence of env-var row is inherently a grep;
# no system-under-test exists for "docs mention X". Waived per acs/AGENTS.md.
REPO_ROOT=$(git rev-parse --show-toplevel)
grep -q "EVOLVE_PHASE_MAX_ATTEMPTS" "$REPO_ROOT/CLAUDE.md" \
  || { echo "RED: EVOLVE_PHASE_MAX_ATTEMPTS not in CLAUDE.md env-var table"; exit 1; }
echo "GREEN: EVOLVE_PHASE_MAX_ATTEMPTS documented in CLAUDE.md"
```

## Negative Cases

### NC-1: value 0 clamps to minimum 1 (retry can't be disabled) [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go"
T="TestPhaseMaxAttempts_OutOfRange"
go test -run "^${T}$" ./internal/core/... -count=1 -timeout 30s \
  || { echo "RED: clamp-to-1 assertion failed"; exit 1; }
echo "GREEN: zero clamps to 1 — retry can't be disabled via env"
```

### NC-2: value above 5 clamps to 5 (no unbounded retry) [code]

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT/go"
T="TestPhaseMaxAttempts_OutOfRange"
go test -run "^${T}$" ./internal/core/... -count=1 -timeout 30s \
  || { echo "RED: clamp-to-5 assertion failed"; exit 1; }
echo "GREEN: value>5 clamps to 5 — no unbounded retry path"
```
