# Eval: resume-inserted-phase

## Goal
`RunCycleFromPhase` in `go/internal/core/resume.go` currently rejects any phase that fails `Phase.IsValid()` — this includes all advisor-inserted phases (e.g. `mutation-gate`, `bug-reproduction`, `test-amplification`). When a crash happens while a spine phase is running and the last-completed phase was an inserted phase, the checkpoint records an inserted-phase name and resume fails. Fix: accept phases registered in `o.runners` even if they fail the spine's `IsValid()` check.

## Criteria

### C1 — inserted phase in runners is accepted [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -run "TestRunCycleFromPhase_InsertedPhase" -count=1 -v 2>&1
```
**Expected:** Test PASS — `RunCycleFromPhase` with `resumePoint.Phase = "mutation-gate"` (a phase registered in `o.runners`) must NOT return "invalid resume phase" error.

### C2 — truly unknown phase still rejected [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -run "TestRunCycleFromPhase_UnknownPhase" -count=1 -v 2>&1
```
**Expected:** Test PASS — `RunCycleFromPhase` with `resumePoint.Phase = "does-not-exist"` (not in runners, not a spine phase) must return an error containing "invalid resume phase".

### C3 — PhaseEnd and PhaseStart still rejected [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -run "TestRunCycleFromPhase_RejectsEndStart" -count=1 -v 2>&1
```
**Expected:** Test PASS — `RunCycleFromPhase` with PhaseEnd or PhaseStart must still return an error (not a valid resume target).

### C4 — full core test suite still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -count=1 -short 2>&1 | tail -5
```
**Expected:** `ok github.com/mickeyyaya/evolve-loop/go/internal/core` — all tests PASS, no regressions.
