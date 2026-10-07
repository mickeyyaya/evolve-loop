# Code-review phase: an independent review loop between build and audit, judged on one shared quality index (plan, 2026-10)

> **Purpose.** This is the living plan for the `code-review` phase.
> - **What the phase does:**
>   - It reviews the build's diff across every quality dimension, justifying each review it runs.
>   - It returns findings with suggested fixes to build, repeating until every concern is resolved.
>   - It hands the result to the audit. The audit scores the same quality index to decide whether the change qualifies.
> - **Approved by the operator on 2026-10-07.** The first design was approved in the morning; the review-loop redesign was approved in plan mode in the afternoon. Both sets of decisions are in §3.
> - **Design (the core logic):** [review-loop-and-quality-index.md](../architecture/review-loop-and-quality-index.md).
> - **Decision record:** [ADR-0124](../architecture/adr/0124-code-review-phase.md).
> - **Updates:** every landing updates the status column in §6 and the landing notes in §10.
>
> Operator requests (2026-10-07):
> - *"ultrathink how to add a 'review' phase in the pipeline, which should be independent from the building phase but before audit (eval) phase."*
> - *"the review phase should provide the feedback for build phase to improve and fix the defects it found with suggestion."*
> - *"code/output simplifier should be done within the build related phases, there should be review phase done after build and before audit … The review phase should include all different review skills, e.g. architecture review, performance, debugability, concurrent, ...etc. The goal of review should continue to provide the feedback to build phase agent until all the concerns and defects are resolved. … The audit should focus on evaluating from different perspecitives shared with review phase agent to learn if the reviewed and fixed output is qualified for all index score. ultrathink with at least 2 different strategies for implementation and pick the winner."*
> - *"the review phase agent should justify what reviews are required for the changes done by build phase agent. It should also read the previous agent output/deliverables if needed to help reasoning what review needs to be done."*
> - *"write the detailed doc for plan and design following the same doc structure and format as the rest of docs/. make sure we have all the details and core logics."*

> **Terms.**
> - *Quality index*: the ten dimensions every change is judged on (design §2), shared by the review and the audit.
> - *Dimension*: one index axis, scored 1–5, or N/A with a reason.
> - *Review Plan*: the reviewer's per-dimension statement of required or N/A, the depth, and the justification (design §3).
> - *Finding*: one reviewed defect: severity, dimension, `file:line`, scenario with evidence, and a suggested fix.
> - *Round*: one completed `code-review` dispatch. Round 1 reviews the whole diff; later rounds are delta re-reviews.
> - *Disposition*: the build's answer to one finding id. FIXED with evidence, or DEFERRED with a reason.
> - *Strict finding*: a finding at or above the configured threshold severity (default MEDIUM). Only strict findings can be rejected or DISPUTED (design §5.2).
> - *DISPUTED*: a strict finding whose deferral the reviewer rejected twice. The audit adjudicates it.
> - *Unresolved mass*: severity-weighted OPEN findings plus the score gaps below threshold; the loop must drive it down.
> - *Judge*: a phase whose findings can earn a repair round (`audit`, `code-review`).
> - *Review digest*: the kernel-written summary of the review the audit reads instead of the raw report (design §6.1).
> - *Correction rung*: the deliverable gate's contract-correction ladder, which re-dispatches a malformed report with its violations, a bounded number of times.
> - *Phase A / phase B*: phase A is the lead reviewer with the full index; phase B adds data-driven specialists (design §8).
> - *Shadow / enforce*: the rollout stages (design §9).
> - *C0 / C0c*: the staged agent-config lane (`dev/cl-agentcfg`). C0 preloads `engineering-craft` and `code-review-simplify` into source writers; C0c is its self-review part.
> - *L2*: the routing lane (`dev/cl-routingl2`), which owns the CLI routing table.
> - *Console*: the operator's interactive session that designs and lands lanes. As a decider (§3), "console" means the console proposed the decision and the operator approved it with the plan.
> - *Plane*: the runtime checkout the live loop runs in.
> - *Lane*: one parallel cycle (loop) or one console work item.
> - *Floor*: the full local test suite run before shipping.
>
> Paths are relative to `go/internal/` unless they start with a top-level directory.

## 1. Why

| Problem | Evidence |
|---|---|
| The audit was the only reviewer and also the gate | A builder first hears of a defect from the audit's rejection, as verdict prose (ADR-0093/0096). The ADR-0096 ship-rate study shows repair converges poorly in that form. |
| One judgment carried every lens | The auditor persona is 23,956 bytes against the 18,102-byte ACS baseline (`go/acs/cycle419`). Performance, concurrency and debuggability got whatever attention was left. |
| The real review ran outside the pipeline | The console's review chain found defects on every lane reviewed on 2026-10-06/07 that only showed in the diff: a rule with four disagreeing homes (agy reliability), an ownership check at one of two sites (fleet leak), an exemption stated twice, and surviving mutants. Every lane needed 2–3 review rounds to converge. |
| Lens phases rarely ran | Ten single-lens evaluate phases exist (perf-profile, race-condition-scan, error-handling-scan, security-scan, …). The advisor inserts them ad hoc, so most code cycles got none. |
| No shared bar | Review lenses, the audit checklist and the lens phases each used their own vocabulary, so "qualified" had no single definition. |

## 2. Strategies considered

The operator asked for at least two strategies and a chosen winner.

| | S1. One multi-lens reviewer | S2. Parallel lens panel every round | **S3. Shared quality index + lead reviewer + triggered specialists (CHOSEN)** |
|---|---|---|---|
| Coverage of every lens | yes, at baseline depth | yes, deep | yes: the lead covers every dimension; specialists add depth where the diff triggers them |
| Depth per lens | shallow: about ten lenses in one prompt, attention spread thin | deep | deep where it matters |
| Cost per round | 1 deep dispatch | about 8 dispatches, several deep; heavy on Claude quota | 1 deep + 0–3 triggered |
| Conflicting feedback | reasoned together | lenses contradict each other (perf vs readability) with no arbiter, so build oscillates | the lead arbitrates; specialist rows merge into one ledger |
| Shared with the audit | needs a rubric | needs a rubric | the rubric IS the design: one quality index for both |
| Reuse | the landing-1 phase | lens phases + the evaluate batch | both |
| Failure isolation | one failure stops the review | per lens | per lens, with the lead as the floor |

**Why S3 won:**
- It meets "all review skills" at bounded cost.
- It is the only option in which the review and the audit share an explicit contract.
- It ships in steps: phase A is S1 plus the index, and phase B adds specialists only where data shows the lead misses. That honours the operator's choice "phase B, driven by data".

## 3. Decisions

| # | Topic | Decision | Decided by |
|---|---|---|---|
| D1 | Coverage | **Every code cycle that touched files.** The registry pins `code-review` with `conditional_mandatory` (`deliverable_kind==code && build.files_touched>0`). Document and no-change cycles skip it. | operator (morning) |
| D2 | Rollout | **Shadow for 2 waves, then enforce,** through `workflow.findings_repair.code-review.stage`. | operator (morning) |
| D3 | Strategy | **S3:** a shared quality index, a lead reviewer, and data-driven specialists. | console, at the operator's request |
| Q-D1 | Loop stop | **At most 4 review rounds plus a no-progress stop:** a round must strictly reduce the unresolved mass, and no still-OPEN id may have been reopened after FIXED twice. Unresolved work goes to the audit with `REVIEW_UNRESOLVED`, and the audit's existing repair or retro path takes over. | operator (afternoon) |
| Q-D2 | "Resolved" | **Strict findings (default CRITICAL, HIGH and MEDIUM):** FIXED with evidence, or DEFERRED with a reason the reviewer accepts. **Below the threshold (default LOW):** dispositioned, either fixed or with a stated reason, and never rejected. **Every applicable dimension** meets its threshold. | operator (afternoon) |
| Q-D3 | Qualification at the audit | **Every applicable dimension ≥ 4/5** in the audit's own scoring. N/A is allowed with a reason, and the audit may contest it. The thresholds are config. | operator (afternoon) |
| Q-D4 | Specialists | **Phase B, driven by data:** add one for a dimension where the audit keeps scoring lower than the reviewer. | operator (afternoon) |
| D4 | Review scope | **A justified Review Plan every round.** Each dimension is required (light, standard or deep) or N/A, with evidence from the diff and from the earlier phases' deliverables, which the reviewer reads as needed. | operator (afternoon) |
| D5 | The simplifier | **It stays inside the build phases** (C0c self-simplify and self-review, scoped to the phase's own files). The review applies no fixes. | operator (afternoon) |
| D6 | Grant mechanism | **Deterministic** (design §5.4). No LLM envelope for code-review, and `remediationDenied` is unchanged. The loop is a findings-driven repair, never a re-roll of a verdict. | console (anti-gaming) |
| D7 | Audit independence | **The audit scores blind,** from a review digest that leaves out the review's Scores, and never opens the raw review report. The kernel computes divergence afterwards. | console |
| D8 | Kernel cross-check | **A shippable audit verdict (PASS or WARN) that does not qualify becomes FAIL** (`AUDIT_QUALIFICATION_GAP`), declaring the failure class `code-audit-fail` so the existing repair is granted. A malformed Scores section goes to the correction rung (enforce only). | console (logic over format) |
| D9 | Disputes | **A strict deferral the reviewer rejects twice becomes DISPUTED.** It is resolved for the loop and ruled on by the audit: UPHOLD reopens it, OVERTURN accepts the deferral, and any row still DISPUTED at cycle close becomes OPEN. This extends Q-D2's "a reason the reviewer accepts" with an arbiter, so a reviewer–builder disagreement cannot stall the loop. | console |
| — | Superseded (morning) | "One fix round plus a delta re-review" is replaced by Q-D1's loop. "The audit only grades dispositions" is replaced by: the audit grades dispositions AND scores the shared index. "Threshold HIGH" is replaced by MEDIUM (Q-D2). The morning components R4–R7 are replaced by Q3–Q6, and R8 is split into R8a (the stage switch) and R8b (the flip). | operator (afternoon) |

## 4. Principles

- **One home per rule.**
  - The quality index (skill plus `qualityindex` package) is the only definition of "qualified".
  - `Qualifies` is the only qualification test.
  - `defectledger.Append` is the only merge rule.
  - The findings-driven repair is the only repair mechanism, keyed by judge.
- **The review is not a gate.** Only the never-re-rolled audit unlocks ship.
- **Logic over format.** Only logic blocks: a gap below threshold, an unresolved strict finding, an upheld dispute. A malformed artifact gets a correction rung, and no capacity gets a skip.
- **Every finding carries a suggested fix.** Every claim the build makes carries evidence.
- **Bounded cost.** One deep reviewer per round, at most 4 rounds, and specialists only on triggers or on evidence of misses.
- **Small components:** red first, unwired before wired. Each lane respects the 4-agent cap and runs its floor one target at a time.

## 5. Design summary

The full core logic is in [review-loop-and-quality-index.md](../architecture/review-loop-and-quality-index.md):

| Design § | Contents |
|---|---|
| §2 | The ten dimensions with their score-4 bar, N/A rules and review procedure; the Scores grammar; threshold resolution; `Qualifies` |
| §3 | The Review Plan grammar and rules; the inputs and how they reach the reviewer; the report grammar check and the report's sections |
| §4 | The finding grammar and the ledger projection |
| §5 | Per-round state; the resolved predicate; unresolved mass and progress; the decision pseudo-code and routing seams; the fix brief; the disposition protocol, `applyReviewDispositions` and the delta-review duties; the tamper check |
| §6 | The blind audit and the digest, the qualification cross-check, divergence and DISPUTED adjudication |
| §7 | Recovery rungs |
| §8 | Phase B specialists |
| §9 | Stages, config and validation |
| §10 | Signals, with the component each lands in |
| §11 | Independence fences |
| §12 | Interplay |
| §13 | Security surface |

## 6. Components

Status values: ☐ not started · ◐ staged · ☑ landed (with PR).

The morning components R4–R7 are replaced by Q3–Q6, and R8 is split into R8a (the stage switch, landing 1) and R8b (the flip).

### Landing 1, amended (shadow)

| # | Component | Where | Status |
|---|---|---|---|
| R0/Q0 | This plan, the design doc, and ADR-0124 (amended) | `docs/plans/`, `docs/architecture/` | ◐ |
| R1 | `skills/architecture-review/SKILL.md` | `skills/` | ◐ |
| Q1 | The quality index:<br>• `skills/quality-index/` (SKILL plus COMPACT): the dimensions, score-4 bars, N/A rules, review procedure per dimension and the grammars;<br>• the `qualityindex` package: `Keys`, `NeverNA`, `ResolveThresholds`, `ParseScores`, `ParsePlan`, `Agree`, `Qualifies`;<br>• `workflow.quality_index.thresholds` resolved into `WorkflowConfig.QualityIndex`, with a root-based loader for the deliverable gate;<br>• protected-manifest rows for both, and for the code-review phase's own files and pin test (design §13). | `skills/`, `qualityindex/`, `policy/`, `guards/integrity_surface.go` (also edited by C0; appended at the end, apart from C0's hunk) | ◐ |
| R2 | The phase: `phase.json`, the persona and profile, the `conditional_mandatory` pin | phases, agents, profiles, registry | ◐ |
| Q2 | Landing-1 amendments to the design:<br>• the report adds `## Review Plan` and `## Scores` and drops `## Scope`; the finding `Category` becomes `Dimension` (`clean-code` → `maintainability`);<br>• the report grammar `code-review-report` (`codereview.ValidateReport`): plan, scores, agreement, gap citations; declared by `classify.grammars` and checked by the deliverable gate (`bad_grammar`), so `evolve phase verify code-review` applies it;<br>• rows gain `dimension`; the cut's stand-in row takes the highest cut severity; inherited rows keep their provenance;<br>• the persona covers every dimension and reads the earlier deliverables: the Task Contract through `prompt_context`, the rest through `inputs.files`;<br>• the default threshold becomes MEDIUM, and `REVIEW_FINDINGS` gains `scores` and `gaps` with the redefined `would_repair`, the severity counts folded into one `findings` field to fit the Signal Center's 12-field cap;<br>• the landing-1 files that still described the morning design. | `codereview/`, `core/defectledger/`, `core/review_findings.go`, `core/task_contract.go`, `deliverable/`, `phasespec/`, `phasecontract/`, `policy/workflow_findings_repair.go`, `.evolve/phases/code-review/phase.json`, `agents/evolve-code-reviewer.md`, `skills/architecture-review/SKILL.md`, `CHANGELOG.md`, `docs/operations/runtime-reference.md`, `docs/architecture/packages/internal-policy.md`, `docs/architecture/packages/internal-codereview.md`, `docs/architecture/continuation-defect-ledger.md`, `docs/architecture/phase-plugin-system.md` | ◐ |
| R3 | Ledger rows (`source`, `round`, `severity`), the shared `Append`, `codereview` parse and rows, and the completion-boundary hook | `core/defectledger/`, `codereview/`, `core/` | ◐ |
| R8a | The stage switch, held in shadow | `policy/workflow_findings_repair.go` | ◐ |
| — | After C0: the phase overlay rule (`{phases:[code-review], when: code} → [engineering-craft, code-review-simplify, architecture-review, quality-index]`) | `policy/overlays.go` | ☐ after C0 |

### Landing 2 (shadow, then the flip)

| # | Component | Where | Status |
|---|---|---|---|
| Q3 | The generalized findings-driven repair:<br>• the `ReviewRepair` cycle state (`review_repair,omitempty`), persisted on every path, with `PendingFixRound` for resume;<br>• unresolved mass, the progress rule and its re-entry baseline;<br>• the oscillation counter (still-OPEN ids only);<br>• `decideAfterReview`, computed in full in shadow;<br>• the routing seams: `scheduledNext`, `decisionOnlyEdge`, the registry's `legal_successors`, re-entry scheduling;<br>• `applyReviewDispositions` for same-cycle rows;<br>• the fix-round brief (`CtxKeyReviewRepairFindings`, `composeReviewRepairBrief`) and per-round archives;<br>• the tamper check (hash and restore);<br>• `max_rounds` in config.<br>Audit-repair stays byte-identical (golden). | `core/` (protected), `cyclestate/`, `policy/`, `docs/architecture/phase-registry.json` | ☐ |
| Q4 | Delta re-review mode: the `## Disposition Review` grammar (one line per id, NOTE for non-strict rows), the kernel applying ACCEPT and REJECT, `defectledger.StatusDisputed`, and DISPUTED at 2 | `codereview/`, `core/defectledger/`, persona | ☐ |
| Q5 | The audit on the index:<br>• the review digest (`code-review-digest.md`, `writeReviewDigest`);<br>• the auditor's own Scores and Adjudication;<br>• the kernel cross-check on PASS and WARN, with the `code-audit-fail` qualification block (`audit-qualification.json`);<br>• divergence (`review-divergence.json`, `quality_index.divergence`);<br>• DISPUTED → OPEN at cycle close;<br>• slimming the auditor persona (at the flip). | `core/`, `phases/audit/`, `agents/evolve-auditor*.md` | ☐ |
| Q6 | The capacity skip (a branch at the quota-wall seam for a non-gate phase), the full signal set (design §10), ordering `code-review` first after build (OQ1), and the metrics readout verb `evolve review metrics [--cycles N] [--json]`, which prints design §9's shadow metrics per dimension | runner, `core/`, `phasespec`, `cmd/evolve` | ☐ |
| R8b | The flip to enforce, after 2 shadow waves | operator, at a boundary | ☐ |

### Phase B

| # | Component | Where | Status |
|---|---|---|---|
| Q7 | Specialists:<br>• a diff classifier of signals;<br>• `workflow.review.{specialists, specialist_trigger, hot_path_packages}`;<br>• lens personas normalized to the finding grammar;<br>• specialists in the round's evaluate batch;<br>• the advisor stops inserting mapped lenses standalone. | `core/`, `phasespec/`, lens personas | ☐ after ≥2 enforce waves of divergence data, then by the trigger (30% audit-lower over the last 20 cycles that scored the dimension) |

## 7. Expected results

| Measure | Today | After the flip |
|---|---|---|
| When a builder first hears of a defect | after the audit rejects | in the same cycle, after the first review round, with a suggested fix |
| What "qualified" means | the auditor's own checklist | one index vector, every applicable dimension ≥ 4, cross-checked by the kernel |
| Auditor persona size | 23,956 bytes | below the 18,102-byte baseline (checklists move to the index and reviewer skills) |
| Review cost per code cycle | none (the console reviews by hand) | at most 4 deep review dispatches and 3 fix builds; 1–2 rounds expected, measured in shadow |
| Audit FAIL rate on code cycles | the pre-shadow baseline | expected lower; shadow measures the would-repair rate first |

## 8. Rollout

1. **Landing 1, amended (shadow).**
   - The review runs, plans and scores on every code cycle.
   - Rows are DEFERRED, there is no fix round, and the auditor is unchanged.
   - `REVIEW_FINDINGS` carries the scores vector, the gaps and the round-1 `would_repair`.
2. **Landing 2 (still shadow).**
   - The loop's decision is computed in full and signalled (`REVIEW_ROUND{decision}`, `would_repair`), but no fix round is granted.
   - The audit's own index scoring, the cross-check (would-fail) and divergence run in shadow, with no correction rung for the audit's index sections.
3. **Two shadow waves.** Read the metrics at each boundary with `evolve review metrics`:
   - findings per dimension and severity;
   - the would-repair rate, and the projected rounds to resolve;
   - audit-versus-review divergence per dimension;
   - added wall time and tokens;
   - the audit FAIL and ship rates against the pre-shadow waves.
4. **The flip.** `stage: enforce` at a wave boundary, by the operator. The auditor persona's slimming lands with it.
5. **Phase B.** Add specialists for the dimensions whose audit-lower divergence stays above the configured share over the evaluation window.

## 9. Verification

- **Unit tests, red first:**
  - `Qualifies`, as a table: threshold, N/A, never-N/A and missing;
  - `ParseScores` and `ParsePlan`: every malformed form goes to correction; the separator tolerance;
  - threshold resolution: `"*"` then per-key, out of range, unknown key;
  - the report grammar: agreement, a gap with no cited finding;
  - the resolved predicate, including a non-strict row that is never rejected (a LOW dispute cannot happen);
  - unresolved mass and progress, including the re-entry baseline;
  - `decideAfterReview`, with every stop: resolved, budget, no-progress, oscillation (only still-OPEN ids), ledger fault, and shadow's as-if-enforced decision;
  - resume after a grant (`PendingFixRound`);
  - the brief carries suggestions, rejected claims and gaps, and the audit brief comes first;
  - `applyReviewDispositions`: FIXED with and without resolving evidence, DEFERRED with and without a reason, a missing disposition;
  - Disposition Review ACCEPT and REJECT, a missing line, DISPUTED at 2;
  - the tamper check restores the ledger and the report;
  - the cross-check flips PASS and WARN to FAIL on a gap and declares `code-audit-fail`, so `decideAfterAuditFail` grants;
  - an upheld dispute stops qualification; a still-DISPUTED row becomes OPEN at close;
  - an audit-repair re-entry with the budget spent emits `REVIEW_BUDGET_SPENT`;
  - the capacity skip continues the walk and does not spend a round;
  - the divergence signal and its direction;
  - audit-repair stays byte-identical (golden).
- **Mutation:** at least 10 `go test -overlay` mutants each, over the grant, the progress and oscillation detectors, `Qualifies` and the cross-check.
- **End to end, with fakes:**
  - a code cycle with HIGH findings: round 1, then the build fix, then round 2 resolved, then the audit qualifies;
  - a non-converging reviewer: stops at no-progress with `REVIEW_UNRESOLVED`;
  - a disputed deferral: the audit adjudicates;
  - a dimension gap at the audit: FAIL, then audit-repair, then a delta round.
- **Live, in shadow:** the §8 metrics over 2 waves before the flip.
- **Every landing:**
  - the full floor, one target at a time: test, integration, e2e, acs-durable, apicover-enforce, cover-strict;
  - simplifier, then architecture and Go review, then fix, then delta;
  - the landing-patch audit: numstat matches, the CHANGELOG entry is first, and comment history is recorded.

## 10. Landing notes

| Landing | Date, lane | What landed | Evidence | Notes |
|---|---|---|---|---|
| Landing 1, as first staged | 2026-10-07, `dev/cl-codereview` | R0–R3 and R8a, held in shadow | red-first runs; 23 overlay mutants, all killed except one equivalent mutant, named apart | The reviewer profile joins `profiles.ClaudeFamilyFloor`, and the `cliroute` legacy-plans golden gains 24 reviewer records. The overlay rule is a patch to apply once C0 lands. Retired eval: `.evolve/evals/wire-code-review-simplify-auditor-hook.md`, whose premise is false since C0 and ADR-0124. |
| Landing 1, amended | 2026-10-07 (afternoon), `dev/cl-codereview` | Q0–Q2 over the staged landing 1: the quality index (skill and package), the report grammar and its gate, Dimension rows, MEDIUM, the persona's inputs | the lane report (red runs, mutants, floor) | The redesign changes the report grammar, the row fields and the default threshold before the landing-1 review, so no superseded schema lands. The doc review's 43 findings were applied first (the lane report lists each as fixed or declined). The defectledger leaf now declares `reportdoc` (for the stand-in's highest severity); its stale header comment was removed and archived under `docs/history/code-comments/`. A test caught the Signal Center's 12-field cap dropping `REVIEW_FINDINGS` fields, so the severity counts travel as one field. |

## 11. Open questions

| # | Question | Recommendation |
|---|---|---|
| OQ1 | User phases anchored `after: build` splice in reverse-alphabetical order (`phasespec.ApplyUserRouting`), so `code-review` does not run first after build. | Enforce needs it first, so the other evaluate phases see the settled diff. Add an explicit order to the registry in Q6. Shadow does not need it. |
| OQ2 | Should the delta re-review run at a cheaper tier? | Keep it deep. The fix diff is small, so the cost is small, and a weaker verifier invites laundering. |
| OQ3 | test-amplification also writes source after build. | In enforce, place it before `code-review`, or count its diff as part of the fix round. Decide with OQ1. |
| OQ4 | Does a digest that omits the Scores fully prevent anchoring? | Not fully: the raw report stays in the workspace. The persona forbids opening it, and divergence makes anchoring visible. Reconsider after the shadow data. |
| OQ5 | Is the phase-B divergence share configured, and over what window? | Yes: `workflow.review.specialist_trigger`, defaulting to 30% audit-lower over the last 20 code cycles that scored the dimension. Revisit with data. |
