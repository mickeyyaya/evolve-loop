# Build Explanation — Cycle 2325

## Build Binding
- Cycle: 2325
- Base SHA: 6e35fc4f7bcf4594d8cf8763782d76ab5f25917f

## Summary
A contract test now ties each boundary writer home to the lane-base prefix list. The list is the one that decides which commits ahead of origin are boundary-owned.

## Rationale
Wave 166 deferred 16 lanes because the retrospective home was missing from the list. The test removes that failure class. It compares the three exported homes with the list through one exported predicate.

## Changed Areas
- `go/internal/core/landed_main.go` — exports `BoundaryOwned` and uses it in the ahead-commit proof, so the test and production read the same list.
- `go/internal/dossier/sweep.go` — exports `Home`, the dossier directory, and keeps `dossierDir` as an alias.
- `go/internal/dossier/apicover_named_test.go` — names `Home` in a real assertion, as the apicover gate requires.
- `go/internal/core/boundary_owned_apicover_test.go` — names `BoundaryOwned` in a real assertion, as the apicover gate requires.
- `go/cmd/evolve/boundary_writers_contract_test.go` — the contract test `TestBoundaryWritersContractBindsEveryHomeToTheLaneBase`; it calls `t.Parallel`, as the parallel-default rule requires.

## Design Decisions
One predicate replaces the private negation. A second copy of the list can break the single home rule. The list stays a literal in core, because inboxstamps imports core and the reverse import forms a cycle.

## Verification
The six cycle predicates pass. The mutation predicates drop each home and add each unlisted home, and the contract test fails on each.

## Compatibility
Behavior of the ahead-commit proof does not change.

## Limitations
A new publisher home in a package that the test does not import is not found. The test lists the three known homes.
