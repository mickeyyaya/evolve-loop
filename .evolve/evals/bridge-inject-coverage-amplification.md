# Eval: bridge-inject-coverage-amplification

## Task
Push bridge package total coverage from 94.2% to ≥95% by covering `injectText` error
paths (LoadBuffer fail, PasteBuffer fail) and fixing the `dismissCodexUpdateNag`
non-atomic write (cycle-276 audit L1). These are the two highest-leverage remaining
coverage gaps in `driver_tmux_repl.go`.

## Criteria

### C1 — injectText coverage improves from 69.2% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -coverprofile=/tmp/c277-cov.out 2>&1 | tail -3 && \
  go tool cover -func=/tmp/c277-cov.out | grep "driver_tmux_repl.go:745"
```
Expected: coverage for `injectText` is ≥80% (not the previous 69.2%).

### C2 — Total bridge coverage ≥95.0% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -coverprofile=/tmp/c277-cov2.out 2>&1 | tail -5 && \
  go tool cover -func=/tmp/c277-cov2.out | grep "^total"
```
Expected: total coverage ≥95.0%.

### C3 — Negative: injectText LoadBuffer error path returns error [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestInjectTextLoadBufferError -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`.

### C4 — dismissCodexUpdateNag uses atomic write [code]
```bash
grep -n "WriteFile\|CreateTemp\|Rename" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/codex_pretrust.go | grep -A2 -B2 "dismissCodexUpdateNag" | head -10
```
Expected: `CreateTemp` and `Rename` (or `os.Rename`) present in the function, no bare `os.WriteFile` for the final write.

### C5 — Bridge suite still passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -timeout 60s 2>&1 | grep -E "^ok|FAIL"
```
Expected: all packages `ok`, no `FAIL`.
