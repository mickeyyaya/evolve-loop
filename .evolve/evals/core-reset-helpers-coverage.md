# Eval: core-reset-helpers-coverage

## Task
Add unit tests in `go/internal/core` covering pure I/O helper functions in `reset.go` currently at 48–66% statement coverage:
`pathWithin` (all branches), `readJSONMapFile` (missing/empty/invalid JSON), `writeJSONMapFileAtomic` (round-trip).

## Criteria

### C1 — pathWithin all branches covered [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestPathWithin -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`.

### C2 — readJSONMapFile: missing file returns empty map (not error) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestReadJSONMapFile -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`.

### C3 — writeJSONMapFileAtomic round-trip (write then read back) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestWriteJSONMapFileAtomic -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS`.

### C4 — Negative: readJSONMapFile rejects malformed JSON [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestReadJSONMapFile_InvalidJSON -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS` (error is returned, not nil).

### C5 — Negative: pathWithin rejects traversal (../escape) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -run TestPathWithin_Traversal -v -count=1 2>&1 | grep -E "PASS|FAIL"
```
Expected: `PASS` (returns false for `../../escape` attempt).

### C6 — Overall core package coverage ≥ 80% after both tasks [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/ -count=1 -coverprofile=/tmp/core-reset-cov.out 2>&1 | tail -3 && \
  pct=$(go tool cover -func=/tmp/core-reset-cov.out | grep '^total' | awk '{print $3}' | tr -d '%') && \
  echo "Total coverage: ${pct}%" && \
  python3 -c "exit(0 if float('${pct}') >= 80.0 else 1)"
```
Expected: exit 0 (coverage ≥ 80.0%).
