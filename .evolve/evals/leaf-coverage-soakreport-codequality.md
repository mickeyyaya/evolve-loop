# Eval: leaf-coverage-soakreport-codequality

## Objective
Add targeted tests covering the uncovered branches in two leaf-package helpers:
- `soakreport.appendOnce` (50% → target ≥80%): the "item already present" early-return path
- `codequality.firstLine` (66.7% → target ≥90%): the "no newline in string" fallback path

## Criteria

### C1: `TestAppendOnce_AlreadyPresent` passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/soakreport/... -run TestAppendOnce_AlreadyPresent -count=1 -v
```
Expected: `--- PASS: TestAppendOnce_AlreadyPresent`

### C2: `TestFirstLine_NoNewline` passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/codequality/... -run TestFirstLine_NoNewline -count=1 -v
```
Expected: `--- PASS: TestFirstLine_NoNewline`

### C3: `soakreport` package coverage for `appendOnce` rises above 66% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/soakreport/... -count=1 -coverprofile=/tmp/c-soakreport.out && go tool cover -func=/tmp/c-soakreport.out | grep appendOnce
```
Expected: the `appendOnce` line shows ≥66.7% (from current 50%).

### C4: Full `soakreport` + `codequality` suites pass without regression [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/soakreport/... ./internal/codequality/... -count=1
```
Expected: both packages report `ok`.

### C5 (negative): `appendOnce` with a missing item still appends [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/soakreport/... -run TestAppendOnce -count=1 -v
```
Expected: ALL `TestAppendOnce*` tests pass (the new test must not break the existing absent-item path).
