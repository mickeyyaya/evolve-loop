# Eval: Extend Anchored Context Injection to Triage & TDD/Builder

## Code Graders (bash commands that must exit 0)
- `grep -q "emit_artifact_with_anchors" go/internal/phases/runner/runner.go`
- `go test -v ./go/internal/phases/runner/... | grep -q "PASS"`

## Regression Evals (full test suite)
- `go test -v ./go/internal/adapters/bridge/... | grep -q "PASS"`

## Acceptance Checks (verification commands)
- `grep -q "ANCHOR:implementation_directives" skills/evolve-loop/SKILL.md`

## Thresholds
- All checks: pass@1 = 1.0
