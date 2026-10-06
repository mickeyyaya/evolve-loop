# Inbox prioritization: one computed rank, re-ranked on every add (2026-10)

> **Status:** approved by the operator on 2026-10-06. The request: "Do we have logic to reprioritize the Todo inbox tasks when adding the new requests? If not, we need to build the logic… with ultrathink".
>
> **Decision record:** ADR-0121, which lands with P1.

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

1. **Class order: correctness first, security last:** correctness > stability > performance > debuggability > feature > maintainability > hygiene > security (operator correction, 2026-10-06). The order first approved that morning was "pipeline health first", security > correctness > stability > performance > debuggability > feature > maintainability > hygiene; the operator corrected it the same day, before P1 landed, and P1 ships the corrected order. CLAUDE.md's "/evo:loop task priority" line (features first) is updated to match.
2. **Preemption.** At a wave boundary, a lane-eligible item that outranks the lowest **uncommitted** planned slot by the policy margin takes that slot. Running lanes and adopted continuations are never displaced.
3. **Overlap at add time depends on origin.**
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
| P1 | **`internal/inboxrank`** (pure, unwired) and **`evolve inbox rank [--json] [--explain <id>] [--top N]`** | <ul><li>Add `inbox_priority` to policy, with strict decode, class order, factor weights, age half-life, `active_campaigns` and preemption margin.</li><li>Add a `PriorityClass` field on `inboxbatch.Item`, as a validated enum that normalizes `correctness` and `maintainability`.</li><li>The score and `Order` function.</li><li>`--explain` prints the factor breakdown.</li><li>A golden test records today's order and the new order for every root item.</li><li>The PR shows the top-30 deltas for operator review.</li></ul> |
| P2-pre | **PREREQUISITE of P2: a class on every autofiled item** | <ul><li>Every autofiler stamps a `priority_class` from the policy order. The writer chooses it for the kind of finding it files, as it chooses the weight today; the proposed class per writer is in [the table below](#p2-prerequisite-a-class-on-every-autofiled-item).</li><li>A contract test requires every inbox writer's item to carry a class the checked-in `class_order` names, and a source scan reds on any production write into `.evolve/inbox/` that the test does not enroll.</li><li>Lands before P2 wires the rank: without it, eight autofilers' items score a class feature of 0 and sink below mid-weight items (ADR-0121 Consequences).</li></ul> |
| P2 | **One ranking for every consumer** | <ul><li>`RankForDispatch`, `Classify` cluster order, the triage injection, and the wave seed and refill paths all use `inboxrank.Order`.</li><li>A source-scan guard bans any other weight sort.</li><li>The stale triage priority prose (`evolve-triage.md`, `evolve-triage-reference.md`) points to the rank.</li></ul> |
| P3 | **The add-time intake** | <ul><li>A validated `origin` field: `operator`, `console`, `cycle` or `retro`. Autofilers set it, and `evolve inbox add --origin` defaults to `console`.</li><li>A deterministic overlap pre-check (shared `files` and dirs as Jaccard, `connects_to`, title-token similarity) above a policy threshold calls the **`inbox-overlap-judge`**. This is a new deep-tier profile on the Claude family, with a JSON contract: verdict, rationale, related ids, confidence.</li><li>Machine origin: the judge's verdict is applied through the lifecycle writer (`supersedes` → withdraw with reason; `merge_into` → append to the target's acceptance plus a ledger line; `relates` → `connects_to`), and the reasoning is stored in `overlap_review`.</li><li>Human origin: `add` exits 3 and writes `.evolve/inbox/pending-intake/<id>.json` with the judge's justification and options. `evolve inbox resolve <id> --supersedes X \| --merge-into X \| --relates X \| --distinct \| --withdraw` finalizes it.</li><li>Every add prints where the item lands and the top movers.</li></ul> |
| P4 | **Boundary preemption** | <ul><li>When the wave plan reuses the previous decision, re-check it against `inboxrank.Order`.</li><li>A lane-eligible item whose score beats the lowest uncommitted planned slot by `inbox_priority.preempt_margin` takes that slot.</li><li>The ledger records `preempted <old> for <new> (score a→b)`.</li><li>Committed in-flight lanes and adopted continuations are never displaced.</li></ul> |
| P5 | **Reasons on every priority change** | <ul><li>`evolve inbox edit --set weight=… / priority_class=…` requires `--reason`, and the ledger records it.</li><li>Recurrence becomes a rank factor (P1), so `recurrence.setItemWeight` is deleted.</li></ul> |
| P6 | **Docs** | <ul><li>ADR-0121.</li><li>`runtime-reference.md`, which drops the false "dep-topological" claim.</li><li>The CLAUDE.md task-priority line.</li><li>`internal-inboxbatch.md` and a new `internal-inboxrank.md`.</li><li>The console skill instruction: on an exit-3 pending intake, ask the user with the recorded justification and options.</li></ul> |

## Landing order

1. **P1:** unwired. The operator reviews the new order in the PR.
2. **P2 prerequisite:** every autofiler stamps a `priority_class`, with its contract test. It changes no order while the rank is unwired, so it can land with P1's follow-ups or as P2's first commit, but never after P2.
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
- **P2 prerequisite:**
  - each autofiler's item builder, driven by its test, yields an item whose `priority_class` the checked-in `class_order` names;
  - the source scan is red on a fixture that writes an item into `.evolve/inbox/` outside the enrolled writers.
- **P2:**
  - every consumer returns `inboxrank.Order`;
  - the source-scan guard is red on a stray weight sort.
- **P3:**
  - a machine-origin overlap is judged (fake judge) and the decision is applied with recorded reasoning;
  - a human-origin overlap exits 3 and writes a pending intake;
  - `resolve` applies each option;
  - with no overlap, no judge call is made.
- **P4:** a fixture plan with a planned-but-unstarted slot and a higher new item gets preempted; a running lane and a continuation do not.
- **P5:** an edit without `--reason` is refused; recurrence moves the rank without writing a weight.
- **Live:** after P2, `evolve inbox rank --top 20` matches what the next wave's triage receives.

## P2 prerequisite: a class on every autofiled item

Found in the P1 review (2026-10-06). `evolve inbox add` is the only path that files through `lifecycle.File`, so it is the only one the class check sees. Eight production autofilers write pending items themselves (with `os.WriteFile`, `atomicwrite.JSON` or `faillearn.writeIfAbsent`), and none sets `priority_class`. P1 stays unwired, so their items only show a `WARN` in `evolve inbox rank` today; once P2 wires the rank they score a class feature of 0. A fresh `ci-red` item at weight 0.95 then scores 0.4275, below a fresh correctness item at weight 0.6 (0.47), and a fail-learning defect at 0.75 (0.3375) ranks below a stability item at 0.5 (0.40).

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

The review's list named `internal/retrofile` and `recurrence/apply.go` separately and named `dispositionrouter` as a writer; they are one write path (row 2), and the disposition router only stages the intent. Rows 7 and 8 were found when the writers were mapped for this table.

### Where the class lives

In the writer, beside the `kind` and the weight it already sets: each writer files one kind of finding, so its class is a fact of the writer, set where its item is built (for example `ciwatch`'s `escalationItem` gains `PriorityClass: "correctness"`). The retro autofile path files whatever its intent names: the class rides on the staged `dispositionrouter.Intent` (a new field the stager sets) through `retrofile.PreventiveAction` into the item, so the one generic filer needs no class of its own. The policy keeps owning the order, that is how much a class matters, not what a finding is.

Considered and rejected: a policy map from writer to class. It would move each writer's judgment of what it found into operator configuration, and a renamed or new writer would silently file with no class. The contract test is what keeps a writer's class inside the policy's vocabulary: renaming a class in `class_order` reds it.

### The contract test

`TestInboxWriters_EveryAutofiledItemCarriesAKnownClass` (P2 prerequisite) builds each writer's item through its own builder, decodes it as `inboxbatch.Item` and requires `inboxbatch.CheckPriorityClass` against the checked-in policy's `class_order` to pass. A table names the writers, so the test is completed by a source scan in the style of P2's weight-sort ban: any production write into `.evolve/inbox/` outside an enrolled writer is red, so a ninth autofiler cannot file without a class. The scan pins the force (a write into the inbox), not a list of names.

Routing every autofiler through `lifecycle.File` would put them all behind decision 6's check, but most autofiled items lack fields `File` requires (`title`, `kind`, `summary`, `fix`, `acceptance`), so that is a larger change than this prerequisite and is not needed for it.

## P1 landing notes (2026-10-06)

P1 landed unwired on branch `feat/inbox-priority-rank` (console lane), from `origin/main` 6490463a8. The decision record is [ADR-0121](../architecture/adr/0121-inbox-priority-is-a-computed-rank.md) (0119 is held by the CLI routing lane and 0120 is reserved for in-flight work, so the ADR keeps the number this plan named). The operator's review artifact is [the preview report](../reports/inbox-rank-preview-2026-10-06.md).

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

1. **The factor defaults** are base 0.45, class 0.20, unblocks 0.15, recurrence 0.10, age 0.05, goal 0.05; they sum to 1, so a score is in [0, 1]. The filer's weight stays dominant because it is still the only per-item judgment of value: one class step (0.025) is worth a weight difference of about 0.056, so the class reorders items of similar weight without overriding a clear difference in value: a correctness item (the first class) at 0.35 outranks an otherwise equal security item (the last) up to 0.73, and between adjacent classes a weight gap above about 0.056 still wins. Unblocks at the cap is worth a third of the weight range. Recurrence and age are tie-movers and anti-starvation, not overrides. Goal is inert until the operator names an active campaign. The full reasoning is ADR-0121 decision 4. On the 241 pending items the 37 distinct weights become 214 distinct scores, and the 33 items tied at 0.5 get 31.
2. **Two caps joined the block**: `unblocks_cap` (3) and `recurrence_cap` (5). The plan says both features are normalized and capped; the caps are tuning knobs, so they are configuration, like the factors, rather than Go literals. Capped counts, not ratios to the queue's maximum, keep one item's score independent of the others'.
3. **The scalar keys are pointers**, so an explicit `preempt_margin: 0` means zero rather than "absent". Every value is validated at load: the block fails `policy.Load` naming `inbox_priority`, as `cli_routing` does. A present `factors` block must name all six factors, so an omitted one never silently weighs 0.
4. **`priority_class` is required at filing**, not only checked when present: absent, unknown and mis-cased classes are all refused (exit 1). Every one of the 241 pending items carries one of the eight classes, so no normalization map was needed (the plan's "normalizes `correctness` and `maintainability`" became a direct match: each is in the order). An unknown class at load ranks below every class (feature 0; the last known class scores 1/8) and `evolve inbox rank` names it in a `WARN`.
5. **A lifecycle filer built without a class order refuses every filing as a fault** (not `ErrInvalidItem`), rather than admit an unjudged class. Only `evolve inbox add` files through it, and it always passes the policy's order.
6. **`Edit` does not judge the class yet.** `evolve inbox edit --set priority_class=…` still accepts any text; the load warning catches a bad one. P5 adds the check together with the `--reason` it requires.
7. **The recurrence linkage.** Items carry no fingerprint field, so an item's recurrence count is the largest non-generic ledger entry whose pattern equals the item's id (an autofiled recurrence item is filed under its pattern) or whose `fix_item_id` names it. The recurrence package owns the linkage (`Ledger.ItemCounts`); the verb reads the ledger with `recurrence.ReadSnapshot`, without its lock, so ranking never creates a sidecar file, and `internal/inboxrank` does not import the recurrence package.
8. **The golden pins a snapshot, not the live inbox.** A golden over `.evolve/inbox` would change with every lane ship and every filing and would red main between landings (ADR-0110's class). `internal/inboxrank/testdata/inbox-snapshot-2026-10-06.jsonl` is a projection of the 241 items (the fields the rank reads, and the file name); `order-2026-10-06.golden` holds `rank score id`. The verb's JSON is pinned separately, byte for byte, against a hand-computed golden.
9. **Today's order for the comparison** is `RankForDispatch` on the ready list: weight descending, a declared surface first, then file-name order. The preview re-sorts the verb's own JSON rows by those keys (the rows carry `weight`, `declared` and `path`), so no second ranking implementation was written.

### The operator's correction of the class order (2026-10-06)

After the first P1 build, the operator corrected the class order to correctness > stability > performance > debuggability > feature > maintainability > hygiene > security: security moved from first to last (operator correction, 2026-10-06). The landing applies it everywhere the order is set or encoded:

- the compiled default (`go/internal/policy/inbox_priority.go`) and the checked-in `.evolve/policy.json`;
- the tests, red first: 13 tests went red against the old order before the default changed, among them `TestOrder_AHygieneItemOutranksAnOtherwiseEqualSecurityItem` (the inverse of the earlier security-first test), the new `TestOrder_ACorrectnessItemOutranksEveryOtherClass`, the policy default and checked-in tests, the add verb's refusal text and the rank verb's table, explain and pinned JSON;
- the snapshot order golden, regenerated with `-update`, and the hand-computed JSON golden of the verb;
- the preview report, regenerated: 52 of 55 ready items move, the new top ten is nine stability items and one debuggability item, and the one lane-ready security item, `cli-scan-secrets`, falls from 35th to 52nd;
- ADR-0121 decisions 3 and 4, the CHANGELOG entry and this plan.

The factor weights are unchanged. A class step is still 0.025, so the order decides near-ties; a security item sits one step below an otherwise equal hygiene item.

### Open questions for P2 to P4, answered by the operator (2026-10-06)

- **P2, class labels.** The preview's biggest fallers are hygiene items from the 2026-09-26 sweep that describe silent failures (`router-silent-errors`, `phasespec-roots-silent-policy-error`, …). **Answer:** they are not relabelled automatically. The P3 overlap judge and the operator re-class an item with `evolve inbox edit <id> --set priority_class=… --reason …` (the `--reason` is P5's).
- **P2, cluster order.** **Answer:** a `Classify` cluster ranks by its maximum member score (today: its maximum member weight).
- **P2, the triage prompt.** **Answer:** the triage menu shows each candidate's score and its top contributing factor.
- **P2, recurrence on the plane.** The live plane's recurrence ledger is runtime state the preview could not read. P2's live check (`evolve inbox rank --top 20` against the next wave's triage input) notes which items the recurrence factor moved.
- **P3, the fingerprint.** **Answer:** recurrence links through a new `fingerprint` field on the item, added in P3. Until then the factor uses the P1 linkage (a ledger pattern equal to the item's id, or a pattern whose `fix_item_id` names it).
- **P4, the margin's scale.** **Answer:** the preemption margin is absolute, 0.05 on the score's [0, 1] scale, and configurable as `inbox_priority.preempt_margin` (as P1 already ships it).
- **P5, edit.** `edit --set priority_class=` runs `CheckPriorityClass` with the same order as `add`; P5 threads the policy's order into the edit path together with `--reason`.

### Review fix round (2026-10-06)

The P1 review raised the findings below; each was fixed in the same unwired landing, with tests written red first.

| Finding | What changed | Pinned by |
|---|---|---|
| W2: the rank decoded the recurrence ledger itself (`readRecurrenceLedger` in the verb, `inboxrank.LedgerRecurrence` in the rank), a second copy of `recurrence.Load`'s decode rules | `recurrence.ReadSnapshot` is the lockless decode and `Load` is the lock wrapped around it; `recurrence.Ledger.ItemCounts()` holds the pattern and `fix_item_id` linkage. Both duplicates are deleted, the verb composes `Context.Recurrence` from `ReadSnapshot(...).ItemCounts()`, and `go list -deps ./internal/inboxrank \| grep -c recurrence` is 0 | `TestReadSnapshot_DecodesAsLoadDoesWithoutTakingTheLock`, `TestReadSnapshot_AMalformedOrUnreadableLedgerIsAnError`, `TestLedger_ItemCounts_CountsThePatternAnItemCarriesOrFixes`; the verb's `TestCmd_InboxRank_ARecurrenceCountMovesTheRankWithoutWritingAnyFile` and `TestCmd_InboxRank_AnUnreadableRecurrenceLedgerIsWarnedAndCountsNothing` |
| NIT-1: the class position was computed three times (`ClassKnown`, `classValue`, the verb's display) | `Facts.ClassPosition` (-1 when unknown) is computed once through `inboxbatch.PriorityClassPosition`, the `slices.Index` rule `CheckPriorityClass` now validates with; `Facts.ClassKnown()`, the class feature and the "`i` of `n`" display derive from it. The verb's JSON carries `class_position` in place of `class_known` | `TestScore_TheClassPositionIsTheOneFactBehindTheClassFeature`, `TestPriorityClassPosition_IsTheOneMembershipRuleCheckPriorityClassJudgesBy`, `TestCmd_InboxRank_JSONIsPinned` |
| NIT-2: the verb's header kept its own factor list | `inboxrank.Factors()` lists the factors in score order and the header prints from it. The policy's `inboxFactorNames` and `weights()` must index-match; the policy cannot import the rank, so the rank's test pins them | `TestFactors_ListsEveryFactorOnceInScoreOrder`, `TestFactors_AreThePolicysFactorNamesInItsOwnOrder`, `TestScore_ContributionsAddUpToTheScore`, `TestCmd_InboxRank_TableShowsRankScoreBreakdownIDAndTitle` |
| LOW: a negative count from the recurrence seam gave a negative feature | `capped` clamps a negative count to 0 | `TestScore_ANegativeRecurrenceCountIsNoRecurrence` (red at -0.6 first) |
| W3: CLAUDE.md still listed "1. New features 2. Bug fixes 3. Security issues" | the section points at the rank and the policy's `class_order`; the tdd skill's reference to it is renamed. No other features-first statement exists anywhere in the repository (a repo-wide grep of the Markdown found one more, `knowledge/architecture/phase-pipeline.md`'s Scout row, now a pointer to the rank); the triage persona's HIGH/MEDIUM/LOW-then-weight prose describes today's unwired dispatch and stays P2's | — |
| NIT: `runtime-reference.md` restated the class order | it links [policy-config.md §inbox_priority](../architecture/policy-config.md#inbox-priority-rank-inbox_priority) instead | — |
| W1: eight autofilers file items with no `priority_class` | not fixed in P1, which stays unwired: ADR-0121 records the gap in its Consequences and [the P2 prerequisite](#p2-prerequisite-a-class-on-every-autofiled-item) above proposes a class per writer and the contract test | — |

The explicit `float64(value × weight)` conversion in the score is unchanged: it is the rounding point that keeps the contributions summing to the score exactly (`TestScore_ContributionsAddUpToTheScore`).
