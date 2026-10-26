# ADR-0121 — Inbox priority is a computed rank

- **Status:** Accepted (2026-10-06, console lane `feat/inbox-priority-rank`). P1 of the approved plan [inbox prioritization](../../plans/inbox-prioritization-2026-10.md) lands with this ADR: the rank, its policy block, the validated class and the read-only verb. It is **unwired**: no consumer reads the rank until P2. The operator reviews the new order in [the preview report](../../reports/inbox-rank-preview-2026-10-06.md).
  - **Amended 2026-10-27 (P3 revision):** the add-time intake and the inbox curator are deterministic. See [the P3 amendment](#amendment-2026-10-27-p3-is-a-deterministic-intake-and-an-event-driven-curator) below.
  - **Amended 2026-10-06 (P2 prerequisite and P2, console lane `feat/inbox-rank-p2`):** the rank is **wired**. Every inbox consumer reads `inboxrank.Order`, and every autofiler stamps a `priority_class`. See [the P2 amendment](#amendment-2026-10-06-p2-prerequisite-and-p2-wire-the-rank) below.
- **Number.** In-flight work (`one-routing-table-one-resolver`, the CLI routing lane) holds 0119. 0120 is reserved for in-flight work. Thus this ADR takes 0121, the number that the plan names.
- **Supersedes nothing.** It replaces the hand-typed `weight` as the sole ordering key once P2 wires it; the weight stays, as one input.
- **Related:**
  - [ADR-0116](0116-inbox-curation-verbs.md): the field-owner table, in which `priority_class` is a curator field.
  - [ADR-0106](0106-logic-first-delivery.md) W3: the ready, console and waiting lane-menu lists the rank keeps separate.
  - [ADR-0074](0074-typed-signal-contracts-routing-authority.md): the console-routing claim floor, unchanged.
  - The package notes [internal-inboxrank.md](../packages/internal-inboxrank.md), [internal-policy.md](../packages/internal-policy.md), [internal-inboxbatch.md](../packages/internal-inboxbatch.md) and [internal-inboxmover-lifecycle.md](../packages/internal-inboxmover-lifecycle.md).

## Context

On 2026-10-06 the operator asked if the inbox reprioritizes when a new request is added. It does not. The investigation (the plan's Context table) found:

- **The order is one hand-typed number.** Every lane consumer sorts by `weight`, highest first (`triagecap.RankForDispatch`, `inboxbatch.Classify`). 33 items were tied at 0.5 and 21 at 0.4, so file-name order decided most of the queue.
- **The class was never read.** Every item carries a `priority_class`, but no `Item` field decoded it. The three security items averaged a weight of 0.37 and the hygiene items 0.49, but the order did not act on a class either way.
- **Recurrence wrote weights behind the writer's back.** The dormant recurrence escalation rewrites an item's `weight` directly, records no reason, and bypasses the lifecycle writer.
- **Nothing else counted.** Three kinds of item sorted by their weight alone. These are an item that unblocks others, an item that has waited for months, and an item in the campaign that the operator drives.

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

2. **The weights are configuration.** The `.evolve/policy.json` block `inbox_priority` holds these keys:
   - `class_order` and `factors {base, class, unblocks, recurrence, age, goal}`;
   - `age_halflife_days`, `unblocks_cap` and `recurrence_cap`;
   - `active_campaigns` and `preempt_margin` (P4 reads it).

   An absent block is the compiled default. The checked-in file names the operator's class order and the factor weights explicitly. Thus they live in config. The block decodes strictly, as `cli_routing` does. `policy.Load` fails and names the block for each of these errors:
   - an unknown key, or a factors block that omits a factor;
   - an empty or duplicated class order;
   - a negative weight, or all-zero factors;
   - a non-positive half-life, a cap below 1 or a negative margin.

3. **The class order is correctness first and security last.** The order is this:

   correctness > stability > performance > debuggability > feature > maintainability > hygiene > security

   The plan approved on 2026-10-06 put security first ("pipeline health first"). The operator corrected it the same day to this order (operator correction, 2026-10-06). Five places carry the corrected order:
   - the compiled default and the checked-in policy;
   - the tests and the order golden;
   - the preview report.

   The class still decides near-ties only (decision 4). A security item scores 0.025 below an otherwise equal hygiene item. A weight about 0.056 higher overturns that difference. Thus a filer who judges a security item urgent says so with its weight.

4. **The default factors keep the filer's weight dominant.** base 0.45, class 0.20, unblocks 0.15, recurrence 0.10, age 0.05, goal 0.05. They sum to 1, so a score is in [0, 1].
   - **base 0.45** stays the largest. The reason is that the weight is still the only per-item judgment of value. One class step (0.025) equals a weight difference of about 0.056. Thus the class reorders items of similar weight, but it does not override a clear difference in value.
   - **class 0.20** is large enough for two effects. The class order decides near-ties, and it outweighs a moderate weight gap between distant classes. Example: a correctness item (the first class) at weight 0.35 outranks an otherwise equal security item (the last) at any weight up to 0.73. Between adjacent classes (one step, 0.025), a weight gap above about 0.056 still wins.
   - **unblocks 0.15** rewards an item that releases work that waits. At the cap of 3 dependents, it is worth a third of the full weight range.
   - **recurrence 0.10** revives the dormant recurrence signal as a rank input, capped at 5 occurrences. It never writes a weight.
   - **age 0.05** with a 30-day half-life is anti-starvation only. It breaks ties among old items and lets an item that waited long move up slowly. It cannot overturn a judged difference.
   - **goal 0.05** lets the operator steer the loop toward a campaign without editing weights; it is inert while `active_campaigns` is empty.

   Measured on the 241 pending items of 2026-10-06, 37 distinct weights become 214 distinct scores; the 33 items tied at 0.5 get 31.

5. **Ties break on facts, then identity.** The keys are, in sequence:
   1. the score, descending;
   2. a declared fix surface first (as `RankForDispatch` does today);
   3. the older filing date, with an undated item after every dated one;
   4. the id;
   5. the file name.

   The order is total. Thus `inboxrank.Order` returns the same order for any input order.

6. **The class is a validated enum.** `inboxbatch.Item.PriorityClass` decodes `priority_class`.
   - `inboxbatch.PriorityClassPosition` is the one membership rule. It gives the class's index in `class_order`, or -1 when absent.
   - `inboxbatch.CheckPriorityClass` validates with it. The rank scores with it (`inboxrank.Facts.ClassPosition`). Thus validation and the score cannot disagree.
   - `evolve inbox add` refuses an item whose class is absent or not in the policy's `class_order` (exit 1).
   - The refusal goes through the lifecycle filer. The filer takes the order from `inboxmover.Options.PriorityClasses`.
   - A filer wired with no order refuses as a fault, not as an invalid item.
   - At load, an existing item whose class the order lacks ranks below every class (feature 0). It never gets a silent default.
   - A loud warning names that item (`inboxrank.ClassWarnings`; `evolve inbox rank` prints `WARN <id>: unknown priority_class …`).

   All 241 pending items carried one of the eight classes on 2026-10-06.

7. **Exclusions are unchanged.** The ready, console and waiting lists stay separate (`inboxmover.PartitionLaneMenu`), and each list is ranked within itself. `unblocks` counts dependents across the whole queue, since a waiting item is what a ready item unblocks.

8. **`evolve inbox rank [--list ready|console|waiting|all] [--top N] [--explain <id>] [--json]`** is read-only.
   - Its table shows each item's rank, score, the six contributions, id and title.
   - `--explain` prints each factor's value, weight, contribution and the fact behind it.
   - The contributions sum to the score exactly.
   - The table's factor columns come from `inboxrank.Factors()`, the order of the score terms.
   - It reads the recurrence ledger with `recurrence.ReadSnapshot`, which takes no lock. Thus it never creates a file.
   - It passes `recurrence.Ledger.ItemCounts()` to the rank.
   - `internal/inboxrank` itself does not import the recurrence package.

## Consequences

- P2 replaces `RankForDispatch`, the `Classify` cluster order and the wave seed and refill paths with `inboxrank.Order`, and a source scan bans any other weight sort. Until then the rank is only shown. *(Landed 2026-10-06; see the P2 amendment below.)*
- The class labels now matter. The preview shows the 2026-09-26 hygiene sweep's items falling about 20 places, several of which describe silent failures. The operator's answer (2026-10-06): they are not relabelled automatically; the operator re-classes an item with `evolve inbox edit --set priority_class=… --reason …`. The P3 overlap judge was dropped on 2026-10-27 (see the P3 amendment).
- `evolve inbox add` needs a valid `priority_class` on every new item.
- The recurrence factor reads an item's pattern in one of two forms. One is a ledger pattern equal to its id (an autofiled recurrence item). The other is a pattern whose `fix_item_id` names it. The recurrence package owns that linkage (`recurrence.Ledger.ItemCounts`). Items carry no fingerprint field yet.
- **Autofiled items carry no class yet (found in the P1 review, 2026-10-06).** Eight production autofilers write pending items without a `priority_class`:
  - the post-push CI watch (`ci-red-*`, weight 0.95);
  - the recurrence boundary applier's retro autofile (`retrofile`, which files the intents that the disposition router stages);
  - the fleet starvation observer and the fail-learning floor;
  - the triage-cap demotion and the goal-stall escalation;
  - the ADR-0072 halt writer and the unexplained-outcome classifier.

  None files through `lifecycle.File`, so decision 6's check never sees them. When P2 wires the rank, they score a class feature of 0 and sink below mid-weight items. Two examples:
  - A fresh `ci-red` item at weight 0.95 scores 0.4275, below a fresh correctness item at weight 0.6 (0.47).
  - A fail-learning defect at 0.75 (0.3375) ranks below a stability item at 0.5 (0.40).

  P1 stays unwired and does not change the writers.

  The fix is a P2 prerequisite. Every autofiler stamps a `priority_class` from the policy order. The writer chooses the class for the kind of finding it files, as it chooses the weight today. A contract test requires that every inbox writer's item carries a class that the checked-in order names. The plan lists the proposed class per writer. *(Fixed 2026-10-06 by the P2 prerequisite; see the P2 amendment below.)*

## Amendment 2026-10-06: P2 prerequisite and P2 wire the rank

Landed on `feat/inbox-rank-p2` from `origin/main` `2f2cefdeb`. The details, the choices and the live check are the plan's [P2 landing notes](../../plans/inbox-prioritization-2026-10.md#p2-prerequisite-and-p2-landing-notes-2026-10-06).

1. **Every autofiled item carries a class** (the P2 prerequisite). Each writer stamps the class of the finding that it files. The class goes beside the kind and the weight that it already chooses. The policy still owns how much a class matters.

   | Writer | Item | Class |
   |---|---|---|
   | Post-push CI watch (`ciwatch`) | `ci-red-<sha12>` | correctness |
   | Contract-gate demotion, filed by the recurrence boundary applier through `retrofile` | `contract-gate-demoted-<phase>` (the class rides on the staged `dispositionrouter.Intent`, built by `core.ContractGateDemotion.Intent`) | correctness |
   | Fleet starvation observer (`fleet`) | `fleet-work-supply-starvation` | stability |
   | Fail-learning floor (`core/failurelearning`, written by `faillearn`) | `retro-<cycle>-<slug>-<sha8>` | correctness |
   | Triage-cap demotion (`triagecap`) | `auto-heuristic-demotion-triagecap-c<a>-c<b>` | correctness |
   | Goal-stall escalation (`cmd/evolve`) | `goal-stall-<hash8>` / `nonprogress-<hash8>` | stability |
   | ADR-0072 halt writer (`cmd/evolve`) | `pipeline-defect-<category>-cycle<N>` | stability |
   | Unexplained-outcome classifier (`cmd/evolve`) | `unexplained-outcome-cycle-<N>` | debuggability |

   `TestInboxWriters_EveryAutofiledItemCarriesAKnownClass` (cmd/evolve) does three things for each item:
   - It builds the item through its writer's own path.
   - It decodes the item as an `inboxbatch.Item`.
   - It requires the row's class and `CheckPriorityClass` against the checked-in `class_order`.

   Its scan pins the force, not a list. Any production function that writes into `.evolve/inbox/` must be an enrolled autofiler. Alternatively, it must be an enrolled writer that only moves or rewrites filed items. Thus a ninth autofiler is red.
2. **One order feeds every consumer** (P2).
   - The wave seed, the widen and the launch refill read `triagecap.ReadInboxBacklog`.
   - `triagecap.ReadInboxBacklog` returns the lane-ready backlog in `inboxrank.Order` against the whole pending queue.
   - `RankForDispatch` is deleted.
   - `inboxbatch.Classify` takes the order through an injected `Config.Order`, because it cannot import the rank.
   - `inboxbatch.Classify` runs the order once. It ranks clusters by their best-ranked member and keeps members in rank order.
   - With no order, `inboxbatch.Classify` keeps the caller's order and never ranks by weight.
   - The triage prompt and `evolve inbox batches` inject `Sequence` of one `Inputs.Order` run.
   - They print each item's score and top factor (`Labels`).

   The dashboard's queue is ranked too. Every consumer loads its inputs through one loader, `internal/inboxrank/rankinputs`.
3. **No other weight sort.** `TestNoProductionCodeOrdersByWeightOutsideTheRank` scans the module's production code.
   - It fails on any order by an inbox weight outside `internal/inboxrank`.
   - Weight validation against a floor stays allowed.
   - `./internal/inboxrank/...` joined the ship-time repo-contract pack. Thus a lane cannot land a stray weight sort.
4. **Superseded acceptance.** Three per-cycle ACS predicates asserted the weight order (`cycle536` 007 and 008, `cycle1724` 002). They are archived under the A2 retention rule ([superseded predicates](../../private/research/archived-2026-10-06/superseded-predicates/README.md)).

5. **The architecture review's fix round (2026-10-06).**
   - The partition-then-rank step is one helper, `inboxmover.RankLaneMenu`, that every consumer calls.
   - The shared funnels refuse a missing class: `dispositionrouter.StageIntent`, `retrofile.FileActions` and faillearn's inbox writer. A legacy classless intent is refused at apply, never filed.
   - The wave planner's loop log and the triage prompt name every queued item with an unknown class. This is the runtime backstop for the force scan's blind spots.
   - The class names are constants beside `inboxbatch.CheckPriorityClass`.
   - The review also judged that the rank can starve the later classes by design. That stays an open question for the operator ([the plan's P4/P5 question](../../plans/inbox-prioritization-2026-10.md#open-question-for-p4-and-p5-starvation-by-design)). The rank is unchanged.

Still to come: P3 (one intake path; revised on 2026-10-27, see the amendment below), P4 (boundary preemption), P5 (a reason on every priority change).

## Amendment 2026-10-27: P3 is a deterministic intake and an event-driven curator

The operator revised P3 on 2026-10-27. Duplicates and wrong routes entered at add time unchecked, and the inbox had no review.

- **The plan's operator decision 3 changes** (overlap at add time depends on origin). No LLM judges an overlap. A shared fix surface plus a token Jaccard of the title and the acceptance decides it, with the thresholds in the strict policy block `inbox.curator`. `evolve inbox add` refuses an overlap at or above the refuse threshold, unless the operator gives `--allow-overlap --reason`. The origin no longer selects a judge. It is a validated fact that `lifecycle.File` stamps on every item.
- **A curator joins the rank.** `evolve inbox curate` proposes to consume an item whose acceptance merged, and to reroute an item whose route no longer matches the protected manifest. It runs on `ship.landed`, on a policy change and at the wave boundary. In the `enforce` stage it applies only the proposals that Go can verify.
- **Consequence.** A judgment case (a merge, a split, a re-scope) is a typed proposal for the operator and the orchestrator. The curator never applies it.

The spec is [inbox-curator.md](../inbox-curator.md). The plan revision is [P3 revision](../../plans/inbox-prioritization-2026-10.md#p3-revision-2026-10-27-a-deterministic-intake-and-an-event-driven-curator).

## Alternatives considered

- **Keep sorting by weight and re-weight on every add.** Rejected: it multiplies hand-typed numbers without recording why, and it is the path the recurrence escalation took.
- **Lexicographic order (class first, then weight).** Rejected. With this order, a correctness item at weight 0.05 outranks every stability item, and no other fact can move an item.
- **Normalize unblocks and recurrence against the queue's maximum.** Rejected. With this rule, one new item changes every other item's score. A fixed cap keeps an item's score a function of its own facts.
- **Pin the order with a golden over the live tracked inbox.** Rejected. Every lane ship consumes an item, so with this golden, the next ship makes main red. The golden pins a fixed 241-item snapshot (`internal/inboxrank/testdata`); the live order is `evolve inbox rank` and the preview report.
