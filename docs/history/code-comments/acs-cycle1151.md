# Comment history: `acs/cycle1151`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1151/predicates_test.go:3` — above `package cycle1151`

```text
// Package cycle1151 materialises the cycle-1151 acceptance criteria for this
// lane's two triage-committed top_n tasks:
//
//	wire-docsfloor-changed-paths-verify (S) — route the `build` phase's
//	    self-check through `deliverable.VerifyBuildWithChangedPathsStage` so the
//	    ADR-0077 blocking-grade classifier finally sees a real diff, fail-open
//	    everywhere else.
//	reconcile-adr0077-labeler-drift (S) — ADR-0077's "Shape" / boundary-2 prose
//	    still names `docsfloor.LabelArchitecture` as the label source while
//	    production (`core.docsFloorWarn`) calls `docsfloor.IsArchitectureClass`,
//	    and `LabelArchitecture` has zero production callers.
//
// Predicate strategy, per task:
//
//   - Task 1 is behavioural-via-subprocess (the cycle-549…1150 precedent): each
//     predicate shells `go test -run` over contract tests that drive the REAL
//     CLI entry point (`runPhaseVerify`) against a REAL git worktree and assert
//     on exit codes / JSON violation codes. No source-grep of production code
//     carries any load (the cycle-85 degenerate-predicate ban).
//   - Task 2's deliverable IS a document, so its predicates assert on that real
//     emitted artifact — but never against a hardcoded magic string. The
//     expected symbol is COMPUTED from production source (which call
//     `docsFloorWarn` actually makes) and the "is it unused" claim is COMPUTED
//     by walking the Go tree for callers. If production later rewires to
//     `LabelArchitecture`, these predicates flip to demanding the opposite text
//     — they detect drift, they do not pin a phrase.
//
// RED expectation at authoring time: Task 2's predicates (C1151_004, C1151_005)
// fail — the ADR still documents `LabelArchitecture` as the labeler and records
// nothing about its non-production status. Task 1's predicates (C1151_001…003)
// are PRE-EXISTING GREEN: the cycle-1150 continuation salvage (commit 0a328df1,
// carried into this lane's worktree base) already landed the wiring and its
// contract tests. They are retained as regression binding, and reported as
// pre-existing GREEN in test-report.md rather than claimed as this cycle's RED.
```

### `go/acs/cycle1151/predicates_test.go:90` — above `func TestC1151_002_FailOpenShapesAreUnchanged(t *testing.T) {`

```text
// C1151_002 — NEGATIVE / fail-open axis. The three shapes that must stay
// byte-identical to pre-wiring behaviour: an architecture-class diff that DOES
// carry docs, a non-architecture diff, an invocation with no --worktree, and a
// non-build phase. A wiring that fails these is over-reaching, which ADR-0077's
// "WARN, never REJECT" boundary forbids.
```

### `go/acs/cycle1151/predicates_test.go:103` — above `func TestC1151_003_SeamContractsHold(t *testing.T) {`

```text
// C1151_003 — the seam the CLI now depends on holds its own contract: the
// changed-path derivation is exported from internal/core (one source, not a
// second implementation — the ADR-0034 no-drift invariant) and the deliverable
// helper threads the caller's resolver + PhaseIO stage rather than silently
// dropping to built-ins.
```

### `go/acs/cycle1151/predicates_test.go:269` — above `if !noteNearMention(strings.ToLower(body), strings.ToLower(broadLabeler), notes, 400) {`

```text
// Proximity-scoped: the note must sit NEXT TO a LabelArchitecture mention.
// A bare document-wide substring search passes on the cycle-1150 addendum's
// "shipped inert … zero production callers" sentence, which is about
// VerifyBuildWithChangedPaths — a different symbol entirely.
```
