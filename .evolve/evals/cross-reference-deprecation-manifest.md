# Eval: cross-reference-deprecation-manifest

## Code Graders (bash commands that must exit 0)
- `[code]` `go test -v ./internal/bridge/...`

## Regression Evals (full test suite)
- `[code]` `go test ./...`

## Acceptance Checks
- `[code]` `grep -q "func translateV1TierKey" go/internal/bridge/manifest.go`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "legacyTierAlias"`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "removed alongside the v1 schema shim in the next release"`
- `[code]` `grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -E -q "ADR-0022.*PR 2.*addendum"`

## Negative Cases
- `[code]` `! grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "should be kept forever"`

## Edge / OOD Cases
- `[code]` `! grep -B 15 "func translateV1TierKey" go/internal/bridge/manifest.go | grep -q "invalid_key"`

## Thresholds
- All checks: pass@1 = 1.0
