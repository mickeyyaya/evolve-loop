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
| Output when unconverged | none | the best qualifying round lands, and the rest is filed | the best attempt, if any qualifies |
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
| V-D4 | Concentration | `C` ≥ 0.6 over a 2-round window with at least 5 countable findings triggers rung 3. The component is judge-declared (for example `bridge:model-check`), falling back to the package directory. L2's real 2-round shares, with INFO excluded, were 0.39, 0.636 and 0.625. The trigger reaches its threshold at `J_3`, after the final round, when no strict finding remains. It then signals and files the component's redesign follow-up, and L2 lands through rung 2. | console, from the doc review (H5) |
| V-D5 | Capability findings | below CRITICAL they never block and are filed at once. A CRITICAL is always a defect. The audit records `kind` only. | console (L2 root cause 1; doc review C2) |
| V-D6 | Judges | `J_0` is full. Later judgments are verify-only (the previous findings plus the fix's hunks) for code-review and console judges. A late finding below HIGH is filed. A late CRITICAL or HIGH blocks only after surviving a falsification check. The audit keeps its full rules. | console (research F3; doc review C1) |
| V-D7 | Headroom | read over (model, effort), within the judge's own family. Today no family has deep→top headroom. | console (research F7; doc review M1) |
| V-D8 | Console adoption | immediate, through `evolve convergence decide`. The operator's memory rule is the interim. | console |
| V-D9 | Stop on marginal gain | if a round's repair damage is at least its repairs, or there is no progress at the current bar, skip straight to the strategy change (or to rung 3) | console (research F2) |
| V-D10 | Keep-best | checkpoint every round's candidate, and land the best qualifying round | console (research F2) |
| V-D11 | Split and accept | both require no open CRITICAL. A cycle's split is a Stop plus a continuation. Accept-with-limits rows go to the audit's adjudication. | console (doc review H1) |
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
- **Round 2, rung 1:** verify-only re-judgment, a judge effort raise for reasoning-class blockers, and a fixer raise.
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
| V1 | The `convergence` package:<br>• `Input`, `Decision`, `Decide`;<br>• the §2.1 schema and edge-case rules;<br>• mass (the default weights, or a loop-supplied function);<br>• progress at the current bar;<br>• repair damage;<br>• concentration (minimum count, ties);<br>• the rungs and triggers;<br>• keep-best;<br>• the headroom read;<br>• red tests per rule;<br>• **the L2 replay**, using a fixture of L2's rounds 4–7 per finding. | `go/internal/convergence/` (new; `.apicover-enforce` row; fixture under `testdata/`) | ☐ |
| V2 | `evolve convergence decide --input <rounds.json> [--json]`: the console's door to V1. The input schema is documented. | `go/cmd/evolve/`, `docs/operations/runtime-reference.md` | ☐ |
| V3 | `workflow.convergence` config: compiled defaults, strict key decode, and unknown enum words warn | `go/internal/policy/` | ☐ |
| V3b | **Claude top = Opus 5.5 at xhigh effort**, by the operator's directive. The tier table carries effort per tier for `claude-tmux` (deep high, top xhigh), so the headroom read sees it. Model-cost note: the operator's own choice. | `go/internal/bridge/manifests/claude-tmux.json`, catalog | ☐ |
| V4 | Console adoption:<br>• review briefs carry the `J_0` / verify-only rule and the `kind`, `blocking`, `component` and `late` fields;<br>• the console runs V2 after every check and follows its decision;<br>• the memory rule points at the verb. | `docs/operations/operating-policy.md`, review-brief templates | ☐ (interim: the memory rule, since 2026-10-07) |
| V5 | Audit repair calls `Decide` after each audit FAIL, in shadow first. It uses rung 1 (the judge effort raise), rung 2(a) (a fresh-context fixer through `composeRepairBrief`'s distilled form) and Stop. No deferrals. The envelope stays the budget's home. | `go/internal/core/audit_fail_decision.go`, `repair_brief.go`, `retry_tier_escalation.go` | ☐ |
| V6 | The code-review loop (ADR-0124 landing 2, Q3) is built on `Decide`:<br>• U(n) as the mass;<br>• `max_fix_rounds` 3;<br>• the amended §6.2 (policy-deferred rows; accept-with-limits rows to adjudication). | code-review plan Q3; `core/` | ☐ (amends Q3) |
| V7 | The finding grammars gain `Kind:`, `Blocking:`, `Component:` and `Late:`: code-review (RL §4.1) and the audit's Criteria findings (ADR-0125). The falsification check runs before a blocking finding blocks. | `codereview/`, `qualityindex/`, personas | ☐ |
| V8 | Signals (10 codes, registered, `signal-codes.md` regenerated). The `evolve convergence metrics` readout: rounds-to-land, deferred-then-closed share, falsification drop rate. | `core/signal.go`, `signalcenter/`, `cmd/evolve` | ☐ |
| V9 | The flip: `workflow.convergence.stage = enforce` after 2 shadow waves, at a boundary | operator | ☐ |
| VB | Phase B, optional: best-of-N for cheap tiers after a strategy change, and judge trust weights from the falsification drop rate | — | ☐ after data |

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
  - keep-best selection (including a later, worse round that does not land);
  - the concentration minimum count and tie-break;
  - progress at the current bar (a bar raise does not fake progress);
  - a late finding below HIGH is filed, and a late CRITICAL or HIGH blocks only with a surviving certificate;
  - a capability finding below CRITICAL is filed, and a CRITICAL is never deferred;
  - split and accept refuse with an open CRITICAL;
  - a cycle's split is a Stop plus a continuation;
  - the headroom read (today: no headroom in any family; with V3b, Claude deep high → top xhigh);
  - `max_fix_rounds` enforcement.
- **The L2 replay**, the acceptance test for the operator's example. L2's rounds 4–7 as `Input` (per-finding fixture) must **land at round 3 through rung 2**: the final round raises the bar to HIGH, and its remaining 1 MEDIUM and 3 LOW are deferred (the INFO is filed). Concentration (0.636, then 0.625) fires at `J_3` with no strict finding left: it signals, and the model check's redesign follow-up is filed. The fixture is built from the round tables in `cli-routing-table-2026-10.md` and the review reports.
- **The audit-repair replay:** cycles 1595–1605 (ADR-0096's data). With audit repair's *N* = 2 (its envelope budget), round 2 is the final round. It shows the judge effort raise and the fresh-context fixer, and never a deferral of an audit finding.
- **Mutation:** at least 10 overlay mutants over `Decide`'s triggers, exits and keep-best, with 0 survivors. These are `J_0`-style quotas, allowed in the first review.
- **Live, in shadow:** two waves of `CONVERGENCE_RUNG` data per loop, then the flip.

## 10. Landing notes

| Landing | Date, lane | What landed | Evidence | Notes |
|---|---|---|---|---|
| Interim | 2026-10-07, console | The console memory rule `review_convergence_rule`, applied to L2 (stopped at round 7, landed as PR #797) and to the code-review and explanation-refresh lanes (both on round 2) | the incident | Policy before code: the console follows the ladder by hand until V2 lands. |
| V0, revision 2 | 2026-10-07, `dev/cl-convergence` (console) | the doc review's 2 CRITICAL, 5 HIGH, 2 MEDIUM and 3 LOW findings applied; the research refinements CR1–CR8; CQ1 answered by the operator | the review report; the research dossier | The L2 replay lands at round 3 through rung 2. |
| V0, final fix | 2026-10-07, console | The verify-only check found H5 unresolved and one HIGH regression. Both are fixed: the INFO is excluded from concentration (shares 0.39, 0.636, 0.625; the trigger fires at `J_3` with nothing strict left), and rule 9 now keeps the round budget cycle-wide. Its MEDIUM and LOW notes were cheap text fixes, applied. | the verify-only report | **No further review round, by the policy itself:** after round 2, only CRITICAL and HIGH block, and both are fixed. |

## 11. Open questions

| # | Question | Recommendation |
|---|---|---|
| CQ2 | Are 3 rounds, a 0.6 concentration threshold and a minimum of 5 findings right? | Start there. Read rounds-to-land, the deferred-then-closed share and the falsification drop rate in shadow, then tune through config. |
| CQ3 | Should the console be bound by `stage`? | No. The console follows the decision from the start (V-D8). |
| CQ4 | Should judge trust weights come from the falsification drop rate (a noisy judge's findings weigh less)? | Phase B, after the data. |
