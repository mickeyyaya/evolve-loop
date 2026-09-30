# Comment history: `acs/cycle675`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle675/predicates_test.go:3` — above `package cycle675`

```text
// Package cycle675 materialises the acceptance criteria for cycle 675's
// triage-committed tasks (inbox new-package-graduation-buildentry-gate RETRY,
// weight 0.92 — 3rd recurrence, cycles 575/587/652):
//
//   - Task 1 build-entry-graduation-guard: a deterministic build-phase gate
//     that FAILS the phase (explicit abort_reason) when a changed
//     go/internal/<pkg> is new this cycle and absent from go/.apicover-enforce.
//     AC1 (C675_001, C675_002): table-driven check + recordAndBranch wiring.
//     AC3 (C675_004): no false positive on delete/rename/self-graduating diffs.
//   - Task 2 build-entry-graduation-guard-audit-regression: the audit-side
//     gate (apicoverNewPackageGraduationDefault, wired 2026-07-07) stays
//     registered through the PRODUCTION constructor. AC2 (C675_003) —
//     verify-only: pre-existing GREEN by design; it regression-proofs the
//     already-landed half so the two seams can never again silently diverge.
//
// Predicate strategy: behavioural-via-subprocess (cycle-549…672 precedent) —
// each predicate shells `go test -run` over unit tests that EXERCISE the SUT
// (buildGraduationCheck over real git worktrees; recordAndBranch(PhaseBuild);
// the production-constructed audit phase end-to-end); none is source-grep.
// RED now: internal/core fails to compile (buildGraduationCheck absent).
// GREEN once Builder lands the guard. The Acceptance-Criteria-Summary line
// "go test -race on touched packages PASS" is dispositioned manual+checklist
// in test-report.md (the cycle audit's repo-wide CI-parity gates own it), not
// predicated here.
```

### `go/acs/cycle675/predicates_test.go:77` — above `func TestC675_003_AuditGraduationGateRegistered(t *testing.T) {`

```text
// TestC675_003_AuditGraduationGateRegistered — AC2 (verify-only regression):
// the PRODUCTION audit constructor (NewDefaultWithStageCompact) still
// registers the new-package graduation gate — an ungraduated package FAILs
// the audit end-to-end, an enrolled one PASSes. Guards the already-landed
// audit half against silently dropping out (the cycle-652 divergence class).
```
