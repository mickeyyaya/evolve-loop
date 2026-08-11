# Eval: chaos-backfill-negative-tests

## Task
Add chaos negative tests to `go/internal/backfill/backfill_test.go` covering malformed and truncated upstream artifacts. The goal: malformed/truncated inputs must drive graceful (false, nil) degradation, not panics or hard failures.

Chaos axes to cover:
1. **Truncated clean file** — header present but body cut off mid-word (verify LastIndex finds the header, content extracted is too-short → false, nil)
2. **Binary garbage** — clean file is binary noise with no valid header marker (false, nil)
3. **Header-only** — only the phase header line, no body (too-short → false, nil)
4. **Multiple headers** — content with several occurrences of the header; verify LastIndex picks the LAST occurrence and extracts from there

## Acceptance Criteria

### AC1: New negative tests exist and pass [code]
```
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/backfill/... -v 2>&1 | grep -E "^=== RUN|^--- (PASS|FAIL)|FAIL|ok" | tail -20
```
Expected: All `--- PASS` lines, `ok` at end, no `--- FAIL`

### AC2: Truncated test exists [code]
```
grep -c "runcated\|truncat" /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill_test.go
```
Expected: `> 0` (a test or comment mentioning truncated input)

### AC3: Binary garbage test exists [code]
```
grep -c "garbage\|binary\|noise\|\\\\x00\|\\\\xff" /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill_test.go
```
Expected: `> 0`

### AC4: Negative case — truncated body is (false, nil) not (false, err) [code]
The truncated-body test must verify `ok == false` AND `err == nil` (graceful WARN-completion, not a crash):
```
grep -A8 "runcated\|Truncated" /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill_test.go | grep -c "false\|== nil"
```
Expected: `> 0`

### AC5: Multiple-header test picks last occurrence [code]
```
grep -c "LastIndex\|last.*header\|multiple.*header\|LastOccurrence" /Users/danleemh/ai/claude/evolve-loop/go/internal/backfill/backfill_test.go
```
Expected: `> 0` (a test verifying LastIndex semantics)
