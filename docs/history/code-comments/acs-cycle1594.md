# Comment history: `acs/cycle1594`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1594/predicates_test.go:3` — above `package cycle1594`

```text
// Package cycle1594 materializes the cycle-1594 acceptance criteria for the one
// task this fleet lane committed (scout-report.md ## Selected Tasks;
// triage-report.md ## top_n): `gitignore-staging-sweep`. Per R9.3 nothing here
// binds to the deferred `ship-addall-staging-surface` item — the broader ship
// staging manifest gets ZERO predicates this cycle.
//
// The defect. go/internal/core/phase_bindings.go:226 computes the cycle's
// content identity with `git add -A`, so any untracked file sitting in the lane
// — a bug-reproduction reproducer, a regenerated go/coverage.*.txt artifact, a
// minted phase stub, another agent's scratch — is adopted into BOTH the audit
// binding's WorktreeTreeSHA (phase_bindings.go:129) and the ADR-0048 Slice B
// verdict-cache key. The binding then attests a tree the auditor never
// reviewed. Reproduced in .evolve/runs/cycle-1594/bug-reproduction-report.md
// (foreign-residue.txt present in the emitted tree).
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 unrelated untracked residue absent from the binding tree   → C1594_001
//	AC2 declared (staged) new builder output retained              → C1594_002
//	AC3 NEGATIVE: unstaged TRACKED edits still captured            → C1594_003
//	AC4 EDGE: residue-only lane keeps its base identity            → C1594_004
//	AC5 WIRING: the production binding path emits the scoped tree   → C1594_005
//	AC6 the cycle eval is rigorous, not vacuous                    → C1594_006
//	AC7 existing core suite (incl. -tags integration) stays green  → manual+checklist
//	    (a package sweep is a banned flaky-predicate shape; the build floor,
//	     the ship gate and CI own it)
//
// Adversarial axes: negative (C1594_003 — the cheapest way to pass the two
// exclusion pins is to bind HEAD^{tree} or stage nothing, which silently drops
// real work and manufactures base-identical identities: the INTEGRITY_TREE_DRIFT
// class of cycle-152 and the fresh-base verdict-cache collision), edge (C1594_004
// — the EMPTY declared selection, where there is no legitimate change to shelter
// residue behind), semantic (exclusion, retention, tracked-capture, production
// reachability and eval rigor are five distinct behaviors).
//
// No source-grep predicates (the cycle-85 ban): C1594_001..005 each execute the
// real production code as a subprocess `go test` run against a real Git
// repository and require a NAMED `--- PASS:` marker (a bare exit 0 would hide a
// renamed, skipped, or deleted test); C1594_006 runs the SSOT eval-quality
// checker. Every invocation names ONE package and is -run-narrowed — never a
// `/...` sweep (the flaky-predicate-shape rule).
```
