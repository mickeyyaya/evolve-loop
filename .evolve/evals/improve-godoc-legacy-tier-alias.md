# Eval: improve-godoc-legacy-tier-alias

## Code Graders (bash commands that must exit 0)
- `[code]` `go test -v ./internal/bridge/...`

## Regression Evals (full test suite)
- `[code]` `go test ./...`

## Acceptance Checks
- `[code]` `grep -q "func legacyTierAlias" go/internal/bridge/realizer.go`
- `[code]` `grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -E -q "delegates to.*translateV1TierKey"`
- `[code]` `grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -q "haiku.*fast"`
- `[code]` `grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -q "sonnet.*balanced"`
- `[code]` `grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -q "opus.*deep"`
- `[code]` `grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -q "removed alongside the v1 schema shim in the next release"`
- `[code]` `grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -E -q "ADR-0022.*PR 2.*addendum"`

## Negative Cases
- `[code]` `! grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -q "haiku.*deep"`

## Edge / OOD Cases
- `[code]` `! grep -B 15 "func legacyTierAlias" go/internal/bridge/realizer.go | grep -q "invalid_key"`

## Thresholds
- All checks: pass@1 = 1.0
