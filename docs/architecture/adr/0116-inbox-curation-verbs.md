# ADR-0116 — Inbox items are curated through the CLI; each field has one owner

- **Status:** Accepted (2026-10-01). Revised twice the same day after architecture reviews:
  - the first review (Block, fix then merge) added the one owner table, the continuation-binding and mid-wave refusals, the console guard on `edit`, and the body in the `withdraw` ledger line;
  - the re-review (Warning, fix then merge) made the `withdraw` record precede the delete, and made the stamp keys named constants every writer uses.
- **Related:**
  - [ADR-0074](0074-typed-signal-contracts-routing-authority.md): the claim floor and the operator's route override.
  - [ADR-0079](0079-cycle-outcome-inbox-lifecycle-seam.md): only `internal/inboxmover` imports the lifecycle leaf.
  - [ADR-0089](0089-continuation-retirement-and-live-scope-guard.md): the continuation registry.
  - [ADR-0112](0112-loop-inbox-stamps-land-at-the-sync.md) (PR #758): the loop's own inbox stamps land at the sync, judged by `IsMoverWritten`.
  - The package notes [internal-inboxbatch.md](../packages/internal-inboxbatch.md), [internal-inboxmover-lifecycle.md](../packages/internal-inboxmover-lifecycle.md) and [internal-inboxmover.md](../packages/internal-inboxmover.md).
  - The operator entries in [runtime-reference.md](../../operations/runtime-reference.md).
  - Consumes inbox items `cli-inbox-edit`, `cli-inbox-withdraw`, `cli-inbox-verify` and `cli-inbox-show-list`.

## Context

The operator's rule (2026-09-28) is that every control goes through the published `evolve` CLI. For the inbox, `evolve inbox add` files an item, `route-console` and `route-lane` route one, and `consume` retires one. Nothing else did. Curating an item meant editing its JSON by hand and landing the edit as a PR:

- A reweight, a corrected file list or a new dependency was a JSON edit nothing validated. A typo in `deps` left an item waiting forever, and a malformed file was skipped with only a WARN from `batches`.
- The 2026-09-27 curation stamped ids into five id-less items by hand.
- A mistaken filing was deleted by hand, leaving two `file` ledger lines for one id and no record of the withdrawal.
- A console re-verification of an item's premise had nowhere to be recorded, so its premise drift, measured from the filing time, never reset (inbox `premise-verification-anchor`).
- Reading one item, or listing items by route, kind or status, meant opening the files and filtering them by hand.

Hand edits also let an operator change fields the lifecycle owns: a route stamp, a failure count, a continuation binding. Before this ADR, which fields the lifecycle owns was stated in three places: `add`'s refusal list (exact keys plus the `routed_` and `retired_` prefixes), the console classifier's skip (four keys plus the `routed_` prefix), and, in PR #758, `IsMoverWritten`. Each list was open in a different way.

## Decision

1. **Five verbs complete the operator's surface.**
   - `evolve inbox show` and `list` read.
   - `edit`, `withdraw` and `verify` write. They are `lifecycle.Mover` transitions reached through `internal/inboxmover` facades, so they use:
     - the pending-item locator the route verbs use (`locatePending`: an id in the inbox root, never under a lane's claim);
     - the one item writer (`rewriteItemJSON`, behind `UpdateItemJSON`);
     - the chained `inbox-lifecycle` ledger line.
2. **One table owns every field of an item.** `inboxbatch.RoleOf(key)` reads one table in `inboxbatch/fieldowner.go`. That package is home to `inboxbatch.Item`, the item schema, and the lifecycle leaf imports it, so the table has one home and no import cycle. Every key has exactly one of four owners, and the stamps are listed by exact key, never by prefix:

   | Owner | Keys | Written by |
   |---|---|---|
   | author | anything not listed (`kind`, `created_at`, `injected_by`, `source`, …) | `evolve inbox add`, at filing |
   | curator | `id` (only on an item that has none), `title`, `summary`, `fix`, `priority_class` (text); `weight` (a number); `acceptance`, `files`, `deps`, `connects_to` (lists) | `add` at filing, then `evolve inbox edit` |
   | loop stamp | `route`, `routed_reason`, `routed_cycle`, `routed_at`, `failure_count`, `last_failure_reason`, `continuation`, `released_continuations`, `consumed`, `unbacked`, `retired_reason`, `retired_cycle`, `git_sha` | the loop's mover: route verbs and closeouts, the failure drain, the continuation stamp and release, consumption, `RetireUnbacked` |
   | operator stamp | `premise_verified_at`, `premise_verified_sha`, `premise_verified_evidence` | `evolve inbox verify` |

   Each rule that needs ownership is derived from the table:
   - **What `add` and `withdraw` treat as written by a verb** (`isLifecycleOwned`): every stamp except `route`. An author may file an item straight to the console (`add` judges an authored `route` separately), and every verb that writes `route` also writes `routed_*`.
   - **What `edit` may write:** the curator's keys, with the shape the table gives each one.
   - **`IsMoverWritten`:** the loop stamps only, with the signature PR #758 introduced. #758's sync commits or replays a plane-side change that touches only these keys, so a `verify` stamp in the plane is "other dirt" to it. It lands through a reviewed data PR, never as the loop's stamp.
   - **The console classifier's skip:** every stamp. A stamp is verb-written text, never the item's own, so evidence naming a protected file never changes where the item routes. Moving the skip onto the table changed the placement of none of the 242 pending items (compared before and after with `evolve inbox list --json`).

   Each stamp key is an exported constant of `inboxbatch` (`RouteField`, `RoutedReasonField`, …, `PremiseVerifiedEvidenceField`). The table is keyed by those constants, and every writer spells a stamp only through them: the lifecycle leaf, the host's continuation retire, the ship phase's consumption and `evolve inbox consume`. A parse of `fieldowner.go` requires the constants to name exactly the stamps. Tests keep the table true to the writers. They drive `RouteConsole`, `RouteLane`, the failure bump (with and without the continuation shed), `RetireUnbacked` and `VerifyPremise`, and require every key each one writes to carry the owner the table gives it.
3. **An edit is judged on what it writes, and never opens an item to lanes.**
   - The edited item must still load (`inboxbatch.Item`).
   - Each field the edit writes must pass the rule `add` applies to that field: the same `itemFieldRules` table, including the loader's length and control-character bound.
   - A new id must be kebab-case and unfiled anywhere in the tree, and an edited `deps` must name filed items other than the item itself.
   - Fields the edit does not touch are not judged. 104 of the 245 pending items fail some rule `add` applies today (a long title, no summary, no acceptance), and a whole-item check would make every one of them uneditable.
   - **The console guard.** An item the claim floor holds on the console must still be held there after the edit. `inboxbatch.ConsoleRouted`, with the Mover's lane predicate (the console arm of `PlaceOnLaneMenu`), judges the item before and after, inside the same read-judge-write. Doing this inside the leaf means a refused edit never touches the file. Removing a protected file from `files`, or declaring a surface over a protected mention, is therefore `ErrConsoleRouted` (exit 1), and the message points to `route-lane`, the one verb that may open an item to lanes.
4. **A withdrawal undoes a filing.** `withdraw` removes a pending, unclaimed item and refuses (`ErrNotWithdrawable`) an item:
   - that carries a verb-written stamp;
   - that a pending or claimed item names in `deps`;
   - that a continuation binds in the registry (`evolve continuation release <id>` first). A withdrawn item would leave a binding that no retirement path releases. The host injects the registry read (`WithBinding`), whose default refuses.

   It deletes the file rather than archiving it, because `add`'s identity check covers the whole inbox tree, and an archived copy would stop the id from being filed again. **For a tracked item, git history is the content record.** For content not yet committed, the `withdraw` ledger line carries the reason, the file's sha256 and its exact bytes. That line is written before the file is removed, through an append that returns its error. If the record cannot be written, `withdraw` fails (exit 2) and the item stays. If the remove fails after the record, a `withdraw-aborted` line follows it.
5. **A premise re-verification is an operator stamp.**
   - `verify <id> --evidence T` writes `premise_verified_at`, `premise_verified_sha` (origin/main's head, resolved by the host) and `premise_verified_evidence`, plus a `verify-premise` ledger line whose git head is that sha.
   - Reading the stamp in the drift computation is left to `premise-verification-anchor`.
6. **The writers refuse mid-wave.** Each item is a tracked file, and a mid-wave write to one has bounced a passed audit's tree binding (cycle 1701). `edit`, `withdraw` and `verify` therefore refuse (exit 1, nothing written) while any run lease under `.evolve/runs/` is live. This is `runlease.LiveRuns`, the check `evolve loop-stop --wait` makes: a fresh heartbeat and a live owner process. `show` and `list` stay safe mid-wave.
7. **The read verbs share the claim floor's placement.** `show` and `list` load through `inboxbatch.LoadDir` and place each item with `inboxmover.PlaceOnLaneMenu`, the rule `PartitionLaneMenu` applies for `batches`, using the same lane-forbidden predicate. `list --status console` therefore returns exactly the items `batches` reports as operator-owned. The status names and every verb's usage line come from one table in `cmd_inbox.go`.

## Alternatives considered

| Alternative | Why it was not chosen |
|---|---|
| A denylist for `edit` (anything not lifecycle-owned) | An allowlist names the owner of every field an edit may write. `kind` is the item's classification: the triage archetype and the `pipeline-*` console derivation read it. `injected_by` is who filed the item, and it clamps an agent's route override. Changing either is re-filing, not curation: withdraw the item and file it again. The console guard would catch the routing effect of such a change, but not the change of meaning. |
| Keep the stamp lists as prefixes (`routed_`, `retired_`, `premise_verified_`) | A prefix owns keys no writer writes. An authored `routed_note` would be hidden from the classifier and refused at filing. Exact keys make the table checkable against its writers. |
| Put the owner table in `lifecycle` | `inboxbatch`'s classifier needs the stamp set, and `lifecycle` already imports `inboxbatch`, so the table there would need a second copy or an import cycle. |
| Judge the whole edited item with `add`'s checks | 104 of 245 items would be uneditable for reasons the edit does not touch. |
| Compare `PlaceOnLaneMenu` in the facade, after the write | A refused edit would be written and then reverted. The leaf judges the console arm inside the one read-judge-write, before anything is written. The waiting arm (dependencies) never changes who owns an item. |
| Move a withdrawn item to `withdrawn/` | `add` refuses any id held anywhere in the tree, so the id could never be filed again, and that is the reason to withdraw. |
| Let `withdraw` leave dependents or a continuation binding | A dependency naming no item resolves as "unknown", which the dispatch gate treats as satisfied. A binding whose scope has no item is released by nothing. |
| Resolve the verified sha or the binding in the leaf | The leaf never imports `gitexec` (`TestLifecycle_ImportAllowlist`), and it knows the inbox dir, not the project root. The host injects both readers (`WithMainHead`, `WithBinding`), as it injects the landing probe. |

## Consequences

- Every curation an operator used to do by hand now has a verb that validates it and writes a ledger line. Hand-editing item JSON is no longer part of the interface.
- `add` now also refuses the three `premise_verified_*` keys at filing. An authored key that merely starts with `routed_` or `retired_` is now authored text, which `add` accepts and the classifier reads.
- **Interaction with PR #758.** #758 lands first, and this branch then rebases onto main. The rebase plan:
  - Take this branch's `lifecycle.RouteField` (now `= inboxbatch.RouteField`) and its table-derived `lifecycle.IsMoverWritten` (the same signature). Drop #758's copies in `lifecycle/file.go`, and its `"route"` → `RouteField` edits in `route.go`, which this branch already makes.
  - Keep #758's host facade `inboxmover.IsMoverWritten` (its sync's caller). It delegates to the leaf, so it reads this branch's projection. Until #758 lands, `IsMoverWritten` has no production caller in this branch.
  - In #758's `TestIsMoverWritten_CoversTheRouteAndEveryLifecycleField`, change the `retired_at` row to `retired_reason`. No writer writes `retired_at`, and under exact keys that row reads false.
- The item writer still rewrites the whole file as compact JSON with HTML escaping, so a curation's data-PR diff is the whole file. The route verbs already behave this way; compare fields, not lines.
- The recurrence engine (`internal/recurrence/apply.go`) reweights inbox items at the boundary through its own writer, although the table gives `weight` to the curator. #758's sync would treat such a plane-side reweight as other dirt. This is filed as a follow-up.

## Evidence

- Tests, red first: the `TestMover_Edit_*`, `TestMover_Withdraw_*` (including `TestMover_Withdraw_AFailedRecordKeepsTheItem`, red on the first fix round's code) and `TestMover_VerifyPremise_*` leaf tests, `inboxbatch`'s `TestStampFieldConstants_NameEveryStampAndNothingElse`, `TestIsMoverWritten_IsTheLoopStampsOnly` and the two `TestFieldOwners_*` writer pins; the host tests `TestEdit_*`, `TestWithdraw_*` (including the reviewer's inverted binding probe, `TestWithdraw_RefusesAnItemAContinuationBinds`) and `TestVerifyPremise_*`; `inboxbatch`'s `TestRoleOf_EveryItemFieldHasOneOwner`, `TestFieldsOwnedBy_ListsEachOwnersExactKeysSorted` and `TestConsoleRouted_NoStampIsTheItemsText`; the CLI tests `TestCmd_InboxShow_*`, `TestCmd_InboxList_*`, `TestCmd_InboxEdit_*`, `TestCmd_InboxWithdraw_*`, `TestCmd_InboxVerify_*`, `TestCmd_InboxCurationVerbs_RefuseWhileALoopLaneIsLive`, `TestCmd_Inbox_EveryUsageLineIsItsVerbsSynopsis` and `TestCmd_Inbox_HelpNamesEveryVerbAndAnUnknownVerbIsAUsageError`.
- A live run of the built binary on a copy of the 245-item inbox exercised each verb and its refusals. The placement of all 242 pending items is unchanged by the table-derived skip.
