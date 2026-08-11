# Eval: fix-kill-unix-tmux-socket-isolation

Phase: builder
Cycle: 293

## Description
Add a `tmuxKillerOnSocket(socket string)` helper to `kill_unix.go` and update `kill_unix_test.go` and `swarmrunner_coverage_test.go` to use a private tmux socket (`tmux -L <test-socket>`) so tests never touch sessions on the shared default tmux server.

## Acceptance Criteria

### C1: Private-socket helper exists [code]
```bash
grep -n "tmuxKillerOnSocket\|tmux -L\|tmux.*-L" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/swarmrunner/kill_unix.go
```
Must match at least one line showing the `-L` socket parameter is wired.

### C2: kill_unix_test.go uses private socket [code]
```bash
grep -n "tmuxKillerOnSocket\|tmux.*socket\|-L.*evolve-test" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/swarmrunner/kill_unix_test.go
```
Must match — tests for `tmuxKiller` must use private socket or skip if tmux is absent.

### C3: Tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/swarmrunner/... -run TestTmuxKiller -count=1 -v
```
All `TestTmuxKiller_*` tests pass without requiring a running default tmux server.

### C4: swarmrunner_coverage_test "kill helpers" uses private socket [code]
```bash
grep -n "tmuxKillerOnSocket\|definitely-missing" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/swarmrunner/swarmrunner_coverage_test.go
```
The call that previously used `tmuxKiller(ctx, "definitely-missing-session")` must be updated.

## Negative Cases

### N1: Default tmuxKiller still compiles and is wired in production [code]
`dispatchDeps` in `swarmrunner.go` must still assign `KillTmux: tmuxKiller` (unchanged) — only test callers use the socket variant.
```bash
grep -n "KillTmux.*tmuxKiller\b" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/swarmrunner/swarmrunner.go
```

### N2: Private socket cleanup — no zombie server [code]
If tmux is available, the test must kill the private server after use so it doesn't accumulate across runs. Verify no `evolve-test-*` socket left in default tmux socket location:
```bash
ls /tmp/tmux-*/evolve-test-* 2>/dev/null | wc -l | tr -d ' '
```
Expected: `0` after tests complete.
