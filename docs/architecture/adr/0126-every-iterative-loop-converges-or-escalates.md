# ADR-0126: Every iterative loop converges or escalates: feedback, then strategy, then scope; land the best round; never a fourth

- **Status:** Proposed (2026-10-07), a console proposal from the operator's directives. It moves to Accepted as its components land ([plan](../../plans/convergence-policy-2026-10.md) §6).
  - **Revision 2** applies the doc review (C1, C2, H1–H5, M1, M2, L1–L3), the research refinements CR1–CR8, and the operator's headroom answer.
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
     - verify-only re-judgment for code-review and console judges;
     - an effort raise within the judge's family (deep high → top xhigh), for reasoning-class blockers, with headroom;
     - a fixer raise.
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
   - A late finding (on code unchanged since `J_0`) below HIGH is filed. A late CRITICAL or HIGH blocks only when its certificate survives a falsification check that sees only the diff.
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

## Alternatives considered

| Alternative | Why not |
|---|---|
| A round cap only | It stops the grind but changes no input, and it lands the last round even when an earlier one was better (research F2). |
| Best-of-N fixers with a verifier | It multiplies cost on a quota-limited subscription. It is phase B, for cheap tiers only, after a strategy change. |
| "Just escalate the model" | There is no deep→top headroom today, and L2 already ran on the deepest model. The research says to escalate effort first, for reasoning failures only, and to change the strategy when refinement stalls. |
| Keep every severity blocking until clean | That is the L2 pattern. Deferral with filed follow-ups keeps the findings without holding the output (Google's "better, not perfect"). |
| Let any new finding block in delta rounds | LLM reviewers vary across runs, so new findings on unchanged code are mostly noise (research F3). Real late blockers still block through the falsification check. |
| Split a cycle by shipping its deferred HIGH rows | That is weaker than accept-with-limits. A cycle's split is a Stop plus a continuation. |

## Consequences

- **At most three fix rounds per loop,** and the best qualifying round lands.
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
