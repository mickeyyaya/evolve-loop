# Comment history: `acs/cycle547`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle547/predicates_test.go:3` — above `package cycle547`

```text
// Package cycle547 materialises the cycle-547 acceptance criteria for the
// three triage-committed (`## top_n`) tasks (scout-report.md "Selected
// Tasks"; no separate triage-report.md this cycle — scout committed all
// three, see scout-report.md's Decision Trace task_proposals):
//
//  1. fleet-min-width-lane-fallback     — cmd/evolve wave dispatch: a
//     quota/budget-shrunk-to-<=1-lane wave with >=1 disjoint candidate must
//     dispatch isolated (1 lane), never fall to the leak-prone sequential
//     path.
//  2. memo-phase-routing-repair          — internal/phasespec: the built-in
//     optional `memo` phase's activation overlay must route without
//     warnings; the two-tier naming floor's new built-in-name exemption must
//     stay scoped to OPTIONAL built-ins only.
//  3. apicover-new-package-graduation-gate — internal/ciparity +
//     internal/phases/audit: a changed go/internal/<pkg> absent from
//     .apicover-enforce must FAIL the audit gate; go/cmd/... changes and
//     already-graduated packages must not be flagged.
//
// Predicate strategy (mirrors cycle499/503/504/507): BEHAVIORAL predicates
// drive the system under test through its in-package RED tests via
// subprocess `go test`, asserting a non-degenerate pass (requireTestsRan
// closes the cycle-85 "no tests to run" trap) — never a source grep. The
// in-package tests were authored by the TDD engineer this cycle:
//
//	cmd/evolve/cmd_loop_wave_minwidth_test.go        (Task 1)
//	internal/phasespec/validate_builtin_exempt_test.go (Task 2)
//	internal/ciparity/newpkg_test.go                 (Task 3, pure fn)
//	internal/phases/audit/ciparity_newpkg_test.go    (Task 3, gate wiring)
//
// The Builder implements production code ONLY (the seams named in those
// files); it must not modify the tests.
```

### `go/acs/cycle547/predicates_test.go:102` — above `func TestC547_003_MemoOverlayRoutesWithoutWarning(t *testing.T) {`

```text
// TestC547_003_MemoOverlayRoutesWithoutWarning (Task 2, all 4 AC clauses
// except ADR-0058 stripping, which is existing regression coverage per
// validate_builtin_exempt_test.go's header comment): the built-in-name
// exemption is scoped to optional built-ins, rejects a genuinely new
// single-word name, rejects a non-optional built-in-name hijack attempt
// (audit), and the real memo overlay routes end to end with zero warnings.
// Drives internal/phasespec/validate_builtin_exempt_test.go. RED today:
// ValidateUserSpecWithCatalog undefined and ApplyUserRouting has the wrong
// arity (package phasespec test build fails).
```
