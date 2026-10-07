# Audit evaluation contract: the audit publishes its standard and this cycle's expectations, so build and review aim at the same bar (plan, 2026-10)

> **Purpose.** This is the living plan for the audit evaluation contract.
> - **What it adds:**
>   - The audit's full standard (the quality-index dimensions plus gate criteria with stable IDs) is published in one home.
>   - An `audit-plan` phase, run by the audit role once per cycle immediately before build, reads every pre-build deliverable and writes this cycle's expectations: criteria, dimensions, rejection triggers, and need-to-know for build and for review.
>   - The final audit is held to that plan, and every rejection is keyed to a criterion, so the next round knows exactly what to fix.
> - **Approved by the operator on 2026-10-07** in plan mode (decisions E-D1 to E-D4).
>   - After the operator asked for research before implementation, the plan was revised with refinements AR1–AR9 and decision E-D5, and approved again.
>   - The E0 doc review's 38 findings were then applied (revision 2, §10). Two of them changed approved semantics, and the console decided them conservatively (E-D8, E-D9).
> - **Design (the core logic):** [audit-evaluation-contract.md](../architecture/audit-evaluation-contract.md).
> - **Decision record:** [ADR-0125](../architecture/adr/0125-audit-publishes-its-evaluation-contract.md).
> - **Research:** [audit-evaluation-contract-prior-art-2026-10.md](../research/audit-evaluation-contract-prior-art-2026-10.md).
> - **Builds on:** the code-review plan, [code-review-phase-2026-10.md](code-review-phase-2026-10.md), and its design, [review-loop-and-quality-index.md](../architecture/review-loop-and-quality-index.md) (ADR-0124). It lands inside that plan's landing 2, under the same shadow-then-flip.
> - **Updates:** every landing updates the status column in §6 and the landing notes in §10.
>
> Operator requests (2026-10-07):
> - *"The audit phase agent should share the guidelines for what directions, fields and score it evaluates, feedbacks and the need-to-know info for both build and review related phase agents, so they have a clear understanding and expectation to align with the same standard to improve when audit rejects the deliverables from build + review phases. Ultrathink from pipeline design and architecture perspective to come up with at least two approaches and pick the winner from them. Use plan mode"*
> - *"Overall is fine. Could you research the latest ai software factory pipeline design and ai driven design similar topics to gather more information before jumping to the implementation"*
> - *"All research, plan, decision, and architecture docs must be properly written under the docs"*

> **Terms.**
> - *Standard*: the audit's published evaluation standard. Part 1 is the ten scored dimensions; Part 2 is the gate criteria (design §2).
> - *Gate criterion*: a binary check with a stable `G-…` ID, a verifier, a certificate kind, a dimension, an N/A rule and anchors.
> - *Expectations*: `audit-expectations.md`, written once by `audit-plan` and immutable after (design §3).
> - *Rejections*: `audit-rejections.md`, the kernel-owned history of audit FAILs (design §4.4).
> - *Kernel facts*: `kernel-facts.json`, the kernel's results for kernel-verified criteria (design §5.2).
> - *Certificate*: reproducible evidence of a closed kind, with a fixed reference syntax (design §2.2).
> - *Probe instance*: a specific input or mutant the final audit uses. It is never published (design §5.3).
> - *Contract gap*: an acceptance item with no proving RED test at planning time (design §3.4).
> - *Digest*: the byte-capped projection of the expectations in a prompt (design §4.1).
> - *NEW criterion*: an unpublished requirement the audit declares and justifies (design §5.4).
> - *Escape*: a later defect traced to a criterion a shipped cycle PASSed (design §7).
> - *AR1–AR9*: this plan's research-derived refinements (§4). The *R* components (R0–R3, R8a, R8b) belong to the code-review plan.
> - *RL*: the review-loop design doc. *Q3–Q6*: the code-review plan's landing-2 components.
> - *Console*, *plane*, *lane*, *floor*: as in the code-review plan's terms.

## 1. Why

- **The audit's bar is private.** Its verdict rules, per-criterion evidence rule, predicate-quality classes, goal integrity, gate lenses, checklist and explanation review live in the auditor persona and its reference (36,659 bytes together). Build sees only the acceptance and one line about ground truth. Code-review sees only the quality index.
- **Rejections arrive as prose.** Nothing ties a rejection to a published rule, so a builder cannot tell a missed requirement from a moved goalpost, and the repair converges slowly (the ADR-0096 study).
- **The review loop needs the same bar.** The code-review redesign gave review and audit one shared quality index. This plan finishes that work: the gate's binary criteria join the index, and the audit states its expectations for the change before anyone writes code.

## 2. Strategies considered

| | A. Static shared rubric | B. Audit preview (a dry audit after review) | **C. Published standard + `audit-plan` before build + criterion-keyed verdicts (chosen)** |
|---|---|---|---|
| What build and review learn | the generic rubric | one judge's opinion of the finished work | the standard **and** this cycle's expectations, before any code |
| When | always | after build and review, every round | before build, once per cycle |
| Rejection feedback | prose | prose | keyed by criterion and dimension, with a certificate and a fix |
| Moving goalposts | invisible | invisible | an unpublished criterion must be declared NEW; the bar is pinned per cycle |
| Cost | prompt only | +1 deep dispatch per round | +1 deep dispatch per non-trivial code cycle |
| Gaming risk | low | builders tune to one judge | low: criteria public, probe instances private (AR1), certificates (AR2) |

**Why C.**
- It is the only option that sets one bar for every phase *before* work starts.
- It makes every rejection traceable to a published criterion.
- It learns when the standard is incomplete, through NEW criteria.

A alone is generic. B arrives late and trains builders to please one judge.

The research (§4) confirmed C's shape: "contract before code" is how Anthropic's harness and the spec-driven tools work. Agentic Rubrics adds that context-grounded criteria written before evaluation help.

## 3. Decisions

| # | Topic | Decision | Decided by |
|---|---|---|---|
| E-D0 | Strategy | **C:** a published standard, an `audit-plan` phase before build, and criterion-keyed verdicts. | console, at the operator's request ("at least two approaches … pick the winner") |
| E-D1 | Unpublished criteria | **The audit may FAIL on one only by declaring it NEW, with a justification.** That raises `AUDIT_UNPUBLISHED_CRITERION`, and the retrospective proposes adding it to the standard. The standard is protected, so any change is operator-approved. | operator |
| E-D2 | Who writes the expectations | **The audit agent itself, in an `audit-plan` phase before build.** It must reference **every deliverable the earlier phases produced before build**; the kernel checks coverage. | operator |
| E-D3 | Where the standard lives | **Extend `quality-index`** (skill and package) into the full standard: the dimensions plus the gate criteria. One home. | operator |
| E-D4 | Rollout | **Follow code-review:** shadow for 2 waves, then enforce, flipped at the same boundary as the review loop. | operator |
| E-D5 | What is published | **Criteria public, instruments private.** Every criterion, its evidence kind, the condition-class triggers and the need-to-know are published. The final audit's probe *instances* are not, and producers cannot write the fenced instruments. Consistent with E-D1: nothing is judged on an unpublished criterion. | console, from the research; approved with the revised plan |
| E-D6 | Persona | **`audit-plan` gets its own small persona,** `agents/evolve-audit-planner.md`, rather than a mode of the auditor persona. A mode would be a flag (no flags, by policy), and the auditor persona is already over budget. | console |
| E-D7 | Plan location | **This plan is its own document** rather than a section of the code-review plan. It is a separate operator request with its own decisions; the code-review plan links it from landing 2. | console |
| E-D8 | A FAIL finding without a certificate | **It is malformed and goes to the correction rung. The kernel never lowers a verdict.** On exhaustion the finding stands as FAIL and is listed as Advisory. This supersedes the revised plan's "advisory, downgrades a sole FAIL to WARN". That rule would have let a CRITICAL finding with a missing field ship (WARN ships), against the auditor's verdict rule and ADR-0124 D8 (the kernel flips only toward FAIL). The operator may override with an explicit decision. | console, from the doc review (D1); the operator may override |
| E-D9 | Kernel facts from WARN-only or shadow floors | **A kernel fact counts only at the stage its own gate runs at.** `docsfloor` (WARN-only) and `comment_floor` (shadow by default) are evidence the audit weighs, never overriding facts, so this contract never makes them blocking by a side door. Promoting either floor is that floor's own operator decision. | console, from the doc review (D5); the operator may override |

## 4. Refinements from the research (AR-numbered; cited in the design)

| # | Refinement | Basis (dossier finding) |
|---|---|---|
| AR1 | Criteria public, instruments private. The expectations, tdd's test files, `go/acs/cycle<N>/**` and the eval file sit under a hash-and-restore fence (new in E3). Rejection Triggers name condition classes, never probe inputs. | F2: StrongDM, SpecBench, manifesto P8; repo P7 |
| AR2 | Evidence is a certificate of a closed kind with a fixed syntax. A FAIL finding without one goes to the correction rung (E-D8). | F3, F6: ImpossibleRubrics (the idea of bounding what may be claimed), Refute-or-Promote, the persona's per-criterion evidence rule |
| AR3 | Enforce-stage kernel facts outrank the judge. Floor-computed criteria fail at build's handoff floor with remediation text. | F4: PROCTOR, OpenAI, Stripe; repo P2 and P6 |
| AR4 | Acceptance traceability: `G-ACCEPT.<task>.<n>`, with contract gaps flagged before build | F1: Anthropic's sprint contract, Kiro |
| AR5 | The expectations name outcomes and evidence, never implementation | F1: Anthropic's planner; Agentic Rubrics' over-specification mode |
| AR6 | The standard's version (SKILL plus COMPACT) is pinned per cycle | F5: PReMISE |
| AR7 | Calibration anchors per criterion and dimension; `AUDIT_SAME_FAMILY` | F5: Anthropic, the self-preference-bias study |
| AR8 | Delivery is a byte-capped digest plus the document's path | F7: OpenAI, ETH Zurich (via marmelab) |
| AR9 | The phase must earn its place: trivial cycles skip it, the shadow exit criteria, the escape rate | F7: Anthropic's harness removal, marmelab |

Repo principles that apply, from [factory-pipeline-findings-2026-09.md](../research/factory-pipeline-findings-2026-09.md) §2:
- P1, one home (the standard; criteria cite dimensions);
- P2, the host owns deterministic work (kernel facts, the pre-rendered skeleton);
- P4, tell the producer what is owed (this whole plan);
- P7, a cycle cannot edit what grades it (AR1);
- P9, settings are config (`workflow.audit_contract`);
- P11, coded signals (design §7);
- P13, process failures recover (design §3.5, §8).

## 5. Design summary

The full design is in [audit-evaluation-contract.md](../architecture/audit-evaluation-contract.md).

- **The standard (design §2).**
  - The quality-index skill gains Part 2, with 13 gate criteria:
    - `G-ACCEPT.<task>.<n>` (and `.eval` for a task with no acceptance list);
    - `G-EVAL`, `G-ACS`, `G-PRED`;
    - `G-GOAL`, `G-GATE`, `G-SCOPE`, `G-DISPO`, `G-CONST`;
    - `G-EXPL`, `G-DOCS`, `G-COMMENT`, `G-SIZE`.
  - Each criterion carries a verifier, a certificate kind, a dimension, an N/A rule and anchors.
  - A failed criterion caps its dimension at 3.
  - Review resolution stays RL's qualification, not a criterion.
  - The version (hash of SKILL plus COMPACT) is pinned per cycle.
- **`audit-plan` (design §3).**
  - It is a Plan-archetype user phase, `after: "build-planner"`, immediately before build, pinned by `cycle_size!=trivial && deliverable_kind==code` (with a protected pin test).
  - It runs on the audit role's Claude floor at the deep tier, at most once per cycle (no re-run on audit-repair re-entry).
  - The kernel computes its inputs (the reports, eval file, ACS predicates and ancestor ledger, never post-build outputs) and pre-renders a skeleton. The planner fills only the judgment cells of `audit-expectations.md`.
  - 8 deterministic checks run through an input-bearing grammar.
  - A new degrade rule covers pinned optional user phases: exhaustion, a failed dispatch or a quota wall gives `AUDIT_PLAN_SKIPPED`, never an abort.
- **Delivery (design §4).**
  - Recipient digests are capped at 2,048 bytes by default (minimum 512), plus the document's path.
  - Build and audit get them through an `auditExpectationsPhase` predicate; code-review through `prompt_context`.
  - A hash-and-restore fence covers the expectations and the cycle's instruments.
  - Floor-computed criteria (`G-SIZE`, the protected-surface half of `G-SCOPE`, and `G-COMMENT` at enforce) reach build at its handoff floor.
  - Rejections go to the kernel-owned `audit-rejections.md`.
- **The final audit (design §5).**
  - `## Criteria` opens with a `Standard:` line, then gives one line per published criterion, with a certificate.
  - Private probe instances; NEW criteria.
  - `kernel-facts.json`.
  - The cross-check only moves verdicts toward FAIL: a red enforce-stage fact wins; a shippable verdict with a failed criterion becomes FAIL; an unresolved PASS certificate means FAIL; an uncertified FAIL finding goes to correction; a version mismatch or an unknown criterion is malformed. Each flip declares `code-audit-fail` through `audit-qualification.json`. In shadow, everything only signals.
- **Feedback (design §6).**
  - The repair brief gets a criterion section first, in enforce only, so the shadow golden is unchanged.
  - Code-review's Review Plan covers the expectations' dimensions and each failed criterion's dimension.
  - Ledger rows gain `criterion`.
- **Learning (design §7).** Nine `AUDIT_*` signals with their fields, plus the exit test that keeps, slims or drops `audit-plan` at the flip.

## 6. Components

Status values: ☐ not started · ◐ staged · ☑ landed (with PR).

These land **inside the code-review plan's landing 2** (with Q3–Q6), in shadow, after landing 1 (PR-A + PR-B) merges. Red tests come first, components land unwired before wired, and at most 4 agents run concurrently.

| # | Component | Where | Status |
|---|---|---|---|
| E0 | The docs:<br>• this plan, the design doc, ADR-0125, and the research dossier with its `research-index.md` entry;<br>• ADR-0124's "Amended by" line;<br>• the code-review plan's landing-2 link;<br>• the `factory-pipeline-findings` P4/P7 evidence pointers and §5.1 rows (its §6 procedure);<br>• the audit-constitution cross-reference;<br>• the doc review (38 findings), applied in revision 2. | `docs/` | ◐ (revision 2 written 2026-10-07; lands as PR-C after code-review PR-B) |
| E1 | The standard:<br>• Part 2 of `skills/quality-index/SKILL.md` and `COMPACT.md`: criteria, verifiers, certificate kinds and syntax, dimensions, N/A rules, anchors; dimension bars cite their criteria; 3-versus-4 dimension anchors;<br>• `qualityindex`: `CriterionIDs`, `CriterionVerifier`, `CriterionDimension`, `CriterionNeverNA`, `CertificateKinds`, `StandardVersion`;<br>• the Scores grammar caps a dimension at 3 when its criterion FAILed;<br>• the criterion drift test, and the skill's examples pinned against the parsers (ADR-0084 I2);<br>• protected-manifest rows. | `skills/quality-index/`, `qualityindex/`, `guards/integrity_surface.go` | ☐ |
| E2 | `audit-plan`:<br>• `.evolve/phases/audit-plan/phase.json` (`after: "build-planner"`, `agent: evolve-audit-planner`, `prompt_context: ["goal","task_contract"]`, `classify.require_sections`, `classify.grammars: ["audit-expectations"]`);<br>• the `agents/evolve-audit-planner.md` persona;<br>• the `.evolve/profiles/audit-planner.json` profile, which joins `ClaudeFamilyFloor`;<br>• the `conditional_mandatory` pin and the protected `router/audit_plan_pin_test.go`;<br>• the overlay rule `{audit-plan, code} → [quality-index, engineering-craft]`;<br>• `audit_plan_inputs` and `standard_version` in cycle state;<br>• the kernel-rendered skeleton;<br>• `qualityindex.ParseExpectations` and `ValidateExpectations` with `ExpectationsInput`, through an input-bearing grammar variant;<br>• once-per-cycle scheduling;<br>• the pre-floor optional-phase degrade (exhaustion, failed dispatch) and Q6's quota-wall skip extended to `audit-plan`;<br>• manifest rows for the persona, spec, profile, overlay and pin test. | phases, agents, profiles, registry, `router/`, `core/`, `qualityindex/`, `deliverable/`, `policy/overlays.go`, `guards/integrity_surface.go` | ☐ |
| E3 | Delivery:<br>• `CtxKeyAuditExpectations`, `auditExpectationsPhase`, and the render under `## Audit Expectations` in `phases/build` and `phases/audit`;<br>• code-review's `prompt_context` gains `audit_expectations`;<br>• recipient digests and `workflow.audit_contract` (stage, `digest_max_bytes`, validation);<br>• the hash-and-restore fence over the expectations, tdd's test files, `go/acs/cycle<N>/**` and the eval file (`audit-snapshots/`, `AUDIT_EXPECTATIONS_TAMPERED`);<br>• `audit-rejections.md`;<br>• `composeRepairBrief`'s criterion section, at enforce only, with the enforce golden re-recorded;<br>• criterion IDs and remediation at the floor's failure sites (`build_floor_reviewer.go`, `comment_floor.go`, `repo_contract_floor.go`). | `core/`, `phases/build/`, `phases/audit/`, `.evolve/phases/code-review/phase.json`, `policy/` | ☐ |
| E4 | The final audit, kernel side:<br>• the `## Criteria` grammar (`qualityindex.ParseCriteria`) with its `Standard:` line;<br>• the `Criterion:` and `Certificate:` finding fields;<br>• `kernel-facts.json`;<br>• the cross-check rules (design §5.2), extending Q5's;<br>• `AUDIT_QUALIFICATION_GAP` with causes `criterion` and `criterion_kernel` and the `criterion` field;<br>• `audit-qualification.json` defect lines;<br>• `AUDIT_UNPUBLISHED_CRITERION`, `AUDIT_FINDING_UNCERTIFIED`, `AUDIT_SAME_FAMILY`, `AUDIT_PLAN_DIVERGENCE`;<br>• the auditor persona writes `## Criteria` (verdict rules unchanged in shadow). | `core/`, `phases/audit/`, `agents/evolve-auditor.md` | ☐ (with Q5) |
| E5 | The ledger's additive `criterion` field; `codereview.ValidateReport` checks the Review Plan against the expectations (required dimensions, depth, failed criteria's dimensions); the delta round reads `audit-rejections.md` first | `core/defectledger/`, `codereview/`, `agents/evolve-code-reviewer.md` | ☐ (with Q3/Q4) |
| E6 | Learning:<br>• `AUDIT_CRITERION_RESULT`, `AUDIT_CRITERION_ESCAPE` (from the retrospective's P7 attribution), `AUDIT_PLAN_CONTRACT_GAP`, `AUDIT_PLAN_SKIPPED`, all registered, with `signal-codes.md` regenerated;<br>• divergence excludes review rounds after an audit-repair re-entry;<br>• the readout verb (`evolve audit contract-metrics [--cycles N] [--json]`, or folded into Q6's `evolve review metrics`; decided in E6). | `core/`, `signalcenter/`, `cmd/evolve`, `docs/architecture/signal-codes.md` | ☐ |
| E7 | The flip:<br>• `workflow.audit_contract.stage = enforce`, in the same commit as code-review's R8b, after the exit test;<br>• the auditor persona's NEW-declaration duty, uncertified handling and checklist removal into the standard (merged with RL's slimming);<br>• correcting the persona's false constitution-check claim. | operator, at a boundary; `agents/evolve-auditor*.md` | ☐ |

## 7. Expected results

- **Build** knows, before it writes code:
  - which criteria apply;
  - what evidence each demands;
  - which acceptance items still lack a proving test;
  - what will trigger a rejection.
- **Code-review** plans its review over the same dimensions the audit will judge.
- **Every audit rejection** names a published criterion (or a declared NEW one), a certificate and a fix. The next round reads all rejections in one place.
- **A gameable bar is avoided:** probe instances stay private, the fenced instruments cannot be edited, and claims without certificates go back for one.
- **No verdict is ever lowered by the kernel,** and no WARN-only floor becomes blocking by a side door.
- **Measured, by the shadow exit test:**
  - first-pass audit PASS rate on code cycles rises;
  - repair rounds per code cycle fall;
  - the added cost is accepted by the operator;
  - the escape rate does not rise.

## 8. Rollout

1. **E0 docs,** revision 2 (this landing, PR-C). At landing they were cross-linked with ADR-0126: ADR-0124 and ADR-0125 gained "Amended by: ADR-0126", and ADR-0126 and the convergence design link ADR-0125. A verify-only check confirmed all 38 review findings fixed; its 16 MEDIUM/LOW findings were filed for E1 (inbox `audit-contract-e0-review-followups`).
2. **Landing 1 of code-review** (PR-A + PR-B): done, merged in train #801 (#799).
3. **Landing 2:** Q3–Q6 with E1–E6, all in shadow (`workflow.findings_repair.code-review.stage` and `workflow.audit_contract.stage` both `shadow`).
4. **Two live shadow waves.** Read the exit-test metrics (design §7).
5. **The flip (R8b + E7)** in one commit at a boundary, if the exit test passes. Otherwise slim or drop `audit-plan` per the data, and record it in §10.

E1–E7 are **not** in the v22.27.0 cutoff unless landing 2 lands in time. Landing 1 (shadow) stays in the cutoff.

## 9. Verification

- **Unit, red first:**
  - the criterion vocabulary, the drift test, and the skill examples parsed by their readers;
  - each of the 8 expectations checks, one malformed case per check;
  - the skeleton pre-render (kernel lines and their applicability, trace IDs including `.eval`, input paths);
  - a contract gap emits `AUDIT_PLAN_CONTRACT_GAP` and seeds build's digest;
  - placement: after build-planner, immediately before build; reads `build-plan.md` when it exists; never on trivial or document cycles;
  - **once per cycle:** a tdd re-entry does not re-dispatch `audit-plan`, and the expectations' hash is unchanged;
  - **degrade:** a malformed `audit-plan` at exhaustion returns `loopNext`, never `loopAbort`, in shadow and in enforce; a failed dispatch and a quota wall both give `AUDIT_PLAN_SKIPPED`;
  - each recipient's digest content, and the byte cap including the trailer; config validation (stage, minimum bytes);
  - **the fence:** a build that edits the expectations, a tdd test or a cycle predicate is restored and signalled;
  - `audit-rejections.md` appends; repair-brief seeding by criterion at enforce, with the shadow golden byte-identical;
  - the Criteria grammar with the `Standard:` line; a FAIL on a plan-N/A criterion requires `Justification:` and signals divergence;
  - NEW handling and its signal;
  - **every cross-check rule** in enforce and in shadow, including: a red enforce-stage fact beats an audit PASS; a red shadow-stage or WARN-only fact does not; an unavailable fact leaves the criterion to the audit; an unresolved PASS certificate means FAIL; an uncertified FAIL finding goes to correction and never becomes WARN;
  - `audit-qualification.json` lines for criterion flips;
  - a failed criterion caps its dimension at 3;
  - ledger `criterion` rows;
  - `AUDIT_SAME_FAMILY`.
- **Mutation:** at least 10 overlay mutants each over the coverage check, the cross-check, the once-per-cycle rule and the NEW handling. 0 surviving non-equivalent mutants.
- **End to end with fakes:**
  - triage → tdd → build-planner → audit-plan → build sees the digest → code-review covers it → audit FAILs `G-ACCEPT.<task>.2` with a certificate → rejections appended → the build brief carries it → the delta review addresses it → audit PASS;
  - an audit FAIL on an unpublished criterion without NEW → correction; with NEW → signal plus lesson;
  - an uncertified sole FAIL → correction in enforce, signal in shadow, and the verdict stays FAIL.
- **Live, in shadow:** over 2 waves, expectations coverage, plan-versus-audit agreement, contract-gap and NEW counts, the exit-test metrics, and added time and tokens.
- **Every landing:** the full floor one target at a time (test, integration, e2e, acs, apicover, cover-strict) with `-count=1`; simplifier → architecture + Go review → fix → delta; the landing-patch audit.

## 10. Landing notes

| Landing | Date, lane | What landed | Evidence | Notes |
|---|---|---|---|---|
| E0 docs, revision 1 | 2026-10-07, `dev/cl-codereview` (console) | this plan, the design doc, ADR-0125, the research dossier and the cross-references | — | Written before any code, per the operator's "all research, plan, decision, and architecture docs must be properly written under the docs". |
| E0 docs, revision 2 | 2026-10-07, `dev/cl-codereview` (console) | the doc review's 38 findings (1 CRITICAL, 9 HIGH, 16 MEDIUM, 12 LOW), applied | the review file (verified claims, plus 12 research sources re-checked) | Changes:<br>• E-D8 (no verdict lowering) and E-D9 (stage-respecting kernel facts);<br>• `G-REVIEW` dropped (RL's qualification owns it);<br>• `G-ACCEPT.<task>.<n>`;<br>• a dimension per criterion, and a failed criterion caps its dimension;<br>• `after: build-planner`;<br>• once per cycle;<br>• the pre-floor degrade rule;<br>• corrected kernel sources (`ProtectedSurfaceFloorChecks`, `Ledger.Reconcile`, no Go constitution check);<br>• the instrument fence is new in E3;<br>• `audit-rejections.md` as its own file;<br>• `kernel-facts.json`;<br>• the input-bearing grammar, 8 checks, the kernel skeleton;<br>• certificate syntax;<br>• `G-EVAL` uses `LevelHalt`;<br>• refinements renamed AR1–AR9;<br>• the research wording fixed (Agentic Rubrics is verifier-side; the ImpossibleRubrics certificate and 36%; the manifesto quoted verbatim). |

## 11. Open questions

| # | Question | Recommendation |
|---|---|---|
| EOQ1 | Any user phase anchored `after: build-planner` would splice ahead of `audit-plan` (`spliceAfter` inserts directly after the anchor). | Pin the placement with a test (audit-plan is the last phase before build), and resolve the order with Q6's explicit registry order if a second phase claims that anchor. |
| EOQ2 | Should `G-PRED`'s grep-only classification move from the audit to the kernel (AR3)? | Yes, later. The persona's classification rule is mechanical. Propose it after the shadow data, as a separate item. |
| EOQ3 | Is 2,048 bytes the right digest cap? | Start there. Read prompt-byte and first-pass metrics in shadow, and tune through config. |
| EOQ4 | Should `audit-plan` run at a cheaper tier? | No. It sets the bar the final audit is held to, and a weaker planner gives a weaker contract. Revisit only if the exit test shows cost without benefit. |
| EOQ5 | The manifesto recommends rotating holdout scenarios between cycles. | It is a future refinement for the probe instances (not the criteria). Consider it after the shadow data shows whether builders converge on the probes. |
| EOQ6 | Should `docsfloor` or `comment_floor` become blocking? | Out of scope (E-D9). Each is its own floor's decision. |
