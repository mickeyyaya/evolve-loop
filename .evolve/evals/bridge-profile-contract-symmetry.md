# Eval: bridge-profile-contract-symmetry

## Task
Fix asymmetric profile-contract between runner and bridge (inbox HIGH:
dispatchable-agent-profile-completeness). The bridge hard-fails (ExitBadFlags=10) when
LoadProfile is given a missing file; the runner silently tolerates a missing profile
(loader.Get miss → prof=nil → passes the unconstructed profilePath to bridge anyway).
The fix: the runner must detect a missing profile at dispatch time and fail fast with a
diagnostic, matching the bridge's strict contract.

## Criteria

### C1 — Runner fails fast on missing profile [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/runner/... -run TestRunnerMissingProfileFastFail -v -count=1 -timeout 30s 2>&1 | grep -E 'PASS|FAIL|missing.*profile|profile.*not.*found|ExitBadFlags'
```
Expected: `PASS`; test confirms runner returns a non-nil error (or non-zero exit) when the
profile JSON file does not exist, instead of proceeding with nil.

### C2 — Diagnostic message includes the profile path [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/runner/... -run TestRunnerMissingProfileDiagnostic -v -count=1 -timeout 30s 2>&1 | grep -E 'PASS|FAIL|profile.*path|\.json'
```
Expected: `PASS`; the error or stderr output names the missing profile file path so the
operator can diagnose immediately without reading logs.

### C3 — Negative: valid profile path proceeds normally [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/runner/... -run TestRunnerValidProfileProceeds -v -count=1 -timeout 30s 2>&1 | grep -E 'PASS|FAIL'
```
Expected: `PASS`; a present profile does not trigger the fast-fail path.

### C4 — Runner package passes full suite [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/runner/... -count=1 -short -timeout 60s 2>&1 | tail -3
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/phases/runner` with no FAIL.
