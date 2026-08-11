# Eval: cyclebudget coverage 95

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -race ./internal/cyclebudget/ -run TestStageString_AllValuesAndUnknown`
- `[code]` `cd go && go test -count=1 -coverprofile=/tmp/cyclebudget-coverage-95.cover ./internal/cyclebudget/ >/tmp/cyclebudget-coverage-95.out && awk '/total:/ {gsub("%","",$3); if ($3+0 < 95) exit 1}' <(go tool cover -func=/tmp/cyclebudget-coverage-95.cover)`

## Regression Evals (full test suite)
- `[code]` `cd go && go test -count=1 ./internal/cyclebudget/...`

## Acceptance Checks
- `[code]` `git diff --name-only -- go/internal/cyclebudget | awk 'BEGIN{ok=1} !/_test\\.go$/ {ok=0} END{exit !ok}'`

## Adversarial Cases
- Negative: an undefined `Stage` value such as `Stage(-1)` must render `"off"` rather than panic or expose an unstable numeric string.
- Edge/OOD: every defined stage (`Off`, `Advisory`, `Enforce`) and a large unknown stage value must be table-driven through `String`.
- Cheapest gaming fake: testing only `Enforce.String()` again. The named all-values-and-unknown test must fail that implementation.

## Thresholds
- All checks: pass@1 = 1.0
- Package statement coverage: >= 95.0%
