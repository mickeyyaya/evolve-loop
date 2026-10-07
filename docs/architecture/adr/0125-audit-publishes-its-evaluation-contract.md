# ADR-0125: The audit publishes its evaluation standard and, before build, this cycle's expectations; its verdicts are keyed to published criteria

- **Status:** Proposed (2026-10-07). Each decision moves to Accepted as its component lands; see the [plan](../../plans/audit-evaluation-contract-2026-10.md) §6.
  - **Approved by the operator in plan mode:** E-D1 to E-D4.
  - **After the operator asked for research before implementation:** refinements AR1–AR9 and E-D5, approved with the revised plan.
  - **Revision 2:** applies the E0 doc review (38 findings). E-D8 and E-D9 resolve the two findings that changed approved semantics, conservatively; the operator may override them.
- **Design (the core logic):** [audit-evaluation-contract.md](../audit-evaluation-contract.md).
- **Research:** [audit-evaluation-contract-prior-art-2026-10.md](../../research/audit-evaluation-contract-prior-art-2026-10.md).
- **Amends:**
  - [ADR-0124](0124-code-review-phase.md):
    - the quality index becomes the audit's full standard, adding Part 2, gate criteria with stable IDs, and dimension bars cite the criteria they cover;
    - the kernel cross-check (`AUDIT_QUALIFICATION_GAP`) gains the causes `criterion` and `criterion_kernel`;
    - code-review's Review Plan covers the dimensions the expectations mark required;
    - divergence excludes review rounds after an audit-repair re-entry.
  - [ADR-0098](0098-task-contract-block.md) (the Task Contract): its acceptance items are indexed `G-ACCEPT.<task>.<n>`, and the contract is delivered to the new `audit-plan` phase.
  - [ADR-0093](0093-retry-envelope-and-terminal-retro.md) and [ADR-0096](0096-repair-rounds-escalate-and-carry-findings.md): at enforce, the repair brief gains a criterion-keyed section, rendered first. Criterion flips declare `code-audit-fail` through `audit-qualification.json`. The envelope is unchanged.
  - [ADR-0028](0028-user-defined-phases.md) and [ADR-0038](0038-structured-phase-plugin-system.md):
    - a second user phase pinned by `conditional_mandatory` (`audit-plan`);
    - a second declared report grammar (`audit-expectations`), the first to take kernel-supplied inputs;
    - a degrade rule for pinned optional user phases that run before the floor.
  - The audit constitution (`docs/architecture/audit-constitution.md`): P1–P8 are unchanged, and `G-CONST` indexes them.
- **Amended by:** [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md). Audit findings also carry `kind` (defect or capability) and `blocking`. The audit records `kind` and never uses it to unblock, and its verdict rules are unchanged; see the [convergence policy](../convergence-policy.md) §3 (Kind) and §11.
- **Evidence:**
  - The auditor's standard is spread over a 23,956-byte persona and a 12,703-byte reference. Build sees only the acceptance.
  - The ADR-0096 ship-rate study: repair converges poorly when feedback arrives as a verdict.
  - The 2026-10-07 research dossier:
    - contract before code (F1);
    - visible checks saturate while hidden ones fail (F2, SpecBench);
    - rubrics that bound what may be claimed resist exploitation (F3);
    - deterministic checks must outrank the judge (F4);
    - same-family judges are lenient (F5);
    - added phases must earn their place (F7).

## Context

The audit is the ship gate, and the bar it applies is private to it:
- the verdict rules, the per-criterion evidence rule, the predicate-quality classes, goal integrity, the ADR-0084 gate lenses and the explanation review live in its persona and reference;
- the constitution and the kernel gates hold the rest.

Producers see fragments. Rejections reach build as prose findings through `composeRepairBrief`, with nothing tying them to a published rule.

The operator asked that the audit share what it evaluates (its directions, fields, scores, feedback and need-to-know) with build and the review phases, so that all of them align with the same standard and improve when the audit rejects their work. The operator also asked for at least two approaches with a winner, and for research into current AI-factory practice before implementation.

## Decision

1. **Strategy: C, a published standard + an `audit-plan` phase before build + criterion-keyed verdicts.** This was chosen over A (a static shared rubric) and B (a dry audit after review). Plan §2 compares them.
2. **One standard, in one home (E-D3).**
   - `skills/quality-index/` and `qualityindex` hold Part 1, the ten scored dimensions (ADR-0124), and Part 2, 13 gate criteria: `G-ACCEPT.<task>.<n>` (and `.eval`), `G-EVAL`, `G-ACS`, `G-PRED`, `G-GOAL`, `G-GATE`, `G-SCOPE`, `G-DISPO`, `G-CONST`, `G-EXPL`, `G-DOCS`, `G-COMMENT` and `G-SIZE`.
   - Each criterion carries a verifier (`kernel`, `audit` or `both`), a certificate kind with a fixed reference syntax, a dimension, an N/A rule, and PASS/FAIL anchors.
   - A failed criterion caps its dimension at 3.
   - Review resolution stays ADR-0124's qualification, not a criterion.
   - The standard's version (a hash of SKILL plus COMPACT) is pinned per cycle.
   - The standard is protected, and changes are operator-approved.
3. **The `audit-plan` phase (E-D2, E-D6).**
   - **What it is.** A Plan-archetype user phase with its own persona (`agents/evolve-audit-planner.md`), dispatched to the audit role's Claude floor at the deep tier.
   - **Placement.** It runs `after: "build-planner"`, immediately before build, and is pinned by `cycle_size!=trivial && deliverable_kind==code`, guarded by a protected pin test. It runs **at most once per cycle**, including on audit-repair re-entry.
   - **Inputs.** The kernel computes them: the pre-build reports, the eval file, the cycle's ACS predicates and the ancestor ledger, never a post-build output. The kernel also pre-renders the deterministic skeleton.
   - **Output.** The planner fills the judgment cells of `audit-expectations.md`:
     - the acceptance trace (each item → its proving RED test, or a contract gap);
     - the evidence each criterion demands for this change;
     - the dimension bars and the risks to watch;
     - the condition-class rejection triggers;
     - the need-to-know for build and for review.
   - **Checks.** 8 deterministic checks through an input-bearing grammar.
   - **Degrade.** An exhausted correction ladder, a failed dispatch or a quota wall degrades to `AUDIT_PLAN_SKIPPED` under a new rule for pinned optional user phases. It never aborts or carries the breaker.
4. **Delivery.**
   - Build, code-review and the final audit receive recipient-specific digests, byte-capped by config, plus the document's path. Build and audit get them through a kernel predicate; code-review through `prompt_context`.
   - The expectations are immutable after `audit-plan`. They, tdd's test files, `go/acs/cycle<N>/**` and the eval file are hash-and-restore fenced against the producers. That fence is new.
   - Floor-computed criteria (`G-SIZE`, the protected-surface half of `G-SCOPE`, and `G-COMMENT` at enforce) fail at build's handoff floor with their ID and remediation text.
   - Each audit FAIL is appended to the kernel-owned `audit-rejections.md`.
5. **Criteria public, instruments private (E-D5).**
   - Every criterion, the evidence it demands and the condition-class triggers are published.
   - The final audit's probe instances (edge inputs, mutants, adversarial cases) are not.
   - A probe failure is reported against its published criterion.
6. **The final audit is held to its plan.**
   - A `## Criteria` section opens with the `Standard:` line, then gives one line per published criterion: PASS with a certificate, FAIL citing a finding, or N/A with a reason.
   - A FAIL on a criterion the plan marked N/A needs a justification and raises `AUDIT_PLAN_DIVERGENCE`.
   - A FAIL on an unpublished requirement must be declared `Criterion: NEW — <name>` with a justification, which raises `AUDIT_UNPUBLISHED_CRITERION` (E-D1).
7. **Kernel facts and the cross-check: logic, not format, and only toward FAIL (E-D8, E-D9).**
   - The kernel records kernel-verified results in `kernel-facts.json`. A fact counts only at its gate's own stage: WARN-only `docsfloor` and shadow `comment_floor` are evidence, never overrides (E-D9).
   - A red enforce-stage fact overrides an audit PASS.
   - A PASS or WARN with a failed applicable criterion becomes FAIL.
   - A PASS certificate that does not resolve FAILs its criterion.
   - A FAIL finding without a certificate is malformed and goes to the correction rung. The kernel never lowers a verdict (E-D8).
   - An unknown non-NEW criterion, or a version mismatch, is malformed.
   - A same-family audit raises `AUDIT_SAME_FAMILY` and never blocks.
   - Every flip declares `code-audit-fail` through `audit-qualification.json`.
8. **Feedback.**
   - At enforce, the repair brief renders the failed criteria first (ID, evidence demanded, finding, certificate, fix, need-to-know). In shadow it is byte-identical.
   - Code-review's delta round reads the rejections first, and its Review Plan covers each failed criterion's dimension.
   - Audit ledger rows gain the additive `criterion` field.
9. **Learning and the exit test (AR9).**
   - The kernel records per-criterion results, NEW criteria, plan-versus-audit divergence, contract gaps, uncertified findings and escapes.
   - At the flip, the operator keeps, slims or drops `audit-plan` on the shadow data: first-pass PASS rate, repair rounds, cost and escape rate.
10. **Rollout (E-D4).**
    - `workflow.audit_contract.stage` defaults to `shadow`: prompts change, the auditor writes `## Criteria` with its verdict rules unchanged, and every cross-check only signals.
    - The auditor's NEW duty, uncertified handling and checklist removal land at the flip.
    - Enforce is flipped in the same boundary commit as code-review's stage, after two shadow waves.
11. **Recovery rungs.** Only logic blocks.
    - No capacity, or a failed `audit-plan`: `AUDIT_PLAN_SKIPPED`, the static standard, and the cycle continues.
    - A fenced file was modified: restore, signal.
    - Absent expectations at the audit: evaluate against the full standard.

## Alternatives considered

| Alternative | Why not |
|---|---|
| A: a static shared rubric only | Generic. Build still learns this change's bar by failing it, and rejections stay unkeyed. C includes A as its Part 2. |
| B: a dry audit after review, every round | It arrives after the work is written, costs a deep dispatch per round, and trains builders to please one judge's opinion. |
| Hold every criterion out (StrongDM-style holdout; the manifesto's "criteria should not be visible") | It contradicts the operator's request, and a hidden bar gives repair nothing to aim at. The research supports holding out *instances*, not criteria (E-D5); rotation is a future refinement. |
| Publish the probe instances too | Visible checks get saturated while held-out ones fail (SpecBench). Publishing them would turn the audit's adversarial pass into a checklist to pass. |
| Downgrade an uncertified sole FAIL to WARN | WARN ships, so a CRITICAL finding with a missing field would ship. The kernel must never lower a verdict (E-D8). |
| Let every kernel fact override the audit | It would make WARN-only `docsfloor` and shadow `comment_floor` blocking through a side door, with no decision of their own (E-D9). |
| Re-run `audit-plan` on audit-repair re-entry | Its inputs would include the final audit's report (leaking the probes), and its output would move the bar mid-cycle. |
| Write the expectations with the kernel alone (a deterministic template) | It cannot read intent, plan reviews or bug reproductions to name this change's risks. The operator chose the audit agent (E-D2). The kernel still pre-renders every deterministic cell. |
| An "audit plan mode" in the auditor persona | It is a flag in disguise, and it grows a persona already over budget (E-D6). |
| Keep rejections inside the expectations document | Two writers in one fenced file race the hash check. A kernel-owned `audit-rejections.md` keeps one writer per file. |
| A JSON sidecar for the expectations | A second home for the format. The markdown grammar is machine-read the same way as code-review's report. |

## Consequences

- **Cost.** One more deep dispatch per non-trivial code cycle. There is no extra cost per review round. The exit test measures whether it pays.
- **One bar, published.** Every non-trivial code cycle has an expectations document. Every audit verdict on a code cycle has a criterion vector, and every rejection names a criterion and a certificate.
- **Gaming resistance.** The fenced instruments cannot be edited by producers, the probe instances stay private, and uncertified claims are sent back for a certificate.
- **No new blocking by a side door.** Kernel facts respect their gates' stages, and the kernel never lowers a verdict.
- **Persona slimming.** The auditor's checklists move into the standard and are referenced by ID at the flip. This merges with ADR-0124's slimming, and it corrects the persona's claim that a Go constitution check exists.
- **Ledger.** Rows gain `criterion` (additive, `omitempty`). Older readers ignore it.
- **Signals.** Module `audit` gains (fields in design §7):
  - `AUDIT_CRITERION_RESULT`
  - `AUDIT_UNPUBLISHED_CRITERION`
  - `AUDIT_PLAN_DIVERGENCE`
  - `AUDIT_PLAN_CONTRACT_GAP`
  - `AUDIT_FINDING_UNCERTIFIED`
  - `AUDIT_SAME_FAMILY`
  - `AUDIT_CRITERION_ESCAPE`
  - `AUDIT_PLAN_SKIPPED`
  - `AUDIT_EXPECTATIONS_TAMPERED`

  `AUDIT_QUALIFICATION_GAP` gains the causes `criterion` and `criterion_kernel`, and a `criterion` field.
