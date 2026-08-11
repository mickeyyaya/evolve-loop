# Eval: bridge-launch-stderr-persistence

## Task
When `Engine.LaunchArgs` exits early with ExitBadFlags=10 (missing profile, bad flags,
unreadable prompt), the human-readable diagnostic message is written to `stderrBuf` but
NOT included in the returned error string. As a result, `failure-diag.json` records
only `"bridge: launch exit=10"` with no cause. Fix: include the stderr content in the
engine-level error message so the cause propagates to `failure-diag.json` and the
runner diagnostics without requiring a separate file.

## Criteria

### C1 — TestLaunchErrorMessageIncludesCause passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestLaunchErrorMessageIncludesCause -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`; the test simulates a missing-profile launch and asserts the error
message contains the `[bridge]` diagnostic line (not just `bridge: launch exit=10`).

### C2 — Negative: ExitOK path is unaffected [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestLaunchSuccessNoStderrInError -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`; on a successful launch the error is nil regardless of stderr content.

### C3 — Engine stderr plumbing (engine.go) does not discard stderrBuf on failure [code]
```bash
grep -n "stderrBuf\|Stderr\|ExitBadFlags" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/engine.go | head -20
```
Expected: at least one line shows `stderrBuf.String()` referenced in the error return
path (the non-OK exit code branch), not just in `resp.Stderr`.

### C4 — Bridge suite still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -timeout 60s 2>&1 | grep -E "^ok|FAIL"
```
Expected: all packages `ok`, no `FAIL`.
