# Inbox prioritization: one computed rank, re-ranked on every add (2026-10)

> **Status:** approved by the operator on 2026-10-06. The request: "Do we have logic to reprioritize the Todo inbox tasks when adding the new requests? If not, we need to build the logic… with ultrathink".
>
> **Decision record:** ADR-0121, which lands with P1.
>
> **Progress:** P1 landed unwired on 2026-10-06 ([notes](#p1-landing-notes-2026-10-06)); the P2 prerequisite and P2 landed the same day and wired the rank ([notes](#p2-prerequisite-and-p2-landing-notes-2026-10-06)). P3 to P5 remain. The inbox items are in [the table at the end](#inbox-items-for-the-open-components-2026-10-27).

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

The **score** is computed in Go from those facts. Its weights live in policy configuration, not Go literals. One function, `inboxrank.Order`, feeds every consumer.

## Operator decisions (2026-10-06)

1. **Class order: correctness first, security last.** The order is in the paragraph below this list.
2. **Preemption.** At a wave boundary, a better item takes the lowest **uncommitted** planned slot. Running lanes stay.
3. **Overlap at add time depends on origin.**
   - **Machine origin** covers previous cycles (retro, halt, fail-learning, recurrence, carryover) and console-agent findings. A deep-model overlap judge decides supersede, merge-into, relate or distinct. Its reasoning is **recorded** in the item (`overlap_review`) and in the ledger, and the decision is applied through the lifecycle writer.
   - **Human origin** covers the operator's requests. The same judge writes a justification and the options. `evolve inbox add` then **stops**, recording a pending intake, and the console agent **asks the user** with that justification and those options. Nothing is filed until the user chooses.

The class order of decision 1 is correctness > stability > performance > debuggability > feature > maintainability > hygiene > security (operator correction, 2026-10-06). The order that the operator first approved that morning was "pipeline health first". That order was security > correctness > stability > performance > debuggability > feature > maintainability > hygiene. The operator corrected it the same day, before P1 landed, and P1 ships the corrected order. The CLAUDE.md line "/evo:loop task priority" (features first) now agrees with it.

For decision 2, a lane-eligible item takes the slot only when its score is higher by the policy margin. Running lanes and adopted continuations are never displaced.

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
| P1 | **`internal/inboxrank`** (pure, unwired) and **`evolve inbox rank [--json] [--explain <id>] [--top N]`** | <ul><li>Add `inbox_priority` to the policy with a strict decode. It holds the class order, the factor weights, the age half-life, `active_campaigns` and the preemption margin. </li><li>Add a validated `PriorityClass` enum on `inboxbatch.Item` that normalizes `correctness` and `maintainability`. </li><li>Add the score and the `Order` function, and `--explain` prints the factor breakdown. </li><li>A golden test records the old order and the new order for every root item. </li><li>The PR shows the top-30 deltas for operator review. </li></ul> |
| P2-pre | **PREREQUISITE of P2: a class on every autofiled item** | <ul><li>Every autofiler stamps a `priority_class` from the policy order, as it chooses the weight. </li><li>The proposed class for each writer is in [the table below](#p2-prerequisite-a-class-on-every-autofiled-item). </li><li>A contract test requires a known class on the item of every inbox writer. </li><li>A source scan reds on any production write into `.evolve/inbox/` that the test does not enroll. </li><li>It lands before P2 wires the rank. Without it, the items of eight autofilers score a class feature of 0 and sink below mid-weight items (ADR-0121 Consequences). </li></ul> |
| P2 | **One ranking for every consumer** | <ul><li>These consumers all use `inboxrank.Order`: `RankForDispatch`, the `Classify` cluster order, the triage injection, and the wave seed and refill paths. </li><li>A source-scan guard bans any other weight sort. </li><li>The stale triage priority prose (`evolve-triage.md`, `evolve-triage-reference.md`) points to the rank. </li></ul> |
| P3 | **The add-time intake** | A validated `origin` field, an overlap pre-check, the `inbox-overlap-judge`, a pending intake for human origin, and `evolve inbox resolve`. The [P3 details](#p3-details) below give each part. |
| P4 | **Boundary preemption** | <ul><li>When the wave plan reuses the previous decision, check it again against `inboxrank.Order`. </li><li>A lane-eligible item takes the lowest uncommitted planned slot when its score is higher by `inbox_priority.preempt_margin`. </li><li>The ledger records `preempted <old> for <new> (score a→b)`. </li><li>Committed in-flight lanes and adopted continuations are never displaced. </li></ul> |
| P5 | **Reasons on every priority change** | <ul><li>`evolve inbox edit --set weight=… / priority_class=…` requires `--reason`, and the ledger records it. </li><li>Recurrence is a rank factor since P1. `recurrence.setItemWeight` is gone, but the escalate path still writes a weight through `ApplyOptions.Reweight`. P5 deletes that write. </li></ul> |
| P6 | **Docs** | <ul><li>ADR-0121. </li><li>`runtime-reference.md`, without the false "dep-topological" claim. </li><li>The CLAUDE.md task-priority line. </li><li>`internal-inboxbatch.md` and a new `internal-inboxrank.md`. </li><li>The console skill instruction: on an exit-3 pending intake, ask the user with the recorded justification and options. </li></ul> |

### P3 details

- A validated `origin` field: `operator`, `console`, `cycle` or `retro`. Autofilers set it, and `evolve inbox add --origin` defaults to `console`.
- A deterministic overlap pre-check uses shared `files` and dirs as Jaccard, `connects_to` and title-token similarity. Above a policy threshold, it calls the **`inbox-overlap-judge`**.
- The judge is a new deep-tier profile on the Claude family. Its JSON contract is the verdict, the rationale, the related ids and the confidence.
- Machine origin: the lifecycle writer applies the verdict of the judge, and `overlap_review` stores the reasoning. `supersedes` withdraws with a reason. `merge_into` appends to the acceptance of the target and writes a ledger line. `relates` sets `connects_to`.
- Human origin: `add` exits 3 and writes `.evolve/inbox/pending-intake/<id>.json` with the justification and the options of the judge.
- `evolve inbox resolve <id> --supersedes X | --merge-into X | --relates X | --distinct | --withdraw` finalizes a pending intake.
- Every add prints where the item lands and the top movers.

## Landing order

1. **P1:** unwired. The operator reviews the new order in the PR.
2. **P2 prerequisite:** every autofiler stamps a `priority_class`, with its contract test. It lands before P2, never after.

The P2 prerequisite changes no order while the rank is unwired. Thus it can land with the follow-ups of P1 or as the first commit of P2.

The remaining steps:


3. **P2:** wiring.
4. **P3:** intake. Needs the overlap judge profile.
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
  - the item builder of each autofiler, driven by its test, gives an item with a `priority_class` that the checked-in `class_order` names. The test is `TestInboxWriters_EveryAutofiledItemCarriesAKnownClass` (cmd/evolve, one subtest for each writer). Each package also has a test for its writer (listed in the landing notes);
  - the source scan is red on a fixture that writes an item into `.evolve/inbox/` outside the enrolled writers. The tests are `TestInboxWriteSites_FindsEveryShapeOfAnInboxWriteAndNoRead` and `TestEnrollmentProblems_NameAnUnenrolledWriterAndAStaleEnrollment`. A mutation that added a ninth production writer turned the contract test red. A mutation that removed the class of the CI watch also turned it red.
- **P2** (done 2026-10-06):
  - every consumer returns `inboxrank.Order`: `TestReadInboxBacklog_IsTheRanksOrderOverTheReadyListAgainstTheWholeQueue`, `TestSeedWavePlanFromInbox_SeedsTheFirstRankedLanesNotTheHeaviest`, `TestWidenNarrowDecision_BackfillsTheFirstRankedLane`, `TestRefill_TakesTheFirstRankedItemNotTheHeaviest`, `TestClassify_ClustersRankByTheirBestRankedMember`, `TestTriageComposePrompt_BatchesFollowTheRankAndShowEachScoreAndTopFactor`, `TestInboxBatches_ListsBatchesInRankOrderWithEachScore`, `TestReadQueue_PendingFollowsTheInboxRankAndCarriesEachScore`;
  - the source-scan guard is red on a stray weight sort. `TestWeightOrderingSites_FindEveryShapeOfAWeightSortAndNoValidation` proves it on a fixture. On the tree before P2, `TestNoProductionCodeOrdersByWeightOutsideTheRank` named exactly the five old orderings.
- **P3:**
  - a machine-origin overlap is judged (fake judge) and the decision is applied with recorded reasoning;
  - a human-origin overlap exits 3 and writes a pending intake;
  - `resolve` applies each option;
  - with no overlap, no judge call is made.
- **P4:** a fixture plan with a planned-but-unstarted slot and a higher new item gets preempted; a running lane and a continuation do not.
- **P5:** an edit without `--reason` is refused; recurrence moves the rank without writing a weight.
- **Live:** after P2, `evolve inbox rank --top 20` matches what the next wave's triage receives. (Checked 2026-10-06, read-only, on the runtime plane: see the P2 landing notes.)

## P2 prerequisite: a class on every autofiled item

The P1 review found this gap on 2026-10-06. `evolve inbox add` is the only path that files through `lifecycle.File`, so the class check sees only that path. Eight production autofilers write pending items themselves, with `os.WriteFile`, `atomicwrite.JSON` or `faillearn.writeIfAbsent`. None of them sets `priority_class`. P1 stays unwired, so their items only show a `WARN` in `evolve inbox rank` today. When P2 wires the rank, they score a class feature of 0.

The effect on the order is large. A fresh `ci-red` item at weight 0.95 then scores 0.4275, below a fresh correctness item at weight 0.6 (0.47). A fail-learning defect at 0.75 (0.3375) ranks below a stability item at 0.5 (0.40).

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

The list of the review named `internal/retrofile` and `recurrence/apply.go` as two writers, and it named `dispositionrouter` as a writer. They are one write path (row 2), and the disposition router only stages the intent. Rows 7 and 8 came from the map of the writers for this table.

### Where the class lives

The class lives in the writer, beside the `kind` and the weight that it already sets. Each writer files one kind of finding, so its class is a fact of the writer. The writer sets it where it builds its item. For example, the `escalationItem` of `ciwatch` gets `PriorityClass: "correctness"`. The retro autofile path files the class that its intent names. The class goes on the staged `dispositionrouter.Intent` (a new field that the stager sets), through `retrofile.PreventiveAction`, into the item.

Thus the one generic filer needs no class of its own. The policy continues to own the order: it says how much a class matters, not what a finding is.

Considered and rejected: a policy map from writer to class. That map moves the judgment of each writer about its finding into operator configuration. A renamed or new writer then files silently with no class. The contract test keeps the class of a writer inside the vocabulary of the policy: a renamed class in `class_order` makes it red.

### The contract test

`TestInboxWriters_EveryAutofiledItemCarriesAKnownClass` (P2 prerequisite) builds the item of each writer through its own builder. It decodes the item as `inboxbatch.Item`. Then it requires `inboxbatch.CheckPriorityClass` to pass against the `class_order` of the checked-in policy. A table names the writers. A source scan in the style of the weight-sort ban of P2 completes the test. Any production write into `.evolve/inbox/` outside an enrolled writer is red, so a ninth autofiler cannot file without a class.

The scan pins the force (a write into the inbox), not a list of names.

All autofilers can go through `lifecycle.File`, and then the check of decision 6 sees them all. But most autofiled items do not have the fields that `File` requires (`title`, `kind`, `summary`, `fix`, `acceptance`). That change is larger than this prerequisite, and the prerequisite does not need it.

## P1 landing notes (2026-10-06)

P1 landed unwired on branch `feat/inbox-priority-rank` (console lane), from `origin/main` 6490463a8. The decision record is [ADR-0121](../architecture/adr/0121-inbox-priority-is-a-computed-rank.md). The CLI routing lane holds 0119, and in-flight work holds 0120, so the ADR keeps the number that this plan named. The review artifact of the operator is [the preview report](../reports/inbox-rank-preview-2026-10-06.md).

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

1. **The factor defaults** are base 0.45, class 0.20, unblocks 0.15, recurrence 0.10, age 0.05, goal 0.05.
2. **Two caps joined the block**: `unblocks_cap` (3) and `recurrence_cap` (5).
3. **The scalar keys are pointers**, so an explicit `preempt_margin: 0` means zero, not "absent".
4. **`priority_class` is required at filing**, not only checked when present.
5. **A lifecycle filer with no class order refuses every filing as a fault.**
6. **`Edit` does not judge the class yet.**
7. **The recurrence linkage** goes through the ledger, because items carry no fingerprint field.
8. **The golden pins a snapshot, not the live inbox.**
9. **The old order for the comparison** is `RankForDispatch` on the ready list.

The reasons for each choice:

- **Choice 1.** The factors sum to 1, so a score is in [0, 1]. The weight of the filer stays dominant, because it is still the only judgment of value for each item. One class step (0.025) is worth a weight difference of about 0.056. Thus the class reorders items of similar weight, but it does not override a clear difference in value.
- **Choice 1, an example.** A correctness item (the first class) at 0.35 outranks an otherwise equal security item (the last class) up to 0.73. Between adjacent classes, a weight gap above about 0.056 still wins.
- **Choice 1, the other factors.** Unblocks at the cap is worth a third of the weight range. Recurrence and age move ties and stop starvation; they do not override. Goal has no effect until the operator names an active campaign. The full reasoning is ADR-0121 decision 4.
- **Choice 1, the result.** On the 241 pending items, the 37 distinct weights become 214 distinct scores. The 33 items tied at 0.5 get 31 distinct scores.
- **Choice 2.** The plan says that both features are normalized and capped. The caps are tuning values, so they are configuration, like the factors, and not Go literals. Capped counts, not ratios to the maximum of the queue, keep the score of one item independent of the others.
- **Choice 3.** Each value is validated at load. A bad block fails `policy.Load` and names `inbox_priority`, as `cli_routing` does. A present `factors` block must name all six factors, so an omitted factor never silently weighs 0.
- **Choice 4.** An absent, unknown or mis-cased class is refused (exit 1). Each of the 241 pending items carries one of the eight classes, so no normalization map was necessary. The "normalizes `correctness` and `maintainability`" of the plan became a direct match, because each is in the order.
- **Choice 4, at load.** An unknown class at load ranks below every class (feature 0; the last known class scores 1/8). `evolve inbox rank` names it in a `WARN`.
- **Choice 5.** The refusal is a fault, not `ErrInvalidItem`, so no unjudged class gets in. Only `evolve inbox add` files through the lifecycle filer, and it always passes the order of the policy.
- **Choice 6.** `evolve inbox edit --set priority_class=…` still accepts any text, and the load warning catches a bad one. P5 adds the check together with the `--reason` that it requires.
- **Choice 7.** The recurrence count of an item is the largest non-generic ledger entry whose pattern equals the item id or whose `fix_item_id` names it. An autofiled recurrence item is filed under its pattern.
- **Choice 7, the reader.** The recurrence package owns the linkage (`Ledger.ItemCounts`). The verb reads the ledger with `recurrence.ReadSnapshot`, without its lock, so a rank never makes a sidecar file. `internal/inboxrank` does not import the recurrence package.
- **Choice 8.** A golden over `.evolve/inbox` changes with each lane ship and each filing, and it makes main red between landings (the class of ADR-0110). `internal/inboxrank/testdata/inbox-snapshot-2026-10-06.jsonl` is a projection of the 241 items: the fields that the rank reads, and the file name.
- **Choice 8, the goldens.** `order-2026-10-06.golden` holds `rank score id`. A separate test pins the JSON of the verb, byte for byte, against a golden computed by hand.
- **Choice 9.** The old order is weight descending, a declared surface first, then file-name order. The preview sorts the JSON rows of the verb again by those keys (the rows carry `weight`, `declared` and `path`). Thus no second rank implementation was necessary.

### The operator's correction of the class order (2026-10-06)

After the first P1 build, the operator corrected the class order to correctness > stability > performance > debuggability > feature > maintainability > hygiene > security. Security moved from first to last (operator correction, 2026-10-06). The landing applies the order at each place that sets or encodes it:

- the compiled default (`go/internal/policy/inbox_priority.go`) and the checked-in `.evolve/policy.json`;
- the tests, red first. 13 tests went red against the old order before the default changed. Among them are `TestOrder_AHygieneItemOutranksAnOtherwiseEqualSecurityItem` (the inverse of the earlier security-first test) and the new `TestOrder_ACorrectnessItemOutranksEveryOtherClass`. The others are the policy default and checked-in tests and the refusal text of the add verb. The table, explain and pinned JSON of the rank verb also went red;
- the snapshot order golden, regenerated with `-update`, and the hand-computed JSON golden of the verb;
- the preview report, generated again. 52 of 55 ready items move, and the new top ten is nine stability items and one debuggability item. The one lane-ready security item, `cli-scan-secrets`, falls from 35th to 52nd;
- ADR-0121 decisions 3 and 4, the CHANGELOG entry and this plan.

The factor weights are unchanged. A class step is still 0.025, so the order decides near-ties; a security item sits one step below an otherwise equal hygiene item.

### Open questions for P2 to P4, answered by the operator (2026-10-06)

- **P2, class labels.** The preview's biggest fallers are hygiene items from the 2026-09-26 sweep that describe silent failures (`router-silent-errors`, `phasespec-roots-silent-policy-error`, …). **Answer:** they are not relabelled automatically. The P3 overlap judge and the operator re-class an item with `evolve inbox edit <id> --set priority_class=… --reason …` (the `--reason` is P5's).
- **P2, cluster order.** **Answer:** a `Classify` cluster ranks by its maximum member score (today: its maximum member weight).
- **P2, the triage prompt.** **Answer:** the triage menu shows each candidate's score and its top contributing factor.
- **P2, recurrence on the plane.** The recurrence ledger of the live plane is runtime state that the preview did not read. The live check of P2 (`evolve inbox rank --top 20` against the triage input of the next wave) records which items the recurrence factor moved.
- **P3, the fingerprint.** **Answer:** recurrence links through a new `fingerprint` field on the item, added in P3. Until then the factor uses the P1 linkage (a ledger pattern equal to the item's id, or a pattern whose `fix_item_id` names it).
- **P4, the margin's scale.** **Answer:** the preemption margin is absolute, 0.05 on the score's [0, 1] scale, and configurable as `inbox_priority.preempt_margin` (as P1 already ships it).
- **P5, edit.** `edit --set priority_class=` runs `CheckPriorityClass` with the same order as `add`; P5 threads the policy's order into the edit path together with `--reason`.

### Review fix round (2026-10-06)

The P1 review raised the findings below; each was fixed in the same unwired landing, with tests written red first.

| Finding | What changed | Pinned by |
|---|---|---|
| W2: the rank decoded the recurrence ledger itself (`readRecurrenceLedger` in the verb, `inboxrank.LedgerRecurrence` in the rank), a second copy of `recurrence.Load`'s decode rules | `recurrence.ReadSnapshot` is the lockless decode and `Load` is the lock wrapped around it; `recurrence.Ledger.ItemCounts()` holds the pattern and `fix_item_id` linkage. Both duplicates are deleted, the verb composes `Context.Recurrence` from `ReadSnapshot(...).ItemCounts()`, and `go list -deps ./internal/inboxrank \| grep -c recurrence` is 0 | `TestReadSnapshot_DecodesAsLoadDoesWithoutTakingTheLock`, `TestReadSnapshot_AMalformedOrUnreadableLedgerIsAnError`, `TestLedger_ItemCounts_CountsThePatternAnItemCarriesOrFixes`; the verb's `TestCmd_InboxRank_ARecurrenceCountMovesTheRankWithoutWritingAnyFile` and `TestCmd_InboxRank_AnUnreadableRecurrenceLedgerIsWarnedAndCountsNothing` |
| NIT-1: the class position was computed three times (`ClassKnown`, `classValue`, the display of the verb) | `Facts.ClassPosition` (-1 when unknown) is computed one time through `inboxbatch.PriorityClassPosition`. `CheckPriorityClass` now validates with the same `slices.Index` rule. `Facts.ClassKnown()`, the class feature and the "`i` of `n`" display come from it. The JSON of the verb carries `class_position` in place of `class_known` | `TestScore_TheClassPositionIsTheOneFactBehindTheClassFeature`, `TestPriorityClassPosition_IsTheOneMembershipRuleCheckPriorityClassJudgesBy`, `TestCmd_InboxRank_JSONIsPinned` |
| NIT-2: the verb's header kept its own factor list | `inboxrank.Factors()` lists the factors in score order and the header prints from it. The policy's `inboxFactorNames` and `weights()` must index-match; the policy cannot import the rank, so the rank's test pins them | `TestFactors_ListsEveryFactorOnceInScoreOrder`, `TestFactors_AreThePolicysFactorNamesInItsOwnOrder`, `TestScore_ContributionsAddUpToTheScore`, `TestCmd_InboxRank_TableShowsRankScoreBreakdownIDAndTitle` |
| LOW: a negative count from the recurrence seam gave a negative feature | `capped` clamps a negative count to 0 | `TestScore_ANegativeRecurrenceCountIsNoRecurrence` (red at -0.6 first) |
| W3: CLAUDE.md still listed "1. New features 2. Bug fixes 3. Security issues" | The section points at the rank and the `class_order` of the policy, and the reference of the tdd skill to it has a new name. A repo-wide search of the Markdown found one more features-first statement: the Scout row of `knowledge/architecture/phase-pipeline.md`, now a pointer to the rank. The HIGH/MEDIUM/LOW-then-weight prose of the triage persona describes the unwired dispatch of that day and stays with P2 | — |
| NIT: `runtime-reference.md` restated the class order | it links [policy-config.md §inbox_priority](../architecture/policy-config.md#inbox-priority-rank-inbox_priority) instead | — |
| W1: eight autofilers file items with no `priority_class` | Not fixed in P1, which stays unwired. ADR-0121 records the gap in its Consequences. [The P2 prerequisite](#p2-prerequisite-a-class-on-every-autofiled-item) above proposes a class for each writer and the contract test | — |

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

1. **The class lives in each writer, as proposed.**
2. **The contract test drives the own path of each writer.**
3. **The scan pins the force with a heuristic data-flow.**
4. **The scan lives in `cmd/evolve`**, not in a ship-pack package.
5. **`ReadInboxBacklog` and `SelectWaveSeedMenus` keep their signatures.**
6. **The import allowlist of `loopwave` does not change.**
7. **`Classify` never ranks by weight itself.**
8. **Labels go on sub-lines.**
9. **`evolve inbox rank` continues to exit 2 on a malformed policy.**

The reasons for each choice:

- **Choice 1.** The retro autofile path carries the class on the staged intent. `core.ContractGateDemotion.Intent` is the pure builder that the stager calls. It is exported, so the contract test stages the intent of the real stager through `recurrence.ApplyBoundary`, not a hand-made intent.
- **Choice 1, validation.** The `Validate` of `fleet` and of the goal stall now require the class. `faillearn` takes the class from its caller and never gives it a default.
- **Choice 2.** The test uses the real `ciwatch.Watch` with a fake fetcher, the demotion staged and applied, and `BuildStarvationItem(...).WriteTo`. It also uses `failurelearning.Engine.WriteFloor`, `triagecap.NewDemotionLedgerRecord` and the three `cmd/evolve` writers.
- **Choice 2, the check.** The test decodes the item with `inboxbatch.LoadDir` or `LoadFile`. It checks the class of the row and `CheckPriorityClass` against the checked-in `.evolve/policy.json`.
- **Choice 3, a site.** A site is a production function that calls a write primitive on a path built from an inbox name. The write primitives are `os.WriteFile`, `os.Create`, `os.OpenFile`, `os.CreateTemp`, any `atomicwrite` call, and the target of `os.Rename` or `os.Link`.
- **Choice 3, an inbox name.** An inbox name is an identifier or field named `inbox` or `*InboxDir`, or an `inbox` or `.evolve/inbox` path literal. It is also a package constant that holds one, or a helper in the same package that returns or writes such a path.
- **Choice 3, the taint.** Taint flows through assignments, `var` declarations, struct fields, `range` and `filepath.Walk` callbacks. It also flows through path-building calls (`filepath`, `path`, `strings`, `fmt.Sprintf`). It does not flow through other call results: an item list loaded from the inbox is not a path.
- **Choice 3, the blind spot.** The scan does not follow flow through a helper of another package. Thus a writer that gets an inbox path under a neutral parameter name from another package is a known blind spot.
- **Choice 3, the enrollment.** Each site found must be the site of an autofiler row or an entry of `inboxWritersThatFileNothing` with its reason. The entries are the lifecycle package, `inboxmover.RecordRootTaskFailure`, the weight write of `recurrence.applyIntent`, the consumption `phases/ship.stage` of the ship and `evolve inbox consume`. Each enrollment must still be found.
- **Choice 4.** Three of the eight builders are package `main`. `cmd/evolve` is already a sanctioned tree reader outside the ship pack (`readsTheTreeOutsideThePack` of `repocontract`). The weight-sort guard needs no `main` builder, so it lives in `internal/inboxrank`, which joined the pack.
- **Choice 5.** The per-cycle ACS predicates 1159, 1180 and 1181 call these functions and still hold. A new rank parameter stops valid predicates from a compile. The backlog loads the inputs of the evolve dir (`rankinputs.Load(evolveDir, time.Now())`, its policy and its ledger) beside the inbox that it already reads. It prints a loader warning to the loop log.
- **Choice 5, the testable core.** The testable core is `readRankedBacklog(evolveDir, isProtected, now, loopLog)`, which takes the clock and the log writer. `ReadInboxBacklog` passes `time.Now()` and `os.Stderr`.
- **Choice 5, one reader.** The core reads items through `inboxbatch.LoadDir`, the reader of the verb and of the triage prompt, so all three rank identical items. The own decode of the backlog, which skipped the sanitizer of the loader, is gone. The core continues to drop an item with no id.
- **Choice 6.** The P2 plumbing note proposed to add `inboxrank` to the allowlist. The inputs load where the backlog is read, so the engine handles no rank type.
- **Choice 7.** With no `Config.Order`, `Classify` keeps the order of the caller. The order runs one time and becomes positions keyed by `(id, path)`. Thus a score never goes O(N²), and items with the same id keep their places. `Batch.Weight` (the maximum member weight) is removed, because in a ranked menu it looked like the priority.
- **Choice 8.** Each batch line keeps its shape `- batch N (<reasons>): id, id`. Each member gets a line `  - <id>: score 0.4475, top factor base` below it. Readers parse the batch line by its last `": "` (`go/acs/cycle1724` 004, `go/acs/cycle1678`), and a label inside the line breaks them.
- **Choice 9.** Each other consumer ranks with the compiled default and a warning, because a consumer inside the loop must not stop on a bad policy.

### Goldens and predicates

| Artifact | Change | Why |
|---|---|---|
| `go/internal/core/failurelearning/testdata/inbox-915304b4.golden.json`, `inbox-4f0ecc16.golden.json` | gained `"priority_class": "correctness"` | the remediation items now carry their class (row 4) |
| `go/internal/loopwave/testdata/decision_{seed,widen,prune}.golden.json` | unchanged, checked again | Their fixtures carry no class and no date, so the rank equals the old weight order on them. New tests with classed fixtures pin the rank order |
| `go/acs/cycle536` 007, 008 and `go/acs/cycle1724` 002 | archived to [superseded predicates](../private/research/archived-2026-10-06/superseded-predicates/README.md) under the A2 retention rule | each asserted the weight order as its acceptance (the seed led by the heaviest item; an un-ranked `Classify` ordered weight-descending) |
| `go/acs/cycle536`'s seven other predicates | compile and pass again | the two archived predicates had kept the package from compiling since `SelectWaveSeedTopN` gained `isProtected`, before P2 |
| `go/acs/cycle1159`, `1180`, `1181`, `1182`, `1206`, `1633` | checked, unchanged | They assert that the committed prefix stays, that consumed ids go, rule-set membership or batch independence. All pass as before. `1181` 003 needs a live `.evolve/state.json`, so it fails in a dev worktree, as on the base |
| `go/acs/cycle541` | checked, unchanged | Its candidates are already in order, so it asserts disjointness, not the weight order. It has not compiled since `fleet.PlanFromTriage` got new arguments, and that is not related to P2 |

The unit tests that pinned the weight order now pin the rank-order contract. These are the rank and amplified tests of `triagecap`, the classify tests of `inboxbatch` and the queue test of the dashboard. Each has a new name for what it now pins.

### The live check (2026-10-06, read-only)

On the runtime plane's inbox (232 pending items: 49 lane-ready, 180 console-owned, 3 waiting), with the branch's binary:

- `evolve inbox rank --top 20` and the backlog of the wave seed list the same 20 items in the same order. The backlog is `triagecap.ReadInboxBacklog` with the `laneForbidden` predicate of the loop. The list goes from `cli-dossier-publish`, `overlay-rule-tier-selectors-canonicalize` and `repo-contract-test-level-selection` down to `phasecmd-silent-policy-fallbacks`.
- `evolve inbox batches`, the composition of the triage prompt, puts all 49 ready items in 37 batches. Members are in rank order, and each cluster is at the rank of its best member.
- 45 of the 49 ready items moved against the old weight order. The old top ten (`router-silent-errors`, `phasespec-roots-silent-policy-error`, `overlay-rule-tier-selectors-canonicalize`, `policy-resolver-hygiene`, `premium-rung-placement-law-and-narrative-dedup`, …) was mostly hygiene items; the new top ten leads with `cli-dossier-publish`, `overlay-rule-tier-selectors-canonicalize`, `repo-contract-test-level-selection`, `cli-phase-cycle-request` and `acs-cycle1723-package-doc-predicate-stale-after-753`.
- **Recurrence moved nothing:** the plane has no `recurrence-ledger.json`, so the factor is 0 for every item (the question P1 left open for P2).
- Every pending item on the plane carries a known class, so no `WARN` was printed.

### Review and floor

- **The diff review** (one combined Go reviewer) passed with no CRITICAL or HIGH finding. Its two MEDIUM findings stay as choices:
  - `triagecap.ReadInboxBacklog` and the triage prompt print the input warnings of the rank to the stderr of the process (the loop log). Their signatures carry no writer, and the signature of the backlog stays compatible with the ACS predicates above.
  - The backlog drops the notices of `inboxbatch.LoadDir` for each file. The plane prints about forty sanitize notices for each read, and the loop chain already reports an invalid root item. A read error still prints.
- **The two LOW findings of the diff review** are recorded above. The verb reads the policy two times to keep its exit 2. The weight-sort guard is a syntactic scan, not a data-flow proof.
- **Ratchets that the landing moved.**
  - The function-size ratchet: `writePipelineEscalation` grew by its class line. A redundant comment went, so it is back at its allowance.
  - `inboxbatch.Classify` (allowance 93) is under the limit after the split, so its allowance was removed.
  - The pinned pack list of the ship (`TestContractRed_NamesEverySuiteOfTheOnePackList`) names `inboxrank`.
  - The file of the guard is `weightsort_ban_test.go`. A `go/internal` file name that contains `guard` is gate-shaped, and it must be on the protected-surface manifest (`TestEveryGateShapedFileIsProtectedSurface`).
  - The operator then protected the file with a file entry in `guards.ProtectedSurfaceManifest` (see the fix round below).
- **The floor, on the staged tree, one target at a time.** This is the final run, after the fix round of the architecture review and the engineering-craft audit:
  - `make test` (258 packages ok), `make test-integration` (259 ok), `make test-e2e` (6 ok) and `make test-acs-durable` (42 ok);
  - `make apicover-enforce` (258 packages, 4859 of 4859 exports covered, 0 false-green);
  - `make cover-strict` (24 of 24 at their floors). `inboxrank`, `rankinputs`, `loopwave`, `core/failurelearning` and `inboxmover/lifecycle` are at 100%;
  - go1.23 `gofmt -l -s` clean, and `go vet ./...` clean;
  - `golangci-lint` clean on each touched package. The whole-tree run reports four findings in files that the landing did not touch: `signalcenter/stream.go` and `pkg/acsassert/go_tests.go`.
  - One earlier run lost `internal/core` to the 10-minute package timeout of Go, under a machine load average of 16 to 21. It passed in 426 s on the next run.

### Architecture review fix round (2026-10-06)

The architecture review judged the landing FIX_THEN_MERGE (0 CRITICAL, 4 WARNING, 4 NIT). It confirmed that the dispatch result did not change, that one source exists, and that the archived predicates moved verbatim. Each fix was written red first. The reds are in the scratchpad of the lane (`fix-W1.txt`, `fix-W2.txt`, `fix-W3.txt`, `fix-NIT.txt`).

| Finding | What changed | Pinned by |
|---|---|---|
| W1: a surviving mutant. `Order(menu.Ready, menu.Ready)` at the triage prompt or at `evolve inbox batches` passed every test, because no fixture had a waiting dependent | One helper, `inboxmover.RankLaneMenu`, partitions and then ranks the ready list against the whole queue. The triage prompt, `evolve inbox batches` and `triagecap.ReadInboxBacklog` use it. The tests of both consumers got a waiting item whose `deps` name a lighter ready item, which must lead | `TestRankLaneMenu_RanksTheReadyListAgainstTheWholeQueue`. The mutant inside the helper makes it red, and also the triage, batches and backlog rank tests. Before the helper, the same mutant at each old site made the extended triage and batches tests red |
| W2: only the producers refused a missing class | The shared funnels refuse it. `dispositionrouter.StageIntent` refuses an autofile intent with no class (`inboxbatch.ErrNoPriorityClass`). `retrofile.FileActions` refuses an action with no class after its dedupe. `recurrence.applyIntent` records a legacy classless line as `ApplyResult.Refused` and never stops the boundary. The inbox writer of faillearn refuses a batch with a classless item before any write. The `preventive_actions` schema of the retrospective persona requires `priority_class` | `TestStageIntent_RefusesAnAutofileWithNoPriorityClass`, `TestFileActions_RefusesAnActionWithNoPriorityClassButNotAnAlreadyFiledOne`, `TestApplyBoundary_AClasslessAutofileStagedBeforeClassesIsRefusedNotFiledAndDoesNotWedge`, `TestWriteInboxItems_RefusesAnItemWithNoPriorityClassAndWritesNone` |
| W3: no runtime backstop for the blind spots of the force scan | `triagecap.ReadInboxBacklog` prints `inboxrank.ClassWarnings` over the whole pending queue to the loop log at each read. The inbox section of the triage prompt starts with an `- unknown_priority_class:` line. The inbox names of the scan now include `inbox(dir\|path\|root)`. [cmd-evolve.md](../architecture/packages/cmd-evolve.md) records the blind spots: imported path builders, write primitives outside `os`, and function values | `TestReadRankedBacklog_NamesEveryUnclassedQueuedItemInTheLoopLog`, `TestTriageComposePrompt_NamesQueuedItemsWithNoKnownClass`, `TestInboxWriteSites_FindsEveryShapeOfAnInboxWriteAndNoRead` (now with `inboxPath` and `inboxRoot` writers) |
| W4: `internal-loopwave.md` still described the refill's `RankForDispatch` order | the refill takes the first non-excluded candidate of `triagecap.ReadInboxBacklog`, in rank order | — |
| NITs | `LoadFile` records the file-name fallback as `Item.IDFromFileName`, and the backlog filters on it. `recordsAnID`, which decoded each file again, is deleted. One exported identity key, `inboxbatch.ItemKey` (`Item.Key()`), replaces the two private copies in `inboxbatch` and `inboxrank`. The class names `inboxbatch.ClassCorrectness` … `ClassSecurity` are beside `CheckPriorityClass`, and each autofiler and the contract table use them. The import allowlist of `core/failurelearning` got `inboxbatch`. One inbox item asks the operator if the ship-pack scanners must be protected as a class (`decide-protecting-ship-pack-scanners-as-a-class`, console-owned, maintainability) | `TestLoadFile_RecordsAnIDTakenFromTheFileName`, `TestItem_KeyIsTheIDAndThePath`, `TestClassNames_AreThePolicysCompiledClassOrder` |

**More superseded acceptance.** After W2, a classless remediation item or autofile intent is invalid input. Thus four more per-cycle predicates that filed one are archived under the A2 retention rule ([superseded predicates](../private/research/archived-2026-10-06/superseded-predicates/README.md)). They are `cycle1062` 008 and 009, `cycle1290` 001 and `cycle1292` 001. Classed unit tests pin their behavior, and the live `cycle1290` 003 and `cycle1292` 003 still run those tests by name.

`cycle1290` 004 compares `inbox_transactional_test.go` with `HEAD`, and its fixture got `priority_class`. Thus it is red until the change is committed. `cycle1292` 004 is red in any checkout for a data reason that is older than P2. The title of a tracked inbox item is over 160 characters.

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
- **Assertion-level reds.** Several first reds were compile errors, which do not count as red. A `go test -overlay` mutant of the staged code proved each new behavior again. There were 38 mutants, and the test named for the behavior killed each one; the first one was a control.
  - Reorder mutants: the class check before the dedupe of retrofile, and the check of faillearn inside the write loop. Also the triage class line after the lists.
  - A shift mutant: `>` to `>=` in the top factor.
  - Wrong-key mutants: an `ItemKey` without the path.
  - A wrong-list mutant: `RankLaneMenu` that ranks the ready list against itself.
- **Four functions keep four parameters, as natural signatures.**
  - `inboxmover.RankLaneMenu` is `PartitionLaneMenu`'s three plus the rank.
  - `triagecap.readRankedBacklog` is the backlog's two plus the injected clock and log.
  - `loadRankInputs` in `cmd/evolve` is the verb label, the directory, the clock and the sink.

**The delta re-review (2026-10-06): FIX_THEN_MERGE, closed here.** The re-review closed W1–W4 and the NITs. It found that four packages killed the original W1 mutant. The 11 live legacy classless intents became `Skipped`, and none was refused. The seven archived predicates were byte-identical. The re-review raised one HIGH and five surviving mutants:

| Finding | Fix | Evidence |
|---|---|---|
| H1: the loop log did not show a refused autofile intent. `applyEscalationBoundary` stayed quiet when only `Refused` had entries, and its signal had no `refused` field | The quiet check counts `Refused`. One `[loop] WARN escalation boundary: refused <id>: no priority_class` line goes out for each id. `LOOP_ESCALATION_BOUNDARY` has a `refused` field (registry text and `signal-codes.md` generated again) | `TestApplyEscalationBoundary_NamesEachRefusedIntentInTheLoopLogAndItsSignal`, red on the prior code. The probe of the reviewer passes. Three mutants are killed (quiet check, WARN line, field) |
| M3: the triage class line can warn over the ready list only | A waiting item with no class must be named | `TestTriageComposePrompt_NamesQueuedItemsWithNoKnownClass` kills it |
| M7: faillearn can check `Priority` in place of `PriorityClass` | Both fixture items carry `Priority: "H"` | `TestWriteInboxItems_RefusesAnItemWithNoPriorityClassAndWritesNone` kills it |
| M6: recurrence can refuse before the dedupe of retrofile | A legacy classless intent whose item is already filed is `Skipped`, not `Refused` | `TestApplyBoundary_AClasslessLegacyAutofileWhoseItemIsFiledIsSkippedNotRefused` kills it |
| M10, M11: the triage prompt and `evolve inbox batches` can load rank inputs from the wrong directory | Each writes a `policy.json` whose `class_order` puts security first, and requires the heavier security item to lead | `TestTriageComposePrompt_RanksWithTheEvolveDirsPolicy`, `TestInboxBatches_RanksWithTheEvolveDirsPolicy` kill them |
| MEDIUM (Boy Scout): `writePipelineEscalation` was 62 lines | extracted into `haltRecord` with `narrative`, `dossier` and `inboxItem`, now 14 lines; its size-ratchet allowance removed | the `TestWritePipelineEscalation_*` tests pass before and after |
| LOW: this plan named `rankedBacklog` with fixed `Inputs` | reworded to `readRankedBacklog(evolveDir, isProtected, now, loopLog)` | — |

### Open question for P4 and P5: starvation by design

The architecture review judged that the rank can starve a class by design (ADR-0121). The rank was not changed. The question is the operator's:

- **The arithmetic.** The class factor is 0.20 over eight classes, so one class step is 0.025. At equal weight, a correctness item leads a feature item by four steps (0.10). It leads a hygiene item by six steps (0.15) and a security item by seven (0.175).
- **Age cannot close the gap.** The age factor has a cap of 0.05 (`1 − 0.5^(days/30)` × 0.05). It is 0.025 after 30 days, 0.0375 after 60 days, and it only goes toward 0.05 after that. Thus age closes at most two class positions, and only in the limit. An item four or more steps behind never passes a fresh item of equal weight on age alone.
- **The consequence.** While correctness and stability items continue to arrive at similar weights, feature, hygiene and security items wait with no limit. Other factors can still move them. These are a weight about 0.056 higher for each class step, or a dependent that they unblock (up to 0.15). A recurrence count (up to 0.10) or the `active_campaigns` of the operator (0.05) can also move them.
- **The question.** Is that starvation the intended result of the class order? Or must P4 or P5 add an anti-starvation term? The options include a larger or uncapped age factor, an age clock for each class, or a share of lane slots for each class. Each of these is a policy change for the operator to approve. This plan makes none of them.

### What remains

P3 (one intake path, the overlap judge), P4 (boundary preemption) and P5 remain. P5 adds `--reason` to each priority change and deletes the last weight write outside the lifecycle: the `Reweight` option of `recurrence.ApplyOptions`.

The writers `ciwatch`, `retrofile`, `dispositionrouter` and `core/failurelearning` have no package page under `docs/architecture/packages/`. Their class is recorded here and in the amendment of ADR-0121. It is also in `internal-core.md`, `internal-recurrence.md`, `internal-faillearn.md`, `internal-fleet.md`, `internal-triagecap.md` and `cmd-evolve.md`.

### Inbox items for the open components (2026-10-27)

| Component | Item | Route |
|---|---|---|
| P3 | `inbox-prioritization-p3-add-time-intake` | console-manual: the console builds it |
| P3 to P5, event triggers | `inbox-curator-event-triggered` | console-manual: the console builds it |
| P4 | `inbox-prioritization-p4-boundary-preemption` | console: protected fix surface `go/internal/loopwave/plan.go` |
| P5 | `inbox-prioritization-p5-reason-on-priority-change` | lane |
| P6 | `inbox-prioritization-p6-intake-docs` | lane |

P4, P5 and P6 depend on P3. The item `inbox-curator-event-triggered` starts each review of the inbox from an event. The events are an add, a landing, a change of the policy or of the protected manifest, and the boundary.
