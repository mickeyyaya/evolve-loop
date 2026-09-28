# A failed cycle's closeout moved main under a passed sibling (cycle 1704, 2026-09-26)

- **Outcome:** cycle 1704 needed two extra rounds after passing Audit, and one of them overturned its verdict on the same bytes. It shipped only after a correction round and a rebuild.
- **Class:** a process issue blocked a passed audit, which the operator's direction of 2026-09-26 forbids: "The pass audit changes should not be blocked because process issue."
- **Landed:** 2026-09-28, on `feat/dossier-pending-at-boundary`. It was salvaged from the unlanded 2026-09-26 worktree (`fix/dossier-commits-at-wave-boundary`) and ported onto a main that had moved two days, test-first.

## What happened

1. Wave 12 ran lanes 1704 and 1705. Cycle 1704's audit returned **WARN**, which ships under the fluent policy; it rated finding H1 MEDIUM.
2. Cycle 1705 sealed FAIL. Its closeout committed `dossier: cycle-1705 closeout` (2198b2d3) to the plane's `main`.
3. Cycle 1704 reached Ship. Ship's plane-HEAD check saw `main` had moved (audited eef4846b, current 2198b2d3) and refused with `AUDIT_BINDING_HEAD_MOVED`.
4. The forced re-audit read the same bytes, rated H1 **HIGH**, and returned FAIL. The audit report itself records the round-to-round severity drift.
5. After the correction round passed, Ship refused again with `GIT_FLEET_REBASE_NEEDED`, because `main` still held the closeout. Recovery rebased the lane and sent it back to Build.

## Root cause

Every cycle committed its closeout dossier to the plane's `main` as it finished. In a fleet wave, sibling lanes are still between Audit and Ship at that moment. A FAIL cycle's dossier is the only thing that moves `main` in that case, so every in-flight sibling is forced through re-audit or rebase for a bookkeeping record that touches none of their files.

The dossier is committed at once, rather than left as a file, for a reason. `knowledge-base/` was not host runtime state to the leak classifier (`isLegitimateMainTreePath`). An uncommitted sibling dossier would therefore be "recovered" as a leak and moved into another lane's worktree. That is why the commit could not simply be left for later in the corpus.

## Fix

- **A fleet lane writes its dossier to the pending dir.** `cycleRun.dossierDestination` sends a lane in `fleetMode(req.Env)` to `dossier.PendingDir`, `.evolve/dossiers-pending/`. That is host runtime state: no phase profile grants writes there, and `.evolve/*` is gitignored. A pending write takes no git-mutation lock and commits nothing. A sequential cycle commits into the corpus as before, and `--simulate` writes without a commit (`WithDossierDestination(DossierFilesOnly)`, which replaces `WithDossierCommit(false)`; a simulate lane keeps files-only).
- **Pending dossiers are published when no lane runs.** `publishPendingDossiers` (`cmd_loop_dossiers.go`) runs:
  - after the plane sync at each wave boundary (`prepareIteration`), because committing before the sync, on a plane behind origin, would diverge it and halt the loop;
  - on every loop exit, in `loopResult.emit`, before the spine fail-open rollup reads the corpus;
  - after `evolve fleet` and each `evolve campaign run` wave return, through `runLanesThenPublish`.

  It holds the shared git-mutation lock, and it commits only onto a plane that holds origin/main's history (current or ahead). A plane behind or diverged, or one whose origin/main was never fetched, keeps the pairs pending and says why. With nothing pending it takes no lock.
- **`dossier.PublishPending` publishes only what the host wrote.** A pair qualifies when its JSON parses, re-renders to the same bytes and holds the cycle it is filed under, and its markdown is `RenderMarkdown` of that dossier. A markdown half lost to a crash is rendered from the JSON; a markdown-only half stays pending. A refused pair, a failed write or a failed commit restores the corpus to its prior bytes and leaves the pair pending.
- **No leak-classifier exemption.** A first design treated corpus files in the plane as host state. Security review blocked it: the doc-sync profile may write `knowledge-base/**` in the plane, so a planted pair would have skipped review and been committed to `main`. `resume_goal` also reads a dossier's goal into later prompts. The corpus now never holds an uncommitted pair, and a planted pair is still relocated for review.

## Consequences

- A lane's closeout reaches the corpus at the next boundary, not at once. The mid-wave readers of the corpus fail soft on a missing record: `resume_goal` falls back to the goal hash, `cycle-health`'s `dossier_commitment` check reports nothing, and a pool lane that starts later in the same iteration does not see an earlier lane's record in the chronicle seed.
- A plane left behind origin at loop exit keeps its pairs pending until the next boundary's sync.

## Regression coverage

- `core/dossier_producer_simulate_test.go::TestWithDossierDestination_IsTheRootsKnob`: the fleet-lane row reproduced the incident (the dossier in the corpus, +1 commit) before the redirect.
- `core/dossier_producer_simulate_test.go::TestDossierDestination_AFleetLaneDefersOnlyACommit` and `::TestWriteCycleDossier_APendingDossierTakesNoLockAndCommitsNothing`.
- `dossier/publish_pending_test.go::TestPublishPending_PublishesExactlyWhatWriteProducesAndCommitsIt`, `::TestPublishPending_RejectsAPairWriteDidNotProduce`, `::TestPublishPending_CompletesAJSONOnlyPairFromItsJSON`, `::TestPublishPending_LeavesAMarkdownOnlyHalfPending`, `::TestPublishPending_AFailedCommitKeepsThePairPendingAndTheCorpusClean`.
- `cmd/evolve/cmd_loop_dossiers_test.go::TestPrepareIteration_PublishesPendingCloseoutsOntoTheSyncedPlane` (a publish moved before the sync halts the boundary on the divergence), `::TestLoopSummaryCountsTheLastWavesPendingCloseouts` (a publish moved after the rollup counts nothing), `::TestCampaignRun_PublishesTheWavesPendingCloseouts`, `::TestPublishPendingDossiers_HoldsThePairsUnlessThePlaneHasOriginsHistory`.
- `bridge/sandbox_dossier_pending_test.go::TestNoProfileCanWriteTheDossierPendingDir`: a profile granted the pending dir fails it.

## Still open

A PASS lane's own landing still moves `main` under its siblings. That is a real peer change, and ADR-0105's recovery ladder handles it: B1 and B2 skip the Build, and the carry (B3/B4, [logic-first-delivery-design.md §5.8](../architecture/logic-first-delivery-design.md)) ships a byte-identical rebase on its audited verdict.
