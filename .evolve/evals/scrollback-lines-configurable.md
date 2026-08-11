# Eval: scrollback-lines-configurable

## Goal
Replace hardcoded `tmuxArtifactScrollback = 10000` with `EVOLVE_SCROLLBACK_LINES` env var (default 10000) in `driver_tmux_repl.go`, enabling operators to tune memory/speed of final pane capture.

## Acceptance Criteria

### 1. Unit test: env var overrides default [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestScrollbackLinesEnvOverride -v 2>&1
# Expected: PASS — EVOLVE_SCROLLBACK_LINES=2000 causes CapturePane to be called with scrollback=2000
```

### 2. Unit test: zero/negative falls back to default 10000 [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestScrollbackLinesEnvFallback -v 2>&1
# Expected: PASS — EVOLVE_SCROLLBACK_LINES=0 uses 10000; EVOLVE_SCROLLBACK_LINES=-1 uses 10000
```

### 3. Build clean [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1 | head -10
# Expected: zero build errors
```

### 4. Existing tmux repl tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run "TestTmux|TestREPL|TestDriver" -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -20
# Expected: all matching tests PASS, no new failures
```

### 5. Negative: non-numeric value falls back to default [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestScrollbackLinesEnvNonNumeric -v 2>&1
# Expected: PASS — EVOLVE_SCROLLBACK_LINES=abc uses 10000
```
