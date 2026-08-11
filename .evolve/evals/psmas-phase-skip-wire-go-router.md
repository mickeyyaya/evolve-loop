# Eval: Wire PSMAS PhaseSkip in Go Router

## Task
The Go router (`go/internal/router/router.go`) extracts `TriageSignals.PhaseSkip` from the triage handoff but never consumes it. Wire it into the routing decision: when a phase appears in `Signals.Triage.PhaseSkip` (and the env-level PSMAS gate passes), skip it with a `phase_skipped` ledger entry.

## Acceptance Criteria

### AC-1: Router uses Triage.PhaseSkip signal [code]
```bash
grep -n "Triage.PhaseSkip\|triage.*phase_skip\|PhaseSkip" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/router/router.go | wc -l | tr -d ' '
# expect: >= 1 (previously 0)
```

### AC-2: Tests pass for PSMAS skip scenario [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/router/... -run "PSMAS\|PhaseSkip\|phase_skip" -v 2>&1 | tail -5
# expect: output contains "PASS" or "no test files"
```

### AC-3: Full router test suite passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/router/... 2>&1 | tail -3
# expect: ok github.com/mickeyyaya/evolve-loop/go/internal/router
```

### AC-4: PhaseSkip additive merge — skip doesn't override non-skip from failure adapter [code]
```bash
grep -n "additive\|union\|merge.*skip\|skip.*merge" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/router/router.go | head -5
# expect: at least 1 line showing the merge semantics are documented in code
```

### AC-5: Negative — PSMAS skip cannot remove mandatory phases [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/router/... -run "Mandatory\|mandatory" -v 2>&1 | grep -E "PASS|FAIL" | head -5
# expect: all mandatory tests PASS (no regressions)
```

## Grader Notes
AC-1 is the primary indicator that the wiring landed. AC-2 verifies targeted test coverage. AC-5 guards the invariant that mandatory phases cannot be PSMAS-skipped (safety floor).
