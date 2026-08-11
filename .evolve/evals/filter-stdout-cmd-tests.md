# Eval: filter-stdout command integration tests

## Task
T2 — add integration tests to go/cmd/filter-stdout covering all 3 exit paths

## Acceptance Criteria

### Gate 1: cmd/filter-stdout has tests [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/filter-stdout/... -v 2>&1
```
Expected: exit 0, at least 3 passing tests (no-args, bad workspace, valid call)

### Gate 2: exit code contract [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./cmd/filter-stdout/... -v -run TestMain 2>&1
```
Expected: tests verify exit 2 on no args, exit 1 on missing workspace, exit 0 on success

### Gate 3: All existing tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... 2>&1 | grep -E "FAIL|ok" | tail -5
```
Expected: exit 0, zero FAIL lines

### Falsifiable claim
- `go test ./cmd/filter-stdout/...` exits 0 after cycle (currently [no test files])
