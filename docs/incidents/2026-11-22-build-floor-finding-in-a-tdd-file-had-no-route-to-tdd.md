# 2026-11-22: a Build floor finding in a TDD file had no route to TDD

Related documents: the reroute is in [internal-core.md](../architecture/packages/internal-core.md) (the predicate reroute). The ratchet is in [internal-testaudit.md](../architecture/packages/internal-testaudit.md). The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** cycles 2599 and 2606 sealed FAIL with a correct implementation. The Build floor named a test-audit finding in `go/acs/cycle<N>/`. Build may not edit that directory, so no Build correction can fix it.
- **Root cause:** the predicate reroute is the one exit from Build to TDD. It knew one kind of failure: a failing predicate unit test. It also waited for a byte-identical repeat. A test-audit line never matched. The exit never fired.
- **Fix:** the loop now classifies the floor failures that only TDD can fix in one function (`tddOwnedFailures`). A test-audit finding in the package of this cycle routes to TDD at the first rejection.
- **Lesson:** one function decides who can fix a failure. A new kind of failure adds a field there, not a second exit.

## Terms used throughout

- **Build floor:** the deterministic checks that Build runs before its handoff. A refusal gets a correction round.
- **Test-audit finding:** a weak test pattern with no baseline row. The floor line starts with `test-audit ratchet: `.
- **TDD-owned failure:** a floor failure in `go/acs/cycle<N>/` of this cycle. Only TDD can fix it.

## Timeline

| Event | Evidence |
|---|---|
| Cycle 2599 seals FAIL after Build corrections that were not allowed to edit the predicate file. | lesson `inst-L2606a` |
| Cycle 2606 seals FAIL for the same cause. | lesson `inst-L2606a` |
| The console files the item `build-floor-finding-in-a-tdd-file-routes-to-tdd`. | the inbox item of this name |

The floor line has this shape:

```text
test-audit ratchet: testaudit: acs/cycle2606 TestX error-text-match has no baseline row — remove the weak pattern
```

## Root cause

**F1.1** `acsPredicateFailures` keeps only lines that match the unit-test FAIL shape. A test-audit line does not match.
*Implication:* the reroute never fired, and each correction re-dispatched a Builder that cannot edit the file.

**F1.2** The reroute waited for two equal failure lists. Build can never fix a test-audit finding in this directory.
*Implication:* a wider match still wastes one Build round.

## What changed

| Change | Regression test |
|---|---|
| `testaudit.MissingRowPackage` and the shared `missingRowFormat` give one home for the line wording. | `testaudit/missing_row_package_test.go::TestMissingRowPackage_RoundTripsEveryRenderedMissingRow` |
| `testAuditLinePackage` reads the package of a rendered floor line. | `core/predicate_reroute_test.go::TestTestAuditLinePackage_RoundTripsTheRenderedFloorLine` |
| `tddOwnedFailures` classifies the failures that only TDD can fix. `tddReroute` routes at the first rejection and names the cause that fired. | `core/predicate_reroute_audit_test.go::TestBuildFloor_ATestAuditFindingInACyclePredicateFileRerunsTDDWithTheFindingAtOnce` |
| The real ratchet output and the classifier agree. | `core/predicate_reroute_audit_test.go::TestTestAuditFailures_TheRealRatchetFindingInACyclePredicatePackageIsTDDOwned` |

## Limits

- A resumed Build has no reroute. A resumed Build with this finding still fails. The follow-up item is open.
- A whole-tree audit line has no package, so it is not routed.
- A cycle has one reroute. A mixed rejection sends the TDD-owned lines to TDD and the Builder lines wait one round.

## What we learned

- A gate that finds a defect must route it to the phase that can fix it.
- A parser of a rendered line shares the format constant with its renderer. A reword then cannot split them.
