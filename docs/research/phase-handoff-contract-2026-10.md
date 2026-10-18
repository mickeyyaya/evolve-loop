# Research: how each phase checks its output against the request of the phase before it

- **Status:** complete, 2026-10-18.
- **Plan:** [phase-handoff-contract-2026-10](../plans/phase-handoff-contract-2026-10.md). **Decision:** [ADR-0141](../architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md). **Design:** [phase-handoff-contract](../architecture/phase-handoff-contract.md).
- **Method:**
  - a read of the Go code and the docs of the runtime plane at `origin/main` 34164e7d5;
  - the artifacts of cycles 1946, 1949 and 1950;
  - about 25 live web searches on 2026-10-18.

## Table of contents

- [1. Local evidence](#1-local-evidence)
- [2. Outside survey](#2-outside-survey)
- [3. Adopt or reject](#3-adopt-or-reject)
- [4. Refinements to adopt](#4-refinements-to-adopt)
- [5. Limits of this research](#5-limits-of-this-research)

## Terms used throughout

- **Phase pair:** two phases where the output of the first phase is an input of the second phase, for example triage and tdd.
- **Request:** the list of asks that the first phase of a pair gives to the second phase.
- **Check:** a test of the output of the second phase against the request.
- **Deterministic check:** a check that Go code does, with the same result on each run.
- **Judge:** a check that a model does.
- **Shadow:** a stage where a check records its result and does not change the outcome.

## 1. Local evidence

### Two cycles that failed on their request, not on their code

**F1.1** Cycle 1946 claimed `core-package-coverage-to-100`. The item text says: "The encode-error branch of writeBuildSelfCheckArtifact … is unreachable; that file is protected control-plane surface (ADR-0064), so its removal is console work". The acceptance still asks for 100.0% of `./internal/core`. Source: `.evolve/inbox/2026-10-18T12-00-00Z-core-package-coverage-batches-after-1944.json`, field `summary`.
*Implication:* the request was unsatisfiable in a lane before any phase ran.

**F1.2** In cycle 1946, the build correction prompts show this sequence (`.evolve/runs/cycle-1946/build-evolve-bridge-*-n7-*`, `-n8-`, `-n9-`, line 19):

| Round | The floor said | The builder did |
|---|---|---|
| 1 | "protected control-plane path changed … Restore it" | restored the file |
| 2 | "cover strict: internal/core: 100.0% < 100.0% FAIL … writeBuildSelfCheckArtifact 81.8%" | wrote "I couldn't fix the coverage failure: the one-line change it needs is in a file this phase isn't allowed to edit" |
| 3 | the same text | the same answer; the cycle sealed FAIL |

*Implication:* the builder diagnosed the conflict correctly, but the loop had no exit for a request that cannot be met. Three rounds were spent, and 1,720 lines of good tests were discarded.

**F1.3** In cycle 1950, the tdd phase wrote `TestC1950_002` ("this batch may not edit production code") and `TestC1950_001` (100.0% of `AppendResolvedFingerprint`, "error branches included"). The function has a `json.MarshalIndent` error return that no input reaches. Source: `.evolve/runs/cycle-1950/audit-fail-reason.json` and the build verdict at line 111 of the n7 pane.
*Implication:* the tdd phase wrote two predicates that contradict each other, and no check found the contradiction before the build ran.

**F1.4** Each build prompt of both cycles says "inbox record unreadable at …: no such file or directory (acceptance unknown …)". The claim had moved the item to `processing/`. Fixed in PR #980.
*Implication:* the builder did not see the acceptance that it had to meet.

**F1.5** The tdd phase and the builder both ran Sonnet 5.5 at medium effort in cycles 1946 and 1950 (the pane header). `evolve cli-routing explain tdd-engineer` warned `cross_family_with.builder+tdd-engineer`. PR #981 sets tdd to Sonnet at high effort. The builder now runs agy first (Gemini 3.8 Flash), so the two seats are of different families.
*Implication:* the author of the tests and the author of the code had the same model.

### What the pipeline checks today

**F1.6** The deterministic checks at a phase boundary compare IDs, slugs or package names, never the content of an acceptance entry:
- `tddScopeGate` compares the slug set of tdd with the committed set of triage (`go/internal/topngate/gate.go:55-97`).
- `floor-binding` compares the packages of the tdd floor with the packages that triage deferred (`go/internal/evalgate/floorbinding.go:23`).
- The audit forces FAIL when `red_count>0`, by predicate name (`go/internal/phases/audit/classification.go:117-130`).

*Implication:* no deterministic check asks "is each acceptance entry met, with evidence?".

**F1.7** The task contract injects the acceptance entries of the claimed item verbatim into the prompts of tdd, build and audit (`go/internal/core/task_contract.go:26,133`). Nothing checks the output against those entries. The tdd persona asks for an AC-Materialization table (`agents/evolve-tdd-engineer.md:303-330`), but Go checks only that the heading exists (`go/internal/phasecontract/contract.go:55-100`).
*Implication:* the request travels forward, but no check comes back.

**F1.8** The auditor writes a seven-link chain: intent, selection, specification, implementation, narrative, delivery and evidence fidelity (`go/internal/auditchain/auditchain.go:15-24`). The audit writes it to `audit-chain-shadow.json` and does not use it for the verdict (`go/internal/phases/audit/audit.go:408-423`).
*Implication:* the only content check against each earlier phase is model-judged, runs once at the end, and does not decide.

**F1.9** The 4 cycles that shipped in wave 128 (1945, 1947, 1948, 1949) have no `intent.md`, no plan-review report and no code-review report. Measured with `ls .evolve/runs/cycle-N/`. The auditor prompt tells the auditor to read `intent.md`.
*Implication:* the intent link of the audit chain can only be "unverifiable".

**F1.10** Gaps with no check of the output against the ask:

| Phase pair | What exists | Gap |
|---|---|---|
| scout → triage | lane-pin checks when `top_n` is empty (`go/internal/core/triage_termination.go:44`) | no check that `top_n` is a subset of the scout selection |
| triage → tdd | the slug set (`tddScopeGate`) | no map from each acceptance entry to a predicate |
| tdd → build, before the build | the task contract in the prompt | no check of the tdd predicates before the build spends rounds on them |
| build → audit | the audit runs the predicates by name | the narrative and delivery links are shadow |
| intent → scout | the headings of `intent.md` (`go/internal/phases/intent/intent.go:56-61`) | the intent link is shadow |

*Implication:* three of five pairs have no check, and two pairs have only a shadow check.

**F1.11** Each check is its own design. `go/internal/evalgate/reviewer.go:16` and `go/internal/topngate/gate.go:23` each define a private `gate{name, appliesTo, check(ReviewInput)}` type. Each check also reads the ask from a different source: `committedset.Committed`, `evalgate.SelectedSlugs`, `inboxbatch.Item.Acceptance` and `listACSPredicates`.
*Implication:* a new check for each gap adds a fifth source and a third private gate type.

**F1.12** `docs/architecture/phase-registry.json` declares a `gate_in` and a `gate_out` name for each phase, for example `gate_plan_review_to_tdd`. No Go code reads them; only the docs and a phasespec test name them.
*Implication:* the registry already names the phase pairs, but nothing gives the names a meaning.

**F1.13** `core.DeliverableReviewer`, `ReviewInput` and `ChainReviewers` (`go/internal/core/reviewer.go:9-23,42,86`) run at every phase boundary. They feed the bounded correction loop (`go/internal/core/cyclerun_correction.go:27-31`): one salvage round, one live-fix round and N redispatch rounds. N is 2 by default and 5 at most (`go/internal/policy/fleet.go:48-49`).
*Implication:* a general check can plug in as one more reviewer and reuse the loop and its caps.

**F1.14** The convergence policy (ADR-0126, `go/internal/convergence/decide.go:30`) already decides the next step of a review chain. Only the CLI verb `evolve convergence decide` calls it (`go/cmd/evolve/cmd_convergence.go:52`).
*Implication:* the vocabulary for rounds and exits exists and can be reused.

## 2. Outside survey

### 2.1 Self-checks and outside checks

**P2.1** Without outside feedback, a model that corrects its own reasoning usually gets worse. Earlier reported gains often used oracle labels. Huang and others, ICLR 2024: https://arxiv.org/abs/2310.01798. A TACL 2024 survey found no success with prompted self-feedback, except on tasks that suit it: https://aclanthology.org/2024.tacl-1.78/.

**P2.2** A check with a tool works. CRITIC runs verify, correct and verify with a search tool or a code interpreter: https://arxiv.org/abs/2305.11738. "LLM-Modulo" pairs a model that generates with sound outside critics: https://arxiv.org/abs/2402.01817. The Anthropic agent guide says that an agent must get ground truth from the environment at each step. It also says that an evaluator loop pays off only with clear criteria: https://www.anthropic.com/engineering/building-effective-agents.

### 2.2 Who checks

**P2.3** A judge prefers its own text, and the preference grows with how well it recognizes that text (NeurIPS 2024): https://arxiv.org/abs/2404.13076. Errors correlate more inside one provider or one architecture (ICML 2025): https://arxiv.org/html/2506.07962v1. Majority votes amplify a shared error: https://arxiv.org/html/2603.06612.

**P2.4** The checker must be at least as strong as the author. In "Self-Verification Cliff" (ICML 2026), a stronger checker from another family added 3.5 points, and a weaker checker cost 2.3 points: https://icml.cc/virtual/2026/82529.

### 2.3 Tests by a separate author

**P2.5** AgentCoder uses a separate programmer, test designer and test executor. It reports 96.3% HumanEval pass@1 with GPT-4 at 56.9K tokens, against 90.2% at 138.2K tokens: https://arxiv.org/html/2312.13010v3.

**P2.6** Tests that a model writes are often wrong in the expected value. In Nexus (2025), about 89% of the failed tests had a wrong expected output: https://arxiv.org/pdf/2510.26423. Weak tests made about 31% of "passing" patches suspect in SWE-bench+ (secondary source): https://arxiv.org/pdf/2506.17208.

**P2.7** A check of each step does better than a check of the final result only. Lightman and others, ICLR 2024: https://arxiv.org/abs/2305.20050.

### 2.4 When to stop

**P2.8** Most of the gain of a refinement loop comes in rounds 1 to 3, with a plateau at about 3 to 5. Sources: GIER, https://arxiv.org/pdf/2509.00325; Nexus; InteracSPARQL, https://arxiv.org/pdf/2511.02002. The LangChain reflection example uses `MAX_ITERATIONS = 5`: https://blog.langchain.com/reflection-agents. Pydantic AI retries an output validator once by default: https://pydantic.dev/docs/ai/core-concepts/output/.

**P2.9** A stop rule on the expected gain of one more round costs less than a fixed count (CDC 2026): https://arxiv.org/abs/2608.10729.

**P2.10** The confidence that a model states in words is too high after RLHF (ICLR 2025): https://proceedings.iclr.cc/paper_files/paper/2025/file/29fb6e1456b3d8b57ede5c45aa2c6537-Paper-Conference.pdf. Conformal abstention gives a bound from sampled agreement and a calibration set: https://arxiv.org/pdf/2405.01563.

### 2.5 A request that cannot be met

**P2.11** Agents are poor at a decision to stop and ask. In AgentAbstain (2026), the best of 17 models got 59.5%, and a main failure is an action before the agent sees the trigger: https://arxiv.org/abs/2607.10059. HiL-Bench (2026) classes blockers as missing, ambiguous or contradictory: https://arxiv.org/abs/2604.09408.

**P2.12** MAST (NeurIPS 2025) studied more than 1,600 traces from 7 frameworks. It has a category for task verification: early stop, a missing check and a wrong check. The authors find that most failures come from the design of the system: https://arxiv.org/abs/2503.13657.

### 2.6 Contracts and a chain of evidence

**P2.13** A design-by-contract layer for model calls gives pre- and post-conditions with a probability of success and a repair step: https://arxiv.org/pdf/2508.03665. A 2026 position paper puts the guarantee of one layer as the assumption of the next layer: https://arxiv.org/abs/2605.18672.

**P2.14** An in-toto attestation binds named subjects, by digest, to a typed predicate. A layout says which step makes which artifact, and verification applies the layout to all attestations: https://github.com/in-toto/attestation/blob/main/spec/predicates/link.md. SLSA records a verification summary: https://slsa.dev/verification_summary/v0.2.

**P2.15** An assurance case in GSN links a goal (a claim) to a strategy and to evidence, with its context and assumptions: https://arxiv.org/pdf/2511.02203.

**P2.16** Verification asks "is it built right, to the spec?". Validation asks "is it the right thing for the need?". A system can pass each verification and still fail validation: https://ics.uci.edu/~taylor/classes/113/VandV.

**P2.17** The OpenAI Agents SDK runs output guardrails only on the last agent and input guardrails only on the first agent. A handoff between agents gets no guardrail by default: https://openai.github.io/openai-agents-python/guardrails/.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A deterministic check before a judge (P2.1, P2.2) | adopt | F1.2 and F1.3 were both decidable by code. A judge adds cost and self-preference. |
| A checker that is independent of the author (P2.3, P2.4) | adopt | F1.5: one model wrote both the tests and the code. |
| A separate author of the tests (P2.5) | keep, and add a check of the tests (P2.6) | F1.3: the tests contradicted each other. |
| A check on each phase pair (P2.7, P2.12, P2.17) | adopt | F1.10: three pairs have no check. |
| A small cap with a stop on no gain (P2.8, P2.9) | adopt | F1.2: rounds 2 and 3 added nothing. ADR-0126 already caps review rounds. |
| Confidence that the model states (P2.10) | reject | It is too high. Measured met entries take its place. |
| Blocked as a result of its own (P2.11) | adopt | F1.1 to F1.3: the request was the defect. |
| An attestation for each output (P2.14, P2.15) | adopt, with digests and no signatures | All phases run in one trust domain. A digest binds the output to its request. A signature adds keys and no new proof here. |
| One validation against the first request (P2.16) | adopt | A chain of passed checks can still miss the need. |
| Conformal calibration (P2.10) | defer | It needs a calibration set of judged cycles. The shadow stage collects that set. |

## 4. Refinements to adopt

- **R1** Each phase pair has one request: a numbered list of asks, each with its source. (F1.7, F1.11)
- **R2** Each output carries a ledger: for each ask, a status and a pointer to evidence. (P2.14, P2.15)
- **R3** A deterministic check reads the ledger first. A judge reads it only where code cannot decide. (P2.1, P2.2)
- **R4** The judge is another family, or a stronger tier, than the author. The ledger records its identity. (P2.3, P2.4)
- **R5** The loop has three exits: pass, cap and blocked. (F1.2, P2.11)
- **R6** The cap is small, and the loop stops early on a round that meets no new ask. (P2.8, P2.9)
- **R7** "Confident" means that each required ask is met with evidence that the check resolved. (P2.10)
- **R8** A blocked result goes back to the phase that wrote the request, with the ids of the asks in conflict. (P2.11, F1.3)
- **R9** The audit walks the chain of ledgers and validates the result against the first request. (F1.8, P2.16)
- **R10** The check plugs in at `core.DeliverableReviewer` and reuses the correction loop. (F1.13, F1.14)

## 5. Limits of this research

- The research agent took five numbers from secondary sources and did not check them against the paper:
  - the AgentCoder ablation;
  - the 2 to 4 rounds of Self-Refine;
  - the 60% figure for the same wrong answer;
  - the SWE-bench+ and UTBoost numbers;
  - the MetaGPT gains, which come from slides.
- The local evidence comes from one wave (wave 128) and two failed cycles. The shadow stage of the plan measures the rate of each gap over more waves.
