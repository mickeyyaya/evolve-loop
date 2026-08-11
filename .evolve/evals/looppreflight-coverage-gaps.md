# Eval: looppreflight-coverage-gaps

## Criteria

### C1 — Coverage threshold [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/looppreflight/... -cover -count=1 2>&1 | grep "coverage:" | awk '{pct=$NF; sub(/%/, "", pct); if (pct+0 >= 82) exit 0; else exit 1}'
```

Expected: exit 0 (coverage ≥ 82%)

### C2 — All tests pass [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/looppreflight/... -count=1 -race 2>&1 | tail -5
```

Expected: `ok github.com/mickeyyaya/evolve-loop/go/internal/looppreflight` (exit 0, no data races)

### C3 — defaultDirWritable covered (positive case) [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/looppreflight/... -run TestDefaultDirWritable -v -count=1 2>&1 | grep -q "PASS" && echo OK || echo FAIL
```

Expected: `OK`

### C4 — defaultTmuxSessions covered (error path) [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/looppreflight/... -run TestDefaultTmuxSessions -v -count=1 2>&1 | grep -q "PASS" && echo OK || echo FAIL
```

Expected: `OK`

### C5 — bootRCName covers unknown code (negative case) [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/looppreflight/... -run TestBootRCName -v -count=1 2>&1 | grep -q "PASS" && echo OK || echo FAIL
```

Expected: `OK`

### C6 — resolve() nil-defaults branch covered [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go
go test ./internal/looppreflight/... -run TestResolve -v -count=1 2>&1 | grep -q "PASS" && echo OK || echo FAIL
```

Expected: `OK`

### C7 — No new files outside go/internal/looppreflight/ [code]

```bash
git -C /Users/danleemh/ai/claude/evolve-loop diff --name-only HEAD | grep -v "^go/internal/looppreflight/" | grep "\.go$" && echo LEAKED || echo CLEAN
```

Expected: `CLEAN` (all new .go files are within the package)
