# Eval: bridge-gofmt-s-gate-fix

## Task
Unify the `gofmt` contract across commit-gate and CI. CI uses `gofmt -s` (simplify);
the commit gate used plain `gofmt`. Fix the gate script, fix the one current violation
in `faketmux_amplify_test.go`, and add a regression predicate so the mismatch cannot
silently recur.

## Criteria

### C1 — No gofmt -s violations in the go/ tree [code]
```bash
gofmt -s -l /Users/danleemh/ai/claude/evolve-loop/go/
```
Expected: empty output (exit 0 and no filenames printed).

### C2 — Commit-gate-runner uses gofmt -s -l [code]
```bash
grep "gofmt" /Users/danleemh/ai/claude/evolve-loop/commit-gate/commit-gate-runner.sh | grep -v "^#"
```
Expected: every gofmt invocation includes the `-s` flag (no bare `gofmt -l` without `-s`).

### C3 — Commit-gate T3 test rejects a gofmt-clean-but-not-s-clean file [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop && bash go/test/commitgate/commit-gate-test.sh 2>&1 | grep -E "T3|gofmt" | head -10
```
Expected: T3 test line shows ok/pass (gate still correctly rejects the planted violation).

### C4 — faketmux_amplify_test.go passes full bridge suite [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -short -timeout 60s 2>&1 | tail -5
```
Expected: all packages `ok`, no `FAIL`.
