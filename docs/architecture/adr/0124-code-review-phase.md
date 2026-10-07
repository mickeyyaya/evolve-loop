# ADR-0124: An independent code-review loop sits between build and audit, and the audit qualifies its result on one shared quality index

- **Status:** Proposed (2026-10-07). Each decision moves to Accepted as its component lands; see the [plan](../../plans/code-review-phase-2026-10.md) §6.
  - **Morning, approved by the operator:** coverage, the loop, rollout and the audit's role.
  - **Afternoon, approved in plan mode:** the review-loop redesign. It supersedes the morning's one-fix-round loop and grade-only audit (plan §3, Q-D1 to Q-D4, D3 to D9).
  - **Landing 1** (R0–R3 plus the stage switch R8a, held in shadow) is staged and amended by Q0–Q2 before its review.
- **Design (the core logic):** [review-loop-and-quality-index.md](../review-loop-and-quality-index.md).
- **Amended by:** [ADR-0125](0125-audit-publishes-its-evaluation-contract.md) (Proposed, 2026-10-07).
  - The quality index becomes the audit's full published standard (Part 2, gate criteria), and the dimension bars cite the criteria they cover.
  - An `audit-plan` phase writes each cycle's expectations before build.
  - The cross-check gains the causes `criterion` and `criterion_kernel`.
  - The Review Plan covers the dimensions the expectations mark required.
  - Divergence excludes review rounds after an audit-repair re-entry.
- **Amended by:** [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md) (Proposed, 2026-10-07).
  - The landing-2 loop calls the convergence policy with `max_fix_rounds` 3, equal to `max_rounds` 4, so the budget is unchanged.
  - A row the policy DEFERS at rung 2 (MEDIUM/LOW, with a filed follow-up) no longer counts as strict OPEN in §6.2.
  - Accept-with-limits rows go to the audit's adjudication, as DISPUTED rows do.
- **Amends:**
  - [ADR-0028](0028-user-defined-phases.md) and [ADR-0038](0038-structured-phase-plugin-system.md):
    - a user phase may be pinned by a `conditional_mandatory` rule. This is the first user phase the registry pins. It stays optional-only and never satisfies or displaces the build → audit → ship floor;
    - a user phase may declare report grammars (`classify.grammars`), which the deliverable gate checks like its required sections.
  - [ADR-0093](0093-retry-envelope-and-terminal-retro.md) and [ADR-0096](0096-repair-rounds-escalate-and-carry-findings.md): the audit-repair machinery generalizes into ONE findings-driven repair keyed by judge. That machinery is the durable attempt counter, the findings seed, `composeRepairBrief` and the disposition protocol. The audit's own grant stays the retry envelope's, byte for byte. Code-review's grant is deterministic.
  - [ADR-0103](0103-component-breakdown-program.md) unit 09 (the defect ledger):
    - entries gain the additive fields `source`, `round`, `severity` and `dimension` (landing 1), and the `DISPUTED` status for code-review rows (Q4);
    - the merge rule becomes one exported function, `defectledger.Append`, and inherited rows keep their provenance through the reconcile gate.
  - The audit's verdict: a shippable verdict (PASS or WARN) is cross-checked against the quality index, and a change that does not qualify is a FAIL (logic, not format), declaring the failure class `code-audit-fail`.
- **Evidence:**
  - the ADR-0096 ship-rate study: repair converges poorly once feedback arrives as a verdict;
  - the auditor persona at 23,956 bytes against the 18,102-byte ACS baseline (`go/acs/cycle419`);
  - the 2026-10-06/07 console review chain: every lane reviewed had diff-only defects and needed 2–3 rounds to converge;
  - the staged agent-config lane C0 (`dev/cl-agentcfg`) and its self-review part C0c: self-simplify and self-review inside build, never independent.

## Context

The audit was both the loop's only code reviewer and its ship gate.
- **Late feedback.** A builder first heard about a defect when the audit rejected the change, as verdict prose.
- **One judgment, every lens.** That single judgment had to cover correctness, architecture, tests, performance, concurrency, debuggability and security.
- **No shared bar.** The ten single-lens evaluate phases fired only on advisor judgment. No definition of "qualified" was shared between reviewer and gate.

The operator asked for these changes:
- an independent review after build and before audit, covering every review lens;
- a review that justifies which reviews a change needs, reading the earlier phases' deliverables as needed;
- feedback with suggestions back to build, repeated until every concern is resolved;
- an audit that evaluates the reviewed and fixed output from the same perspectives, deciding whether it qualifies on every index score;
- the simplifier staying inside build.

## Decision

1. **Strategy: a shared quality index, a lead reviewer, and triggered specialists.** This is S3, chosen over S1 (one multi-lens reviewer) and S2 (a lens panel every round); plan §2 compares them.
   - Phase A ships the lead reviewer with the full index.
   - Phase B adds specialist lenses only where the audit keeps out-scoring the reviewer on a dimension.
2. **One quality index** is the shared contract (design §2):
   - ten dimensions, each with a 4/5 bar, N/A rules and a review procedure;
   - one `## Scores` grammar;
   - one `Qualifies(scores, thresholds)` used by the loop, the audit and the shadow metrics.

   It lives in the protected skill `skills/quality-index/` and the package `qualityindex`. The default threshold is 4 for every dimension, and thresholds are config.
3. **The `code-review` phase.**
   - It is an Evaluate-archetype user phase pinned by `conditional_mandatory["code-review"] = "deliverable_kind==code && build.files_touched>0"`, so it covers every code cycle that touched files.
   - Every round opens with a justified **Review Plan** (design §3): each dimension is required (light, standard or deep) or N/A, with evidence from the diff and from the earlier phases' deliverables. The Task Contract reaches it in the prompt; the other deliverables are read from the workspace.
   - Every finding carries a suggested fix and names its dimension.
   - Every round scores the full index.
   - The report grammar (plan, scores, their agreement, and a cited finding for every gap) is checked by the deliverable gate, so a malformed report gets the correction rung.
4. **It is not a gate.** Its verdict never unlocks ship. The audit stays the only gate and is never re-rolled, and `remediationDenied` keeps refusing LLM-judgment re-rolls.
5. **Findings are defect-ledger rows** (`source: code-review`, `round`, `severity`, `dimension`). They merge through `defectledger.Append`, and build dispositions every id.
6. **The loop until resolved** (design §5).
   - **The decision is deterministic,** made after every round. "Resolved" (Q-D2) means:
     - every strict finding (at or above the configured threshold, default MEDIUM) is FIXED with evidence, or DEFERRED with a reason the reviewer accepts;
     - every finding below the threshold is dispositioned, and is never rejected;
     - the review's Scores qualify.
   - **Stops:**
     - resolved;
     - the budget of 4 rounds;
     - no progress: the unresolved mass did not strictly decrease;
     - oscillation: a still-OPEN id had its FIXED claim rejected twice;
     - a ledger fault.

     Every stop except resolved continues to the audit with `REVIEW_UNRESOLVED` (or `REVIEW_SKIPPED` for the fault).
   - **A strict deferral rejected twice becomes DISPUTED.** The audit adjudicates it.
   - **The delta re-review** verifies FIXED claims, judges deferrals, scans the fix diff and re-scores. Same-cycle rows take their dispositions through `applyReviewDispositions`.
   - **The fix-round brief** carries the unresolved findings with their suggestions, the rejected claims, and the dimension gaps. TDD-owned tests and `go/acs/**` stay read-only for the builder.
   - **The backward edges** (`code-review → build`, `build → code-review`) are scheduled by the kernel through `scheduledNext` and are decision-only, so the advisor never proposes them.
7. **The audit is the final evaluation on the same index** (design §6).
   - It scores the index itself, blind to the review's Scores: it works from a kernel-written review digest and never opens the raw review report.
   - It adjudicates every DISPUTED id: UPHOLD reopens it, OVERTURN accepts the deferral, and a row still DISPUTED at cycle close becomes OPEN.
   - It keeps acceptance, evals, integrity, scope and the disposition GRADE.
   - **The kernel cross-checks** the result: a shippable verdict (PASS or WARN) with a dimension gap, an OPEN strict finding, or an upheld or unruled dispute is a FAIL (`AUDIT_QUALIFICATION_GAP`). The flip declares the class `code-audit-fail`, so the existing audit-repair path grants a repair.
   - **Divergence** between the audit's and the review's final vectors (≥2 on a dimension, or an N/A mismatch) is recorded (`REVIEW_AUDIT_DIVERGENCE`). An audit-lower divergence is the learning signal and the phase-B trigger.
   - The auditor persona drops its code-review checklists at the flip.
8. **The simplifier stays in build.** C0c's self-simplify and self-review are the author's own pass, scoped to the build's files. The reviewer loads code-review-simplify in review mode and applies nothing.
9. **Rollout by stage** (`workflow.findings_repair.code-review.stage`):
   - **Shadow** (compiled default):
     - the review plans, finds and scores every round;
     - rows are DEFERRED, so a continuation never owes them;
     - no fix round runs, but the loop's decision is computed as if enforced and signalled;
     - the audit's index scoring, the cross-check (would-fail) and divergence run in shadow, without changing verdicts or adding a correction dispatch.
   - **Enforce:** the operator flips it at a wave boundary after two shadow waves.
10. **Independence.**
    - A fresh agent with no builder transcript.
    - A read-only fence (ADR-0097), and a hash-and-restore check on the review's own artifacts, since the Claude-only role hook cannot stop another CLI's writes.
    - A Claude-floor profile at the deep tier: a different family from the codex builder today, and from the agy builder once the routing lane L2 lands. A same-family dispatch is a WARN (`REVIEW_SAME_FAMILY`), never a block.
    - A blind audit.
11. **Recovery rungs.** Only logic blocks.
    - No deep capacity: `REVIEW_SKIPPED`, and the audit still scores the index.
    - A malformed section: the correction rung.
    - A missing disposition: the row stays OPEN, plus a signal.
    - A ledger fault: the loop stops; the audit treats the unrecorded findings as OPEN.

## Alternatives considered

| Alternative | Why not |
|---|---|
| S1: one multi-lens reviewer, without a shared index | Shallow per lens, with about ten rubrics in one prompt; and the audit would still judge on its own vocabulary. S3's phase A is S1 *plus* the index. |
| S2: a parallel lens panel every round | About 8 dispatches per round across up to 4 rounds strains Claude quota, and contradicting lenses with no arbiter make the build oscillate. |
| Extend the audit's own review | It keeps the gate reviewing its own fix requests, and grows a persona already over budget. |
| Remediate via `maybeRemediate` (re-run the review as a gate) | `remediationDenied` exists to refuse LLM-judgment re-rolls; the loop must be driven by defects, not by a re-rolled verdict. |
| An unbounded "until resolved" loop | A livelock risk with unbounded cost. The operator chose a 4-round budget plus a no-progress stop. |
| Let the audit read the review's Scores | Anchoring would turn the audit into a rubber stamp. The divergence signal would lose its meaning. |
| Cross-check PASS only | A WARN ships too, so an unqualified WARN would ship. |
| `routing.insert_when` only | At the Advisory stage it gates the advisor's plan rather than inserting the phase, so coverage would depend on the advisor. |
| A text prefix instead of fields, or a JSON sidecar of findings | Either gives the format a second home; the report grammar is already machine-read by `reportdoc`. |
| A phase-name branch in the deliverable gate for the report grammar | A declared, registered grammar keeps the check config-driven and lets the audit declare its own Scores grammar later. |
| `cross_family_with: builder` on the reviewer profile | The routing table's compile check would make a same-family start an error; the operator wants a WARN. |

## Consequences

- **Review cost.** Every code cycle pays at least one deep review dispatch. A non-converging cycle pays at most 4, plus 3 fix builds. The shadow waves measure cost and benefit before enforce.
- **A defined bar.** "Qualified" has one definition, and every shipped code cycle carries a full audit index vector, all ≥ 4.
- **One judgment per role.** The audit's persona sheds its code-review checklists. Review depth comes from a reviewer that plans its scope and from data-driven specialists.
- **Ledger growth.** The defect ledger exists on most code cycles and gains the `dimension` field and, with Q4, the `DISPUTED` status. Readers that predate them ignore the field. A DISPUTED row is adjudicated or reopened at close, never inherited unruled.
- **Signals.** Module `review` (design §10): `REVIEW_FINDINGS`, `REVIEW_PLAN`, `REVIEW_ROUND`, `REVIEW_REPAIR_GRANTED`, `REVIEW_RESOLVED`, `REVIEW_UNRESOLVED`, `REVIEW_FINDING_DISPUTED`, `REVIEW_DISPOSITION_MISSING`, `REVIEW_ROW_TAMPERED`, `REVIEW_SKIPPED`, `REVIEW_SAME_FAMILY`, `REVIEW_BUDGET_SPENT` and `REVIEW_AUDIT_DIVERGENCE`. Module `audit` gains `AUDIT_QUALIFICATION_GAP`.
