# 2026-09-14 — the ship gate refused a PASSed audit on the lane's own added test, and the repair rounds could not apply the fix

**Class:** pipeline (an audit/ship disagreement, then a repair ladder routed to a phase whose sandbox forbids the remedy).
**Surface:** `internal/core/build_floor_reviewer.go` (the build floor), `internal/evalgate/materialization.go` (the scout's eval gate), `internal/phases/ship/native.go` (the ship journal).
**Found by:** the operator, wave 3 — "how could it happen as the verdict is already passed; it should figure out the solution to resolve the ship issue instead of seal or abandon the progress".

## What happened

Lane 1679 (item `crossartifact-invariant-stack`, itself about durable code-graded evals) built and audited PASS at 22:31. At 22:33 the ship's added-test backstop ran the lane's ADDED package `go/acs/cycle1676` under its `//go:build acs` tag and one of the lane's own predicates went red:

```
TestC1676_006_MaterializedEvalIsDurableAndBehavioral:
  RED: .evolve/evals/crossartifact-invariant-stack.md carries 0 [code] checks, want >=4 behavioral ones
```

The scout had materialized that eval with a `score_cap` front-matter and acceptance-criteria headings but no `[code]` grader; the lane's test demands the code-graded half. The gate refused the tree ("pushing would red main"), correctly.

The orchestrator routed the RED to a recovery audit (FAIL, EGPS red 2), then through the retry envelope into tdd (PASS) and build. The round-2 build report is explicit: the builder diagnosed the defect, wrote the exact remedy to `eval-append.crossartifact-invariant-stack.md`, and declared it "unappliable by this phase, class infrastructure-systemic" — the builder's sandbox denies `.evolve/evals` on purpose (a builder must never edit its own grading rubric). The orchestrator ignored that class and ran audit (FAIL) and a third tdd/build round; the tdd phase, whose sandbox does allow `.evolve/evals`, applied the append at 23:27. The predicate has been green in the lane since; two repair rounds and about an hour were spent reaching it.

## Root causes — three gaps, one chain

1. **The floor could not see the package.** `changedPackageFloorChecks` runs changed packages in the default build context and `buildTagVisiblePackages` drops any package with no Go files in that context — an added `//go:build acs` package is invisible to it. The ship's added-test backstop (#612) groups added tests by their declared tags and runs each group under them. So a red tag-gated added test was first executed at ship, after the builder had handed the tree over.
2. **The scout gate did not check what it promised.** `materializationGate.check` verified only that each selected slug's eval file exists; its own remediation text has always said "Each must contain at least one `[code]` grader and test BEHAVIOR, not existence." A graderless eval therefore left the scout, the only phase whose sandbox may write it, and surfaced two phases later where nobody could touch it.
3. **The remedy ladder had no owner for the path.** A builder-declared `infrastructure-systemic` failure naming a path the builder cannot write was re-run as if a rebuild could fix it.

A fourth, from the same evening: **push-only could not recover the strand it exists for.** The ship journal (`appendShipJournal`) ran on finalize's success path — after the push — so lane 1678's rejected push left a commit with no journal entry, and `evolve ship --push-only` refused it by name.

## Fix (this change)

- `internal/addedtests` — the ONE derivation of "which test packages did this tree add, under which tags" (moved from the ship's backstop). The build floor now runs every added tag-gated package under its own tags (`addedTaggedTestFailures`, seam `buildSelfCheckTaggedRunner`), from the same seed (`changedpkgs.ChangedFilesChecked`, working tree vs the cycle base) and the same grouping the ship gate uses — even when nothing default-visible changed. A red is a floor failure naming the package, its tags and the failing test, handed to the builder while it still owns the tree. Red first: `TestChangedPackageFloorChecks_RunsAddedTagGatedPackagesUnderTheirTags` (the cycle-1679 shape).
- The scout gate enforces its rule: an eval without a `[code]` grader blocks the scout with the slug and the rule named, and the remediation lists the exact path to fix (`TestMaterializationGate_RequiresACodeGraderPerEval`). Legacy evals are untouched — the gate inspects only the slugs the current scout selected.
- The ship journals every MINTED commit, success or not: a rejected push now leaves an attested strand push-only completes (`TestFinalize_MintedCommitIsJournaledEvenWhenThePushFailed`).

## Not fixed here — the remedy routing

The retry envelope should route a builder-declared "unappliable by this phase" failure to the phase that owns the named path (`.evolve/evals` → scout or tdd), with the builder's remedy file attached, instead of re-audit plus rebuild. With fixes 1 and 2 the case cannot recur for evals (the scout is failed while it holds the pen, and the floor finds any other tag-gated red before handoff), so the routing is recorded as an open design item rather than patched around.

## Operator notes

- A floor line `./acs/<pkg> (-tags acs): unit tests FAIL` is the floor running an added tag-gated package; the builder re-runs it with exactly those tags.
- A scout block `scout materialized evals without a [code] grader for selected slug(s): …` is the new gate; the remediation names the file to fix.
- A rejected push now journals its commit; `evolve sync-main` then `evolve ship --push-only` completes the strand. Until this lands, the recovery is a normal `evolve ship --class manual` on the plane (recorded in memory).
