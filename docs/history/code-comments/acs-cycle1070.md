# Comment history: `acs/cycle1070`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1070/predicates_test.go:3` — above `package cycle1070`

```text
// Package cycle1070 materialises the cycle-1070 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - tdd-topn-scope-gate → extend internal/topngate so the TDD phase's
//     authored predicate/scaffold set is bound to triage's ## top_n commitment.
//
// The defect (cycle-660, inbox tdd-topn-binding-gate, 3rd recurrence): triage
// commits an EMPTY ## top_n, but TDD reads scout-report.md — not
// triage-report.md — and still authors RED scaffolds for a slug triage
// explicitly declined. Build then honours the empty top_n correctly and chokes
// on the orphan scaffolds. topngate already binds the build->audit transition
// (topNBindingGate, gate.go); nothing binds triage->TDD.
//
// Predicate strategy — every predicate below DRIVES the real system through its
// exported seam, topngate.NewReviewer(stage).Review(ctx, core.ReviewInput{...}),
// against t.TempDir()-backed fixture workspaces. No predicate greps production
// source for a magic string (the cycle-85 degenerate-predicate ban): a builder
// who adds the string but not the gate still fails 001/002, and a builder who
// blanket-blocks the TDD phase fails 003/004.
//
//   - 001 is the cycle-660 crux: empty committed top_n + a TDD deliverable whose
//     handoff JSON testFiles[] is non-empty must be REJECTED at StageEnforce.
//   - 002 is the out-of-lane case: a non-empty top_n that does not contain the
//     slug the TDD report claims must be REJECTED at StageEnforce.
//   - 003 is the anti-overreach negative: an in-lane TDD deliverable must be
//     APPROVED (a gate that blocks everything is not a gate).
//   - 004 is the second anti-overreach negative: StageShadow observes but never
//     blocks, even on the 001 violation — the rollout control for this package.
//   - 005 is the no-regression predicate: the build-side gate's own package
//     tests still pass under -race (subprocess `go test`).
//
// Fixture shapes are the CONTRACTED report shapes, not invented ones:
// triage-report.md's "## top_n (commit to THIS cycle)" + "- <slug>: ..." bullets
// (agents/evolve-triage.md Step 4, already parsed by readTopNSlugs), and
// test-report.md's "## Task: <slug>" header + "## Handoff to Builder" JSON
// fence carrying "testFiles" (agents/evolve-tdd.md Step 6).
```

### `go/acs/cycle1070/predicates_test.go:54` — above `func writeTriage(t *testing.T, ws string, topN ...string) {`

```text
// writeTriage writes a triage-report.md whose ## top_n section lists slugs.
// Passing no slugs writes the EMPTY-section form — the cycle-660 input.
```

### `go/acs/cycle1070/predicates_test.go:103` — above `func TestC1070_001_EmptyTopNBlocksAuthoredTDDFiles(t *testing.T) {`

```text
// TestC1070_001_EmptyTopNBlocksAuthoredTDDFiles is the cycle-660 regression: an
// empty committed top_n means TDD must author NOTHING; a non-empty handoff
// testFiles[] under an empty top_n is an unambiguous out-of-lane authoring and
// must be rejected at StageEnforce with a populated reason.
```

### `go/acs/cycle1070/predicates_test.go:121` — above `func TestC1070_002_OutOfLaneSlugAdvisoryTDD(t *testing.T) {`

```text
// TestC1070_002_OutOfLaneSlugAdvisoryTDD covers the non-empty-top_n case: TDD
// claims a slug with zero overlap against the committed set.
//
// POLICY CHANGE 2026-07-23 (cycle-1073 tdd-topn-scope-gate): this case is now
// an ADVISORY, not a block — the same conversion #348/cbd088a1 made for the
// sibling build-side gate after cycles 916 + 1012 recorded two fatal
// rejections that discarded CORRECT work over label drift between two
// LLM-authored strings, with zero true-fraud catches. The lane exists BECAUSE
// triage committed these ids, so the committed set is the binding authority.
// The predicate is rebound (not deleted) to the advisory contract: approved,
// with the drift still surfaced in a populated reason. The empty-top_n case
// (001) stays fatal — there is no committed item the files could be a
// differently-labelled response to.
```
