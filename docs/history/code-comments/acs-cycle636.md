# Comment history: `acs/cycle636`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle636/predicates_test.go:3` — above `package cycle636`

```text
// Package cycle636 materialises the cycle-636 acceptance criteria for the single
// triage-committed top_n task, ship-sha-repin-after-build (weight 0.96, inbox
// 2026-07-08T03-05-00Z-ship-sha-repin-after-build.json). SELF_SHA_TAMPERED denied
// the terminal ship gate on 8 consecutive cycles (625->634) with a byte-identical
// FROZEN pin, both within plugin version 22.0.1: the cycle-514 boot healer re-pins
// only at boot, so a legitimate within-version rebuild of go/bin/evolve leaves a
// doomed pin for every later cycle. The fix runs the SAME provenance-gated repin
// immediately AFTER a successful build phase, reusing one shared primitive
// (phaseintegrity.RepinIfDrifted) so the boot and post-build paths never diverge.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…623 precedent) —
// each predicate shells `go test -run` over the RED unit tests authored this cycle
// (with -count=1 to defeat the test cache, so it always exercises current source).
// None is a source-grep; every one exercises the system under test — the shared
// primitive phaseintegrity.RepinIfDrifted and the orchestrator entry
// repinShipSHAAfterBuild, each called with real on-disk state.json + binary
// fixtures and asserted on their RepinResult / the resulting pin / ShipSHAMismatch.
// RED now: internal/phaseintegrity and internal/core both fail to compile
// (RepinIfDrifted / repinShipSHAAfterBuild / postBuildRepinProvenanceFn undefined).
// GREEN once Builder implements the primitive + the post-build entry and wires it
// into recordAndBranch's PhaseBuild branch (test-report.md WIR-1).
//
// The recordAndBranch call-site wiring (WIR-1) is dispositioned manual+checklist
// in test-report.md — a full-cycle recordAndBranch run is disproportionately heavy
// to unit-drive; the entry function's behaviour is fully predicated here and AC-3
// (TestC636_004) is the mechanical ship-gate backstop.
```
