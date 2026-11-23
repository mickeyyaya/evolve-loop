# internal/mainsync

## Purpose

`internal/mainsync` is the one home of the steps that bring the local `main` of a plane up to `origin/main`. Two callers need these steps: the wave-boundary sync (`cmd_loop_wavesync.go`) and the ship-window sync (`internal/phases/ship/shipwindow_sync.go`). Before this package each caller held its own copy of the steps, and the copies drifted. The wave copy ran a plain `git fetch -q origin main`, which leaves `origin/main` stale when `remote.origin.fetch` is narrowed or unset. The ship copy used the explicit refspec.

## Design

`Run(ctx, g, Policy)` does these steps, in this order:

1. It fetches with the refspec `+refs/heads/main:refs/remotes/origin/main`. A failed fetch ends the run with `OutcomeFetchFailed`. No caller trusts a stale `origin/main`.
2. It reads the relation of `HEAD` to `origin/main` (`gitexec.RelationToRemote`). A failed read ends the run with `OutcomeRelationFailed`.
3. It calls `Policy.Gate` unless the relation is `diverged`. A gate error ends the run with `OutcomeGated`.
4. It returns `OutcomeCurrent`, `OutcomeAhead` or `OutcomeDiverged` when the local main is not strictly behind. It never resets or forces a main.
5. When the local main is behind, it calls `Policy.Around`. It then runs `git merge --ff-only origin/main`. Last, it calls the `settle` function that `Around` returned. The outcome is `OutcomeBlocked` or `OutcomeFastForwarded`.
6. After a fast-forward it absorbs the root copies of claimed inbox items (`inboxmover.AbsorbRootCopies`). A root copy that equals a claim then cannot make the loop claim the item twice.

The package reports and never prints. Each caller turns a `Result` into its own log lines, with its own prefix.

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The policy differences are two hooks, `Gate` and `Around`, and not a flag. The wave boundary sets both. The ship window sets neither. | The wave boundary checks main CI and replays the inbox stamps of the loop around the merge. The ship window must not do either (see [internal-phases-ship.md](internal-phases-ship.md)). | A boolean per step: each new caller adds a flag, and the skeleton has two paths. |
| D2 | The package may import `inboxmover` and `gitexec`. It must not be imported by `core`. | `inboxmover` imports `core`, so an import from `core` makes a cycle. `go list -deps ./internal/inboxmover` shows it. | Take the absorb out of the package: the absorb step is then a third copy. |
| D3 | The package assumes no more than the ship-window lease gives. The caller holds the lease for the whole run. The caller bounds the run with a context (60 seconds in both callers). The package does not heartbeat, test or renew the lease. | The lease is broken at 15 minutes (`shipwindow.DefaultTTL`) while ship sections run 14 to 38 minutes. A separate item builds the heartbeat. The sync is short and bounded, so a broken lease can at worst let a sibling move main during the sync. `merge --ff-only` then still refuses any move that is not a fast-forward. | Renew the lease inside the sync: this changes `internal/shipwindow`, and that change belongs to the heartbeat item. |

Pinned by (TestRun_ABehindMainIsFastForwardedAndSettledAsAdvanced, TestRun_AFetchConfigThatOmitsMainStillRefreshesOriginMain, TestRun_CurrentAheadAndDivergedMoveNothing, TestRun_AFailedFetchIsReportedAndNothingIsTrusted, TestRun_AnUnreadableRelationIsReported, TestRun_ATrackedChangeBlocksTheFastForwardAndIsSettledAsBlocked, TestRun_AGateErrorStopsTheSyncBeforeAnyFastForwardAndSkipsADivergedMain, TestSyncMainAtWaveBoundary_AFetchConfigThatOmitsMainStillRefreshesOriginMain).
