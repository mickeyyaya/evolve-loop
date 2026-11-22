# 2026-11-22: the TDD floor did not run the test-audit ratchet, and a correct build failed for the fifth time

Related documents: the spec [test-system.md §6.4](../architecture/test-system.md#64-the-duplicate-ratchet), the package note [internal-core.md](../architecture/packages/internal-core.md) and the earlier record [the TDD floor deadlocked on a copy of an unimportable helper](2026-11-20-the-tdd-floor-deadlocked-on-a-copy-of-an-unimportable-helper.md).

## Summary

- **Impact:** a correct implementation failed at the Build floor in two cycles: 2599 and 2606. Cycle 2606 (item `recycle-x2-gocache-counts-executable-dirs`) lost its full build and audit run.
- **Root cause:** two floors judge the same predicate file with two different rule sets. The Build floor runs the test-audit ratchet. The TDD floor checks only the helper copies.
- **Effect:** a predicate that breaks a test-audit rule passes the TDD handoff. The Builder cannot edit `go/acs/**`, so no phase can correct the file.
- **Fix:** the TDD floor and `evolve phase verify tdd` call one function, `core.TddTestAuditFailures`. It audits the packages that the Build floor audits, with the rules and the configuration of the Build floor. A finding goes back to TDD.
- **Regression test:** tests replay the cycle 2606 shape, a skip that hides an error. They are red on the earlier code.

## Terms used throughout

- **The Build floor:** the check `build_handoff_floor` that runs when Build hands off.
- **The TDD floor:** the check `tddPredicateFloor` that runs when TDD hands off.
- **The test-audit ratchet:** the pure-AST check in `go/internal/testaudit`. It refuses a test shape that a new baseline row cannot allow.
- **A predicate file:** a test file in `go/acs/cycle<N>/`. TDD writes it. No later phase can change it.

## Timeline

| Cycle | Event | Evidence |
|---|---|---|
| 2599 | The Build floor refuses `TestC2599_009_TheAbnormalEventLogParsesByKindAndTime` in `go/acs/cycle2599/predicates_test.go:304` for the rule `error-text-match`. The Builder cannot edit the file. | lesson `inst-L2599a` and its source files |
| 2606 | The Build floor refuses `TestC2606_010_EvolveGcSparesAnExecutableDirALiveProcessHolds` for the rule `skip-hides-error`. | `.evolve/runs/cycle-2606/build-verify-output.txt:3`, lesson `inst-L2606a` |

The Build floor of cycle 2606 reported:

```text
FAIL: build deliverable has 1 violation(s):
  - [build_handoff_floor] test-audit ratchet: testaudit: acs/cycle2606 TestC2606_010_EvolveGcSparesAnExecutableDirALiveProcessHolds skip-hides-error has no baseline row
```

## Related: the same pattern elsewhere

- **Cycle 2532** is a different check with the same pattern. It failed at the TDD review gate, not at Build. The TDD hand-check `evolve eval quality-check` (`go/internal/cli/guardcmd/eval.go:132,141`) runs no `LintSourceTextPredicates`, which the review gate enforces. Source: lesson `inst-L2532a`.
- **Cycles 1448 and 1488** come only from cross-references in the lessons `inst-L2599a` and `inst-L2606a`. Their run directories are gone, and the defects that the lessons name for them are not test-audit rules. They are related evidence for the separate item `build-floor-finding-in-a-tdd-file-routes-to-tdd`.

PR #1580 and PR #1605 gave the Build floor its test-audit check (item `build-floor-runs-the-test-audit-ratchet`). They did not change the TDD floor.

## Root cause

**F1.1** `testAuditFailures` ran in the Build floor only (`go/internal/core/build_floor_reviewer.go`, `StaticBuildFloorChecks`).
*Implication:* a finding of the ratchet exists only after Build starts.

**F1.2** `tddPredicateFloor.Review` called `tddHelperCopyGroups` only. `evolve phase verify tdd` checked the structure of the deliverable, the frozen pins and the comments.
*Implication:* no check before the Build handoff ran the test-audit rules.

**F1.3** The test-audit check reads the files with `go/parser` and needs no compile. It can run at the TDD handoff.
*Implication:* the floors differed by omission, not by a technical limit.

**F1.4** Build cannot edit `go/acs/**`, and the correction loop sent the finding to the Builder.
*Implication:* the finding cannot be fixed in Build (item `build-floor-finding-in-a-tdd-file-routes-to-tdd` tracks the routing).

## What changed

- `core.PackageAuditFailures(moduleDir, pkgs)` holds the rules and the configuration. The Build floor scopes it to the packages of the changed paths. `core.TddTestAuditFailures` uses the same scope code, and adds `acs/cycle<N>`. `tddPredicateFloor` and `evolve phase verify tdd` both call it, so each audits every package that TDD changed. There is no second rule list and no second scope.
- A refusal names the rule and the test. The TDD floor returns it with `Retry`, so the correction goes to TDD.
- The code `tdd_test_audit_floor` is the violation code of `evolve phase verify tdd`.
- Regression tests:
  - `core/tdd_predicate_floor_test.go::TestTddPredicateFloor_AuditsEveryPackageTddChangedNotOnlyThePredicateDirectory`
  - `cli/phasecmd/phase_verify_tddcomment_test.go::TestPhaseVerifyTDD_ReportsTheLinesOfTheTddFloorForEveryPackageTddChanged`
  - `core/tdd_predicate_floor_test.go::TestTddPredicateFloor_RefusesAPredicateThatSkipsToHideAnError`
  - `core/tdd_predicate_floor_test.go::TestTddPredicateFloor_ReportsATestAuditFindingAndAHelperCopyTogether`
  - `cli/phasecmd/phase_verify_tddcomment_test.go::TestPhaseVerifyTDD_PredicateThatSkipsToHideAnError_Exit1`

## What we learned

- Two floors that judge one file must call one function. A new rule in one floor now reaches both.
- A finding in a file that a phase cannot edit must stop at the phase that owns the file.
