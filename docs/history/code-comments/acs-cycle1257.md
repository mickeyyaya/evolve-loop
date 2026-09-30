# Comment history: `acs/cycle1257`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/acs/cycle1257/predicates_test.go:3` — above `package cycle1257`

```text
// Package cycle1257 materialises the cycle-1257 acceptance criteria for the two
// tasks this fleet lane committed in triage `## top_n`:
//
//	egps-tia-manifest-and-selection          (M — predicates 001-003)
//	egps-tia-stage-config-and-boundary-sweep (S — predicates 003-005)
//
// Nothing was deferred or dropped this cycle, so every top_n task carries
// predicates and no predicate gates non-committed work (the R9.3 floor-binding
// rule).
//
// # The task
//
// The EGPS Go lane runs the FULL `./acs/regression/<sub>` corpus every cycle,
// unconditionally: `goLanePatterns` (acssuite.go:356) enumerates every dir under
// `go/acs/regression/` with no reference to what the cycle actually changed.
// `changedPackagesForCycle` (acssuite.go:716) already computes the changed set,
// but it is used ONLY to export CHANGED_PACKAGES into the predicate env — never
// to select which regression scopes run. This cycle closes that gap in the
// deterministic Meta-PTS shape (no ML): intersect the changed set — widened by
// reverse dependencies — against a per-regression-scope target mapping, and gate
// the whole thing behind a policy-injected `stage: off|shadow|enforce`, with
// full-corpus override triggers so selection can never permanently hide a
// regression class.
//
// The reverse-dependency half is load-bearing, not decoration: cycle-1250
// changed `internal/router` and no changed-scope gate ever selected
// `internal/routingtest`, which imports router and owns the keystone parity
// invariant — main stayed red for 5 commits. `changedpkgs.ImporterClosure`
// (importerclosure.go:42) already landed for exactly this and is the SSOT to
// reuse; a naive exact-string intersect would reproduce the miss.
//
// # Predicate strategy
//
// Every predicate below EXECUTES the system under test through its unit suite,
// filtered by `-run` to named tests in ONE named package — never a source-grep
// of production code (the cycle-85 degenerate-predicate ban) and never a `/...`
// test sweep or a 40s+ suite (the flaky-predicate-shape ban; measured baselines
// are internal/policy 0.74s, internal/acssuite 1.20s).
//
// Asserting each individual `--- PASS: <name>` line — not merely the package
// exit code — is what makes a renamed, skipped, or never-authored test a RED. A
// `-run` expression that matches nothing exits 0, so the exit code alone would
// green a package with zero of the required tests.
//
// The stage/override predicates (004, 005) drive the EXPORTED production entry
// `acssuite.Run` through its `Options.GoExec` seam, so what they assert is the
// pattern set the PRODUCTION path actually hands to `go test`. A test that called
// a selection helper directly would pass on dead code and prove nothing — the
// wiring proof is a reachability test.
//
//   - 001 is the crux: the cycle-1250 reverse-dependency reproducer PLUS its
//     anti-no-op negative (a select-everything filter must FAIL).
//   - 002 pins the scope→target mapping and that the always-on scopes
//     (this-cycle, redteam) can never be filtered away.
//   - 003 is the fail-open floor: an underivable changed set and an unknown
//     stage string both degrade to the FULL corpus, never to a narrower one.
//   - 004 pins the three stage semantics end-to-end through Run.
//   - 005 pins the four full-corpus override triggers, including the inbox
//     item's explicit wiring proof (an out-of-selection regression scope still
//     EXECUTES at the boundary sweep).
//   - 006 is the ADR-0069 repo-wide apicover guard, scoped to the two enrolled
//     packages this cycle mutates, plus the blast-radius build/vet check.
```

### `go/acs/cycle1257/predicates_test.go:103` — above `func TestC1257_001_ReverseDependencySelectionReproducer(t *testing.T) {`

```text
// -----------------------------------------------------------------------------
// AC1 + AC2 — the cycle-1250 reverse-dependency reproducer, and its negative.
// -----------------------------------------------------------------------------
```

### `go/acs/cycle1257/predicates_test.go:107` — above `func TestC1257_001_ReverseDependencySelectionReproducer(t *testing.T) {`

```text
// TestC1257_001_ReverseDependencySelectionReproducer is the crux predicate.
//
// AC1: with selection active, a cycle whose only changed package is a
// router-shaped leaf MUST still select the regression scope that covers the
// package IMPORTING it — the cycle-1250 miss. Forward-only intersection
// (exact-string package equality, the `topn_gate_slug_identity` disease) does not
// satisfy this; the changed set has to be widened through
// changedpkgs.ImporterClosure before the intersect.
//
// AC2 is the anti-no-op negative and rides in the same predicate deliberately:
// a regression scope covering a package that neither is nor imports anything in
// the changed set MUST NOT be selected. Without it, "select every scope" — i.e.
// today's unconditional behaviour — satisfies AC1 and the feature is worthless.
```

### `go/acs/cycle1257/predicates_test.go:247` — above `func TestC1257_006_ApicoverEnforcedAndModuleClean(t *testing.T) {`

```text
// TestC1257_006_ApicoverEnforcedAndModuleClean is a GUARD (ratchet) predicate: it
// is green on today's tree by design (measured baseline: acssuite 10/10 exported
// covered, policy 127/127, 0 false-green, ~2.1s) and goes red only if this
// cycle's work breaks it.
//
// AC14: both `internal/acssuite` and `internal/policy` are enrolled in
// go/.apicover-enforce (lines 122 and 62), so ADR-0069's repo-wide gate requires
// every export this cycle adds — the stage field, the selection entry, any new
// Options/Verdict surface — to be NAMED in a real assertion that EXECUTES it.
// Enrolled-but-unnamed and named-but-uncovered both fail the tree AFTER the
// cycle ships; running the actual gate scoped to these two packages catches it
// inside the cycle instead. Scoped by an explicit APICOVER_PKGS override so this
// never becomes a whole-repo sweep.
//
// The build/vet half is the blast-radius check: a new field on a widely-embedded
// config struct breaks callers the changed package's own tests never touch.
```
