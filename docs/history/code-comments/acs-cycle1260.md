# Comment history: `acs/cycle1260`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1260/predicates_test.go:3` — above `package cycle1260`

```text
// Package cycle1260 materialises the cycle-1260 acceptance criteria for the one
// fleet-scoped, triage-committed task:
//
//	egps-regression-tia-shadow-wiring  (inbox P1 0.91, 3rd live instance)
//
// The item asks for deterministic test-impact selection over the EGPS Go
// regression corpus, staged off → shadow → enforce via .evolve/policy.json,
// with shadow logging would-skip counts before anything is ever skipped.
//
// CONTROL-PLANE DEVIATION (recorded loudly; see test-report.md §Deviation).
// scout-report targeted go/internal/acssuite/acssuite.go. That path is
// PROTECTED CONTROL PLANE — guards.ProtectedSurfaceManifest carries
// {"/go/internal/acssuite/", "the gate runner"} — so NO phase of any cycle may
// write it (verified live: the role guard DENIED + alarmed this phase's write).
// Honoring the boundary yields the better design: the shadow stage changes
// nothing about what the gate runs, so it needs no code in the gate runner. It
// is observability computed beside the suite by the suite's own production
// caller, the audit phase (internal/phases/audit/audit.go:638 generateACSVerdict).
// Scout's Task-1 acceptance intent is preserved verbatim: off/absent stays
// byte-identical, shadow logs would-skip evidence, nothing is skipped yet.
//
// Predicate strategy — every predicate EXECUTES the system under test through a
// scoped subprocess and asserts on its exit code (the cycle-85 degenerate-
// predicate ban: no predicate's load-bearing assertion is a source grep).
// Each invocation names exactly ONE package and narrows it with -run, per the
// flaky-predicate-shape rules (no ./... sweeps, no whole-repo staleness checks,
// no wall-clock bounds, no literal PIDs, cmd.Dir always set explicitly).
//
//   - 001 the policy config surface (off default, closed vocabulary, typo ⇒ off)
//   - 002 selection semantics + the fail-safes that keep selection from ever
//     hiding a regression class
//   - 003 the ImporterClosure wiring proof — the cycle-1250 router/routingtest
//     reproducer, run against the REAL repository import graph
//   - 004 the shadow decision is computed and emitted as a readable artifact
//   - 005 the CRUX reachability proof: the decision is emitted from the real
//     audit-phase path, so the selection logic is not dead code (the exact
//     shape ImporterClosure sat in since cycle-1253 — GREEN, zero callers)
//   - 006 new-package graduation: the repo-wide apicover gate's dual edit
```

### `go/acs/cycle1260/predicates_test.go:123` — above `func TestC1260_003_importer_closure_wired(t *testing.T) {`

```text
// TestC1260_003_importer_closure_wired is the cycle-1250 reproducer: a change
// confined to internal/router MUST widen to internal/routingtest, which imports
// it and holds the keystone parity invariant. Forward-only scope would mark the
// routing regression package skippable — the miss that kept main red for 5
// commits. Runs against the real repository import graph.
```

### `go/acs/cycle1260/predicates_test.go:146` — above `func TestC1260_005_audit_phase_reachability(t *testing.T) {`

```text
// TestC1260_005_audit_phase_reachability is the CRUX. A seam whose only caller
// is a test is dead code: changedpkgs.ImporterClosure shipped GREEN in
// cycle-1253 with ZERO callers, so the fix never executed once. This predicate
// requires the decision to be emitted from generateACSVerdict — the real
// audit-phase function that runs the EGPS suite — with off/absent policy
// leaving that path byte-identical and a broken evidence sink never failing the
// audit.
```

### `go/acs/cycle1260/predicates_test.go:160` — above `func TestC1260_006_new_package_graduation(t *testing.T) {`

```text
// TestC1260_006_new_package_graduation is the repo-wide apicover gate's DUAL
// edit (ADR-0069, distinct from the per-cycle ACS coverage gate): a new
// go/internal/<pkg> must be enrolled in go/.apicover-enforce AND carry real
// assertions over every exported symbol. The enrollment line is an inherent
// config-presence check; the load-bearing half below EXECUTES the package and
// requires the gate's own >=85% line-coverage floor.
//
// acs-predicate: config-check
```
