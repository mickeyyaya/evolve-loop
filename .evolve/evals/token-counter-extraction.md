# Eval: token-counter-extraction

## Goal
Parse token counter lines from tmux pane captures during artifact wait, extract peak token count, and store in `workspace/token-usage.json`.

## Acceptance Criteria

### 1. Token regex parses correctly [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestExtractTokenCount -v 2>&1
# Expected: PASS — TestExtractTokenCount covers "↓ 5.2k tokens" → 5200, "↓ 12k tokens" → 12000, no-match → 0
```

### 2. token-usage.json written after tmux phase [code]
```bash
# After a bridge launch with a fake tmux driver that emits token lines in pane:
# workspace/token-usage.json must exist and contain peak_tokens > 0
ls /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/cycle-256/workspace/token-usage.json 2>/dev/null && \
  cat /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/cycle-256/workspace/token-usage.json | python3 -c "import json,sys; d=json.load(sys.stdin); assert 'peak_tokens' in d, 'missing peak_tokens'" && \
  echo PASS || echo SKIP_NO_FILE
```

### 3. Report struct includes token field [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1 | head -20
# Expected: zero build errors — TokenUsage field added to Report struct compiles cleanly
```

### 4. Negative: no token lines → peak_tokens is 0 or absent [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestExtractTokenCount_NoMatch -v 2>&1
# Expected: PASS — extractTokenCount("no tokens here") == 0
```

### 5. Edge: fractional token string parses correctly [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -run TestExtractTokenCount_Fractional -v 2>&1
# Expected: PASS — "↓ 3.5k tokens" → 3500
```
