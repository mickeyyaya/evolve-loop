# Eval: improve-godoc-translate-v1-tier-key

## Code Graders (bash commands that must exit 0)
- `[code]` `go test -v ./internal/bridge/...`

## Regression Evals (full test suite)
- `[code]` `go test ./...`

## Acceptance Checks
- `[code]` `grep -q "func translateV1TierKey" go/internal/bridge/manifest.go`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "sonnet.*balanced"`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "opus.*deep"`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "large"`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -E -q "ADR-0022.*PR 2.*addendum"`

## Negative Cases
- `[code]` `! grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "haiku.*deep"`

## Edge / OOD Cases
- `[code]` `! grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "invalid_key"`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "empty"`

## Thresholds
- All checks: pass@1 = 1.0
