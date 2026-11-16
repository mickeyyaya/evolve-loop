# 2026-11-14: ship spent 88% of its lane time on failed attempts

Related documents: the incident [2026-11-09-duplicate-ratchet-misses-staged-predicates.md](2026-11-09-duplicate-ratchet-misses-staged-predicates.md), which also shows a gate that ran at the wrong time.

## Summary

- **Impact:** in waves 171 and 172 (cycles 2357 to 2377), 37 ship tries gave 12 landings. The 25 failed tries used 575 of the 653 ship lane minutes (88%).
- **Cost of one failure kind:** `GIT_FLEET_REBASE_NEEDED` caused 14 tries and 442 minutes, about 31 minutes each. An try that lands takes about 6.5 minutes.
- **Root cause:** `runNative` runs the repo-contract pack (20 to 30 minutes) before any ancestry check. The try finds that main moved only at the fast-forward, after the pack.
- **Second defect:** an early failure leaves the audited change uncommitted. The recovery code then reads the lane as "already landed" and ends the cycle.
- **Fix:** an early fast-forward check before the pack, and a recovery that keeps the pending audited change. The fix is design PR #1466 (see F1.7). It is not on main in this worktree. A structural fix is not built.

## Terms used throughout

- **The pack:** the repo-contract gate that ship runs in the lane worktree.
- **A fleet cycle ship:** a ship of a cycle that runs in a lane next to other lanes.
- **Moved main:** main has a commit that the lane base does not hold.
- **A pending audited change:** the change that the audit approved and that no commit holds yet.

## Timeline

| Time | Event | Evidence |
|---|---|---|
| Waves 171 and 172 (cycles 2357 to 2377) | 37 ship tries run. 25 fail and 12 land. | Signal Center measurement in the fact sheet |
| 2026-11-14 | The design is written and goes through 3 design rounds and 3 code-review rounds. | Fact sheet, PR #1466 |

The fact sheet gives no clock times. The record keeps the dates only.

## Root cause

**F1.1** The ship phase runs the pack first. `runNative` calls `runRepoContractGate` (`go/internal/phases/ship/ship.go:126`) and only then `runStages` (`ship.go:133`).
*Implication:* the 20 to 30 minute pack runs before ship knows if main moved.

**F1.2** `land` is at `go/internal/phases/ship/landing_intent.go:30`. It runs in the stages, after the pack. It finds the moved main at the fast-forward.
*Implication:* a lane that loses the race pays the full pack time and then fails.

**F1.3** Measured on 2026-11-14 (Signal Center, waves 171 and 172): in 15 of 22 race failures a peer landed on main before the try started. In 5, a peer landed during the try.
*Implication:* most failures were knowable at the start of the try. The 5 late cases need the check at land time.

**F1.4** An early failure leaves the audited change uncommitted, so `HEAD` equals the cycle base. `ClassifyFleetRebaseCandidate` (`go/internal/core/carryforward_filter.go:40`) reads this as "already landed" and ends the cycle.
*Implication:* a lane that must rebase is dropped, not retried.

**F1.5** Two ship checks already raise the rebase error before the commit: the base-red check (`go/internal/phases/ship/base_red.go:104`) and the main-red stop (`go/internal/phases/ship/main_red_stop.go:89-93`).
*Implication:* these two paths hit the same recovery defect as F1.4.

**F1.6** `namesMovedMain` (`go/internal/core/carrier_commit.go:22`) returns true for `CodeAuditBindingHeadMoved` and `CodeGitFleetRebaseNeeded`.
*Implication:* the recovery already has one place that names the moved-main codes.

**F1.7** The fact sheet describes a design in PR #1466. I searched this worktree for `checkEarlyFastForward`, `earlyFastForwardApplies`, `binaryChurnPaths` and `pendingAuditedChange`. I found none, and `git log` shows no commit for the PR. I cannot verify the PR. Its details below come from the fact sheet only.
*Implication:* the "What changed" section lists the design, not code on main.

## What changed

The fact sheet gives this design for PR #1466 (not verified on main):

| Change | Regression test |
|---|---|
| `checkEarlyFastForward` runs before the pack, for fleet cycle ships only (`earlyFastForwardApplies`). It is a lock-free query and shares the ancestry rule of land time. It keeps the same error code. | The pack runs first today. |
| The check skips in three cases: a catch-up is due, the lane carries no work (`binaryChurnPaths`), or the latest audit is on another base. | Same fact-sheet test set. |
| Recovery carries the pending audited change. `namesMovedMain` and one query, `pendingAuditedChange`, decide it: the audit row decides. | A pending lane ends as "already landed" today. Rows exist for the early, base-red and main-red-stop paths. |
| A `Config.RepoContract` seam lets a test replace the pack. | Used by the tests above. |

The fact sheet gives no test names. I do not invent them.

## What we learned

- A gate that costs 20 to 30 minutes must run after the cheap checks that can fail the try. The ancestry check is cheap and must run first.
- Two checks that run before the commit must leave the state that the recovery reads. A recovery that reads `HEAD` alone cannot see a pending change. The audit row is the source.
- A structural fix is not built: the integration `EnforceActor` lands lanes in batches, so lanes do not each race for main. A card is under review.
