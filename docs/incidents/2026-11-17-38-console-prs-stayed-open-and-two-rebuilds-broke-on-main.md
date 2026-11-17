# 2026-11-17: 38 console PRs stayed open, and two rebuilds broke on the current main

Related documents: the train is `go/internal/integration/train.go`, and the earlier train incident is [the integration train bisected for three hours on a red main](2026-11-17-the-integration-train-bisected-for-three-hours-on-a-red-main.md). PR #1532 adds the rule that the console finishes its open work first, as rule 6 in section 2 of [operating-policy.md](../operations/operating-policy.md).

## Summary

- **Impact:** 38 console PRs were open at the boundary of wave 177. 28 of them were stale or held no work that main did not have. A dry run of the train refused the whole batch on 6 of them.
- **Root cause 1:** the 2026-11-16 batch stopped during a bisect. The train closes a landed PR only at the end of the run that landed it. So 9 landed PRs, 9 batch PRs and their records stayed.
- **Root cause 2:** two rebuilds composed wrongly with code that main got after their first review. One was a wrong order of proposals (#1537). The other was a protected-surface gap and a fixture that only Go 1.23 fails (#1538).
- **Fix:** the console closed 28 PRs with a reason, and rebuilt 10 as one-commit PRs. It fixed #1537 and #1538 with red-first tests. The train defect is inbox item `integration-train-settles-landed-prs-after-an-interrupted-batch`.
- **Lesson:** a rebuild is a new change. It gets a fresh review against the current main, and its tests run on the CI Go version.

## Terms used throughout

- **Train:** the `evolve integration batch` verb. It composes PRs on main, runs CI on the composed branch, and fast-forwards main when CI is green.
- **Record:** the deliverable record of one PR, in `.evolve/waves/integration-console/`. The train reads it to bind the PR to its review.
- **Rebuild:** a new one-commit branch from the current main that carries the change of an old PR.
- **Settle:** the step of the train that closes each landed PR and removes its record.

## Timeline

| Time (UTC, 2026-11-17) | Event | Evidence |
|---|---|---|
| 08:20 | 38 console PRs are open. The oldest is from 2026-11-09. | `gh pr list --state open` |
| 08:30 | A dry run of the train refuses: 6 PRs have no review binding. | `evolve integration batch --dry-run`, rc 1 |
| 08:35 | A three-way merge of each PR with main shows that 9 PRs add nothing. Their changes are on main as `evolve-pr N: integrate` commits from 2026-11-15 and 2026-11-16. | `git merge-tree`, `git log --grep` |
| 08:40 | The console closes the 9 landed PRs and 9 leftover batch PRs. | `gh pr close`, rc 0 for each |
| 08:55 | The console closes the ADR-0165 stack (#1386 to #1395). #1407 and #1425 had landed the same work. #1462 is closed as landed. | close comments |
| 09:05 to 09:35 | 10 PRs are rebuilt on main and get review seats, the commit gate and ship: #1533 to #1539. | ship logs |
| 09:15 | The simplifier finds a HIGH in the #1487 rebuild: a reroute comes before the history consume of the same item. | review run `agent-code-simplifier-20261117T091531Z` |
| 09:37 | CI of #1538 fails two tests. Local runs on Go 1.27 pass. | run 44056351104 |
| 09:50 | The batch of #1530 to #1535 starts on a green main (`616d06f1d`). | `integration watch`, rc 0 |
| 09:57 | The #1538 fix ships. It is red on `GOTOOLCHAIN=go1.23.12` before the fix. | ship `38b56de81` |

## Root cause

**F1.1** `Train.settle` closes a landed PR and removes its record (`go/internal/integration/train.go:270-283`). It runs only at the end of the batch run that landed the PR. On 2026-11-16 the console stopped a batch during a bisect.
*Implication:* the PRs that the stopped run landed stayed open, and their records stayed. The next dry run read all records and refused the batch.

**F1.2** The ADR-0165 stack (#1386 to #1395) and the docs PRs had rebuilds that landed (#1407, #1425), or had conflicts with main after 403 commits. No step closes a PR that a rebuild replaced.
*Implication:* the open PR count looked like 38 units of work. The real count was 10.

**F2.1** Main added `RerouteBuildableKinds` and `HasOpenPR` to `inboxcurator.Inputs` after #1487 was reviewed. `Curate` put `Reroutes` before `ClosedOnMain`. The apply step runs proposals in order, so it moved the item before the consume ran, and the consume failed.
*Implication:* each change was correct alone. The defect was only in the composition, and only a fresh review of the rebuild found it.

**F2.2** #1515 adds `repocontract/datafiles.go` and its guard test. Neither file was on `guards.ProtectedSurfaceManifest`, and `TestEveryGateShapedFileIsProtectedSurface` runs only with the `acs` build tag.
*Implication:* the local check without the tag passed. Only CI found the gap.

**F2.3** The floor fixture of `gateregistry` made stub packages only for the tree-reading tests. The new whole-repository rows of #1515 name `internal/router`, `internal/skillcheck` and `internal/stelint`. On Go 1.23, `go test` fails on a missing directory. On Go 1.27 it does not.
*Implication:* the console ran tests on Go 1.27, and CI runs Go 1.23. A local green did not predict the CI result.

**F3.1** Two tool signals misled. `gh pr checks --required` exits 1 at once when the repository has no required checks. `evolve integration watch --tip` exits 10 (usage) for a short SHA.
*Implication:* each looked like a CI failure. The console read the check rollup and the full SHA to get the true state.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| 28 PRs closed, each with the landing commit or the replacing PR in its comment | none (`gh pr close`) | none |
| 10 PRs rebuilt as one-commit PRs on the current main | #1533 to #1539 | the original tests of each PR, on the composed tree |
| `Curate` proposes the history consume before the reroute | #1537 | `inboxcurator/history_test.go::TestCurate_AnItemThatMainClosedHasItsConsumeBeforeItsReroute` (red before the fix) |
| The data-file reads and their guard join the protected surface | #1538 | `acs/regression/protectedsurface::TestEveryGateShapedFileIsProtectedSurface` |
| The floor fixture stubs each package of `repocontract.ByNameTests()` | #1538 | `gateregistry/seed_floors_test.go::TestRegistry_FloorEntriesGiveAGreenLaneNoLaneRed` on Go 1.23 (red before the fix) |
| The train settles landed PRs at the start of each batch | inbox item `integration-train-settles-landed-prs-after-an-interrupted-batch` | open |

## What we learned

- A rebuild gets a fresh review against the current main. It is not a copy of a reviewed change. (F2.1)
- Before a ship, run the changed packages on the CI Go version (`GOTOOLCHAIN=go1.23.12`) and with the `acs` tag. (F2.2, F2.3)
- Before a batch, compare each open PR with main. Close a PR that adds nothing, with the landing commit in the comment. (F1.1, F1.2)
- Read the check rollup of a PR, not the exit code of `gh pr checks --required`. Give `evolve integration watch` the full SHA. (F3.1)
