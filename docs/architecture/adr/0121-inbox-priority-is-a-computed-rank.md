# ADR-0121 — Inbox priority is a computed rank

- **Status:** Accepted (2026-10-06, console lane `feat/inbox-priority-rank`). P1 of the approved plan [inbox prioritization](../../plans/inbox-prioritization-2026-10.md) lands with this ADR: the rank, its policy block, the validated class and the read-only verb. It is **unwired**: no consumer reads the rank until P2. The operator reviews the new order in [the preview report](../../reports/inbox-rank-preview-2026-10-06.md).
- **Number.** 0119 is held by in-flight work (`one-routing-table-one-resolver`, the CLI routing lane) and 0120 is reserved for in-flight work, so this ADR takes 0121, the number the plan names.
- **Supersedes nothing.** It replaces the hand-typed `weight` as the sole ordering key once P2 wires it; the weight stays, as one input.
- **Related:**
  - [ADR-0116](0116-inbox-curation-verbs.md): the field-owner table, in which `priority_class` is a curator field.
  - [ADR-0106](0106-logic-first-delivery.md) W3: the ready, console and waiting lane-menu lists the rank keeps separate.
  - [ADR-0074](0074-typed-signal-contracts-routing-authority.md): the console-routing claim floor, unchanged.
  - The package notes [internal-inboxrank.md](../packages/internal-inboxrank.md), [internal-policy.md](../packages/internal-policy.md), [internal-inboxbatch.md](../packages/internal-inboxbatch.md) and [internal-inboxmover-lifecycle.md](../packages/internal-inboxmover-lifecycle.md).

## Context

On 2026-10-06 the operator asked whether the inbox reprioritizes when a new request is added. It does not. The investigation (the plan's Context table) found:

- **The order is one hand-typed number.** Every lane consumer sorts by `weight`, highest first (`triagecap.RankForDispatch`, `inboxbatch.Classify`). 33 items were tied at 0.5 and 21 at 0.4, so file-name order decided most of the queue.
- **The class was never read.** Every item carries a `priority_class`, but no `Item` field decoded it. The three security items averaged a weight of 0.37 and the hygiene items 0.49, but the order could not act on a class either way.
- **Recurrence wrote weights behind the writer's back.** The dormant recurrence escalation rewrites an item's `weight` directly, records no reason, and bypasses the lifecycle writer.
- **Nothing else counted.** An item that unblocks others, an item that has waited for months, and an item in the campaign the operator is driving all sorted by their weight alone.

## Decision

1. **Priority is a deterministic projection of item facts plus operator policy.** `score(item) = Σ factor_k × feature_k(item)`, computed in Go by `inboxrank.Score`, with every feature normalized to [0, 1]:

   | Feature | Value | Source |
   |---|---|---|
   | `base` | the filer's `weight`, clamped to [0, 1] | the filer's judgment |
   | `class` | `(n − i) / n` for the class at position `i` of the `n` in `class_order`; 0 for a class the order lacks | the operator's class order |
   | `unblocks` | `min(d, unblocks_cap) / unblocks_cap`, where `d` counts the distinct queued items whose `deps` reach this item, transitively | the queue |
   | `recurrence` | `min(r, recurrence_cap) / recurrence_cap`, where `r` is the recurrence-ledger count of the item's failure pattern | `.evolve/recurrence-ledger.json` |
   | `age` | `1 − 0.5^(days / age_halflife_days)` since the item's filing date; 0 when it has none | `created_at`, else the file-name stamp |
   | `goal` | 1 when the item's `campaign` is in `active_campaigns` | the operator's steering |

   Judgment stays where it belongs (agent rule 5): the filer supplies the weight and the class; the operator supplies the policy; the arithmetic is code.

2. **The weights are configuration.** `.evolve/policy.json` `inbox_priority` holds `class_order`, `factors {base, class, unblocks, recurrence, age, goal}`, `age_halflife_days`, `unblocks_cap`, `recurrence_cap`, `active_campaigns` and `preempt_margin` (read by P4). An absent block is the compiled default; the checked-in file names the operator's class order and the factor weights explicitly, so they live in config. The block decodes strictly, following `cli_routing`: an unknown key, a factors block that omits a factor, an empty or duplicated class order, a negative weight, all-zero factors, a non-positive half-life, a cap below 1 or a negative margin fails `policy.Load` naming the block.

3. **The class order is correctness first and security last:** correctness > stability > performance > debuggability > feature > maintainability > hygiene > security. The plan approved on 2026-10-06 put security first ("pipeline health first"); the operator corrected it the same day to this order (operator correction, 2026-10-06), and the compiled default, the checked-in policy, the tests, the order golden and the preview report all carry the corrected order. The class still decides near-ties only (decision 4): a security item scores 0.025 below an otherwise equal hygiene item, which a weight about 0.056 higher overturns, so a filer who judges a security item urgent says so with its weight.

4. **The default factors keep the filer's weight dominant.** base 0.45, class 0.20, unblocks 0.15, recurrence 0.10, age 0.05, goal 0.05. They sum to 1, so a score is in [0, 1].
   - **base 0.45** stays the largest because the weight is still the only per-item judgment of value. One class step (0.025) equals a weight difference of about 0.056, so the class reorders items of similar weight rather than overriding a clear difference in value.
   - **class 0.20** is large enough that the class order decides near-ties and outweighs a moderate weight gap between distant classes: a correctness item (the first class) at weight 0.35 outranks an otherwise equal security item (the last) at any weight up to 0.73, while between adjacent classes (one step, 0.025) a weight gap above about 0.056 still wins.
   - **unblocks 0.15** rewards an item that releases waiting work; at the cap of 3 dependents it is worth a third of the full weight range.
   - **recurrence 0.10** revives the dormant recurrence signal as a rank input, capped at 5 occurrences, without ever writing a weight.
   - **age 0.05** with a 30-day half-life is anti-starvation only: it breaks ties among old items and lets a long-waiting item creep up, but cannot overturn a judged difference.
   - **goal 0.05** lets the operator steer the loop toward a campaign without editing weights; it is inert while `active_campaigns` is empty.

   Measured on the 241 pending items of 2026-10-06, 37 distinct weights become 214 distinct scores; the 33 items tied at 0.5 get 31.

5. **Ties break on facts, then identity.** Score descending; then a declared fix surface first (as `RankForDispatch` does today); then the older filing date, with an undated item after every dated one; then the id; then the file name. The order is total, so `inboxrank.Order` returns the same order for any input order.

6. **The class is a validated enum.** `inboxbatch.Item.PriorityClass` decodes `priority_class`, and `inboxbatch.PriorityClassPosition` (the class's index in `class_order`, -1 when absent) is the one membership rule: `inboxbatch.CheckPriorityClass` validates with it and the rank scores with it (`inboxrank.Facts.ClassPosition`), so validation and scoring cannot disagree. `evolve inbox add` refuses an item whose class is absent or not in the policy's `class_order` (exit 1), through the lifecycle filer, which takes the order from `inboxmover.Options.PriorityClasses`; a filer wired with no order refuses as a fault, not as an invalid item. At load, an existing item whose class the order lacks is ranked below every class (feature 0) and named in a loud warning (`inboxrank.ClassWarnings`; `evolve inbox rank` prints `WARN <id>: unknown priority_class …`), never given a silent default. All 241 pending items carried one of the eight classes on 2026-10-06.

7. **Exclusions are unchanged.** The ready, console and waiting lists stay separate (`inboxmover.PartitionLaneMenu`), and each list is ranked within itself. `unblocks` counts dependents across the whole queue, since a waiting item is what a ready item unblocks.

8. **`evolve inbox rank [--list ready|console|waiting|all] [--top N] [--explain <id>] [--json]`** is read-only. Its table shows each item's rank, score, the six contributions, id and title; `--explain` prints each factor's value, weight, contribution and the fact behind it; the contributions sum to the score exactly. The table's factor columns come from `inboxrank.Factors()`, the order the terms are scored in. It reads the recurrence ledger with `recurrence.ReadSnapshot`, which takes no lock, so it never creates a file, and passes the rank `recurrence.Ledger.ItemCounts()`; `internal/inboxrank` itself does not import the recurrence package.

## Consequences

- P2 replaces `RankForDispatch`, the `Classify` cluster order and the wave seed and refill paths with `inboxrank.Order`, and a source scan bans any other weight sort. Until then the rank is only shown.
- The class labels now matter. The preview shows the 2026-09-26 hygiene sweep's items falling about 20 places, several of which describe silent failures. The operator's answer (2026-10-06): they are not relabelled automatically; the P3 overlap judge and the operator re-class an item with `evolve inbox edit --set priority_class=… --reason …`.
- `evolve inbox add` needs a valid `priority_class` on every new item.
- The recurrence factor reads an item's pattern as either a ledger pattern equal to its id (an autofiled recurrence item) or a pattern whose `fix_item_id` names it. The recurrence package owns that linkage (`recurrence.Ledger.ItemCounts`). Items carry no fingerprint field yet.
- **Autofiled items carry no class yet (found in the P1 review, 2026-10-06).** Eight production autofilers write pending items without a `priority_class`: the post-push CI watch (`ci-red-*`, weight 0.95), the recurrence boundary applier's retro autofile (`retrofile`, filing the intents the disposition router stages), the fleet starvation observer, the fail-learning floor, the triage-cap demotion, the goal-stall escalation, the ADR-0072 halt writer and the unexplained-outcome classifier. None files through `lifecycle.File`, so decision 6's check never sees them. Once P2 wires the rank they score a class feature of 0 and sink below mid-weight items: a fresh `ci-red` item at weight 0.95 scores 0.4275, below a fresh correctness item at weight 0.6 (0.47), and a fail-learning defect at 0.75 (0.3375) ranks below a stability item at 0.5 (0.40). P1 stays unwired and does not change the writers. The fix is a P2 prerequisite: every autofiler stamps a `priority_class` from the policy order, chosen by the writer for the kind of finding it files, as it chooses the weight today, and a contract test requires every inbox writer's item to carry a class the checked-in order names. The plan lists the proposed class per writer.

## Alternatives considered

- **Keep sorting by weight and re-weight on every add.** Rejected: it multiplies hand-typed numbers without recording why, and it is the path the recurrence escalation took.
- **Lexicographic order (class first, then weight).** Rejected: a correctness item at weight 0.05 would outrank every stability item, and no other fact could move an item.
- **Normalize unblocks and recurrence against the queue's maximum.** Rejected: one new item would change every other item's score. A fixed cap keeps an item's score a function of its own facts.
- **Pin the order with a golden over the live tracked inbox.** Rejected: every lane ship consumes an item, so the golden would red main on the next ship. The golden pins a fixed 241-item snapshot (`internal/inboxrank/testdata`); the live order is `evolve inbox rank` and the preview report.
