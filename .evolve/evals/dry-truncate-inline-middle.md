# Eval: dry-truncate-inline-middle

## Task
Extract the byte-identical `truncateInline` and `truncateMiddle` helpers from
`go/internal/logfilter/streamjson.go` and `go/internal/phasestream/classify.go`
into a new `go/internal/textutil/textutil.go` package, then replace both
local definitions with calls to the shared functions.

## Acceptance Criteria

### AC1: textutil package exists with both exported functions [code]
```bash
grep -n "^func TruncateInline\|^func TruncateMiddle" /Users/danleemh/ai/claude/evolve-loop/go/internal/textutil/textutil.go
# Expected: two lines found (one per function), exit 0
```

### AC2: logfilter no longer defines truncateInline or truncateMiddle [code]
```bash
grep -c "^func truncateInline\|^func truncateMiddle" /Users/danleemh/ai/claude/evolve-loop/go/internal/logfilter/streamjson.go
# Expected output: 0
```

### AC3: phasestream no longer defines truncateInline or truncateMiddle [code]
```bash
grep -c "^func truncateInline\|^func truncateMiddle" /Users/danleemh/ai/claude/evolve-loop/go/internal/phasestream/classify.go
# Expected output: 0
```

### AC4: logfilter tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/logfilter/... -count=1 2>&1 | tail -3
# Expected: ok  github.com/mickeyyaya/evolve-loop/go/internal/logfilter
```

### AC5: phasestream tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phasestream/... -count=1 2>&1 | tail -3
# Expected: ok  github.com/mickeyyaya/evolve-loop/go/internal/phasestream
```

### AC6: textutil tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/textutil/... -count=1 2>&1 | tail -3
# Expected: ok  github.com/mickeyyaya/evolve-loop/go/internal/textutil
```

### AC7: Negative — local truncateInline definitions are gone from both callers [code]
```bash
# A gaming fake would add the functions back under a different name — this checks the
# canonical names are absent and callers reference textutil
grep -rn "func truncateInline\|func truncateMiddle" /Users/danleemh/ai/claude/evolve-loop/go/internal/logfilter/ /Users/danleemh/ai/claude/evolve-loop/go/internal/phasestream/ 2>/dev/null | grep -v "_test.go" | wc -l | tr -d ' '
# Expected: 0
```

### AC8: Build clean across all three packages [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./internal/textutil/... ./internal/logfilter/... ./internal/phasestream/... 2>&1; echo "EXIT:$?"
# Expected: EXIT:0 with no errors
```
