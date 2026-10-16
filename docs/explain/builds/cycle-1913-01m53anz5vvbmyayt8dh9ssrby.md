# Build Explanation — Cycle 1913

## Build Binding
- Cycle: 1913
- Base SHA: 4e653995338ab59b58858b98cfd8339a9049206b

## Summary
The cycle-1911 predicates now read the agy manifest baseline and ignore the model catalog of the host.

## Rationale
`bridge.LoadManifest` overlays the host catalog through `modelCatalogDirFn` in `go/internal/bridge/catalog_overlay.go:14-23`. A host catalog that puts agy `deep` on Flash made `agyTierGroups` fail in `TestC1911_003` and `TestC1911_006`. A `TestMain` that points the catalog seam at an empty temporary directory removes the host input. Production code does not change.

## Changed Areas
- `go/acs/cycle1911/predicates_test.go` — adds a `TestMain` that calls `bridge.SetModelCatalogDirFn` with an empty temporary directory before the tests run, so each verdict is the same on every host.
- `go/acs/cycle1913/predicates_test.go` — the TDD phase added four predicates that run the cycle-1911 tests under a Flash, a Pro, an absent and a malformed host catalog.
- `.evolve/evals/c1911-test-hermeticity.md` — the TDD phase added the eval with the score caps for the three host-catalog patterns.

## Design Decisions
One `TestMain` covers the direct `LoadManifest` call in `agyTierGroups` and the indirect call in `bridgechain`. A per-test seam swap with `t.Cleanup` was rejected, because two tests need it and a third test can forget it. A change to `catalog_overlay.go` was rejected, because the production overlay is correct and the defect is in the test input. The chain assertions and the deep-versus-Flash guard stay unchanged.

## Verification
`go test -tags acs -count=1 ./acs/cycle1913 ./acs/cycle1911` passes 4/4 and 6/6. Before the change, `TestC1913_001` failed with the deep-on-Flash message.

## Compatibility
The predicate names, the assertions and the production catalog overlay stay the same.

## Limitations
The cycle-1911 predicates no longer test the live catalog. A live-catalog check belongs in a separate doctor probe.
