# Eval: bridge-injecttext-atomic-coverage

## Task
Three open items from cycles 276-277:
1. `FakeTmuxController` lacks `LoadBufferErr`/`PasteBufferErr` error-injection fields,
   leaving `injectText` error paths at 69.2% (uncovered: LoadBuffer fail, PasteBuffer fail,
   mkdir fail).
2. `dismissCodexUpdateNag` (codex_pretrust.go:180) uses bare `os.WriteFile`; `pretrustCodexProjects`
   already uses `CreateTemp+Write+Rename`. The non-atomic write is a cycle-276 audit L1 defect.
3. `writeTokenUsage` at 66.7% — the `json.MarshalIndent` error path is unreachable in
   production but the early-return branch is uncovered.

Fix: add `LoadBufferErr`/`PasteBufferErr` to `FakeTmuxController`, write three injectText
behavioral tests, fix the atomic write in `dismissCodexUpdateNag`, and add a `writeTokenUsage`
nil-workspace test. Total bridge coverage should remain ≥95.0%.

## Criteria

### C1 — injectText coverage improves to ≥85% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -coverprofile=/tmp/c278-cov.out 2>&1 | tail -3 && \
  go tool cover -func=/tmp/c278-cov.out | grep "driver_tmux_repl.go:745"
```
Expected: injectText function shows ≥85.0% (up from 69.2%).

### C2 — TestInjectTextLoadBufferError passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestInjectTextLoadBufferError -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS: TestInjectTextLoadBufferError`.

### C3 — TestInjectTextPasteBufferError passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -run TestInjectTextPasteBufferError -v -count=1 2>&1 | grep -E "PASS|FAIL|--- "
```
Expected: `--- PASS: TestInjectTextPasteBufferError`.

### C4 — dismissCodexUpdateNag uses atomic write (CreateTemp+Rename) [code]
```bash
grep -n "CreateTemp\|os.Rename\|os.WriteFile" /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/codex_pretrust.go | head -10
```
Expected: `dismissCodexUpdateNag` (around line 158) shows `CreateTemp`/`Rename` pattern;
no bare `os.WriteFile` for the version-json path.

### C5 — Total bridge coverage ≥95.0% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -coverprofile=/tmp/c278-total.out 2>&1 | tail -3 && \
  pct=$(go tool cover -func=/tmp/c278-total.out | grep "^total" | awk '{print $3}' | tr -d '%') && \
  echo "Total coverage: ${pct}%" && \
  python3 -c "exit(0 if float('${pct}') >= 95.0 else 1)"
```
Expected: `Total coverage: ≥95.0%`.

### C6 — Bridge suite passes with no regressions [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/bridge/... -count=1 -short -timeout 90s 2>&1 | grep -E "^ok|FAIL"
```
Expected: all packages `ok`, no `FAIL` lines.
