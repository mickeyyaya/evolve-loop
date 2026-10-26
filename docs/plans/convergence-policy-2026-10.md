# Convergence policy: every iterative loop converges or escalates (plan, 2026-10)

> **Purpose.** This is the living plan for ADR-0126.
> - **What it adds:** one convergence policy for every repeat-until-accepted loop, in the pipeline and in the console.
>   - Loops land in at most three fix rounds.
>   - Each round past the first changes something: the feedback, then the strategy, then the scope.
>   - The best qualifying round lands.
>   - What the loop did not resolve is filed as follow-ups, never held back as "no output".
> - **Priority:** set by the operator on 2026-10-07.
> - **Revision 2:** applies the doc review (C1, C2, H1–H5, M1, M2, L1–L3), the research refinements CR1–CR8 and the operator's headroom answer.
> - **Design (the core logic):** [convergence-policy.md](../architecture/convergence-policy.md).
> - **Decision record:** [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md).
> - **The example issue:** [L2's seven review rounds](../incidents/2026-10-07-l2-seven-review-rounds.md).
> - **Research:** [converging review feedback within limited rounds](../research/review-convergence-limited-rounds-2026-10.md).
> - **Updates:** every landing updates §6's status column and §10.
>
> Operator requests (2026-10-07):
> - *"Could you investigate L2 fix round 7 if it's reasonable or we have escalate with deep model to resolve it."*
> - *"It just retry for too many times."*
> - *"Prioritize a convergence rule/solution using L2 with 7 rounds as the example issue that should converge earlier by escalating to top / deep model or other approaches to proceed with persuing the perfection with no output."*
> - *"The latest top model is opus 5.5 with xhigh effort, not fable 5.1"*
> - *"I would like you to research online to learn the best policy to coverage the review feedback with limited rounds"*
> - *"Convergence rule should also apply to evo loop cycle pipeline to avoid infinite back and forth endless loop"*

> **Terms:** as in the design doc's Terms block:
> - judgment `J_r` (with `J_0` the full first judgment), round, finding (`kind`, `component`, `late`, `blocking`);
> - blocking bar, mass, progress, repair damage, concentration, headroom (model, effort), follow-up.
>
> *Console*, *plane*, *lane* and *floor* mean what they mean in the code-review plan.

## 1. Why

- **Loops grind, and the data shows it.**
  - Audit repair's ship probability fell **100% → 50% → 17% → 0%** by audit-round count (ADR-0096).
  - L2 needed **seven** fix rounds on the deepest model (the incident).
- **The research is consistent:**
  - two rounds capture 76–95% of the gain;
  - fixed budgets can lower quality, because repair damages correct work;
  - LLM reviewers vary from run to run;
  - more revisions do not converge without reduced uncertainty;
  - strong review cultures approve "better, not perfect".
- **Four loops have four bespoke stops.** None of them raises the bar, stops on marginal gain, keeps the best round, or checks headroom.

## 2. Strategies considered

| | A. Round cap only | **B. Escalation ladder (chosen)** | C. Best-of-N fixers + verifier |
|---|---|---|---|
| What changes per round | nothing, then stop | the feedback (verify-only re-judgment, effort raise), then the strategy (fresh context, HIGH bar), then the scope (split, or accept with limits) | N parallel attempts |
| Which candidate lands | the last | **the best qualifying round** | the best of N |
| L2 replayed | stops with nothing landed | **lands at round 3 through rung 2**, with its MEDIUM and LOW findings deferred and the redesign follow-up filed | sooner, at N× cost |
| Output when unconverged | none | the best qualifying round lands, and the rest is filed | the best try, if one qualifies |
| Fits a quota-limited Claude subscription | yes | yes | no; cheap tiers only |
| Research support | partial | full (F1–F8) | only with a deterministic verifier |

**Why B.**
- It bounds the rounds and **uses** them: every round changes an input.
- It lands the best round, not the last.
- It keeps output flowing.

C stays as a phase-B option for cheap tiers, after a strategy change.

## 3. Decisions

| # | Topic | Decision | Decided by |
|---|---|---|---|
| V-D1 | Strategy | **B**, behind one pure `convergence.Decide`. The contract-correction ladder does not call it, because it is a format loop. | console, at the operator's request |
| V-D2 | Round cap | `max_fix_rounds` = 3: two sequential rounds, and a third only after a strategy change. Never a fourth. For code-review this equals ADR-0124's `max_rounds` 4, so the budget is unchanged. | console (research F1, F6) |
| V-D3 | Blocking bar | base MEDIUM, rising to HIGH at the final round. CRITICAL is never deferred. Polish never blocks. | console (research F5) |
| V-D4 | Concentration | `C` ≥ 0.6 over a 2-round window with at least 5 countable findings triggers rung 3. The component is judge-declared (for example `bridge:model-check`), falling back to the package directory. L2's real 2-round shares, with INFO excluded, were 0.39, 0.636 and 0.647 (0.625 before V1 fix round 1 corrected the replay fixture). The trigger reaches its threshold at `J_3`, after the final round, when no strict finding remains. It then signals and files the component's redesign follow-up, and L2 lands through rung 2. | console, from the doc review (H5) |
| V-D5 | Capability findings | below CRITICAL they never block and are filed at once. A CRITICAL is always a defect. The audit records `kind` only. | console (L2 root cause 1; doc review C2) |
| V-D6 | Judges | `J_0` is full. Later judgments are verify-only (the previous findings plus the fix's hunks) for code-review and console judges. A late finding below HIGH is filed. A late CRITICAL or HIGH is filed only when its falsification check refuted it (`falsification: refuted`); while the check has not run, it blocks. The audit keeps its full rules. | console (research F3; doc review C1; V1 review H2) |
| V-D7 | Headroom | read over (model, effort), within the judge's own family. Today no family has deep→top headroom. | console (research F7; doc review M1) |
| V-D8 | Console adoption | immediate, through `evolve convergence decide`, which decides loop `console-lane` only (the pipeline loops call `Decide` in process, V5, V6, V10–V12). The operator's memory rule is the interim. | console |
| V-D9 | Stop on marginal gain | When a round repairs no more than it damages, or shows no progress at the current bar, go to the strategy change (or rung 3). | console (research F2) |
| V-D10 | Keep-best | Checkpoint the candidate of every round, and land the best qualifying round. The rank is the fewest strict findings at the bar, then the lowest open mass at LOW and above, then the earliest. A split or accept never exempts a CRITICAL from the ranking. | console (research F2; V1 review C1, H1) |
| V-D11 | Split and accept | both require no open CRITICAL. A cycle's split is a Stop plus a continuation. Accept-with-limits rows go to the audit's adjudication. | console (doc review H1) |
| V-D12 | The cycle pipeline | The policy also governs the whole cycle (`max_backward_edges`, default 3, across every backward edge). It governs an inbox item across cycles (*N* = `TaskRetryCeiling`; try 3 changes the strategy). It also governs ship recovery. The 32-iteration crash guard stays as a guard. | **operator, 2026-10-07** |
| CQ1 | Claude top-tier headroom | **Top is Opus 5.5 at xhigh effort**, not Fable 5.1. V3b makes the tier table carry it. | **operator, 2026-10-07** |

## 4. Principles

- **Change something every round after the first**: the feedback, then the strategy, then the scope. Never repeat the same refinement a third time.
- **Stop on marginal gain, land the best.** A later round is not automatically a better one.
- **Verify, do not re-hunt.** Delta judgments resolve the previous concerns, and new late findings must survive falsification.
- **Better, not perfect.** Ship what improves the code, and file the rest.
- **Effort first, and only for reasoning failures.**
- **One home.** One pure policy (repo P1), with deterministic decisions (repo P2).
- **Only logic blocks.** Settings are config, with no flags.

## 5. Design summary

The full design is in [convergence-policy.md](../architecture/convergence-policy.md).

- **`J_0`:** a full judgment.
- **Round 1:** a normal fix (ADR-0096's raise for audit repair).
- **Round 2, rung 1:** a judge effort raise for reasoning-class blockers and a one-time fixer raise. Code-review and console judgments are verify-only from `J_1` on.
- **Round 3, rung 2:** a fresh context or a re-plan; HIGH bar; MEDIUM and LOW deferred.
- **Rung 3:** split, accept with limits (the audit adjudicates), or stop.
- **Throughout:** the marginal-gain stop and keep-best.
- **Signals:** 10 codes.

## 6. Components

Status values: ☐ not started · ◐ staged · ☑ landed (with PR).

Ordered by priority: the console adopts the policy first, then the pipeline loops pick it up in shadow.

| # | Component | Where | Status |
|---|---|---|---|
| V0 | Docs: this plan, the design doc, ADR-0126, the L2 incident with its REGRESSION-COVERAGE-INDEX row, the research dossier, and the research-index entries. One full doc review, then one fix round (revision 2), then verify-only. | `docs/` | ◐ (revision 2) |
| V1 | The `convergence` package. It holds `Input`, `Decision` and `Decide`, and the §2.1 schema and edge-case rules. It holds the mass (the default weights, or a function that the loop supplies). It holds the progress at the current bar, the repair damage and the concentration (minimum count, ties). It holds the rungs and triggers, keep-best, the headroom read and red tests for each rule. It has **the L2 replay**, with a fixture of the rounds 4–7 of L2 for each finding. | `go/internal/convergence/` (new; `.apicover-enforce` row; fixture under `testdata/`) | ☑ landed (`feat/convergence-decide-land`, wave-77 boundary train) |
| V2 | `evolve convergence decide --input <rounds.json> [--json]`: the console's door to V1. The input schema is documented. | `go/cmd/evolve/`, `docs/operations/runtime-reference.md` | ☑ landed (`feat/convergence-decide-land`, wave-77 boundary train) |
| V3 | `workflow.convergence` config: compiled defaults, strict key decode, and unknown enum words warn | `go/internal/policy/` | ☑ landed (`feat/convergence-decide-land`, wave-77 boundary train) |
| V3b | **Claude top = Opus 5.5 at xhigh effort**, by the operator's directive. The tier table carries effort per tier for `claude-tmux` (deep high, top xhigh), so the headroom read sees it. Model-cost note: the operator's own choice. | `go/internal/bridge/manifests/claude-tmux.json`, catalog | ☐ |
| V4 | Console adoption. The review briefs carry the `J_0` / verify-only rule and the `kind`, `blocking`, `component` and `late` fields. The console runs V2 after every check and follows its decision. The memory rule points at the verb. | `docs/operations/operating-policy.md`, review-brief templates | ☐ (interim: the memory rule, since 2026-10-07) Inbox item: `convergence-policy-v4-console-adoption`. |
| V5 | Audit repair calls `Decide` after each audit FAIL, in shadow first. It uses rung 1 (the judge effort raise), rung 2(a) (a fresh-context fixer through `composeRepairBrief`'s distilled form) and Stop. No deferrals. The envelope stays the budget's home. | `go/internal/core/audit_fail_decision.go`, `repair_brief.go`, `retry_tier_escalation.go` | ☐ Inbox item: `convergence-policy-v5-audit-repair-decide`. |
| V6 | The code-review loop (ADR-0124 landing 2, Q3) is built on `Decide`. It uses U(n) as the mass and `max_fix_rounds` 3. It uses the amended §6.2 (policy-deferred rows; accept-with-limits rows to adjudication). | code-review plan Q3; `core/` | ☐ (amends Q3) Inbox item: `convergence-policy-v6-code-review-loop-decide`. |
| V7 | The finding grammars gain `Kind:`, `Blocking:`, `Component:` and `Late:`: code-review (RL §4.1) and the audit's Criteria findings (ADR-0125). The falsification check runs before a blocking finding blocks. | `codereview/`, `qualityindex/`, personas | ☐ Inbox item: `convergence-policy-v7-finding-grammar-fields`. |
| V8 | Signals (10 codes, registered, `signal-codes.md` regenerated). The `evolve convergence metrics` readout: rounds-to-land, deferred-then-closed share, falsification drop rate. | `core/signal.go`, `signalcenter/`, `cmd/evolve` | ☐ Inbox item: `convergence-policy-v8-signals-and-metrics`. |
| V9 | The flip: `workflow.convergence.stage = enforce` after 2 shadow waves, at a boundary | operator | ☐ Inbox item: `convergence-policy-v9-stage-enforce-flip`. |
| V10 | **Cycle-wide convergence.** Every backward edge that a cycle takes is a round of loop `cycle`. `max_backward_edges` (default 3) ends the cycle with a Stop (continuation, work kept). Cross-loop oscillation (one failure fingerprint behind two backward edges) jumps to rung 3. The 32-iteration crash guard stays as a guard. Replay test: a cycle that goes audit → build → audit → retro → tdd stops at the 3rd backward edge, not at iteration 32. | `go/internal/core/` (`cyclerun.go`, `resume_execution.go`, the backward-edge sites), `policy/` | ☐ Inbox item: `convergence-policy-v10-cycle-backward-edges`. |
| V11 | **Inbox-item convergence.** Each cycle try on an item is a round of loop `inbox-item`, with *N* = `TaskRetryCeiling`. Try 2 carries the findings and raises effort. Try 3 changes the strategy (a deep-tier fresh plan, a narrowed scope, or a split). After the final try, split the item, route it to the console, or quarantine it. Replay test: an item that fails two times with one fingerprint is never retried a third time the same way. | `go/internal/inboxmover/`, `core/` (the continuation brief), triage persona | ☐ Inbox item: `convergence-policy-v11-inbox-item-rounds`. |
| V12 | **Ship recovery under the policy:** signals plus the documented bound (`maxRecoveryDepth` or width + 1); no new budget | `core/ship_recovery*.go` | ☐ Inbox item: `convergence-policy-v12-ship-recovery-signals`. |
| VB | Phase B, optional: best-of-N for cheap tiers after a strategy change, and judge trust weights from the falsification drop rate | — | ☐ after data Inbox item: `convergence-policy-vb-best-of-n-and-trust`. |

## 7. Expected results

- **No loop runs a fourth fix round,** and the best qualifying round lands. Rounds-to-land has a target median of 2 or fewer.
- **An L2-shaped grind lands at round 3,** with its MEDIUM and LOW findings filed.
- **Deferred and filed findings are not lost.** Their close rate is measured.
- **Escalation is honest.** It happens only with headroom, and only for reasoning-class blockers.

## 8. Rollout

1. **V0** docs, revision 2.
2. **V1–V4** (console first) and **V3b**.
3. **V5 and V6 in shadow.** V6 lands inside the code-review landing 2.
4. **V7 and V8.**
5. **Two shadow waves,** then the **V9** flip at a boundary.

## 9. Verification

- **Unit, red first:** every §2.1 rule, every rung and its trigger, and these behaviours:
  - the marginal-gain stop;
  - keep-best selection (including a later, worse round that does not land, and a tie at the bar going to the round with less open mass);
  - the concentration minimum count and tie-break;
  - progress at the current bar (a bar raise does not fake progress);
  - a late finding below HIGH is filed. A late CRITICAL or HIGH is filed only when its falsification check refuted it, and an absent result blocks;
  - a capability finding below CRITICAL is filed, and a CRITICAL is never deferred;
  - split and accept refuse with an open CRITICAL;
  - a cycle's split is a Stop plus a continuation;
  - the headroom read (today: no headroom in any family; with V3b, Claude deep high → top xhigh);
  - `max_fix_rounds` enforcement.
- **The L2 replay**, the acceptance test for the operator's example. L2's rounds 4–7 as `Input` (a fixture for each finding) must **land at round 3 through rung 2**. The final round raises the bar to HIGH, and its remaining 1 MEDIUM and 4 LOW are deferred (the INFO is filed). Concentration (0.636, then 0.647) fires at `J_3` with no strict finding left. It signals, and the redesign follow-up of the model check is filed. The fixture is built from the round tables in `cli-routing-table-2026-10.md` and the review reports.
- **The audit-repair replay:** cycles 1595–1605 (ADR-0096's data). With audit repair's *N* = 2 (its envelope budget), round 2 is the final round. It shows the judge effort raise and the fresh-context fixer, and never a deferral of an audit finding.
- **Mutation:** at least 10 overlay mutants over `Decide`'s triggers, exits and keep-best, with 0 survivors. These are `J_0`-style quotas, allowed in the first review.
- **Live, in shadow:** two waves of `CONVERGENCE_RUNG` data per loop, then the flip.

## 10. Landing notes

| Landing | Date, lane | What landed | Evidence | Notes |
|---|---|---|---|---|
| Interim | 2026-10-07, console | The console memory rule `review_convergence_rule`. The console applied it to L2 (stopped at round 7, landed as PR #797). It also applied it to the code-review and explanation-refresh lanes (both on round 2). | the incident | Policy before code: the console follows the ladder by hand until V2 lands. |
| V0, revision 2 | 2026-10-07, `dev/cl-convergence` (console) | the doc review's 2 CRITICAL, 5 HIGH, 2 MEDIUM and 3 LOW findings applied; the research refinements CR1–CR8; CQ1 answered by the operator | the review report; the research dossier | The L2 replay lands at round 3 through rung 2. |
| V1–V3, fix round 1 | 2026-10-07, `dev/cl-convergence-v1` (staged, not landed) | The console applied the FIX_THEN_MERGE of the review (1 CRITICAL, 4 HIGH, 8 MEDIUM, 7 LOW) by its own decisions. The list after this table gives each decision. L3 (a DISPUTED CRITICAL lands with no reason line) is filed to the console inbox, not fixed. | red-first logs, the mutation run (the reviewer's 7 survivors killed), the 20,000-input property sweep (no CRITICAL deferred or landed) | The L2 replay still lands at round 3 through rung 2. |
| V0, final fix | 2026-10-07, console | The verify-only check found H5 unresolved and one HIGH regression. Both are fixed. The INFO is excluded from concentration (shares 0.39, 0.636, 0.625; the trigger fires at `J_3` with nothing strict left). Rule 9 now keeps the round budget cycle-wide. Its MEDIUM and LOW notes were cheap text fixes, applied. | the verify-only report | **No further review round, by the policy itself:** after round 2, only CRITICAL and HIGH block, and both are fixed. |
| Tracking | 2026-10-27, console | One inbox item for each open component, V4 to V12 and VB, from the plan audit of 2026-10-27 | `.evolve/inbox/` | Each row of section 6 names its item. |

The decisions of the V1–V3 fix round 1:

- **C1:** a split or accept never exempts a CRITICAL, and every landing checks the landed round again.
- **H1:** keep-best ranks by strict count, then open mass, then round.
- **H2:** `falsification: survived|refuted` replaces `certificate_survived`, and an absent result blocks.
- **H3:** the cycle loop keeps its base bar.
- **M1–M8:** a DEFERRED or FILED CRITICAL is refused, and a re-entry round skips the marginal-gain test. The fixer is raised once, and headroom compares the actual (model, effort). The verb decides `console-lane` only. The schema of the design lives in runtime-reference, and reasons come back beside the values.
- **L1, L2, L4–L7:** a tierless family exits 10, a land names its own cause, and a write failure exits 1. The L2 fixture keeps the re-raised `fix-H2` and adds the LOW-1 of round 7 (share 0.647). The bar words and ranges have one home in `policy`.

## 11. Open questions

| # | Question | Recommendation |
|---|---|---|
| CQ2 | Are 3 rounds, a 0.6 concentration threshold and a minimum of 5 findings right? | Start there. Read rounds-to-land, the deferred-then-closed share and the falsification drop rate in shadow, then tune through config. |
| CQ3 | Is the console bound by `stage`? | No. The console follows the decision from the start (V-D8). |
| CQ4 | Do the judge trust weights come from the falsification drop rate (a noisy judge's findings weigh less)? | Phase B, after the data. |
