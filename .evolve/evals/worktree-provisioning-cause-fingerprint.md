# Eval: Worktree provisioning cause fingerprint

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -v ./internal/core -run 'TestWorktreeProvisionFailure_(PersistsCause|DigestUsesProvisioningCause|SuccessAddsNoFailureReason)$'`

## Regression Evals (full test suite)
- `[code]` `cd go && go test -count=1 -race ./internal/core/...`

## Acceptance Checks
- `[code]` `cd go && go test -count=1 -v ./internal/core -run 'TestWorktreeProvisionFailure_DigestUsesProvisioningCause$'`
- `[code]` `cd go && go test -count=1 ./internal/core -run 'TestWorktreeProvisionFailure_SuccessAddsNoFailureReason$' # negative: success must_not persist failure`
- `[code]` `cd go && go test -count=1 ./internal/core -run 'TestWorktreeProvisionFailure_DigestUsesProvisioningCause$' # edge: empty stderr boundary`
- `[model]` Rubric: "A failed worktree provision remains fail-fast for source phases, but failure-digest.json identifies the git provisioning error rather than a downstream scout/build role-gate refusal; successful provisioning does not add a failure reason." — threshold: >= 80

## Adversarial Cases
- Negative: successful worktree creation must leave cycle failure reasons unchanged.
- Edge/OOD: a provisioning error with empty stderr must still produce a non-empty, stable cause identity from exit code/error context.
- Cheapest gaming fake: logging the cause only to stderr; the digest assertion reads the persisted artifact and fails that fake.

## Thresholds
- All checks: pass@1 = 1.0
