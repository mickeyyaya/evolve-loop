# 2026-11-20: the boundary batch ejected PR 1582 on two real failures

Related documents: the batch verb is `evolve integration batch`. The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** the batch of the wave 182 boundary ran PRs 1582, 1583, 1584 and 1585. It landed three of them. PR #1582 (retire the `regression_tia` stage, F5b) was ejected and stayed open. The batch opened 5 batch PRs (#1588 to #1592) and split twice before it found the PR that fails. The console rebuilt the work as #1595.
- **Root cause:** the CI of the batch PRs found two real failures in the content of #1582. One is an STE finding, a paragraph of 10 sentences in `docs/architecture/shadow-stage-contract.md:3`. The other is the test `TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline`, which needs a new duplicate baseline. The PR had reached the batch with both.
- **Fix:** the console rebuilt #1582 on a new linear branch with the STE fix and the baseline file. The replacement landed as `evolve-pr 1595: integrate` at 09:12 UTC (`f13d47c7d`).
- **Lesson:** the pre-ship check set of the console missed two whole-tree gates. This is the same escape as in [the record of the four red console PRs](2026-11-20-four-console-prs-failed-pr-ci-after-green-local-runs.md).

## Terms used throughout

- **Batch PR:** a PR that `evolve integration batch` opens from a set of PRs, to run the CI on the composed tree.
- **Eject:** the batch removes the PR that a split proves red. The PR stays open.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| before 07:36 | The batch tests 1582 to 1585 in batch PR #1588. It is red twice (run 44679245685). The splits #1589 (run 44684647668) and #1590 (run 44690639140) are red for the PRs that hold 1582. | batch output in the console log |
| 07:36 | PR #1583 lands through batch PR #1591 (run 44696277152). | `git log origin/main` (`296d48100`, 15:36 +08:00) |
| 08:08 | PRs #1584 and #1585 land through batch PR #1592 (run 44700164892). | `git log origin/main` (`da40d1c67`, 16:08 +08:00) |
| 08:10 | The batch exits with rc 1: `integration: PR #1582 was ejected and stays open: read the brief on the PR`. | console log |
| 08:11 | The console reads the failed steps of run 44690639140: `STE lint` and `test (race + cover, incl. integration tier)`. | `gh run view` output in the console log |
| 09:12 | The replacement PR #1595 lands. | `git log origin/main` (`f13d47c7d`) |

The STE finding in the CI log:

```text
docs/architecture/shadow-stage-contract.md:3 STE-PARAGRAPH a paragraph has 10 sentences; the limit is 6
ste-lint: 1 finding(s) in 1 file(s); 8 file(s) checked
```

## Root cause

**F1.1** PR #1582 was a rebuild of PR #1574. An earlier batch ejected #1574 on conflicts in `signalcenter/registry.go` and `signal-codes.md` (the close comment of #1574).
*Implication:* the PR had been through two integrations. Its docs and its test baseline did not match main.

**F1.2** The duplicate-copies test compared the count of copies with the baseline. The console measured 332 and then 330 on one tree, and then 330 stable. It then compared the count with the count of `origin/main`.
*Implication:* this test is sensitive to the tree. A baseline set on a stale tree fails after a rebase.

**F1.3** The log of the failed run has many other FAIL lines, for example `TestC1950_001` and `TestC1761_009`. They print fixture output of tests that run a nested cycle. The console fixed only the two causes above, and the batch PR of the replacement was green. I did not verify the other lines one by one.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The paragraph is split. The duplicate baseline is set from a stable count. The work is rebased as one linear branch. | #1595 (replaces #1582 and #1594) | `repocontract::TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline` (red in the batch run, green in the batch of #1595); `evolve docs ste-lint --strict --changed <base> --added` |
| The pre-ship check set of the console runs the STE lint and the duplicate count | console practice, and the item `ship-backstop-runs-the-test-audit-ratchet` for the ratchet part | owed by the item |

## Limits

- The brief that the batch writes on the ejected PR was not read in this record. The causes above come from the CI log.
- The state of the head CI of #1582 before the batch is not verified.

## What we learned

- A PR that an earlier batch ejected needs a full local run of the whole-tree gates before it is recorded again.
- Read the failed step names first. Then read the first real FAIL line, not the count of FAIL lines.
