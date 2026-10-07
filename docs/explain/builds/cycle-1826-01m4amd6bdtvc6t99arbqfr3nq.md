# Build Explanation — Cycle 1826

## Build Binding
- Cycle: 1826
- Base SHA: b0860ce11360ff54405cc88fb3ced50b27e3019f

## Summary
`evolve dossier publish [--dry-run] [--project-root P]` publishes the closeouts that a killed loop left in `.evolve/dossiers-pending`. It runs the loop exit's checks through the same function the loop exit calls, then calls `dossier.PublishPending`. Exit 0 means it published, or nothing was pending. Exit 1 means the publish was held by a live run, a busy git-mutation lock, or a plane behind origin/main, and the output names which. Exit 2 is an I/O failure, and exit 10 is a usage error.

## Rationale
Pending closeouts were committed only when a fleet run's lanes finished. After a killed loop, the pairs waited for the next launch, or for raw git, which skips the ship lock, the live-run check and the publish hold. A verb that reuses the loop exit's checks gives the operator a safe flush at the wave boundary. Extracting those checks into one function means the verb and the loop cannot drift apart.

## Changed Areas
- `go/cmd/evolve/cmd_loop_dossiers.go` — the checks `publishPendingDossiers` ran inline (plane classification, the live-run check, the git-mutation lock, then the post-lock live-run and origin/main re-check) move into `holdDossierPublish`, which takes the lock function as a parameter. The loop keeps its blocking `dossierPublishLock`. `tryDossierPublishLock` takes the lock without waiting, and `probeDossierPublishLock` is the dry run's version, which never creates the lock file.
- `go/cmd/evolve/cmd_dossier.go` — `runDossier` routes `publish` to `runDossierPublish`, and both usage lines name it. The verb lists the pending pairs and half pairs and returns 0 when nothing is publishable, before it takes any lock. It then calls `holdDossierPublish` and either reports the hold (exit 1), reports a dry run, or publishes through `publishDossierPairs`.
- `go/cmd/evolve/cmd_loop_dossiers_test.go` — covers a dry run while the lock is busy: it exits 1 naming the lock and the pending cycle, and publishes nothing.
- `go/internal/dossier/publish_pending.go` — new `ListPending` splits the pending directory into complete pairs and half pairs. `PublishPending` now uses it, so the verb's listing and the publish share one parser.
- `go/internal/dossier/publish_pending_test.go` — names `ListPending`: it covers the pair and half-pair split, a stray file, an absent directory (empty) and an unlistable one (an error naming `dossiers-pending`).
- `go/acs/cycle1826/predicates_test.go` — the TDD phase's ACS predicates for this cycle, committed with the build.
- `.evolve/evals/cli-dossier-publish.md` — the task's eval, committed with the build.
- `docs/operations/runtime-reference.md` — step 3 of the wave-boundary procedure documents the verb, its exit codes and `--dry-run`.
- `docs/architecture/packages/internal-dossier.md` — records `ListPending` and that the loop exit and the verb share `holdDossierPublish`.

## Design Decisions
The lock is injected into `holdDossierPublish` and not chosen inside it. The loop exit must keep waiting on the lock as it always has, and the verb must never wait. Injecting the lock keeps every other check, and their order, identical between the two callers. A failure to take the lock is a hold, as it has always been for the loop. The dry run probes the lock only when the lock file already exists, so it leaves the plane's `git status` unchanged. A dry run uses the same exit codes as a real publish, so a script can tell whether a publish would be held.

## Verification
The cycle-1826 ACS predicates pass, 8 of 8: publish and leave the half pair; hold on a live run, but not on a dead lease; an inert dry run; holds on a busy lock and on a plane behind origin/main without waiting; exit 2 on an unlistable directory; one shared checks function; usage with exit 10; the boundary doc. The loop exit's regression set (`TestPublishPendingDossiers_*`, `TestLoopSummary*`, `TestPrepareIteration_PublishesPendingCloseouts`, `TestCampaignRun_PublishesTheWaves`, `TestRunFleet_PublishesTheLanes`) and the dossier package tests also pass.

## Compatibility
The loop exit's output lines and its behavior are unchanged; a lock error is still reported as `git-mutation lock: <err>`. `PublishPending` returns the same result shape (half pairs in `Skipped`, a nil slice when there are none).

## Limitations
`--project-root` defaults to the current directory and does not read `$EVOLVE_PROJECT_ROOT`, matching `dossier retro-mislabel`. A lock that cannot be opened at all is reported as a hold (exit 1), not as an I/O failure, because the loop has always classified it that way.
