# Comment history: `acs/cycle1225`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1225/predicates_test.go:3` — above `package cycle1225`

```text
// Package cycle1225 ports the cycle-1225 ACS predicates for the
// tdd-structural-test-reachability-probe inbox item (weight 0.92).
//
// Root cause (cycle-644): TDD froze a `doNotModifyTests:true` structural test
// pinning `storage.UpdateStateMap(` inside a `core`-package file while
// `storage` already imported `core` — a compiler-proven import cycle — dooming
// the cycle to a permanently-RED, unsatisfiable acceptance criterion.
//
// Two tasks, one item:
//   - tdd-reachability-probe-doc: agents/evolve-tdd-engineer.md must name the
//     obligation to compiler-probe a pinned package-qualified call site before
//     freezing it, citing cycle-644 as the worked example (TestC1225_001).
//   - tdd-reachability-probe-check: a deterministic
//     go/internal/reachabilityprobe package must flag the cycle-644 shape as
//     unreachable and pass an acyclic pin unchanged (TestC1225_002).
```

### `go/acs/cycle1225/predicates_test.go:28` — above `func TestC1225_001_DocCitesReachabilityProbeObligation(t *testing.T) {`

```text
// TestC1225_001_DocCitesReachabilityProbeObligation is a behavioral doc
// predicate (not source-grep gaming — it locates the exact contractual
// language a TDD engineer must follow, not an arbitrary string): the TDD agent
// contract must document, in prose reachable near the RED-freeze steps, that
// a package-qualified pin requires a compiler-probe before freezing, and must
// cite cycle-644's storage/core shape as the worked example so a future TDD
// engineer recognizes the failure mode by name.
```

### `go/acs/cycle1225/predicates_test.go:46` — above `func TestC1225_002_ReachabilityProbeFlagsCycle644Shape(t *testing.T) {`

```text
// TestC1225_002_ReachabilityProbeFlagsCycle644Shape is the primary negative
// test (strongest anti-no-op signal per skills/adversarial-testing/SKILL.md
// §6): given an import graph reproducing the cycle-644 shape verbatim
// (storage already imports core; a frozen test wants to pin
// storage.UpdateStateMap( inside a core-package file), CheckCallSite MUST
// return a non-nil Violation citing the cycle. A no-op stub always returning
// nil would pass the GREEN case below but fail here — the negative case is
// load-bearing.
```

### `go/acs/cycle1225/predicates_test.go:130` — above `func TestC1225_004_ReachabilityProbeSemanticTransitiveCycle(t *testing.T) {`

```text
// TestC1225_004_ReachabilityProbeSemanticTransitiveCycle covers the semantic
// diversity axis (distinct behavior from the direct-edge case above): the
// cycle-644 class is not limited to a direct A->B / B->A pair — a
// TRANSITIVE chain (storage -> mid -> core, pin wants core -> storage) is the
// same unbuildable-cycle disease and must be caught identically.
```

### `go/acs/cycle1225/predicates_test.go:156` — above `func TestC1225_005_ReachabilityProbePackageGraduatesApicover(t *testing.T) {`

```text
// TestC1225_005_ReachabilityProbePackageGraduatesApicover enforces House Rule
// 1 (new-package graduation, the repo-wide apicover gate, ADR-0069's second
// gate): a brand-new go/internal/reachabilityprobe package must be enrolled in
// go/.apicover-enforce AND ship its own apicover_named_test.go naming every
// exported symbol, in the same diff as the package itself — an enrolled-but-
// unnamed or unenrolled-but-present package each abort a later phase
// (cycle-1218: three lanes, one halt, same cause).
```
