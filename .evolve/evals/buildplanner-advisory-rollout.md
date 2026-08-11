# Eval: buildplanner advisory rollout

## Task
T1 — add tests to buildplanner package + wire build-plan.md injection into Builder's ComposePrompt

## Acceptance Criteria

### Gate 1: buildplanner package has tests [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/buildplanner/... -v 2>&1
```
Expected: exit 0, at least 4 passing tests (ShouldSkip shadow, ShouldSkip advisory, Classify empty, Classify valid)

### Gate 2: Builder injects build-plan.md when present [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/phases/build/... -v -run TestComposePrompt 2>&1
```
Expected: exit 0, test named TestComposePrompt_InjectsBuildPlan or similar passes

### Gate 3: All existing tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... 2>&1 | tail -10
```
Expected: exit 0, zero FAIL lines

### Falsifiable claim
- `go test ./internal/phases/buildplanner/...` exits 0 after cycle (not before: currently [no test files])
- `go test ./internal/phases/build/... -v -run TestComposePrompt` shows a test that reads a temp workspace file
