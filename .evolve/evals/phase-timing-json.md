# Eval: phase-timing-json

<!-- challenge-token: bd89ec70f56cebf4 -->

## Task
After each `RunCycle` execution, the orchestrator must persist per-phase latency data to
`<workspace>/phase-timing.json` and emit a `kind=phase_retry` ledger entry on retry.

## Acceptance Criteria

### AC-1: `phase-timing.json` is written after RunCycle [code]
```bash
# Positive: after a cycle runs at least one phase, workspace must contain phase-timing.json
ls /Users/danleemh/ai/claude/evolve-loop/go/internal/core/
grep -q "phase-timing.json\|phaseTiming\|PhaseTimingEntry" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Should exit 0 — code must reference phase-timing.json
```

### AC-2: PhaseTimingEntry struct has required fields [code]
```bash
grep -q "duration_ms\|DurationMS\|CostUSD\|cost_usd\|Verdict\|verdict" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Should exit 0 — struct fields for phase timing must exist
```

### AC-3: phase_retry ledger entry emitted on retry [code]
```bash
grep -q "phase_retry\|PhaseRetry" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Should exit 0 — kind=phase_retry must be referenced in orchestrator
```

### AC-4: Unit test for phase-timing.json write [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestRunCycle.*Timing\|TestPhaseTimingJSON\|TestPhaseTiming" -v 2>&1 | grep -E "PASS|RUN|FAIL"
# Should find and pass at least one timing-related test
```

### AC-5: Negative — no phase-timing.json when no phases run [code]
```bash
# The build must not regress existing tests
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... 2>&1 | tail -5
# Should exit 0 — no regressions
```
