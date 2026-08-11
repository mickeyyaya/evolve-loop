# Eval: fix-evolve-cli-env-test-isolation

Phase: builder
Cycle: 293

## Description
Tests in `internal/llmroute` and `internal/phases/runner` that verify CLI resolution from profiles or defaults break when `EVOLVE_CLI=claude-p` is set in the operator shell (e.g., during a soak batch). Fix by adding `t.Setenv("EVOLVE_CLI", "")` in tests that should exercise the profile/default tier without OS-env interference.

## Acceptance Criteria

### C1: Suite passes with EVOLVE_CLI=claude-p [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  EVOLVE_CLI=claude-p go test ./internal/llmroute/... ./internal/phases/runner/... -count=1
```
Both packages must exit 0.

### C2: t.Setenv isolation present [code]
```bash
grep -rn "t.Setenv.*EVOLVE_CLI" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/llmroute/ \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/runner/
```
Must show at least one `t.Setenv("EVOLVE_CLI", "")` in the affected test files.

### C3: Explicit env-override tests still pass [code]
Tests that intentionally test EVOLVE_CLI env priority (e.g., `env_wins_over_profile` in `TestRun_CLIResolutionPrecedence`) must still pass:
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/phases/runner/... -run TestRun_CLIResolutionPrecedence/env_wins_over_profile -v -count=1
```

## Negative Cases

### N1: Tests do NOT suppress env override when env is explicitly set [model]
After the fix, tests that pass `env = map[string]string{"EVOLVE_CLI": "claude-tmux"}` must still see `claude-tmux` win over a profile — `t.Setenv("EVOLVE_CLI", "")` must only appear in subtests/test bodies where the explicit envCLI is empty (no override intended).

### N2: No EVOLVE_CLI leak from profile-isolation to global env [code]
The `t.Setenv` call is scoped per test (Go restores env after test cleanup). Verify the other tests in the same file still see the correct env:
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  EVOLVE_CLI=claude-p go test ./internal/llmroute/... -run TestResolve_EnvWinsOverProfile -v -count=1
```
Must pass (env still wins when explicitly set in reqEnv).
