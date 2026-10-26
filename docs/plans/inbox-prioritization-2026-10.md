# Inbox prioritization: one computed rank, re-ranked on every add (2026-10)

> **Status:** approved by the operator on 2026-10-06. The request: "Do we have logic to reprioritize the Todo inbox tasks when adding the new requests? If not, we need to build the logic… with ultrathink".
>
> **Decision record:** ADR-0121, which lands with P1.
>
> **Progress:** P1 landed unwired on 2026-10-06 ([notes](#p1-landing-notes-2026-10-06)); the P2 prerequisite and P2 landed the same day and wired the rank ([notes](#p2-prerequisite-and-p2-landing-notes-2026-10-06)). On 2026-10-27 the operator revised P3 into a deterministic intake and an event-driven curator ([the revision](#p3-revision-2026-10-27-a-deterministic-intake-and-an-event-driven-curator)). P4 and P5 remain.

## Context: what existed on 2026-10-06

The investigation recorded these facts, with file:line references in the session report.

| Area | Fact |
|---|---|
| `evolve inbox add` | Validates the item, checks that the id is unique and that its `deps` exist, then writes **one file**. It never reads or changes another item (`inboxmover/lifecycle/file.go:41-64,164-188`). |
| Ranking | A static sort on the hand-typed `weight`, highest first (`inboxbatch/classify.go:114-116`, `triagecap/topn_width.go:99-109`). 33 items are tied at 0.5 and 21 at 0.4. |
| `priority_class` | Never decoded: there is no `Item` field, and it is free text in the owner table. "security" items average a weight of 0.37, below "hygiene" at 0.49. |
| Recurrence reweight | Dormant. Nothing stages `escalate` intents. When it does run, it writes weights directly, bypassing the item writer, and records no reason. |
| Fleet wave plan | Reuses the previous cycle's `triage-decision.json` verbatim (`loopwave/plan.go:17-31`). A newer, more urgent item cannot displace planned lanes. |
| Overlap | None. Only exact-id checks exist. De-duplication has been manual (`docs/reports/inbox-review-2026-09-30.md`). |
| "Why did its priority change?" | Never recorded. The edit ledger stores `set weight=X` with no reason. |
| Doc drift | `runtime-reference.md:180` claims a "dep-topological order" that the code does not implement. The `evolve-triage-reference.md` priority scheme is stale. |

## Principle

Priority is a **deterministic projection of item facts plus operator policy**. Judgment stays where judgment belongs, per rule 5:
- **The filer** supplies the base weight and the class.
- **A deep-model judge** decides overlap for machine-origin items.
- **The operator** decides overlap for human-origin items.
- *(Superseded on 2026-10-27 by the [P3 revision](#p3-revision-2026-10-27-a-deterministic-intake-and-an-event-driven-curator): a deterministic rule decides overlap, and no LLM edits the inbox.)*

The **score** is computed in Go from those facts. Its weights live in policy configuration, not Go literals. One function, `inboxrank.Order`, feeds every consumer.

## Operator decisions (2026-10-06)

1. **Class order: correctness first, security last.** The operator corrected the order on 2026-10-06 (operator correction, 2026-10-06):

   correctness > stability > performance > debuggability > feature > maintainability > hygiene > security

   The order first approved that morning was "pipeline health first":

   security > correctness > stability > performance > debuggability > feature > maintainability > hygiene

   The operator corrected it the same day, before P1 landed. P1 ships the corrected order. CLAUDE.md's "/evo:loop task priority" line (features first) is updated to match.
2. **Preemption.** At a wave boundary, an item takes the lowest **uncommitted** planned slot.
   The item takes the slot when it is lane-eligible and outranks that slot by the policy margin. Running lanes and adopted continuations are never displaced.
3. **Overlap at add time depends on origin.** *(Superseded on 2026-10-27 by the [P3 revision](#p3-revision-2026-10-27-a-deterministic-intake-and-an-event-driven-curator).)*
   - **Machine origin** covers previous cycles (retro, halt, fail-learning, recurrence, carryover) and console-agent findings. A deep-model overlap judge decides supersede, merge-into, relate or distinct. Its reasoning is **recorded** in the item (`overlap_review`) and in the ledger, and the decision is applied through the lifecycle writer.
   - **Human origin** covers the operator's requests. The same judge writes a justification and the options. `evolve inbox add` then **stops**, recording a pending intake, and the console agent **asks the user** with that justification and those options. Nothing is filed until the user chooses.

## Score

`score(item) = Σ factor_k × feature_k(item)`. Each feature is normalized to [0, 1], and the factor weights come from `.evolve/policy.json` `inbox_priority`.

| Feature | Definition | Why |
|---|---|---|
| `base` | The filer's `weight`. | Human or retro judgment of value. |
| `class` | The rank of `priority_class` in the policy order, mapped to [0, 1]. The enum is validated at add time and at load. | Encodes the operator's class order (decision 1, as corrected). |
| `unblocks` | The number of queued items whose `deps` (transitively) include this one, normalized. | An item that unblocks others rises. |
| `recurrence` | The recurrence count of the item's failure fingerprint, from the recurrence ledger, normalized and capped. | Revives the dormant recurrence path as a rank factor. No weight writes, which closes `recurrence-reweight-bypasses-the-item-writer`. |
| `age` | `1 − 0.5^(age_days / halflife)`. | Anti-starvation. |
| `goal` | 1 when the item belongs to an `inbox_priority.active_campaigns` entry. | Lets the operator steer the loop without editing weights. |

**Ties** break on the declared fix surface (as today), then on the older `created_at`.

**Exclusions.** Console-owned and waiting items (unmet `deps`) stay out of the lane order, exactly as today; they are ranked within their own lists.

## Components (each is one commit with its own tests; red first; unwired before wired)

| # | Component | Change |
|---|---|---|
| P1 | **`internal/inboxrank`** (pure, unwired) and **`evolve inbox rank [--json] [--explain <id>] [--top N]`** | <ul><li>Add `inbox_priority` to policy, with strict decode, class order, factor weights, age half-life, `active_campaigns` and preemption margin.</li> <li>Add a `PriorityClass` field on `inboxbatch.Item`, as a validated enum that normalizes `correctness` and `maintainability`.</li> <li>The score and `Order` function.</li> <li>`--explain` prints the factor breakdown.</li> <li>A golden test records today's order and the new order for every root item.</li> <li>The PR shows the top-30 deltas for operator review.</li></ul> |
| P2-pre | **PREREQUISITE of P2: a class on every autofiled item** | <ul><li>Every autofiler stamps a `priority_class` from the policy order. The writer chooses it for the kind of finding it files, as it chooses the weight today. The proposed class per writer is in [the table below](#p2-prerequisite-a-class-on-every-autofiled-item).</li> <li>A contract test requires that every inbox writer's item carries a class that the checked-in `class_order` names. A source scan reds on any production write into `.evolve/inbox/` that the test does not enroll.</li> <li>Lands before P2 wires the rank: without it, eight autofilers' items score a class feature of 0 and sink below mid-weight items (ADR-0121 Consequences).</li></ul> |
| P2 | **One ranking for every consumer** | <ul><li>`RankForDispatch`, `Classify` cluster order, the triage injection, and the wave seed and refill paths all use `inboxrank.Order`.</li> <li>A source-scan guard bans any other weight sort.</li> <li>The stale triage priority prose (`evolve-triage.md`, `evolve-triage-reference.md`) points to the rank.</li></ul> |
| P3 | **The add-time intake** *(Superseded on 2026-10-27 by the [P3 revision](#p3-revision-2026-10-27-a-deterministic-intake-and-an-event-driven-curator).)* | <ul><li>A validated `origin` field (`operator`, `console`, `cycle` or `retro`) that autofilers set; `evolve inbox add --origin` defaults to `console`.</li> <li>Above a policy threshold, a deterministic overlap pre-check (shared `files` and dirs as Jaccard, `connects_to`, title-token similarity) calls the **`inbox-overlap-judge`**. The judge is a new deep-tier Claude-family profile with a JSON contract: verdict, rationale, related ids, confidence; `overlap_review` stores the machine-origin reasoning.</li> <li>Machine origin: the lifecycle writer applies the judge's verdict (`supersedes` → withdraw with reason; `merge_into` → append to the target's acceptance plus a ledger line; `relates` → `connects_to`).</li> <li>Human origin: `add` exits 3 and writes `.evolve/inbox/pending-intake/<id>.json` with the judge's justification and options, and `evolve inbox resolve <id> --supersedes X \| --merge-into X \| --relates X \| --distinct \| --withdraw` finalizes it.</li> <li>Every add prints where the item lands and the top movers.</li></ul> |
| P4 | **Boundary preemption** | <ul><li>When the wave plan reuses the previous decision, re-check it against `inboxrank.Order`.</li> <li>A lane-eligible item whose score beats the lowest uncommitted planned slot by `inbox_priority.preempt_margin` takes that slot.</li> <li>The ledger records `preempted <old> for <new> (score a→b)`.</li> <li>Committed in-flight lanes and adopted continuations are never displaced.</li></ul> |
| P5 | **Reasons on every priority change** | <ul><li>`evolve inbox edit --set weight=… / priority_class=…` requires `--reason`, and the ledger records it.</li> <li>Recurrence becomes a rank factor (P1), so `recurrence.setItemWeight` is deleted.</li></ul> |
| P6 | **Docs** | <ul><li>ADR-0121.</li> <li>`runtime-reference.md`, which drops the false "dep-topological" claim.</li> <li>The CLAUDE.md task-priority line.</li> <li>`internal-inboxbatch.md` and a new `internal-inboxrank.md`.</li> <li>The console skill instruction (superseded on 2026-10-27: no pending intake exists): on an exit-3 pending intake, ask the user with the recorded justification and options.</li></ul> |

## Landing order

1. **P1:** unwired. The operator reviews the new order in the PR.
2. **P2 prerequisite:** every autofiler stamps a `priority_class`, with its contract test. It changes no order while the rank is unwired. Thus it can land with P1's follow-ups or as P2's first commit, but never after P2.
3. **P2:** wiring.
4. **P3:** intake. Needs the overlap judge profile. *(Superseded on 2026-10-27 by the [P3 revision](#p3-revision-2026-10-27-a-deterministic-intake-and-an-event-driven-curator).)*
5. **P4:** preemption.
6. **P5 and P6:** these ride along with the components above.

## Verification

- **P1:**
  - golden of the full order;
  - the factor-explain output;
  - an unknown class is refused;
  - `unblocks` raises a dependency target;
  - `age` raises an old item;
  - an `active_campaigns` member rises;
  - a hygiene item outranks an otherwise equal security item (the corrected order puts security last).
- **P2 prerequisite** (done 2026-10-06):
  - each autofiler's item builder, driven by its test, yields an item whose `priority_class` the checked-in `class_order` names. The proof is `TestInboxWriters_EveryAutofiledItemCarriesAKnownClass` (cmd/evolve, one subtest per writer). Each package also has a per-writer test (listed in the landing notes);
  - the source scan is red on a fixture that writes an item into `.evolve/inbox/` outside the enrolled writers. The proofs are `TestInboxWriteSites_FindsEveryShapeOfAnInboxWriteAndNoRead` and `TestEnrollmentProblems_NameAnUnenrolledWriterAndAStaleEnrollment`. A mutation that added a ninth production writer turned the contract test red. A mutation that dropped the CI watch's class did the same.
- **P2** (done 2026-10-06):
  - every consumer returns `inboxrank.Order`: `TestReadInboxBacklog_IsTheRanksOrderOverTheReadyListAgainstTheWholeQueue`, `TestSeedWavePlanFromInbox_SeedsTheFirstRankedLanesNotTheHeaviest`, `TestWidenNarrowDecision_BackfillsTheFirstRankedLane`, `TestRefill_TakesTheFirstRankedItemNotTheHeaviest`, `TestClassify_ClustersRankByTheirBestRankedMember`, `TestTriageComposePrompt_BatchesFollowTheRankAndShowEachScoreAndTopFactor`, `TestInboxBatches_ListsBatchesInRankOrderWithEachScore`, `TestReadQueue_PendingFollowsTheInboxRankAndCarriesEachScore`;
  - the source-scan guard is red on a stray weight sort. `TestWeightOrderingSites_FindEveryShapeOfAWeightSortAndNoValidation` is red on a fixture. `TestNoProductionCodeOrdersByWeightOutsideTheRank` named exactly the five old orderings when run on the pre-P2 tree.
- **P3:** *(Superseded on 2026-10-27: the P3 revision lists the new checks.)*
  - a machine-origin overlap is judged (fake judge) and the decision is applied with recorded reasoning;
  - a human-origin overlap exits 3 and writes a pending intake;
  - `resolve` applies each option;
  - with no overlap, no judge call is made.
- **P4:** a fixture plan with a planned-but-unstarted slot and a higher new item gets preempted; a running lane and a continuation do not.
- **P5:** an edit without `--reason` is refused; recurrence moves the rank without writing a weight.
- **Live:** after P2, `evolve inbox rank --top 20` matches what the next wave's triage receives. (Checked 2026-10-06, read-only, on the runtime plane: see the P2 landing notes.)

## P2 prerequisite: a class on every autofiled item

Found in the P1 review (2026-10-06). `evolve inbox add` is the only path that files through `lifecycle.File`, so it is the only one the class check sees. Eight production autofilers write pending items themselves (with `os.WriteFile`, `atomicwrite.JSON` or `faillearn.writeIfAbsent`), and none sets `priority_class`. P1 stays unwired, so their items only show a `WARN` in `evolve inbox rank` today; once P2 wires the rank they score a class feature of 0. A fresh `ci-red` item at weight 0.95 then scores 0.4275, below a fresh correctness item at weight 0.6 (0.47). A fail-learning defect at 0.75 (0.3375) then ranks below a stability item at 0.5 (0.40).

### The proposed class per writer

| # | Writer | Builds and writes the item | What it files | Weight today | Proposed class | Why |
|---|---|---|---|---|---|---|
| 1 | Post-push CI watch | `internal/ciwatch/ciwatch.go:214-225`, written at :237-242 | `ci-red-<sha12>`: the GitHub run on a pushed commit came back red | 0.95, a Go literal | `correctness` | the tree on main fails a check until it is fixed |
| 2 | Recurrence boundary applier, retro autofile | `internal/recurrence/apply.go:129-145` calls `internal/retrofile/retrofile.go:99-114`; the one production stager is `internal/core/contract_escalation.go:350-363`, through `dispositionrouter.StageIntent` (which only stages, never writes the inbox) | `contract-gate-demoted-<phase>`: the contract gate was demoted from enforce to advisory, so later phases ship ungated | the policy's `retro_autofile.default_weight` (0.75) | `correctness`, carried on the staged intent | a gate stopped judging deliverables; other intent kinds later choose their own class |
| 3 | Fleet starvation observer | `internal/fleet/starvation.go:79-93`, written at :114-122 | `fleet-work-supply-starvation`: K waves ran fewer working lanes than the fleet was sized for | the policy's `fleet.starvation_weight` (0.9) | `stability` | the loop runs below the width it commits to |
| 4 | Fail-learning floor | `internal/core/failurelearning/remediation.go:46-55`, written by `internal/faillearn/inbox.go:50` | `retro-<cycle>-<slug>-<sha8>`: one item per defect a failed phase reported | the policy's `retro_autofile.default_weight` (0.75) | `correctness` | each item is a defect in the work (kind `bug`) |
| 5 | Triage-cap demotion | `internal/triagecap/demotion.go:171-200` | `auto-heuristic-demotion-triagecap-c<a>-c<b>`: the capacity clamp rejected two cycles with a byte-identical reason, so the gate itself is suspect | 0.7, a Go literal | `correctness` | a gate suspected of a false verdict |
| 6 | Goal-stall escalation | `cmd/evolve/cmd_loop_goalstall.go:110-134`, written at :152-161 | `goal-stall-<hash8>` / `nonprogress-<hash8>`: one goal shipped nothing for N cycles in a row | the policy's `goal_stall.weight` (0.9) | `stability` | the loop is stuck on a goal; the item's campaign is already `pipeline-stability` |
| 7 | ADR-0072 halt writer | `cmd/evolve/cmd_loop_escalation.go:82-105` | `pipeline-defect-<category>-cycle<N>`: the loop halted on a pipeline defect | 0.99, a Go literal | `stability` | the loop is down until it is fixed; kind `pipeline-repair` already routes it to the console |
| 8 | Unexplained-outcome classifier | `cmd/evolve/cmd_loop_outcome.go:341-363` | `unexplained-outcome-cycle-<N>`: a cycle ended `FAILED_UNEXPLAINED` with no recorded ship, salvage or abort reason | 0.8, a Go literal | `debuggability` | the defect is a missing record: an outcome nobody can explain |

The review's list named `internal/retrofile` and `recurrence/apply.go` separately and named `dispositionrouter` as a writer. They are one write path (row 2), and the disposition router only stages the intent. Rows 7 and 8 were found when the writers were mapped for this table.

### Where the class lives

The class lives in the writer, beside the `kind` and the weight that it already sets. Each writer files one kind of finding, so its class is a fact of the writer. The writer sets the class where it builds its item (for example, `ciwatch`'s `escalationItem` gains `PriorityClass: "correctness"`).

The retro autofile path files whatever its intent names. The class rides on the staged `dispositionrouter.Intent` (a new field that the stager sets) through `retrofile.PreventiveAction` into the item. Thus the one generic filer needs no class of its own. The policy still owns the order, that is how much a class matters, not what a finding is.

Considered and rejected: a policy map from writer to class. Such a map moves each writer's judgment of what it found into operator configuration. The risk is that a renamed or new writer silently files with no class. The contract test is what keeps a writer's class inside the policy's vocabulary: renaming a class in `class_order` reds it.

### The contract test

`TestInboxWriters_EveryAutofiledItemCarriesAKnownClass` (P2 prerequisite) builds each writer's item through its own builder, decodes it as `inboxbatch.Item` and requires `inboxbatch.CheckPriorityClass` against the checked-in policy's `class_order` to pass. A table names the writers. Thus a source scan in the style of P2's weight-sort ban completes the test. Any production write into `.evolve/inbox/` outside an enrolled writer is red. Thus a ninth autofiler cannot file without a class. The scan pins the force (a write into the inbox), not a list of names.

If every autofiler files through `lifecycle.File`, decision 6's check covers them all. But most autofiled items lack fields that `File` requires (`title`, `kind`, `summary`, `fix`, `acceptance`). Thus that route is a larger change than this prerequisite, and the prerequisite does not need it.

## P1 landing notes (2026-10-06)

P1 landed unwired on branch `feat/inbox-priority-rank` (console lane), from `origin/main` 6490463a8. The decision record is [ADR-0121](../architecture/adr/0121-inbox-priority-is-a-computed-rank.md). The CLI routing lane holds 0119 and 0120 is reserved for in-flight work, so the ADR keeps the number that this plan named. The operator's review artifact is [the preview report](../reports/inbox-rank-preview-2026-10-06.md).

### What landed

| Component | Where |
|---|---|
| The strict `inbox_priority` policy block, its compiled default and its resolver `InboxPriorityConfig()` | `go/internal/policy/inbox_priority.go`; the checked-in `.evolve/policy.json` names the class order and the factors |
| `Item.PriorityClass` and the one class rule `CheckPriorityClass` | `go/internal/inboxbatch/item.go`, `priority_class.go` |
| `evolve inbox add` refuses an absent or unknown class | `go/internal/inboxmover/lifecycle/priority_class.go` (`WithPriorityClasses`, checked by `File`); `inboxmover.Options.PriorityClasses`; `cmd_inbox_add.go` loads the policy |
| The rank: `Score`, `Order`, `ClassWarnings`, `Factors` | `go/internal/inboxrank`, enrolled in `.apicover-enforce` and in `.cover-strict` at 100 |
| The ledger's lockless read and its item linkage: `recurrence.ReadSnapshot` (which `Load` wraps in its lock) and `Ledger.ItemCounts` | `go/internal/recurrence/ledger.go` |
| `evolve inbox rank [--list ready\|console\|waiting\|all] [--top N] [--explain <id>] [--json]` | `go/cmd/evolve/cmd_inbox_rank.go`, `cmd_inbox_rank_print.go` |
| The order golden over a fixed 241-item snapshot | `go/internal/inboxrank/testdata/` |

### Choices made in the landing

1. **The factor defaults** are base 0.45, class 0.20, unblocks 0.15, recurrence 0.10, age 0.05, goal 0.05. They sum to 1, so a score is in [0, 1].
   - The filer's weight stays dominant. The reason is that it is still the only per-item judgment of value.
   - One class step (0.025) is worth a weight difference of about 0.056. Thus the class reorders items of similar weight, but it does not override a clear difference in value.
   - Example: a correctness item (the first class) at 0.35 outranks an otherwise equal security item (the last) up to 0.73.
   - Between adjacent classes, a weight gap above about 0.056 still wins.
   - Unblocks at the cap is worth a third of the weight range.
   - Recurrence and age are tie-movers and anti-starvation, not overrides.
   - Goal is inert until the operator names an active campaign.

   The full reasoning is ADR-0121 decision 4. On the 241 pending items, the 37 distinct weights become 214 distinct scores. The 33 items tied at 0.5 get 31.
2. **Two caps joined the block**: `unblocks_cap` (3) and `recurrence_cap` (5). The plan says both features are normalized and capped. The caps are tuning knobs, so they are configuration, like the factors, not Go literals. Capped counts, not ratios to the queue's maximum, keep one item's score independent of the others'.
3. **The scalar keys are pointers**, so an explicit `preempt_margin: 0` means zero rather than "absent". Every value is validated at load: the block fails `policy.Load` naming `inbox_priority`, as `cli_routing` does. A present `factors` block must name all six factors, so an omitted one never silently weighs 0.
4. **`priority_class` is required at filing**, not only checked when present: absent, unknown and mis-cased classes are all refused (exit 1). Every one of the 241 pending items carries one of the eight classes. Thus no normalization map was needed. The plan's "normalizes `correctness` and `maintainability`" became a direct match, because each is in the order. An unknown class at load ranks below every class (feature 0; the last known class scores 1/8). `evolve inbox rank` names it in a `WARN`.
5. **A lifecycle filer built without a class order refuses every filing as a fault** (not `ErrInvalidItem`). It does not admit an unjudged class. Only `evolve inbox add` files through it, and it always passes the policy's order.
6. **`Edit` does not judge the class yet.** `evolve inbox edit --set priority_class=…` still accepts any text; the load warning catches a bad one. P5 adds the check together with the `--reason` it requires.
7. **The recurrence linkage.** Items carry no fingerprint field. Thus an item's recurrence count is the largest non-generic ledger entry that names the item in one of two ways:
   - its pattern equals the item's id (an autofiled recurrence item is filed under its pattern);
   - its `fix_item_id` names the item.

   The recurrence package owns the linkage (`Ledger.ItemCounts`). The verb reads the ledger with `recurrence.ReadSnapshot`, without its lock. Thus the rank never creates a sidecar file. `internal/inboxrank` does not import the recurrence package.
8. **The golden pins a snapshot, not the live inbox.** If a golden reads `.evolve/inbox`, it changes with every lane ship and every filing. It then makes main red between landings (ADR-0110's class). `internal/inboxrank/testdata/inbox-snapshot-2026-10-06.jsonl` is a projection of the 241 items (the fields the rank reads, and the file name); `order-2026-10-06.golden` holds `rank score id`. The verb's JSON is pinned separately, byte for byte, against a hand-computed golden.
9. **Today's order for the comparison** is `RankForDispatch` on the ready list: weight descending, a declared surface first, then file-name order. The preview re-sorts the verb's own JSON rows by those keys (the rows carry `weight`, `declared` and `path`). Thus no second rank implementation was written.

### The operator's correction of the class order (2026-10-06)

After the first P1 build, the operator corrected the class order to correctness > stability > performance > debuggability > feature > maintainability > hygiene > security. Security moved from first to last (operator correction, 2026-10-06). The landing applies it everywhere the order is set or encoded:

- the compiled default (`go/internal/policy/inbox_priority.go`) and the checked-in `.evolve/policy.json`;
- the tests, red first. 13 tests went red against the old order before the default changed. Among them are `TestOrder_AHygieneItemOutranksAnOtherwiseEqualSecurityItem` (the inverse of the earlier security-first test) and the new `TestOrder_ACorrectnessItemOutranksEveryOtherClass`. They also include the policy default and checked-in tests, the add verb's refusal text, and the rank verb's table, explain and pinned JSON;
- the snapshot order golden, regenerated with `-update`, and the hand-computed JSON golden of the verb;
- the preview report, regenerated. 52 of 55 ready items move. The new top ten is nine stability items and one debuggability item. The one lane-ready security item, `cli-scan-secrets`, falls from 35th to 52nd;
- ADR-0121 decisions 3 and 4, the CHANGELOG entry and this plan.

The factor weights are unchanged. A class step is still 0.025, so the order decides near-ties; a security item sits one step below an otherwise equal hygiene item.

### Open questions for P2 to P4, answered by the operator (2026-10-06)

- **P2, class labels.** The preview's biggest fallers are hygiene items from the 2026-09-26 sweep that describe silent failures (`router-silent-errors`, `phasespec-roots-silent-policy-error`, …). **Answer:** they are not relabelled automatically. The operator re-classes an item with `evolve inbox edit <id> --set priority_class=… --reason …` (the `--reason` is P5's).
- **P2, cluster order.** **Answer:** a `Classify` cluster ranks by its maximum member score (today: its maximum member weight).
- **P2, the triage prompt.** **Answer:** the triage menu shows each candidate's score and its top contributing factor.
- **P2, recurrence on the plane.** The live plane's recurrence ledger is runtime state that the preview was not able to read. P2's live check (`evolve inbox rank --top 20` against the next wave's triage input) notes which items the recurrence factor moved.
- **P3, the fingerprint.** **Answer:** recurrence links through a new `fingerprint` field on the item, added in P3. Until then the factor uses the P1 linkage (a ledger pattern equal to the item's id, or a pattern whose `fix_item_id` names it).
- **P4, the margin's scale.** **Answer:** the preemption margin is absolute, 0.05 on the score's [0, 1] scale, and configurable as `inbox_priority.preempt_margin` (as P1 already ships it).
- **P5, edit.** `edit --set priority_class=` runs `CheckPriorityClass` with the same order as `add`; P5 threads the policy's order into the edit path together with `--reason`.

### Review fix round (2026-10-06)

The P1 review raised the findings below; each was fixed in the same unwired landing, with tests written red first.

| Finding | What changed | Pinned by |
|---|---|---|
| W2: the rank decoded the recurrence ledger itself (`readRecurrenceLedger` in the verb, `inboxrank.LedgerRecurrence` in the rank), a second copy of `recurrence.Load`'s decode rules | `recurrence.ReadSnapshot` is the lockless decode and `Load` is the lock wrapped around it; `recurrence.Ledger.ItemCounts()` holds the pattern and `fix_item_id` linkage. Both duplicates are deleted, the verb composes `Context.Recurrence` from `ReadSnapshot(...).ItemCounts()`, and `go list -deps ./internal/inboxrank \| grep -c recurrence` is 0 | `TestReadSnapshot_DecodesAsLoadDoesWithoutTakingTheLock`, `TestReadSnapshot_AMalformedOrUnreadableLedgerIsAnError`, `TestLedger_ItemCounts_CountsThePatternAnItemCarriesOrFixes`; the verb's `TestCmd_InboxRank_ARecurrenceCountMovesTheRankWithoutWritingAnyFile` and `TestCmd_InboxRank_AnUnreadableRecurrenceLedgerIsWarnedAndCountsNothing` |
| NIT-1: the class position was computed three times (`ClassKnown`, `classValue`, the verb's display) | `Facts.ClassPosition` (-1 when unknown) is computed once through `inboxbatch.PriorityClassPosition`. That is the `slices.Index` rule that `CheckPriorityClass` now validates with. `Facts.ClassKnown()`, the class feature and the "`i` of `n`" display derive from it. The verb's JSON carries `class_position` in place of `class_known` | `TestScore_TheClassPositionIsTheOneFactBehindTheClassFeature`, `TestPriorityClassPosition_IsTheOneMembershipRuleCheckPriorityClassJudgesBy`, `TestCmd_InboxRank_JSONIsPinned` |
| NIT-2: the verb's header kept its own factor list | `inboxrank.Factors()` lists the factors in score order and the header prints from it. The policy's `inboxFactorNames` and `weights()` must index-match; the policy cannot import the rank, so the rank's test pins them | `TestFactors_ListsEveryFactorOnceInScoreOrder`, `TestFactors_AreThePolicysFactorNamesInItsOwnOrder`, `TestScore_ContributionsAddUpToTheScore`, `TestCmd_InboxRank_TableShowsRankScoreBreakdownIDAndTitle` |
| LOW: a negative count from the recurrence seam gave a negative feature | `capped` clamps a negative count to 0 | `TestScore_ANegativeRecurrenceCountIsNoRecurrence` (red at -0.6 first) |
| W3: CLAUDE.md still listed "1. New features 2. Bug fixes 3. Security issues" | the section points at the rank and the policy's `class_order`; the tdd skill's reference to it is renamed. No other features-first statement exists anywhere in the repository. A repo-wide grep of the Markdown found one more, `knowledge/architecture/phase-pipeline.md`'s Scout row, which is now a pointer to the rank. The triage persona's HIGH/MEDIUM/LOW-then-weight prose describes today's unwired dispatch and stays P2's | — |
| NIT: `runtime-reference.md` restated the class order | it links [policy-config.md §inbox_priority](../architecture/policy-config.md#inbox-priority-rank-inbox_priority) instead | — |
| W1: eight autofilers file items with no `priority_class` | not fixed in P1, which stays unwired. ADR-0121 records the gap in its Consequences, and [the P2 prerequisite](#p2-prerequisite-a-class-on-every-autofiled-item) above proposes a class per writer and the contract test | — |

The explicit `float64(value × weight)` conversion in the score is unchanged: it is the rounding point that keeps the contributions summing to the score exactly (`TestScore_ContributionsAddUpToTheScore`).

## P2 prerequisite and P2 landing notes (2026-10-06)

P2's prerequisite and P2 landed together on branch `feat/inbox-rank-p2` (console lane), from `origin/main` `2f2cefdeb`, the prerequisite first. The rank is now **wired**: every inbox consumer reads `inboxrank.Order`. The decision record is [ADR-0121's P2 amendment](../architecture/adr/0121-inbox-priority-is-a-computed-rank.md#amendment-2026-10-06-p2-prerequisite-and-p2-wire-the-rank).

### What landed

| Component | Where |
|---|---|
| A class on every autofiled item (the table [above](#the-proposed-class-per-writer), as proposed) | `ciwatch.go` (`escalationItem.PriorityClass`), `core/contract_escalation.go` (`ContractGateDemotion.Intent`), `dispositionrouter.Intent.PriorityClass`, `recurrence/apply.go` → `retrofile.PreventiveAction.PriorityClass`, `fleet/starvation.go`, `core/failurelearning/remediation.go` with `faillearn.InboxItem.PriorityClass`, `triagecap/demotion.go`, and in `cmd/evolve` `cmd_loop_goalstall.go`, `cmd_loop_escalation.go`, `cmd_loop_outcome.go` |
| The contract test and the force scan | `go/cmd/evolve/inbox_writers_contract_test.go` (`TestInboxWriters_EveryAutofiledItemCarriesAKnownClass`), `inbox_writers_scan_test.go` (`inboxWriteSites`) |
| The consumer's whole input, the injected order and the labels | `go/internal/inboxrank/inputs.go`: `Inputs`, `Inputs.Order`, `Sequence`, `Labels` (and the unexported `Breakdown.topFactor` the labels read) |
| One loader of the rank's inputs | `go/internal/inboxrank/rankinputs` (`Load`), enrolled in `.apicover-enforce` and `.cover-strict` at 100; [internal-inboxrank-rankinputs.md](../architecture/packages/internal-inboxrank-rankinputs.md) |
| The wave seed, widen and refill | `triagecap.ReadInboxBacklog` ranks; `RankForDispatch` and `FleetCandidate.Declared` deleted; `loopwave/launcher.go`'s refill reads the ranked backlog |
| The batch order | `inboxbatch.Config.Order`; `Classify` split into `clusterByRules`, `rankPositions`, `rankOrder`, `chunkCluster`; `Batch.Weight` removed; `RenderMarkdown(batches, label)` |
| The triage prompt and `evolve inbox batches` | `phases/triage/triage.go` (`inboxBatchesSection`, `selectableBatchesNote`), `cmd/evolve/cmd_inbox.go` |
| `evolve inbox rank` | `cmd_inbox_rank.go` loads through `loadRankInputs` (`rankinputs.Load`); `ledgerItemCounts` deleted |
| The dashboard's queue | `dashboard/queue.go` (`readQueue(root, now)`), `QueueItem.Score`, `static/app.js` shows the score |
| The weight-sort guard | `go/internal/inboxrank/weightsort_ban_test.go`; `./internal/inboxrank/...` added to `repocontract.Packages()` |

### Choices made in the landing

1. **The class lives in each writer, as proposed.** The retro autofile path carries it on the staged intent. `core.ContractGateDemotion.Intent` is the pure builder that the stager calls. It is exported so that the contract test stages the real stager's intent through `recurrence.ApplyBoundary`, not a hand-made one. `fleet`'s and the goal stall's `Validate` now require the class. `faillearn` takes the class from its caller and never defaults it.
2. **The contract test drives each writer's own path.** The paths are these:
   - the real `ciwatch.Watch` with a fake fetcher;
   - the demotion staged and applied;
   - `BuildStarvationItem(...).WriteTo`, `failurelearning.Engine.WriteFloor` and `triagecap.NewDemotionLedgerRecord`;
   - the three `cmd/evolve` writers.

   The test decodes the item with `inboxbatch.LoadDir` or `LoadFile`. It checks the row's class and `CheckPriorityClass` against the checked-in `.evolve/policy.json`.
3. **The scan pins the force with a heuristic data-flow.** A site is a production function that calls a write primitive on an inbox path.
   - The write primitives are `os.WriteFile`, `os.Create`, `os.OpenFile`, `os.CreateTemp`, any `atomicwrite` call, and the target of `os.Rename`/`os.Link`.
   - An inbox path is built from one of these sources:
     - an inbox name (an identifier or field named `inbox` or `*InboxDir`);
     - an `inbox` or `.evolve/inbox` path literal, or a package constant that holds one;
     - a same-package helper that returns or writes such a path.
   - Taint flows through assignments, `var` declarations, struct fields, and `range` and `filepath.Walk` callbacks.
   - Taint also flows through path-building calls (`filepath`, `path`, `strings`, `fmt.Sprintf`).
   - Taint does not flow through arbitrary call results (an item list loaded from the inbox is not a path).
   - Flow through another package's helper is not followed. Thus a writer that receives an inbox path under a neutral parameter name from another package is a known blind spot.
   - Every site found must be an autofiler row's site or an entry of `inboxWritersThatFileNothing` with its reason.
   - The entries are the lifecycle package, `inboxmover.RecordRootTaskFailure`, `recurrence.applyIntent`'s weight write, the ship's consumption `phases/ship.stage`, and `evolve inbox consume`.
   - Every enrollment must still be found.
4. **The scan lives in `cmd/evolve`**, not in a ship-pack package. Three of the eight builders are package `main`. Also, `cmd/evolve` is already a sanctioned tree reader outside the ship pack (`repocontract`'s `readsTheTreeOutsideThePack`). The weight-sort guard, which needs no `main` builder, lives in `internal/inboxrank`, which joined the pack.
5. **`ReadInboxBacklog` and `SelectWaveSeedMenus` keep their signatures.** The per-cycle ACS predicates 1159, 1180 and 1181 call them and still hold. If the signatures get a new rank parameter, valid predicates do not compile.
   - The backlog loads the evolve dir's own inputs: `rankinputs.Load(evolveDir, time.Now())`, its policy and its ledger, beside the inbox it already reads.
   - It prints a loader warning to the loop log.
   - The testable core is `readRankedBacklog(evolveDir, isProtected, now, loopLog)`, which takes the clock and the log writer.
   - `ReadInboxBacklog` passes `time.Now()` and `os.Stderr`.
   - It reads items through `inboxbatch.LoadDir`, the reader that the verb and the triage prompt use. Thus all three rank identical items.
   - The backlog's own decode, which skipped the loader's sanitizer, is gone.
   - The backlog still drops an id-less item.
6. **`loopwave`'s import allowlist is unchanged.** The P2 plumbing note proposed to add `inboxrank` to it. But the inputs load where the backlog is read, so the engine handles no rank type.
7. **`Classify` never ranks by weight itself.** With no `Config.Order`, it keeps the caller's order. The order is run once and turned into positions keyed by `(id, path)`. Thus the score computation never goes O(N²), and same-id items keep their places. `Batch.Weight` (the maximum member weight) is removed: in a ranked menu it read as the priority.
8. **Labels go on sub-lines.** Each batch line keeps its `- batch N (<reasons>): id, id` shape. Each member gets a `  - <id>: score 0.4475, top factor base` line beneath it. The reason is that readers parse the batch line by its last `": "` (`go/acs/cycle1724` 004, `go/acs/cycle1678`). If a label is inside the line, those readers break.
9. **`evolve inbox rank` still exits 2 on a malformed policy.** Every other consumer ranks with the compiled default and a warning. The reason is that a consumer inside the loop must not stop on it.

### Goldens and predicates

| Artifact | Change | Why |
|---|---|---|
| `go/internal/core/failurelearning/testdata/inbox-915304b4.golden.json`, `inbox-4f0ecc16.golden.json` | gained `"priority_class": "correctness"` | the remediation items now carry their class (row 4) |
| `go/internal/loopwave/testdata/decision_{seed,widen,prune}.golden.json` | unchanged, re-checked | their fixtures carry no class and no date, so the rank equals the old weight order on them. New tests with classed fixtures pin the rank order instead |
| `go/acs/cycle536` 007, 008 and `go/acs/cycle1724` 002 | archived to [superseded predicates](../private/research/archived-2026-10-06/superseded-predicates/README.md) under the A2 retention rule | each asserted the weight order as its acceptance (the seed led by the heaviest item; an un-ranked `Classify` ordered weight-descending) |
| `go/acs/cycle536`'s seven other predicates | compile and pass again | the two archived predicates had kept the package from compiling since `SelectWaveSeedTopN` gained `isProtected`, before P2 |
| `go/acs/cycle1159`, `1180`, `1181`, `1182`, `1206`, `1633` | checked, unchanged | they assert committed-prefix preservation, consumed-id pruning, rule-set membership or batch independence. All pass as before. `1181` 003 needs a live `.evolve/state.json` and fails in a dev worktree exactly as on the base |
| `go/acs/cycle541` | checked, unchanged | its candidates are already given in order, so it asserts disjointness, not the weight order. It has not compiled since `fleet.PlanFromTriage` gained arguments, which is unrelated to P2 |

The unit tests that pinned the weight order were rewritten to the rank-order contract. They are `triagecap`'s rank and amplified tests, `inboxbatch`'s classify tests and the dashboard's queue test. Each is renamed for what it now pins.

### The live check (2026-10-06, read-only)

On the runtime plane's inbox (232 pending items: 49 lane-ready, 180 console-owned, 3 waiting), with the branch's binary:

- `evolve inbox rank --top 20` and the wave seed's backlog (`triagecap.ReadInboxBacklog` with the loop's `laneForbidden` predicate) list the same 20 items in the same order. The list runs from `cli-dossier-publish`, `overlay-rule-tier-selectors-canonicalize` and `repo-contract-test-level-selection` down to `phasecmd-silent-policy-fallbacks`.
- `evolve inbox batches`, the triage prompt's composition, places all 49 ready items in 37 batches. Members are in rank order and each cluster sits at its best member's rank.
- 45 of the 49 ready items moved against the old weight order. The old top ten (`router-silent-errors`, `phasespec-roots-silent-policy-error`, `overlay-rule-tier-selectors-canonicalize`, `policy-resolver-hygiene`, `premium-rung-placement-law-and-narrative-dedup`, …) was mostly hygiene items; the new top ten leads with `cli-dossier-publish`, `overlay-rule-tier-selectors-canonicalize`, `repo-contract-test-level-selection`, `cli-phase-cycle-request` and `acs-cycle1723-package-doc-predicate-stale-after-753`.
- **Recurrence moved nothing:** the plane has no `recurrence-ledger.json`, so the factor is 0 for every item (the question P1 left open for P2).
- Every pending item on the plane carries a known class, so no `WARN` was printed.

### Review and floor

- **The diff review** (one combined Go reviewer) passed with no CRITICAL or HIGH finding.
  - Its two MEDIUM findings are kept as choices.
  - The first MEDIUM finding: `triagecap.ReadInboxBacklog` and the triage prompt print the rank's input warnings to the process's stderr (the loop log). The reason is that their signatures carry no writer. Also, the backlog's signature stays compatible with the ACS predicates above.
  - The second MEDIUM finding: the backlog drops `inboxbatch.LoadDir`'s per-file notices, but a read error still prints. The plane prints about forty sanitize notices per read, and the loop chain already reports an invalid root item.
  - Its two LOW findings are recorded above. The verb reads the policy twice to keep its exit 2. The weight-sort guard is a syntactic scan, not a data-flow proof.
- **Ratchets the landing moved.**
  - In the function-size ratchet, `writePipelineEscalation` grew by its class line. Thus a redundant comment went, and it is back at its allowance.
  - In the same ratchet, `inboxbatch.Classify` (allowance 93) is under the limit after the split, so its allowance was removed.
  - The ship's pinned pack list (`TestContractRed_NamesEverySuiteOfTheOnePackList`) names `inboxrank`.
  - The guard's file is `weightsort_ban_test.go`. The reason is that a `go/internal` file name that contains `guard` is gate-shaped. Such a file must be on the protected-surface manifest (`TestEveryGateShapedFileIsProtectedSurface`).
  - The operator then protected it with a file entry in `guards.ProtectedSurfaceManifest` (see the fix round below).
- **The floor, on the staged tree, one target at a time.** This is the final run, after the architecture review's fix round and the engineering-craft audit.
  - `make test`: 258 packages ok.
  - `make test-integration`: 259 ok.
  - `make test-e2e`: 6 ok.
  - `make test-acs-durable`: 42 ok.
  - `make apicover-enforce`: 258 packages, 4859 of 4859 exports covered, 0 false-green.
  - `make cover-strict`: 24 of 24 at their floors, with `inboxrank`, `rankinputs`, `loopwave`, `core/failurelearning` and `inboxmover/lifecycle` at 100%.
  - go1.23 `gofmt -l -s` clean; `go vet ./...` clean.
  - `golangci-lint` clean on every touched package. The whole-tree run reports four findings in untouched files: `signalcenter/stream.go`, `pkg/acsassert/go_tests.go`.
  - One earlier run lost `internal/core` to Go's 10-minute package timeout under a machine load average of 16 to 21. It passed in 426 s on the rerun.

### Architecture review fix round (2026-10-06)

The architecture review judged the landing FIX_THEN_MERGE (0 CRITICAL, 4 WARNING, 4 NIT). It confirmed three facts: dispatchability unchanged, a single source, and the archived predicates moved verbatim. Each fix was written red first; the reds are recorded in the lane's scratchpad (`fix-W1.txt`, `fix-W2.txt`, `fix-W3.txt`, `fix-NIT.txt`).

| Finding | What changed | Pinned by |
|---|---|---|
| W1: a surviving mutant. `Order(menu.Ready, menu.Ready)` at the triage prompt or at `evolve inbox batches` passed every test, because no fixture had a waiting dependent | one helper, `inboxmover.RankLaneMenu` (partition, then rank the ready list against the whole queue). The triage prompt, `evolve inbox batches` and `triagecap.ReadInboxBacklog` use it. Both consumers' tests gained a waiting item whose `deps` name a lighter ready item, which must lead | `TestRankLaneMenu_RanksTheReadyListAgainstTheWholeQueue`. The mutant inside the helper reds it and the triage, batches and backlog rank tests. Before the helper, the same mutant at each old site redded the extended triage and batches tests |
| W2: a missing class was refused only by the producers | the shared funnels refuse it. `dispositionrouter.StageIntent` refuses an autofile intent with no class (`inboxbatch.ErrNoPriorityClass`). `retrofile.FileActions` refuses an action with no class after its dedupe (`inboxbatch.ErrNoPriorityClass`). `recurrence.applyIntent` records a legacy classless line as `ApplyResult.Refused` and never wedges the boundary. faillearn's inbox writer refuses a batch with a classless item before any write (`inboxbatch.ErrNoPriorityClass`). The retrospective persona's `preventive_actions` schema requires `priority_class` | `TestStageIntent_RefusesAnAutofileWithNoPriorityClass`, `TestFileActions_RefusesAnActionWithNoPriorityClassButNotAnAlreadyFiledOne`, `TestApplyBoundary_AClasslessAutofileStagedBeforeClassesIsRefusedNotFiledAndDoesNotWedge`, `TestWriteInboxItems_RefusesAnItemWithNoPriorityClassAndWritesNone` |
| W3: no runtime backstop for the force scan's blind spots | `triagecap.ReadInboxBacklog` prints `inboxrank.ClassWarnings` over the whole pending queue to the loop log at every read. The triage prompt's inbox section opens with an `- unknown_priority_class:` line. The scan's inbox names widen to `inbox(dir\|path\|root)`. The blind spots (imported path builders, non-`os` write primitives, function values) are recorded in [cmd-evolve.md](../architecture/packages/cmd-evolve.md) | `TestReadRankedBacklog_NamesEveryUnclassedQueuedItemInTheLoopLog`, `TestTriageComposePrompt_NamesQueuedItemsWithNoKnownClass`, `TestInboxWriteSites_FindsEveryShapeOfAnInboxWriteAndNoRead` (now with `inboxPath` and `inboxRoot` writers) |
| W4: `internal-loopwave.md` still described the refill's `RankForDispatch` order | the refill takes the first non-excluded candidate of `triagecap.ReadInboxBacklog`, in rank order | — |
| NITs | `LoadFile` records the file-name fallback as `Item.IDFromFileName`, and the backlog filters on it. `recordsAnID`, which re-decoded each file, is deleted. One exported identity key, `inboxbatch.ItemKey` (`Item.Key()`), replaces the two private copies in `inboxbatch` and `inboxrank`. The class names `inboxbatch.ClassCorrectness` … `ClassSecurity` sit beside `CheckPriorityClass`, and every autofiler and the contract table use them (`core/failurelearning`'s import allowlist gained `inboxbatch`). One inbox item asks the operator to decide on the protection of ship-pack scanners as a class (`decide-protecting-ship-pack-scanners-as-a-class`, console-owned, maintainability) | `TestLoadFile_RecordsAnIDTakenFromTheFileName`, `TestItem_KeyIsTheIDAndThePath`, `TestClassNames_AreThePolicysCompiledClassOrder` |

**More superseded acceptance.** W2 makes a classless remediation item or autofile intent invalid input. Thus four more per-cycle predicates whose premise filed one are archived under the A2 retention rule ([superseded predicates](../private/research/archived-2026-10-06/superseded-predicates/README.md)). The four are `cycle1062` 008 and 009, `cycle1290` 001 and `cycle1292` 001. Classed unit tests pin their behavior; the live `cycle1290` 003 and `cycle1292` 003 still run those tests by name.

`cycle1290` 004 compares `inbox_transactional_test.go` with `HEAD`, whose fixture gained `priority_class`, so it is red until the change is committed. `cycle1292` 004 is red in any checkout for a data reason that predates P2 (a tracked inbox item's title is over 160 characters).

**The operator kept two additions in the staged index:** a protected-surface manifest entry for `inboxrank/weightsort_ban_test.go` (`go/internal/guards/integrity_surface.go`) and its dated bullet in [internal-guards.md](../architecture/packages/internal-guards.md).

**The engineering-craft audit (2026-10-06).** The operator required the repo's `engineering-craft` and `golang-test-review` skills to be loaded and the staged diff audited against them. What it changed:

- **One home for a class rule.** `inboxbatch.RequirePriorityClass` and `ErrNoPriorityClass` replace three package sentinels in `dispositionrouter`, `retrofile` and `faillearn`.
- **Parameter objects instead of long signatures.**
  - `core.ContractGateDemotion{…}.Intent()` replaces a six-parameter builder.
  - `inboxbatch`'s chunking is two three-parameter helpers, `chunk` and `itemsAt`.
  - `recurrence.autofileIntent` derives its id itself.
- **No flag argument.** The force scan's taint function takes seed names instead of a bool.
- **No export without an outside caller.** `inboxrank`'s top factor is unexported (`Breakdown.topFactor`), so it no longer needs a guard for an impossible empty case. Its tie is pinned through `Labels`.
- **Shared harness.** The new tests write fixtures with `fixtures.MustWrite` instead of four local copies, and the contract test's builders call `t.Helper`.
- **Assertion-level reds.** Several first reds were compile errors, which do not count as red. Every new behavior was re-proven with a `go test -overlay` mutant of the staged code. There were 38 mutants, all killed by the test named for the behavior, and the first one was a control. The mutants include these kinds:
  - reorder mutants (the class check before retrofile's dedupe, faillearn's check inside the write loop, the triage class line after the lists);
  - a shift mutant (`>` to `>=` in the top factor);
  - wrong-key mutants (an `ItemKey` without the path);
  - a wrong-list mutant (`RankLaneMenu` that ranks the ready list against itself).
- **Four functions keep four parameters, as natural signatures.**
  - `inboxmover.RankLaneMenu` is `PartitionLaneMenu`'s three plus the rank.
  - `triagecap.readRankedBacklog` is the backlog's two plus the injected clock and log.
  - `loadRankInputs` in `cmd/evolve` is the verb label, the directory, the clock and the sink.

**The delta re-review (2026-10-06): FIX_THEN_MERGE, closed here.** The re-review closed W1–W4 and the NITs. It found the original W1 mutant killed in four packages. It also found that the 11 live legacy classless intents degraded to `Skipped`, with none refused. It found the seven archived predicates byte-identical. It raised one HIGH and five surviving mutants:

| Finding | Fix | Evidence |
|---|---|---|
| H1: a refused autofile intent was invisible in the loop log. `applyEscalationBoundary` stayed quiet when only `Refused` was non-empty, and its signal had no `refused` field | the quiet check counts `Refused`; one `[loop] WARN escalation boundary: refused <id>: no priority_class` line per id; a `refused` field on `LOOP_ESCALATION_BOUNDARY` (registry text and `signal-codes.md` regenerated) | `TestApplyEscalationBoundary_NamesEachRefusedIntentInTheLoopLogAndItsSignal`, red on the prior code; the reviewer's own probe passes; three mutants killed (quiet check, WARN line, field) |
| M3: the tests allowed a triage class line that warns over the ready list only | a waiting unclassed item must be named | `TestTriageComposePrompt_NamesQueuedItemsWithNoKnownClass` kills it |
| M7: the tests allowed faillearn to check `Priority` instead of `PriorityClass` | both fixture items carry `Priority: "H"` | `TestWriteInboxItems_RefusesAnItemWithNoPriorityClassAndWritesNone` kills it |
| M6: the tests allowed recurrence to refuse before retrofile's dedupe | a legacy classless intent whose item is already filed is `Skipped`, not `Refused` | `TestApplyBoundary_AClasslessLegacyAutofileWhoseItemIsFiledIsSkippedNotRefused` kills it |
| M10, M11: the tests allowed the triage prompt and `evolve inbox batches` to load rank inputs from the wrong directory | each writes a `policy.json` whose `class_order` puts security first and requires the heavier security item to lead | `TestTriageComposePrompt_RanksWithTheEvolveDirsPolicy`, `TestInboxBatches_RanksWithTheEvolveDirsPolicy` kill them |
| MEDIUM (Boy Scout): `writePipelineEscalation` was 62 lines | extracted into `haltRecord` with `narrative`, `dossier` and `inboxItem`, now 14 lines; its size-ratchet allowance removed | the `TestWritePipelineEscalation_*` tests pass before and after |
| LOW: this plan named `rankedBacklog` with fixed `Inputs` | reworded to `readRankedBacklog(evolveDir, isProtected, now, loopLog)` | — |

### Open question for P4 and P5: starvation by design

The architecture review judged that the rank can starve a class by design (ADR-0121). The rank was not changed. The question is the operator's:

- **The arithmetic.** The class factor is 0.20 over eight classes, so one class step is 0.025. At equal weight, a correctness item leads a feature item by four steps (0.10) and a hygiene item by six (0.15). It leads a security item by seven (0.175).
- **Age cannot close the gap.** The age factor is capped at 0.05 (`1 − 0.5^(days/30)` × 0.05): 0.025 after 30 days, 0.0375 after 60, and only toward 0.05 after that. So age closes at most two class positions, and only in the limit. An item four or more steps behind never overtakes a fresh item of equal weight on age alone.
- **The consequence.** While correctness and stability items keep arriving at similar weights, feature, hygiene and security items wait indefinitely. What can still move them is one of these:
  - a weight about 0.056 higher per class step;
  - a dependent they unblock (up to 0.15);
  - a recurrence count (up to 0.10);
  - the operator's `active_campaigns` steering (0.05).
- **The question.** Is that starvation the intended reading of the class order? Or is an anti-starvation term in P4 or P5 necessary? Options include a larger or uncapped age factor, a per-class age clock, or a share of lane slots per class. Any of these is a policy change for the operator to approve; none is made here.

### What remains

P3 (one intake path, the overlap judge; revised on 2026-10-27, see below) and P4 (boundary preemption) remain. P5 also remains: `--reason` on every priority change, and the deletion of `recurrence.setItemWeight`, the one remaining weight writer outside the lifecycle. The writers `ciwatch`, `retrofile`, `dispositionrouter` and `core/failurelearning` have no package page under `docs/architecture/packages/`. Their class is recorded here, in ADR-0121's amendment and in `internal-core.md`, `internal-recurrence.md`, `internal-faillearn.md`, `internal-fleet.md`, `internal-triagecap.md` and `cmd-evolve.md`.

## P3 revision (2026-10-27): a deterministic intake and an event-driven curator

The operator's request of 2026-10-27: "Let's build this directly now, as it's critical to the pipeline." The inbox had no review. 256 of 258 items were console-owned, and 4 items whose work merged days ago ranked in the top 12.

The revision replaces the LLM overlap judge of the P3 row above. No LLM edits the inbox. The spec of record is [inbox-curator.md](../architecture/inbox-curator.md), and the research is [inbox-curator-2026-10.md](../research/inbox-curator-2026-10.md).

| Part | Change | PR |
|---|---|---|
| P3a | **The add-time intake.** `evolve inbox add` validates the origin, the kind, the fix surface and the route. It refuses an overlap with an open item at or above `inbox.curator.overlap_refuse` (exit 3), unless `--allow-overlap --reason`. It warns between the two thresholds. It prints the rank of the filed item. `lifecycle.File` enforces the origin for every filer. | the intake PR |
| P3b | **The curator.** `evolve inbox curate [--json] [--apply] [--event]` runs on `ship.landed`, on a policy or protected-manifest change, and at the wave boundary. It publishes typed proposals. In the `enforce` stage it applies the deterministic ones at the boundary. | the curator PR |

The three triggers and the boundary backstop are in [inbox-curator.md §The design](../architecture/inbox-curator.md#the-design). The `merge`, sweep `consume` and lane `reroute` proposals are judgment proposals for the operator and the orchestrator item X13.

**Two promises of the old P3 move.** The `fingerprint` field of the item moves to P5, with the recurrence work. "The top movers" on every add is dropped: `evolve inbox add` prints the rank of the filed item, and `evolve inbox rank` shows the rest.

**Verification of P3** (replaces the P3 list above):
- A duplicate is refused, and a near overlap is warned.
- A gate-file item cannot be route lane.
- An item whose acceptance merged is proposed for consume in the same boundary.
- A lane cannot apply a proposal.
- On a read-only copy of the live inbox, the curator finds the 4 stale items.
