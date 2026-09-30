# ADR-0112 — The loop's inbox stamps stay in the tracked item and land at the sync

- **Status:** Accepted (2026-10-01)
- **Resolves:** inbox item `console-route-stamps-block-wave-sync` (F30 architecture review M3). **Related:** [ADR-0074](0074-typed-signal-contracts-routing-authority.md) (routing authority, which the route stamp records), [ADR-0081](0081-boot-base-divergence-halt-and-in-band-ledger-seal.md) (base divergence), the procedure in [runtime-reference](../../operations/runtime-reference.md) ("The console route at a boundary").

## Context

A cycle that does not ship still writes to the inbox item it worked on: a route to the console when triage finds the pick on a protected surface, or a failure-count bump. Those writes land in the plane's working tree, in tracked files, and nothing commits them. At the next boundary `evolve sync-main` refused to run on the dirty tree, and the loop's own wave sync could not fast-forward over a later origin change to the same item. The operator had to copy each stamp into a data PR by hand (PR #752 was one) and discard the plane copy with plain git, every wave that routed an item.

Two designs were open, and the inbox item named both:

1. keep route stamps out of tracked files, in a gitignored sidecar that the console partition and the claim floor read; or
2. keep them in the tracked item and give landing and discarding them a CLI home.

## Decision

The stamp stays in the tracked item, and one rule, `internal/inboxstamps`, decides its fate at a sync.

**What a stamp is.** A tracked inbox item modified in the plane is a stamp only when every key that changed, compared as decoded JSON values (the mover re-encodes items, escaping `<`, `>` and `&`), is one the mover writes: `route` or a lifecycle-owned field (`consumed`, `failure_count`, `routed_*`, `retired_*` and the rest). The rule is `inboxmover.IsMoverWritten`, a facade over the lifecycle leaf's own list. Any other change is other dirt: an operator's hand edit, a deleted or added item, any tracked file outside the inbox.

**What happens to it**, judged against the merge base of the plane and origin, so the plane's own unpublished commits never count as origin's:

| Origin since the merge base | At `evolve sync-main` | At the loop's wave-boundary sync |
|---|---|---|
| did not change the item's bytes (`git diff HEAD...origin`), or the item is newer than the merge base | committed in the plane (`chore(inbox): the loop's inbox stamps from the plane`); the next lane ship publishes it | left in the tree |
| retired the item (removed or moved it) | dropped, by name: origin's decision wins | dropped, by name |
| edited the item, including only reformatting it | restored before the merge, then its fields are replayed onto origin's version and committed (`… replayed onto origin's edits`); a field origin also changed keeps origin's value | restored before the fast-forward, then replayed, uncommitted, until `evolve sync-main` |
| changed every field the stamp sets (it landed the stamp by hand, or set its own values) | restored and reported as superseded by origin: origin's values stand, nothing committed | the same, uncommitted |

Other dirt refuses `evolve sync-main` untouched, as before, and the wave sync then leaves every stamp alone. When a step fails after stamps were restored, the plan puts them back (`Plan.Restore`), so no failure path loses the loop's write. The wave sync never commits: a commit there would turn "behind origin" into "diverged", which halts the loop by design (ADR-0081).

## Why not a sidecar

- **One fact, one home.** An item's route already lives in the item: the operator's `route-lane` and `route-console` decisions are committed there and reviewed in PRs. A sidecar would split the same fact between a shared tracked file and a local untracked one, and every reader (the dispatchability rule, triage's menu, `evolve inbox batches`) would have to merge the two.
- **A route is shared, a sidecar is local.** A stamp on origin is visible to every plane and to the console that acts on it; a sidecar in one plane is invisible to the rest.
- **The commit path already exists.** The dossier closeout commits in the plane and the next lane ship publishes it; the stamp commit rides the same path.

## Consequences

- A boundary no longer needs raw git or a data PR for the loop's own inbox writes: `evolve sync-main` lands them.
- When the console retires an item the loop just stamped, the stamp is dropped, by name. When the console edits it, the stamp's fields are replayed onto the console's version, and only a field both changed keeps the console's value.
- Plane-local commits (dossier closeouts, stamp commits, sync merges) reach origin only with the next lane ship, as before.
- Not done here, filed separately: carrying the landing probe's result in `routed_reason` for skip-shipped and close-class routes (the item's m7).

## Evidence

- `internal/inboxstamps`: `TestClassify_*`, `TestPlanAgainst_*` (including the ahead plane and an item added since the merge base), `TestPlan_*` (landing, a staged stamp, restore) and `TestChangedOnRemote_*`, on real repositories built with `internal/gittest`; a mutation sweep killed all seven mutants, each by a named test.
- Wiring: `TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync`, `TestSyncMain_AnOperatorEditToAnInboxItemStillRefuses` and `TestSyncMain_RefusesToMergeWhenTheStampsCannotLand` drive `evolve sync-main`; `TestSyncMainAtWaveBoundary_DiscardsInboxStampsOriginSupersededAndFastForwards` and `TestSyncMainAtWaveBoundary_WarnsAndKeepsTheStampsWhenTheyCannotBePrepared` drive the wave sync.
