# Eval: chaos-malformed-artifact-warn-completion

## Purpose
Verify that when a phase receives a malformed or truncated upstream artifact,
it reaches graceful WARN-completion (errors-as-observations pattern) rather
than a hard abort, panic, or unrecovered crash.

## Code Graders [code]

- `grep -rn "malform\|truncat\|chaos\|Malform\|Truncat\|Chaos" go/internal/phases/ship/ --include="*_test.go" | grep -qv "^Binary"` — chaos test exists in ship package
- `cd go && go test ./internal/phases/ship/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — ship package tests pass
- `grep -rn "malform\|truncat\|chaos\|Malform\|Truncat\|Chaos" go/internal/phases/ --include="*_test.go" | grep -c "malform\|truncat\|Chaos" | awk '{exit($1<2)}'` — at least 2 chaos test scenarios exist

## Regression Graders

- `cd go && go test ./internal/phases/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — all phase tests pass
- `cd go && go test ./internal/core/... 2>&1 | grep -c "^--- FAIL" | grep "^0$"` — core tests unaffected

## Acceptance Notes
- Chaos tests feed malformed/truncated JSON or invalid content as the upstream artifact
- Assert WARN verdict (or graceful handling), NOT panic, NOT hard abort
- Two scenarios minimum: (1) malformed acs-verdict.json content to checkEGPSGate, (2) truncated audit handoff to audit-binding path
- Tests must be deterministic (no real subprocesses, use the existing fake-runner seams)
