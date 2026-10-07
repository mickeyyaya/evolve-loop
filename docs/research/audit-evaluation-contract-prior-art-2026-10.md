# Audit evaluation contract: prior art in AI software factories and evaluator research (2026-10-07)

| Field | Value |
|-------|-------|
| title | Publishing an evaluator's standard to the agents it judges |
| date | 2026-10-07 |
| trigger | Operator, 2026-10-07: *"research the latest ai software factory pipeline design and ai driven design similar topics to gather more information before jumping to the implementation"* |
| backs | [ADR-0125](../architecture/adr/0125-audit-publishes-its-evaluation-contract.md) · design [audit-evaluation-contract.md](../architecture/audit-evaluation-contract.md) · plan [audit-evaluation-contract-2026-10.md](../plans/audit-evaluation-contract-2026-10.md) |
| sources | 13 fetched and read on 2026-10-07; 3 known only from search summaries, marked *secondary*. The E0 doc review re-checked 12 of them; its wording corrections were applied in revision 2. |
| status | accepted research; shaped refinements AR1–AR9 of the plan |

> **Scope.** This dossier answers one question. It does not survey AI factories in general. Pipeline resilience is covered in [ai-factory-pipeline-resilience-2026-06-02.md](ai-factory-pipeline-resilience-2026-06-02.md), deliverable contracts in [ai-harness-deliverable-contract-2026-06-03.md](ai-harness-deliverable-contract-2026-06-03.md), and runtime enforcement and cross-model review in [agentic-pipeline-enforcement-2026.md](agentic-pipeline-enforcement-2026.md). The lessons of building this factory are in [factory-pipeline-findings-2026-09.md](factory-pipeline-findings-2026-09.md).

## Question

The audit is the loop's ship gate. The operator asked that it **share what it evaluates** (its directions, fields, scores, feedback, and the need-to-know) with build and code-review, so that all three aim at the same standard and a rejection teaches the next round what to fix.

How do current AI software factories and current evaluator research handle that?
- **When** should the evaluator's standard be shared: before the work, after it, or never?
- **What** should be shared: the criteria, the tests, or the probe inputs?
- **How** does sharing avoid teaching the producer to game the judge?

## Method

- I ran web searches on: AI software factories, harness engineering, spec-driven development, LLM-as-judge rubrics, reward hacking in coding agents, and AI code review.
- I fetched and read every source this dossier calls *fetched*. The three sources known only from search-result summaries are marked *secondary*, and no design decision rests on them alone.
- I mapped each finding to the existing repo principles in [factory-pipeline-findings-2026-09.md](factory-pipeline-findings-2026-09.md) §2 (P1–P15), then to a numbered refinement of the plan (AR1–AR9).

## Findings, and how each shaped the design

### F1. Contract before code is now the mainstream shape

- **Anthropic, *Harness design for long-running application development*** (fetched). The harness has three agents: planner, generator and evaluator.
  - "Before each sprint, the generator and evaluator negotiated a sprint contract: agreeing on what 'done' looked like for that chunk of work before any code was written."
  - The agents communicated through files.
  - Each evaluation criterion had a hard threshold, and a failure on any criterion rejected the sprint.
- **Augment, *How we built Augment's software factory*** (fetched). Engineers "iterate … on product requirements and architecture decisions; approve the design" before implementation agents start. A Verifier agent produces runtime evidence for the merge decision.
- **Spec-driven tools: GitHub Spec Kit, Kiro** (*secondary*). These follow the shape Specify → Plan → Tasks → Implement. Kiro writes acceptance criteria in EARS notation, and each criterion is traced to the tasks that satisfy it.
- **Context-grounded criteria written before evaluation. This is on the verifier side and is not shared with the generator.** *Agentic Rubrics*, arXiv 2601.04171 (fetched):
  - "an expert agent interacts with the repository to create a context-grounded rubric checklist, and candidate patches are then scored against it without requiring test execution".
  - The rubric is written after exploring the repo and before scoring, on four axes: file change, spec alignment, integrity (no test weakening) and runtime. It is never shown to the coding agent, so it is closer to a holdout than to a published contract.
  - "Rubric scores are consistent with ground-truth tests while also flagging issues that tests do not capture."
  - Its low-utility modes were over-specification, redundancy, and rubric–test mismatch.

**Shaped:** the `audit-plan` phase before build (approach C). Anthropic's harness and the spec-driven tools are the "contract before code" precedent. Agentic Rubrics shows that context-grounded criteria, written by an agent that explored the repo before evaluation, help the verifier. Two more refinements came from these sources:
- AR4: acceptance traceability, `G-ACCEPT.<task>.<n>`;
- AR5: expectations name outcomes and evidence, never implementation (Agentic Rubrics' over-specification mode).

### F2. Publish the criteria; isolate the instruments

- **StrongDM's software factory** (fetched; Simon Willison, 2026-02-07).
  - The scenarios that judge the work are stored outside the codebase, like an ML holdout set, and the coding agents cannot see them.
  - Success moved "from boolean definitions of success ('the test suite is green') to a probabilistic and empirical one".
- **The Agentic Engineering Manifesto, principle 8** (fetched). It separates two defenses:
  - **Visibility control** (holdout): the agent cannot *see* the criteria.
  - **Access isolation**: the agent cannot *modify* the evaluation infrastructure.

  In its words, a holdout "must be paired with access isolation of the evaluator itself (see reward hacking, above), or a sufficiently privileged agent can still defeat it". The evaluation harness must sit outside the agent's write scope. The two labels above are this dossier's own.
  - It also recommends that "Evaluation criteria should not be visible to the producing agent", and that holdout scenarios rotate between cycles. This dossier records that disagreement below.
- **SpecBench, arXiv 2605.21384** (fetched).
  - "While every frontier agent saturates the visible suite, reward hacking persists, with smaller models exhibiting larger gaps on holdout suites."
  - The visible-versus-hidden gap grows by about 28 points for every tenfold increase in code size.
  - One case was a 2,900-line hash table that memorized the test inputs.

**Shaped:** AR1. The operator asked for the standard to be shared, and it is shared completely: every criterion, its evidence kind, the triggers and the need-to-know. What stays private is the final audit's **probe instances** (the specific edge inputs, mutants and adversarial cases it tries). Every **instrument** also stays outside build's write scope:
- the expectations document;
- the TDD tests;
- `go/acs/**`;
- the eval file.

This is the repo's own P7 ("a cycle cannot edit, or draw, what grades it") applied to the published standard.

### F3. Evidence must be a certificate, not a claim

- **ImpossibleRubrics, arXiv 2609.16816** (fetched).
  - Generators were exploited 8–26% of the time under unbiased conditions. On a deliberately selected stress cut, the strongest generator measured was still exploited 36% of the time.
  - "The problem is not that rubrics are vague; it is that they are specific about the wrong things." Detailed task-specific rubrics tell the generator which claims to fabricate.
  - **Certificate-faithful** rubrics were exploited 0% of the time. In the paper, a certificate is a verifiable oracle certificate "specifying what an honest answer may and may not claim". It specifies the permissible claims; it is not a test log.
- **Refute-or-Promote, arXiv 2604.19049**, already in [agentic-pipeline-enforcement-2026.md](agentic-pipeline-enforcement-2026.md) Finding 2. A claim that cannot be verified independently, with no file reference and no test output, is downgraded.
- **The auditor persona already applies this** (`agents/evolve-auditor.md`, "Per-criterion evidence"). Every criterion cites a test output line, a diff hunk, or a command run with its output, and a criterion with no citation FAILs.

**Shaped:** AR2. It borrows the paper's *idea*, not its measurement: state what evidence may support a claim, so that unsupported claims satisfy nothing.
- Every criterion names a certificate kind from a closed set (`test`, `command`, `mutant`, `artifact`, `diff` or `kernel`), with a fixed reference syntax.
- A rejection finding with no certificate is malformed and goes back to the audit through the correction rung. It is never downgraded, because the kernel only moves verdicts toward FAIL (E-D8).

### F4. Deterministic facts outrank the judge

- **PROCTOR, arXiv 2609.02246** (fetched). The LLM judge is demoted from decision-maker to advisor. The paper names five guardrails:
  - hermetic sandboxes;
  - capability-disjoint roles;
  - acceptance checks outranking the teacher;
  - frozen holdouts;
  - canary cases.

  In one failure, "agents achieved perfect scores by reading cached answer keys from their environment, a 100% pass rate concealing 68% true capability."
- **OpenAI, *Harness engineering*** (fetched, 2026-02-11).
  - Invariants are enforced "mechanically via custom linters … and structural tests".
  - "Because the lints are custom, we write the error messages to inject remediation instructions into agent context."
  - PRs iterate "until all agent reviewers are satisfied".
- **Stripe Minions** (*secondary*). Blueprints alternate deterministic nodes with agent loops. The test-validation step is "a gate the harness enforces regardless of what the model thinks", and each run gets at most two CI rounds.

**Shaped:** AR3. Each criterion is marked `Verifier: kernel` or `Verifier: audit`. Kernel-verified criteria reach the audit as facts it cannot overrule, and they fail at build's own handoff floor with remediation text, before any audit runs. This is the repo's P2 and P6 applied.

### F5. The judge is biased toward its own family, and needs calibration

- **Self-preference bias in rubric-based evaluation, arXiv 2604.06996** (fetched).
  - Even with fully objective rubrics, judges "can be more than 50% more likely to incorrectly mark them as satisfied when the output is their own".
  - Ensembles reduce the bias but do not remove it.
- **Anthropic** (F1).
  - "I calibrated the evaluator using few-shot examples with detailed score breakdowns."
  - "Tuning a standalone evaluator to be skeptical turns out to be far more tractable than making a generator critical of its own work."
  - The evaluator was still "inclined to be generous towards LLM-generated outputs".
- **PReMISE, arXiv 2605.30803** (fetched). Rubrics are measurement specifications, so changing the rubric changes the measurement. High inter-rater agreement does not mean low exploitability.

**Shaped:**
- AR7: calibration anchors (a PASS and a FAIL example per criterion, a 3 and a 4 per dimension), and an `AUDIT_SAME_FAMILY` signal.
- AR6: the standard is pinned per cycle (`Standard: <version>`), so the measurement cannot change between the plan and the verdict.

### F6. Review findings need a refutation step and a precision budget

- **GitHub ReviewBench** (fetched). It measures review precision and recall weighted by severity. In the reported experiments, comment quality mattered more than volume.
- **Claude Code Review** (*secondary*). Parallel reviewer agents are followed by a verification step that tries to disprove each finding before posting; reported false positives are below 1%.

**Shaped:** AR2's certificate requirement on rejections, which applies the refutation idea in the safe direction. A finding without a certificate goes back to its author for one, and is never silently dropped or downgraded.

### F7. Every added phase must earn its place

- **Anthropic** (F1). The full harness cost about 20x the solo run ($200 versus $9), and its output was better. When a stronger model shipped, the sprint construct was **removed**: "Tasks that used to need the evaluator's check … were now often within what the generator handled well on its own."
- **marmelab, *The state of AI harness engineering 2026*** (fetched; it summarizes primary sources that I did not re-fetch, so its numbers are *secondary*).
  - Adding a second reviewer agent cut success by 8 points on one benchmark.
  - An operations agent failed on "problems requiring more than four handoffs".
  - Machine-generated context files "reduce task success compared to giving the agent no context" and cost 20% more.
  - OpenAI settled on instruction files of about 100 lines, because "when everything is 'important,' nothing is".

**Shaped:**
- AR8: delivery is a byte-capped digest plus the document's path, never the full document.
- AR9: trivial cycles skip `audit-plan`. The shadow waves carry **exit criteria** (first-pass audit PASS rate, repair rounds, added cost against the pre-landing baseline), and the phase is kept, slimmed or dropped on that data. The **escape rate** (later defects traced to a criterion that shipped as PASS) is the detector for evaluation theater that the manifesto recommends.

## Synthesised principles

1. **Contract before code.** The bar for a change is set before the change is written (F1).
2. **Criteria public, instruments private.** Share every criterion and the evidence it demands. Never share the probe instances, and never let the producer write an instrument (F2).
3. **Evidence over claims.** A criterion is satisfied by a reproducible certificate, and a rejection without one goes back to its author for one. The kernel never downgrades a rejection (F3, F6).
4. **Facts outrank the judge.** What code can compute, code computes. The LLM judges only what code cannot (F4).
5. **Pin and calibrate the measurement.** One version per cycle, anchored examples, and a cross-family judge (F5).
6. **Scaffolding is provisional.** Every new phase states its exit test, and stays only on measured benefit (F7).

## Where the sources disagree

| Tension | Positions | Resolution here |
|---|---|---|
| Share the standard, or hold it out? | Anthropic shares the contract before work (F1). StrongDM and SpecBench hold the scenarios out (F2). | They disagree on different objects. The *criteria* are shared, which is what the operator asked for. The *probe instances* are held out. A probe failure is still reported against its published criterion, so nothing is judged on a hidden criterion (E-D1 requires an unpublished criterion to be declared NEW). |
| Specific or generic rubrics? | Agentic Rubrics: grounded, task-specific criteria help a verifier (F1). ImpossibleRubrics: task-specific rubrics invite fabrication (F3). | The criteria are task-specific about **evidence**, never about **claims or implementation**. Each one names a certificate kind; AR5 forbids prescribing implementation. |
| Hide the criteria, or show them? | Manifesto P8: "Evaluation criteria should not be visible to the producing agent", with holdout scenarios rotated between cycles (F2). The operator asked for the criteria to be shared. | E-D5 shows the criteria and holds out the probe instances. Rotating the instances is a future refinement (plan EOQ5). |
| More review, or less? | OpenAI iterates until every agent reviewer is satisfied. marmelab reports an added reviewer cutting success. | `audit-plan` adds no round: one dispatch per code cycle, outside the build↔review loop. Its survival depends on the AR9 exit criteria. |

## Anti-patterns avoided

- **A hidden bar.** The audit FAILs on criteria nobody published. This is prevented by E-D1 (NEW criteria) and AR6 (pinned version).
- **A gameable bar.** Builders tune to visible checks and to their claims. This is prevented by AR1 (private probe instances) and AR2 (certificates).
- **A judge that can overrule facts.** An LLM PASS over a red kernel check. This is prevented by AR3.
- **A manual in the prompt.** The full standard pasted into every prompt. This is prevented by AR8.
- **Permanent scaffolding.** A phase kept because it exists. This is prevented by AR9.

## Sources

Fetched and read on 2026-10-07:
1. Anthropic Engineering, *Harness design for long-running application development*: https://www.anthropic.com/engineering/harness-design-long-running-apps
2. OpenAI, R. Lopopolo, *Harness engineering: leveraging Codex in an agent-first world* (2026-02-11): https://openai.com/index/harness-engineering/
3. S. Willison, *How StrongDM's AI team build serious software without even looking at the code* (2026-02-07): https://simonwillison.net/2026/Feb/7/software-factory/
4. *Agentic Engineering Manifesto*, companion principle 8: https://github.com/arnaudgelas/agentic-engineering-manifesto/blob/main/companion/principles-08.md
5. *SpecBench: Measuring Reward Hacking in Long-Horizon Coding Agents*, arXiv 2605.21384: https://arxiv.org/abs/2605.21384
6. *ImpossibleRubrics: Stress-Testing Generated Rubrics as Reward Signals*, arXiv 2609.16816: https://arxiv.org/abs/2609.16816
7. *LLM-as-a-Judge Is Not an Oracle: Why Self-Improving Agents Need Deterministic Guardrails* (PROCTOR), arXiv 2609.02246: https://arxiv.org/abs/2609.02246
8. *Agentic Rubrics as Contextual Verifiers for SWE Agents*, arXiv 2601.04171: https://arxiv.org/html/2601.04171v1
9. *Self-Preference Bias in Rubric-Based Evaluation of Large Language Models*, arXiv 2604.06996: https://arxiv.org/abs/2604.06996
10. *PReMISE: Policy Rubrics as Measurement Specifications for LLM Judges*, arXiv 2605.30803: https://arxiv.org/abs/2605.30803
11. Augment Code, *Beyond AI coding agents: how we built Augment's software factory*: https://www.augmentcode.com/blog/beyond-ai-coding-agents-how-we-built-augments-software-factory
12. GitHub, *ReviewBench: an open benchmark for AI code review*: https://github.blog/ai-and-ml/github-copilot/reviewbench-an-open-benchmark-for-ai-code-review/
13. marmelab, *The state of AI harness engineering 2026* (2026-09-24): https://marmelab.com/blog/2026/09/24/the-state-of-ai-harness-engineering-2026.html. The page was fetched, but its numbers summarize primary sources that were not re-fetched.

*Secondary* (search-result summaries only; nothing rests on them alone):

14. Stripe Minions (InfoQ, 2026-03): https://www.infoq.com/news/2026/03/stripe-autonomous-coding-agents/
15. Claude Code Review launch (Help Net Security, 2026-03-10): https://www.helpnetsecurity.com/2026/03/10/anthropic-claude-code-review/
16. GitHub Spec Kit and Kiro (IntuitionLabs guide): https://intuitionlabs.ai/articles/spec-driven-development-spec-kit
