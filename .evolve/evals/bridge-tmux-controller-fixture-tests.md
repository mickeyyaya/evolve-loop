# Eval: bridge-tmux-controller-fixture-tests

## Task
Build a deterministic fixture corpus from recorded pane transcripts. Implement `FakeTmuxController` replay. Write `tmux_repl_fixture_test.go` covering boot, inject, artifact delivery, and stall/timeout state machine transitions.

## Criteria

### C1 — FakeTmuxController type exists in test package [code]
```bash
grep -rn 'FakeTmux\|fakeTmux' /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/ --include="*_test.go" | head -5
```
Expected: at least one match defining the type.

### C2 — Fixture corpus directory populated [code]
```bash
ls /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/testdata/fixtures/ 2>/dev/null || \
  ls /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/testdata/ | grep -E 'fixture|corpus|replay'
```
Expected: at least one fixture file or directory visible.

### C3 — Fixture tests run and pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestTmuxFixture -count=1 -timeout 30s 2>&1 | tail -5
```
Expected: `ok  github.com/mickeyyaya/evolve-loop/go/internal/bridge` with no FAIL.

### C4 — tmux.go coverage improves from 0% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -coverprofile=/tmp/bridge-tmux-cov.out 2>&1 | tail -3 && \
  go tool cover -func=/tmp/bridge-tmux-cov.out | grep 'tmux.go' | awk '{print $1, $2, $3}' | grep -v '100'
```
Expected: the `0.0%` lines from `tmux.go` are gone or reduced to at most `KillSession`/`run` which may remain at 0% on short mode.

### C5 — Boot-timeout path covered (negative case) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestTmuxFixtureBoot -v -count=1 2>&1 | grep -E 'boot|timeout|ExitREPL|PASS|FAIL'
```
Expected: `PASS` and at least one line mentioning boot or timeout scenario.

### C6 — Artifact delivery path covered (edge: artifact appears on first poll) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestTmuxFixtureArtifact -v -count=1 2>&1 | grep -E 'artifact|PASS|FAIL'
```
Expected: `PASS`.
