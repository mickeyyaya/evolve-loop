# ADR-0126: Every iterative loop converges or escalates: feedback, then strategy, then scope; land the best round; never a fourth

> **Note (2026-11-06, operator ruling, [ADR-0164](0164-all-decisions-serve-one-goal-and-one-amendment-resolves-their-conflicts.md)):** Rule 10 of ADR-0160 and decision 16 are one count. A charge is one block. A failed ladder goes to the redesign rung first.

> **Note (2026-11-06, [ADR-0164](0164-all-decisions-serve-one-goal-and-one-amendment-resolves-their-conflicts.md)):** Decision 15 gains a step: the redesign rung starts with live research, and its card scores at least three options. No code for this step exists yet.

- **Status:** Proposed (2026-10-07), a console proposal from the operator's directives. It moves to Accepted as its components land ([plan](../../plans/convergence-policy-2026-10.md) §6).
  - **Revision 2** applies the doc review (C1, C2, H1–H5, M1, M2, L1–L3), the research refinements CR1–CR8, and the operator's headroom answer.
  - **Revision 3** (2026-11-05) adds decisions 12 to 17: one escalation path, with a redesign rung after the retries. No code for this revision exists yet.
- **Design (the core logic):** [convergence-policy.md](../convergence-policy.md).
- **The example:** [incident, L2's seven review rounds](../../incidents/2026-10-07-l2-seven-review-rounds.md).
- **Research:** [converging review feedback within limited rounds](../../research/review-convergence-limited-rounds-2026-10.md).
- **Amends:**
  - [ADR-0096](0096-repair-rounds-escalate-and-carry-findings.md): the first-repair tier raise is kept. It becomes part of a shared ladder, which adds a judge effort raise for reasoning-class blockers and a fresh-context fixer at the final round.
  - ADR-0124 (the code-review loop; it lands with the code-review train):
    - its landing-2 loop calls the policy, with `max_fix_rounds` 3 (equal to its `max_rounds` 4, so the budget is unchanged) and its own U(n) as the mass;
    - §6.2: a row DEFERRED by the policy at rung 2 (MEDIUM/LOW, with a filed follow-up) no longer counts as strict OPEN;
    - accept-with-limits rows go to the audit's adjudication, as DISPUTED rows do.
  - [ADR-0125](0125-audit-publishes-its-evaluation-contract.md) (the audit evaluation contract): findings carry `kind` and `blocking`. The audit records `kind` and never uses it to unblock.
  - [ADR-0161](0161-a-pipeline-blocker-fix-meets-six-test-rows-and-a-signal-states-its-cause-and-next-step.md): its decisions 9 and 10 become a rung of this ladder (Revision 3).
- **Relates to:**
  - [ADR-0142](0142-a-capability-failure-climbs-one-bounded-retry-ladder.md): its ladder stays inside one phase, and an exhausted ladder is one block.
  - [ADR-0158](0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md): a hand-off finding that blocks still goes to the console at once, with no count.
- **Evidence:**
  - ADR-0096 measured ship probability by audit-round count at 100% → 50% → 17% → 0%.
  - L2: seven fix rounds on the deepest Claude model, with about 18 of the last 27 findings in one heuristic that a review had added.
  - The research:
    - two rounds capture 76–95% of the gain;
    - a fixed budget can lower quality, so stop on marginal gain and keep the best;
    - LLM reviewers vary across runs, so verify rather than re-hunt;
    - more revisions do not converge without reduced uncertainty;
    - "better, not perfect";
    - escalate effort first, for reasoning failures only.

## Context

Every repeat-until-accepted loop invents its own stop, and none escalates past a tier. None raises the blocking bar, stops on marginal gain, or keeps the best round. Reviews can add capabilities to the change under review, and delta checks hunt for new findings in a nondeterministic reviewer instead of verifying the old ones.

On 2026-10-07 the operator stopped L2 at round 7 ("It just retry for too many times"). The operator asked for a rule that converges earlier, "by escalating to top / deep model or other approaches", instead of "persuing the perfection with no output". The operator also asked for online research into the best policy for converging review feedback within limited rounds, and stated that "the latest top model is opus 5.5 with xhigh effort".

## Decision

1. **One policy, one home.** A pure `convergence.Decide(Input) Decision`, called at the decision points of audit repair, the code-review loop, the explanation re-author and console lanes (`evolve convergence decide`). The contract-correction ladder is a format loop and does not call it.
2. **Rounds.**
   - **`J_0`** is a full judgment.
   - **Round 1** is a normal fix (ADR-0096's raise stays here for audit repair).
   - **Round 2, rung 1, changes the feedback:**
     - an effort raise within the judge's family (deep high → top xhigh), for reasoning-class blockers, with headroom;
     - a fixer raise, once. This is the once-only raise: the fixer and judge raises go together with the first rung-1 round, or with rung 2 when no rung-1 round ran, and no later round raises again.
     - Code-review and console judgments are already verify-only from `J_1` on (decision 6).
   - **Round 3, rung 2, changes the strategy:** a fresh-context fixer or a re-plan; the bar rises to HIGH; MEDIUM/LOW are deferred and filed (code-review and console only).
   - **No round 4.**
3. **Stop on marginal gain.** A round whose repair damage is at least its repairs, or with no progress at the current bar, skips straight to the strategy change (or to rung 3).
4. **Keep the best.** Every round's candidate is checkpointed, and the best qualifying round lands, never automatically the last.
5. **Rung 3 changes the scope:**
   - **split:** for a console lane, unstage the component when the rest passes the floor; for a cycle, Stop plus a continuation that unwires the component;
   - **accept a certified fail-safe residue:** for code-review, the audit adjudicates it;
   - **stop:** preserve the work and file follow-ups.

   Split and accept both require no open CRITICAL.
6. **Judges.**
   - Only `J_0` carries probe and mutant quotas.
   - Later judgments verify the previous findings and the fix's own hunks.
   - A late finding (on code unchanged since `J_0`) below HIGH is filed. A late CRITICAL or HIGH is filed only when a falsification check that sees only the diff refutes its certificate; until that check has run, it blocks.
   - Every blocking finding passes that check.
   - A dispute goes to an adjudicator, never to another round.
7. **Kind.** A capability finding below CRITICAL never blocks; it is filed. A CRITICAL is always a defect. The audit records `kind` only.
8. **Headroom is read, never assumed.**
   - Today no family has deep→top headroom.
   - Per the operator, Claude's top is Opus 5.5 at **xhigh** effort; component V3b makes the tier table say so.
   - Raises stay within the judge's own family.
   - A disagreement between two efforts of one model is settled by deterministic evidence, because their errors are correlated.
9. **The audit is untouched.** CRITICAL is never deferred, and no audit FAIL becomes a PASS. The audit keeps its full verdict rules. Audit repair uses rung 1, rung 2(a) and Stop only, with no deferrals.
10. **The cycle pipeline converges too** (the operator, 2026-10-07: *"Convergence rule should also apply to evo loop cycle pipeline to avoid infinite back and forth endless loop"*).
    - **Within a cycle:** every backward edge (audit/retro/ship/debugger/code-review → an earlier phase, and ship → ship) is a round. `max_backward_edges` (default 3) ends the cycle with a Stop, and one failure fingerprint behind two edges jumps to rung 3. The 32-iteration crash guard stays as a guard.
    - **Across cycles:** an inbox item's attempts climb the same ladder, with *N* = `TaskRetryCeiling`. Attempt 3 changes the strategy, and after it the item is split, routed to the console, or quarantined, never retried the same way.
    - **Ship recovery** keeps its bound, under the policy's signals.
11. **Rollout:** `workflow.convergence.stage`, shadow then enforce. The console follows the decision from the start.
12. **One escalation path.** A blocked task climbs one path: retries, scope exit, redesign, operator.
    - The operator (2026-11-05): "review and update the adr related to the 3 retry on fixing the review CRITICAL and HIGH defects and trigger redesign as partner of ADR and pipeline flow."
    - The operator (2026-11-05): "It should be part of escalation path when pipeline blocked due to continues failed/errors".
13. **The host counts blocks.** One count for each subject crosses rounds, rebuilds, cycles and resumes.
    - A subject is a task, or a failure fingerprint when a failure has no task.
    - A block is one judgment with an open CRITICAL or HIGH defect, or one failure event of decision 16.
    - Only the pipeline writes the count. A seat cannot write it.
14. **Three retries, then a redesign.** A failed fourth review starts the redesign rung.
    - The operator (2026-11-06): "once we failed to 4th review, it should trigger the redesign".
    - A fourth failure starts the rung even when a seat finds no shared design fault. The rung then writes its card, and the review reads it.
    - When each judgment holds a CRITICAL or HIGH defect, `J_0` is block 1 and `J_3` is block 4. Then "no round 4" and "blocked over 3 times" are one count.
    - A Stop with three blocks or fewer keeps the rule of decision 5, with no redesign. A round with only MEDIUM findings adds no block.
    - The count does not depend on a loop's budget. The audit repair envelope caps its repairs, so block 4 often comes in a later cycle.
    - The count can end the `inbox-item` ladder of decision 10 before its final try. It then replaces the quarantine.
    - The task ceiling adds no block. An item at the ceiling routes to the console with its count, in place of the quarantine, and later blocks add to that count.
    - Split and accept keep their order and their conditions. A split component keeps its count, so past three blocks it takes the redesign rung.
15. **The redesign rung replaces a fourth patch.**
    - The fix rounds stop. The best round stays preserved, and every open finding is filed.
    - The subject leaves the lane queue and routes to the console, with a digest of all its findings.
    - A top-tier seat names what the findings share, and writes a new design card (ADR-0161).
    - The architecture review reads the card before any code. No seat and no console can waive this rung.
    - The count restarts when the redesign starts. Card blocks and code blocks both count.
    - A second count past three goes to the operator, with no second automatic redesign.
16. **Pipeline failures feed the same count.** The breakers keep their ceilings and their halts.
    - One event adds at most one block to a subject. A backward edge and a halt are not events of their own.
    - A failed cycle with a task adds one block to the task, and one to its fingerprint when it has one.
    - A failure with no task adds one block to its fingerprint. A host fault counts only on its fingerprint.
    - A zero-ship halt has no fingerprint, so it adds one block to the halt code.
    - A quota wall and a logout count nothing, and each signal names its own step.
    - An integrity block goes to the operator at once, with no redesign.
    - A halt files its item as today, and the item carries the count and the rung.
    - The table in the [convergence policy §3.3](../convergence-policy.md#33-one-escalation-path-the-redesign-rung) maps each class.
17. **The signal states the rung.** `CONVERGENCE_REDESIGN` names the subject, the count, the trigger, the cause or "unknown", the evidence and the next step. `CONVERGENCE_TO_OPERATOR` does the same for the last rung.

Console proposals: the block definition, the host-held count, the restart, the task ceiling rule, one redesign, and decision 16. The operator has not ruled on them. They ride `workflow.convergence.stage`, and each change of stage is an operator decision.

## Alternatives considered

| Alternative | Why not |
|---|---|
| A round cap only | It stops the grind but changes no input, and it lands the last round even when an earlier one was better (research F2). |
| Best-of-N fixers with a verifier | It multiplies cost on a quota-limited subscription. It is phase B, for cheap tiers only, after a strategy change. |
| "Just escalate the model" | There is no deep→top headroom today, and L2 already ran on the deepest model. The research says to escalate effort first, for reasoning failures only, and to change the strategy when refinement stalls. |
| Keep every severity blocking until clean | That is the L2 pattern. Deferral with filed follow-ups keeps the findings without holding the output (Google's "better, not perfect"). |
| Let any new finding block in delta rounds | LLM reviewers vary across runs, so new findings on unchanged code are mostly noise (research F3). Real late blockers still block through the falsification check. |
| Split a cycle by shipping its deferred HIGH rows | That is weaker than accept-with-limits. A cycle's split is a Stop plus a continuation. |
| A block rule beside the ladder (Revision 3) | Two rules can disagree on one task. Their counts are equal when each judgment holds a CRITICAL or HIGH defect, so one path is enough. |
| A redesign only for review blocks (Revision 3) | The operator placed it on the path for repeated failures and errors too. |
| A halt with no next rung (Revision 3) | That is today's flow. A loop stops, an item is filed, and the next try can repeat the same approach. |
| An automatic second redesign (Revision 3) | A second failed design is a question for the operator. It also has an unbounded cost at the top tier. |

## Consequences

- **At most three fix rounds per loop,** and the best qualifying round lands.
- **The redesign has a cost at the top tier,** and the fourth patch is gone. A fourth failed review pays for one new design card, and no seat can waive the rung.
- **Today no code does this.** `Decision.Redesign` holds only a component list, and only the CLI prints it. The halts stop the loop and start no redesign. Plan components R1 to R6 of [pipeline-blockers-2026-11](../../plans/pipeline-blockers-2026-11.md) build it.
- **Code-review's budget is unchanged** (3 fixes = 4 reviews), but each round now changes an input.
- **Inbox growth from deferrals and filings.** It is tracked: each follow-up carries its origin loop and round, and the deferred-then-closed share is measured.
- **The code-review landing-2 design (Q3) is built on `Decide`.**
- **Signals.** Module `convergence`:
  - `CONVERGENCE_RUNG`
  - `CONVERGENCE_NO_PROGRESS`
  - `CONVERGENCE_REPAIR_DAMAGE`
  - `CONVERGENCE_CONCENTRATION`
  - `CONVERGENCE_DEFERRED`
  - `CONVERGENCE_FILED`
  - `CONVERGENCE_SPLIT`
  - `CONVERGENCE_ACCEPTED_LIMITS`
  - `CONVERGENCE_STOP`
  - `CONVERGENCE_NO_HEADROOM`
  - `CONVERGENCE_REDESIGN`
  - `CONVERGENCE_TO_OPERATOR`

> **Note (2026-11-06):** The code also emits `CONVERGENCE_DISPUTED`. The one list of codes is [convergence-policy §9](../convergence-policy.md#9-signals-module-convergence).
