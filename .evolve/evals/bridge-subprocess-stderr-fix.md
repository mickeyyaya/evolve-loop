# Eval: bridge-subprocess-stderr-fix

## Task
`driver_claudep.go:84` sends subprocess stderr only to the log file (`stderrF`), not to
`deps.Stderr` (which becomes `stderrBuf` in `engine.Launch`). When a headless launch fails,
`engine.go:380` returns `"bridge: launch exit=X"` with no cause string — the subprocess
diagnostic (e.g. `"binary not found"`) is silently dropped. Fix: wire subprocess stderr to
`io.MultiWriter(stderrF, deps.Stderr)` in `driver_claudep.go`, then include
`stderrBuf.String()` in `engine.go:380`'s error format — closing both the driver gap and
the engine-level threading T3 fix that cycle-277 did not ship.

## Criteria

### C1 — TestEngineLaunch_SubprocessStderr_IncludedInError passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestEngineLaunch_SubprocessStderr_IncludedInError -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS: TestEngineLaunch_SubprocessStderr_IncludedInError` — the returned
error message includes the custom subprocess stderr string, not just the exit code.

### C2 — TestEngineLaunch_MultilineStderr_IncludedInError passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestEngineLaunch_MultilineStderr_IncludedInError -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS: TestEngineLaunch_MultilineStderr_IncludedInError` — multiline
subprocess stderr is trimmed and included in the error.

### C3 — Negative: subprocess stderr does NOT appear on successful launch [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestEngineLaunch_SubprocessStderr_AbsentOnSuccess -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS` — on ExitOK the returned error is nil; stderr content stays in
`resp.Stderr` but is not injected into a non-nil error.

### C4 — engine.go non-OK error path references stderrBuf [code]
```bash
grep -n "stderrBuf\|Stderr\|ExitBadFlags\|launch exit" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/engine.go | grep -v "resp.Stderr" | head -10
```
Expected: at least one line shows `stderrBuf.String()` or `strings.TrimSpace(stderrBuf`
referenced in a `fmt.Errorf` call (the non-OK branch around line 380).

### C5 — driver_claudep.go uses io.MultiWriter for subprocess stderr [code]
```bash
grep -n "MultiWriter\|stderrF\|deps\.Stderr" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/driver_claudep.go | head -10
```
Expected: `io.MultiWriter` appears in driver_claudep.go, wiring `stderrF` and `deps.Stderr`
as the subprocess stderr writer.

### C6 — Bridge suite still passes with no regressions [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -timeout 90s 2>&1 | grep -E "^ok|FAIL"
```
Expected: all packages `ok`, no `FAIL` lines.
