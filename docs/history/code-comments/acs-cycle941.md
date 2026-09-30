# Comment history: `acs/cycle941`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle941/predicates_test.go:3` — above `package cycle941`

```text
// Package cycle941 materializes the cycle-941 acceptance criteria for the
// fleet-scoped todo merge-rung2-scoped-merge-review — the merge ladder's
// missing RUNG 2 (scoped merge review), between RUNG 0's trivial-rebase
// carry-forward and RUNG 3's full re-audit (knowledge-base/research/
// merge-concurrency-2026, MergeBERT lineage).
//
// Two committed tasks (triage top_n):
//   - merge-rung2-scoped-review-core: the pure decision core in
//     go/internal/core/mergerung2.go + the `Method` field on the ledger's
//     CompositionVerdictInput.
//   - merge-rung2-wire-ship-recovery: the re-entry (patch-id) invariant and
//     the orchestrator wiring observability.
//
// Every predicate executes the system under test via a `go test` subprocess
// requiring an explicit "--- PASS: <name>" for each named unit test (rename/
// skip gaming is caught — exit 0 alone never satisfies a predicate), or via a
// whole-module build/vet, or the SSOT eval quality checker. No source-grep
// predicates (cycle-85 rule). Adversarial axes are carried by the underlying
// unit tests: NEGATIVE (TestScopedReview_EmptyIntersectionNoDispatch — reviewer
// must NOT fire, armed to return entangled), EDGE (…MalformedDiffFailsClosed —
// corrupt hunk header fail-closed), SEMANTIC (only-intersecting, compatible-vs-
// entangled, patch-id re-entry, and method persistence are distinct behaviors).
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	A1 only-intersecting-hunks dispatched   → C941_001 (core, SeesOnlyIntersectingHunks)
//	A2 compatible composes / entangled esc. → C941_002 (core, CompatibleComposesEntangledEscalates)
//	A3 negative: empty intersection no-op   → C941_003 (core, EmptyIntersectionNoDispatch)
//	A4 edge: malformed diff fail-closed     → C941_004 (core, MalformedDiffFailsClosed)
//	A5 ledger Method field (scoped + default)→ C941_005 (ledger, Method{ScopedReview,DefaultsTrivialRebase})
//	B1 orchestrator wiring observability    → C941_006 (core, ScopedMergeReviewWired)
//	B2 MergeBERT patch-id re-entry invariant→ C941_007 (core, TestLLMResolution_ReentersRung0Verification)
//	A6/B-build whole module builds + vets   → C941_008 (go build ./...), C941_009 (go vet core+ledger)
//	Step 6b eval files pass quality-check    → C941_010 (both task slugs)
```
