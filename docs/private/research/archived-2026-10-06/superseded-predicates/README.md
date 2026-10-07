# Superseded ACS predicates: the inbox weight order (2026-10-06)

> Archived by P2 of [the inbox prioritization plan](../../../../plans/inbox-prioritization-2026-10.md), which made `inboxrank.Order` the one order every inbox consumer reads ([ADR-0121](../../../../architecture/adr/0121-inbox-priority-is-a-computed-rank.md), P2 amendment). The predicates below asserted the hand-typed `weight` order as their acceptance, so P2 supersedes them. They are archived here under the A2 retention rule (never deleted, never silently edited); the rest of each package stays live in `go/acs/`.

## What was archived and why

| Predicate | It asserted | Why P2 supersedes it | What pins the new contract |
|---|---|---|---|
| `TestC536_007_WaveSeedPicksDisjointRepsNotRawTopWeight` | the wave seed's first lane is the highest-weight candidate | the seed is the rank's first disjoint lanes: a lighter item of an earlier class, or one that unblocks waiting work, can lead | `internal/loopwave` `TestSeedWavePlanFromInbox_SeedsTheFirstRankedLanesNotTheHeaviest`; `internal/triagecap` `TestReadInboxBacklog_IsTheRanksOrderOverTheReadyListAgainstTheWholeQueue` |
| `TestC536_008_WaveSeedCountBelowTwoIsLegacySingleFocus` | at `count < 2` the seed is the single highest-weight candidate | at `count < 2` the seed is the first-ranked candidate | `internal/triagecap` `TestTopN_FleetWidthAware_CountOneOrAbsent_ReturnsTheFirstRankedCandidate`, `TestSelectFleetWidthTopN_KeepsTheBacklogsRankOrder` |
| `TestC1724_002_ClassifyIgnoresDepsForOrdering` | `inboxbatch.Classify` with no order orders a cluster weight-descending (`child,parent`), so deps cannot reorder it | `Classify` keeps the order it is given (`Config.Order`; none means the caller's order) and ranks nothing by weight itself; the triage path injects the rank, whose `unblocks` factor deliberately lets an item that unblocks queued work rise | `internal/inboxbatch` `TestClassify_DepsNeitherBindNorReorderTheInjectedOrder`, `TestClassify_WithoutAnOrderKeepsTheCallersOrder`, `TestClassify_ClustersRankByTheirBestRankedMember` |

## Superseded by the architecture review's fix round (2026-10-06): a classless remediation item is refused

The review's W2 made faillearn's inbox writer refuse a remediation item that names no `priority_class` (`inboxbatch.ErrNoPriorityClass`), as `retrofile.FileActions` and `dispositionrouter.StageIntent` now do for their funnels. Two predicates filed classless remediation items as the premise of their acceptance:

| Predicate | It asserted | Why the fix round supersedes it | What pins the behavior now |
|---|---|---|---|
| `TestC1290_001_FloorArtifactsPublishAtTheAtomicwriteMode` | the floor's artifacts, including two classless remediation items, publish at the atomicwrite mode | a classless item is now refused before any write, so the fixture can no longer reach the inbox | `internal/faillearn` `TestWriteArtifacts_PublishedArtifactsHaveMode0644` (classed fixture), which the live `TestC1290_003` still runs by name |
| `TestC1292_001_PartialInboxWriteNamesOnlyUnqueuedItems` | a partial write of classless items names only the unqueued ones | the refusal now happens before the first write, so a classless batch never writes partially | `internal/faillearn` `TestWriteArtifacts_PartialWriteNamesOnlyUnqueuedItems` (classed fixture), run by the live `TestC1292_003`; `TestWriteInboxItems_RefusesAnItemWithNoPriorityClassAndWritesNone` |

`dispositionrouter.StageIntent` now refuses a classless autofile intent (`inboxbatch.ErrNoPriorityClass`), which supersedes two more:

| Predicate | It asserted | Why the fix round supersedes it | What pins the behavior now |
|---|---|---|---|
| `TestC1062_008_ShadowStageWritesReportOnly` | a shadow apply of a staged escalate plus a staged classless autofile writes only its report | the classless autofile can no longer be staged | `internal/recurrence` `TestApplyBoundary_ShadowWritesReportOnly` (its staged autofile carries a class) |
| `TestC1062_009_AutofileGoesThroughRetrofile` | a staged classless autofile is filed through retrofile | the same; and a legacy classless line already staged is now refused at apply, recorded in `ApplyResult.Refused`, never filed | `internal/recurrence` `TestApplyBoundary_AutofileUsesRetrofile`, `TestApplyBoundary_AClasslessAutofileStagedBeforeClassesIsRefusedNotFiledAndDoesNotWedge` |

`go/acs/cycle1292` lost its now-unused `encoding/json` import with the archive. `TestC1290_004` compares `go/internal/faillearn/inbox_transactional_test.go` with `HEAD`; the fixture there gained `priority_class`, so it is red in an uncommitted tree and green once the change is committed. `TestC1292_004` is red in any checkout of this tree for a data reason: the tracked inbox item `2026-08-04T22-40-00Z-audit-eval-existence-path-convention.json` has a title over 160 characters, so the loader reports a sanitize notice. That predates P2.

## Notes

- **`go/acs/cycle536` compiled again after the archive.** Its two archived predicates called `triagecap.SelectWaveSeedTopN` with two arguments, a signature that gained `isProtected` long before P2, so the whole package had not compiled since. With them archived, its other seven predicates compile and pass (`go test -tags acs ./acs/cycle536/`, 2026-10-06).
- **Checked and kept:** `cycle1159`, `cycle1180`, `cycle1181` (002), `cycle1182`, `cycle1206` and `cycle1633` assert committed-prefix preservation, consumed-id pruning, rule-set membership or batch independence, not the weight order, and pass unchanged. `cycle541` asserts disjointness with candidates already given in order; it has not compiled since `fleet.PlanFromTriage` gained arguments (unrelated to P2) and is left as it was. `cycle1181`'s `003` needs a live `.evolve/state.json` and fails in a dev worktree exactly as before P2.
- The archived files keep each predicate's original package clause and body, so they read as they ran. They live outside the Go module and never build.
