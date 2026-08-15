# Eval: Worktree retry diagnostic integrity

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -v ./internal/gitexec -run 'TestAddWorktreeWithRetry_(PreservesFirstFailure|AnnouncesBeforeBackoff|PermanentFailureSkipsBackoff)$'`

## Regression Evals (full test suite)
- `[code]` `cd go && go test -count=1 ./internal/gitexec ./internal/core ./internal/swarm ./cmd/evolve`

## Acceptance Checks
- `[code]` `cd go && go test -count=1 -race ./internal/gitexec -run 'TestAddWorktreeWithRetry_(PreservesFirstFailure|AnnouncesBeforeBackoff)$'`
- `[code]` `cd go && go test -count=1 ./internal/gitexec -run 'TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff$' # negative: permanent failure should_fail retry`
- `[code]` `cd go && go test -count=1 ./internal/gitexec -run 'TestAddWorktreeWithRetry_PreservesFirstFailure$' # edge: empty stderr boundary`
- `[model]` Rubric: "The terminal diagnostic retains both the first retryable failure and final failure, OnRetry executes before Sleep, and comments do not assert that every rc=255 is a repository-lock failure." — threshold: >= 80

## Adversarial Cases
- Negative: a permanent `not a git repository` failure must return after one attempt with no retry callback or sleep.
- Edge/OOD: an empty-stderr rc=255 followed by a different rc=128 failure must report both attempts, not only the last.
- Cheapest gaming fake: moving only the callback while continuing to overwrite the first failure; `TestAddWorktreeWithRetry_PreservesFirstFailure` fails that fake.

## Thresholds
- All checks: pass@1 = 1.0
