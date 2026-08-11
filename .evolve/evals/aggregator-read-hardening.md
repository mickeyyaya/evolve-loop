# Eval: aggregator-read-hardening

## Task Slug
aggregator-read-hardening

## Description
The aggregator's pre-flight check verifies existence and non-empty size but does
NOT verify readability. Merge functions silently ignore os.ReadFile errors with
`body, _ := os.ReadFile(w)`. This task adds a ReadFile seam to aggregator.Inputs,
performs a readability check in pre-flight, and propagates the seam through all
merge functions to make read errors explicit and testable.

## Acceptance Criteria

### AC1 — ReadFile seam field exists in aggregator.Inputs [code]

```bash
grep -c "ReadFile" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/aggregator/aggregator.go
```

Expected: output ≥ 3 (Inputs field + pre-flight call + at least one merge function use)

### AC2 — Pre-flight readability check returns ExitUsageErr on unreadable worker [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/aggregator/... \
    -run "TestAggregate_UnreadableWorker" \
    -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected: `--- PASS: TestAggregate_UnreadableWorker`

### AC3 — Merge functions use the ReadFile seam rather than os.ReadFile directly [code]

```bash
grep -c "os\.ReadFile" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/aggregator/aggregator.go
```

Expected: 0 (all direct os.ReadFile calls replaced by seam)

### AC4 — All existing aggregator tests still pass [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/aggregator/... -count=1 2>&1 | tail -3
```

Expected: `ok github.com/mickeyyaya/evolve-loop/go/internal/aggregator`

### AC5 — Coverage of Aggregate() function increases from 82.1% baseline [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/aggregator/... -count=1 -cover 2>&1 | grep "coverage:"
```

Expected: coverage ≥ 95% (Aggregate() function fully covered with new error paths tested)

### Negative case — merge with unreadable worker must fail, not silently produce empty content [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/aggregator/... \
    -run "TestAggregate_Unreadable" \
    -count=1 -v 2>&1 | grep -E "ExitUsageErr|rc=2|PASS|FAIL"
```

Expected: test passes and verifies rc == ExitUsageErr (2)

### Edge case — nil ReadFile seam must default to os.ReadFile without panic [code]

```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/aggregator/... \
    -run "TestAggregate_NilReadFileSeamDefaultsToOS" \
    -count=1 -v 2>&1 | grep -E "^--- (PASS|FAIL)"
```

Expected: `--- PASS` (nil seam defaults cleanly, no panic)
