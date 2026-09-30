# Comment history: `acs/cycle1679`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1679/predicates_test.go:3` — above `package cycle1679`

```text
// Package cycle1679 materialises the cycle-1679 acceptance criteria for the one
// fleet-scoped task pinned to this lane: `crossartifact-invariant-stack`.
//
// WHAT THE CONTRACT ACTUALLY IS. The lane's inbox record
// (.evolve/inbox/processing/cycle-1679/2026-08-04T07-11-00Z-crossartifact-invariant-stack.json)
// carries TWO halves in its `fix` field, and `connects_to` names both homes:
//
//	"SWE-agent rule enforced: each invariant ships ADVISORY until its
//	 false-positive rate is evidenced ~0 (the 1054/1060 breaker lesson), then
//	 graduates to blocking. DOCS per 3.8 into
//	 docs/research/deliverable-alignment-2026-08/README.md."
//
//	connects_to: ["go/internal/coherence/",
//	              "docs/research/deliverable-alignment-2026-08/README.md"]
//
// The CODE half is already landed on this branch and green — verified this
// phase, not assumed: internal/coherence/crossartifact.go implements all four
// invariants, internal/core/crossartifact_invariants.go:36 records them, and
// cyclerun.go:241 is the production caller. Predicates 001-003 pin that behaviour
// so it cannot silently regress; they are expected PRE-EXISTING GREEN and are
// declared as such in test-report.md rather than deleted (a landed contract that
// stops being checked is how a shipped invariant rots).
//
// The DOCS half has never landed, and that is this cycle's real RED. §6 of the
// README ("Experience record for the new moves (to be extended per §3.8)") holds
// 6.1, 6.2 and 6.3 — there is no 6.x entry for item rank 5. Line 140's portfolio
// row still reads "**NEW — filed 0.85**" and line 124 still calls the stack
// "partial (`coherence`)", both stale against code that exists. Because the doc
// deliverable its own acceptance contract names was never written, the item's
// acceptance is unmet, so it is re-claimed every cycle: the identical record sits
// unconsumed in .evolve/inbox/processing/cycle-<N>/ for 1601 through 1679.
// Predicates 004-005 are the RED that ends that streak.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - NEGATIVE : 002 breaks each invariant class one at a time and demands that
//     class be named — an aggregate that returns "ok" for everything, or
//     "violated" for everything, fails it.
//   - EDGE/OOD : 003 drives the empty workspace and demands `indeterminate`,
//     never `violated` — the property that keeps an advisory's false-positive
//     rate at zero, which is the inbox record's own graduation condition.
//   - SEMANTIC : 004 does not merely look for a heading; it demands the §6.4
//     body carry the Issue/Gap/Solution/Measured shape 6.1-6.3 use, be
//     git-TRACKED (a gitignored doc is dropped at ship — the cycle-93 lesson),
//     and that every repo path it cites RESOLVE ON DISK. That last check is the
//     README being held to the same referenced-paths-exist invariant the feature
//     it documents enforces on everyone else.
```

### `go/acs/cycle1679/predicates_test.go:221` — above `writeRaw(t, ws, "acs-verdict.json", acsBody(t, "PASS", 3, 3, 0, 0, []string{"green", "green", "red"}))`

```text
// Claims zero red while its own results carry one — the cycle-1673 M1 shape.
```

### `go/acs/cycle1679/predicates_test.go:385` — above `const (`

```text
// ---------------------------------------------------------------------------
// AUDIT REPAIR (round 2) — the two defects the auditor named, as RED tests.
//
// H1 -> 006. `TestC1676_006_MaterializedEvalIsDurableAndBehavioral` is RED in
// the shipped tree: it requires the eval it grades to carry >=4 `[code]`
// behavioural checks, and .evolve/evals/crossartifact-invariant-stack.md
// carries zero. BOTH sides are added paths in this diff, so this is the lane's
// own inconsistency. The frozen predicate is NOT the thing to change — 23 live
// evals already carry `score_cap` frontmatter AND `### ACn: … [code]` sections
// (docs/eval-grader-best-practices.md §"Recognized grader formats";
// .evolve/evals/retro-delivery-format-binding.md:49 is the canonical shape), so
// the eval is simply missing its code-graded half.
//
// M1 -> 007. The cycle claimed a verification it never ran, "and no gate before
// ship could have caught it". The remedy cannot be report prose — a predicate
// that greps build-report.md for an invocation string is exactly the gameable
// grep cycle-85 forbids, and the cycle-75 lesson is that Builder-authored
// verification prose must never be the load-bearing evidence. So 007 RUNS the
// ship-time added-test backstop DURING the cycle: it re-derives the same seed
// (go/**/*_test.go the tree ADDS), groups by declared build tags and executes
// each group, mirroring internal/phases/ship/repocontract.go:353
// addedTestPackageGroups. A claim cannot outrun evidence the gate itself
// produces.
//
// Flaky-shape contract (Gate D): no `/...` sweep and no known-slow suite named
// — the run set is the diff's own added packages, which after self-exclusion is
// `./acs/cycle1676` alone (4.5s measured); every git call is `-C` anchored and
// every go call is `go -C` anchored, so cwd never decides the answer; no
// wall-clock bounds, no literal PIDs, no un-reaped load generators.
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1679/predicates_test.go:577` — above `var recordedAddedTests = []string{`

```text
// TestC1679_007_EveryAddedGoTestPackageIsGreenBeforeShip — audit M1.
//
// The ship-time added-test backstop is the gate that caught H1, and it fires at
// push, after the cycle's budget is spent. This runs that same gate DURING the
// cycle, from the same seed, so "the added packages are green" is a claim the
// cycle cannot make without the evidence having been produced.
//
// Scope is the TAG-GUARDED added packages, and that is the whole point rather
// than a shortcut. An untagged added test (internal/coherence,
// internal/core) already runs in the `go test -count=1 ./...` every cycle owes
// the Go conventions, so it was never the hole. A `//go:build acs` package is
// invisible to every ordinary run — which is precisely how
// go/acs/cycle1676/predicates_test.go reached ship red while a report claimed
// it verified. Narrowing here also keeps the flaky-shape contract: the run set
// is one small package, not the 89s ./internal/core suite the full seed pulls
// in (cycles 1173/1175/1178 FAILed on sound work for exactly that).
//
// The empty-seed case is RED, not a silent pass: this diff demonstrably adds
// two tag-guarded test packages, so a discovery that finds none means the
// discovery broke. A vacuous green here would reproduce the exact M1 shape it
// exists to close.
// recordedAddedTests is the lane's own added test files — the DURABLE seed.
// The live seed (addedGoTestFiles: the diff against the merge base plus the
// working tree) is what proves the claim while the lane is unmerged; once the
// lane's commit is on main that diff is empty by construction, and a durable
// predicate that fatals on it is red on every clean checkout (2026-09-15,
// research F21: RED on main c5883955 in every whole-module floor). On a
// merged tree the predicate verifies its recorded set instead — the same
// tag-guarded packages, executed the same way — so the proof M1 demanded
// never goes vacuous and never depends on the lane's tree state.
```

### `go/acs/cycle1679/predicates_test.go:718` — above `func TestC1679_008_ExplanationDocumentCountsAgreeWithTheShippedTree(t *testing.T) {`

```text
// TestC1679_008_ExplanationDocumentCountsAgreeWithTheShippedTree — audit M1.
//
// The explanation document is cycle-OWNED narrative, and M1 caught it asserting
// "this cycle's five acceptance predicates are 5/5 PASS" over a tree carrying
// seven, two of them RED. That is the claim-discrepancy class the explanation
// contract exists to catch, and nothing graded it — the document was left at its
// round-1 text while TDD round 2 added 006/007.
//
// Both sides of the comparison are derived from reality: the actual count is
// parsed out of the predicate file's own declarations, and the claimed count is
// parsed out of the document's prose. A magic string cannot satisfy it — the
// only way to green is for the narrative's arithmetic to match the tree's. The
// document is located by GLOB rather than by a pinned ULID filename so a
// regenerated explanation is still graded instead of silently skipped.
```
