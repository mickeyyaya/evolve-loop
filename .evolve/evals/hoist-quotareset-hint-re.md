# Eval: hoist-quotareset-hint-re

## Task
Hoist the per-call `regexp.MustCompile(...)` inside `parseHint` in `go/internal/quotareset/quotareset.go` to a package-level variable, eliminating per-call compilation.

## Acceptance Criteria

### AC1: Package-level var declared [code]
```bash
grep -n "^var hintTimeRE\s*=\s*regexp\.MustCompile" ~/ai/claude/evolve-loop/go/internal/quotareset/quotareset.go
# Expected: one line found, exit 0
```

### AC2: No per-call regexp.MustCompile in parseHint [code]
```bash
awk '/^func parseHint/,/^}/' ~/ai/claude/evolve-loop/go/internal/quotareset/quotareset.go | grep -c "regexp\.MustCompile"
# Expected output: 0
```

### AC3: All existing tests still pass [code]
```bash
cd ~/ai/claude/evolve-loop/go && go test ./internal/quotareset/... -count=1 -v 2>&1 | tail -5
# Expected: "ok  github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
```

### AC4: Negative — per-call usage is gone [code]
```bash
# The old inline `re :=` assignment inside parseHint must not exist
awk '/^func parseHint/,/^}/' ~/ai/claude/evolve-loop/go/internal/quotareset/quotareset.go | grep -c "re :=.*MustCompile"
# Expected: 0 (gaming fake would be renaming the var; test above checks pkg-level declaration)
```

### AC5: Build still clean [code]
```bash
cd ~/ai/claude/evolve-loop/go && go build ./internal/quotareset/... 2>&1; echo "EXIT:$?"
# Expected: EXIT:0 with no error output
```
