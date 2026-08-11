# Eval: add-realizer-package-godoc-example

## Code Graders (bash commands that must exit 0)
- `[code]` `go test -v ./internal/bridge/...`

## Regression Evals (full test suite)
- `[code]` `go test ./...`

## Acceptance Checks
- `[code]` `grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "ParamSpec.From=\"model_tier_map\""`
- `[code]` `grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "ModelTier=\"balanced\""`
- `[code]` `grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "codex"`
- `[code]` `grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "gpt-5.4"`
- `[code]` `grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "claude"`
- `[code]` `grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "sonnet"`

## Negative Cases
- `[code]` `! grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "balanced.*opus"`

## Edge / OOD Cases
- `[code]` `! grep -B 35 "package bridge" go/internal/bridge/realizer.go | grep -q "invalid_tier"`

## Thresholds
- All checks: pass@1 = 1.0
