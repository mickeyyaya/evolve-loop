# Eval: checkpoint integrity coverage

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -race ./internal/checkpoint/ -run 'TestRecordIntegrityWithHooks_ErrorMatrix|TestReadExistingIntegrity_(MissingMalformedValid)|TestDecodeIntegrity_(MalformedValid)|TestHasEscalationCheckpoint_EdgeCases'`
- `[code]` `cd go && go test -count=1 -coverprofile=/tmp/checkpoint-integrity.cover ./internal/checkpoint/ >/tmp/checkpoint-integrity.out && awk '/total:/ {gsub("%","",$3); if ($3+0 < 95) exit 1}' <(go tool cover -func=/tmp/checkpoint-integrity.cover)`

## Regression Evals (full test suite)
- `[code]` `cd go && go test -count=1 ./internal/checkpoint/...`

## Acceptance Checks
- `[code]` `cd go && go test -count=1 ./internal/checkpoint/ -run 'TestRecordPhaseIntegrity_PreservesOtherFields|TestRecordPhaseIntegrity_ConcurrentPipeline_NoLostUpdates|TestPhaseBoundaryCheckpointer_CarriesIntegrityChain'`
- `[code]` `git diff --name-only -- go/internal/checkpoint | awk 'BEGIN{ok=1} !/_test\\.go$/ {ok=0} END{exit !ok}'`

## Adversarial Cases
- Negative: injected read, decode, encode, write, and rename failures must return the documented wrapped error; rename failure must attempt temp-file cleanup.
- Edge/OOD: missing files, malformed JSON, nil integrity values, malformed untyped integrity, and a valid empty chain must be distinguished without panic.
- Cheapest gaming fake: a happy-path-only test or direct call that never injects a failing hook. The required named error matrix and wrapped-error assertions must fail that implementation.

## Thresholds
- All checks: pass@1 = 1.0
- Package statement coverage: >= 95.0%
