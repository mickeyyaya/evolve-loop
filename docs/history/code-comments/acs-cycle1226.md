# Comment history: `acs/cycle1226`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1226/predicates_test.go:3` — above `package cycle1226`

```text
// Package cycle1226 covers the reachabilityprobe-build-import-graph task
// (inbox tdd-structural-test-reachability-probe, weight 0.92; fleet-scoped
// todo tdd-reachability-probe-check).
//
// Gap closed: reachabilityprobe.ImportGraph (landed cycle-1225) is
// caller-supplied only — nothing in the repo builds it from the real
// toolchain, even though the package doc says the caller must derive it from
// `go list -deps`. This cycle adds
// reachabilityprobe.BuildImportGraph(repoRoot string, pkgs ...string)
// (ImportGraph, error), shelling out to `go list -deps -json` the same way
// go/internal/fleet/packagegraph.go's TransitivePackageSet already does for a
// different purpose (fleet partitioning), and proves the deterministic
// cycle-644 check (CheckCallSite) round-trips correctly against REAL
// toolchain output, not just hand-built literal graphs.
```

### `go/acs/cycle1226/predicates_test.go:83` — above `func TestC1226_003_BuildImportGraphRoundTripsIntoCheckCallSite(t *testing.T) {`

```text
// TestC1226_003_BuildImportGraphRoundTripsIntoCheckCallSite is the frozen
// regression test (doNotModifyTests:true — AC2): it proves the deterministic
// cycle-644 check works against a REAL go-list-deps-derived graph, not a
// hand-built literal, with both a positive (cycle detected) and a negative
// (no false cycle) case in one table.
```

### `go/acs/cycle1226/predicates_test.go:106` — above `name:      "sysexec pinning fleet is an unbuildable cycle (real edge)",`

```text
// fleet already imports sysexec (real edge, asserted in
// TestC1226_001). Freezing a structural test that pins
// fleet.SomeFunc( inside a sysexec-package file would require
// sysexec to import fleet back — the cycle-644 shape, against
// real toolchain data this time instead of a literal.
```

### `go/acs/cycle1226/predicates_test.go:140` — above `func TestC1226_004_ReachabilityProbeRaceClean(t *testing.T) {`

```text
// TestC1226_004_ReachabilityProbeRaceClean enforces AC4: the package's own
// test suite (including this cycle's new BuildImportGraph tests once
// unfrozen at build/test file level) must pass under -race. Scoped to the
// single named package (no trailing "/...") per the flaky-predicate-shape
// rules — a subtree sweep is contention-sensitive under fleet load
// (cycles 1173/1175/1178).
```

### `go/acs/cycle1226/predicates_test.go:155` — above `func TestC1226_005_ApicoverNamedTestCoversBuildImportGraph(t *testing.T) {`

```text
// TestC1226_005_ApicoverNamedTestCoversBuildImportGraph enforces the apicover
// house rule for an existing enrolled package gaining a new exported symbol:
// reachabilityprobe is already in go/.apicover-enforce (cycle-1225), so its
// apicover_named_test.go must name every exported symbol including the new
// BuildImportGraph — an enrolled-but-unnamed symbol trips the repo-wide gate
// later (cycle-1218 precedent).
```
