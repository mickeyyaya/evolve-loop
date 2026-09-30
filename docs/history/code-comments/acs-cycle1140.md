# Comment history: `acs/cycle1140`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1140/predicates_test.go:3` — above `package cycle1140`

```text
// Package cycle1140 materialises the cycle-1140 acceptance criteria for the two
// fleet-scoped `## top_n` tasks pinned to this lane:
//
//   - phasecontract-role-artifact-ssot        (merges artifact-name-ssot-retro-backfill
//   - required-roles-ssot)
//   - optional-phase-ev-gating-by-cycle-class
//
// The deferred task (`triage-unified-solution-synthesis`) gets ZERO predicates
// here — R9.3: predicates bind only to triage-committed work.
//
// # Predicate strategy
//
// Every predicate CALLS the system under test and asserts on its return value
// or side effect — never a source-grep of production code (the cycle-85
// degenerate-predicate ban). Concretely:
//
//   - 001 calls phasecontract.For() and asserts on the returned Contract, plus a
//     negative lookup so a fail-open registry cannot satisfy it.
//   - 002 drives the REAL backfill.TryExtract code path over a temp workspace,
//     with the artifact filename SOURCED FROM phasecontract — so the two
//     independent filename declarations (backfill.phaseHeaders and
//     core.backfillArtifactPath) are pinned to the registry rather than to each
//     other. Includes an unknown-phase negative.
//   - 003 derives the required-role vocabulary from phasecontract.RequiredRoles()
//     and feeds a synthesized ledger to the REAL consumer
//     (redteamcheck.LedgerRoleCompleteness), asserting complete⇒pass and
//     role-removed⇒error. The consumer's own literal is thereby forced to agree
//     with the registry.
//   - 004 calls router.Route() on a trivial-class cycle whose advisor plan runs
//     an optional phase, and asserts the phase is SKIPPED — plus the two
//     anti-overreach negatives (non-trivial class still runs it; a floor phase is
//     never skipped by the same rule).
//
// # Test contract for Builder (do NOT modify this file)
//
// Predicate 003 requires one NEW exported accessor — the SSOT surface the
// required-roles half of task 1 exists to create:
//
//	// package phasecontract
//	func RequiredRoles() []string   // canonical roles every completed cycle must ledger
//
// Until it exists this package does not compile, which is the intended RED for
// the SSOT criterion (go/acs/README.md: a predicate package that fails to
// compile is a HARD suite error, never a silent PASS). Builder implements the
// accessor and re-points cyclehealth/redteamcheck/ledgerverify at it; the
// literal `[]string{"scout", "builder", "auditor"}` sites go away.
```
