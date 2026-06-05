# Micro-Phase Catalog — Online Research Findings

**Date:** 2026-06-05
**Method:** Two-round web research. Round 1: deep-research harness (106 agents, 5 search angles, 24 primary sources fetched, 117 claims extracted, 25 adversarially verified with 3-vote refutation panels → 9 confirmed high-confidence). Round 2: 3 targeted gap-fill research agents (bugfix / refactor+perf / security+release+docs+learning).
**Request:** Research the most essential, popular, and interesting development pipeline phases (papers, open-source projects, blogs) that the advisor could select or reference — with explicit emphasis on **small, atomic, single-responsibility micro-phases** the advisor can flexibly combine per goal type (feature / bugfix / refactor / security / performance / release / docs).
**Companion design doc:** [docs/architecture/micro-phase-catalog.md](../../docs/architecture/micro-phase-catalog.md)

---

## 1. Round-1 verified findings (adversarial 3-vote panels, primary sources)

Each claim below survived a refutation panel (vote shown as confirm-refute).

### 1.1 AgentCoder — test-design split from build (3-0)

Three single-responsibility agents: Programmer; **Test Designer that writes tests *without seeing the code*** ("the tests generated immediately following the code … can be biased"); Test Executor returning pass/fail + errors. Coverage explicitly spans basic, edge, and large-scale inputs. Plus an **iterate-on-test-feedback control loop**: executor error messages drive regeneration until green (code-first, not test-first).
→ Template for a routable `test-amplification` micro-phase with an anti-bias isolation requirement.
Sources: <https://arxiv.org/html/2312.13010v3> · <https://github.com/huangd1999/AgentCoder>

### 1.2 AlphaCodium — flow engineering (3-0)

Decomposes code generation into atomic phases: linear pre-processing (**problem reflection → public-test reasoning → generate 2-3 candidate solutions → rank by correctness/simplicity/robustness → generate 6-8 diverse AI tests**) then iterative code-iterations against public + AI tests. Key asymmetry: "Generating additional tests is easier than generating a full solution code." Result: GPT-4 pass@5 on CodeContests validation **19% → 44%**.
Caveats: no ablation isolates test-gen alone; GPT-4-era absolute numbers; newer frameworks dropped autonomous test-gen citing reliability.
Sources: <https://arxiv.org/abs/2401.08500> · <https://www.qodo.ai/blog/qodoflow-state-of-the-art-code-generation-for-code-contests/>

### 1.3 Reflexion — self-reflection + episodic memory as distinct phases (3-0)

Agents "verbally reflect on task feedback signals, then maintain their own reflective text in an episodic memory buffer" — improvement "not by updating weights, but instead through linguistic feedback". **91% pass@1 HumanEval vs GPT-4's 80%** (camera-ready revision: 88 vs 67; directional effect robust).
→ Validates the existing retro/memo/KB-recall phases as evidence-backed; lesson-extraction and episodic-recall are legitimately separate responsibilities.
Source: <https://arxiv.org/abs/2303.11366>

### 1.4 OpenHands SDK — context condensation + per-action risk gating (3-0 × 2)

- **Context condensation:** "Before sending the event history to the LLM, the agent applies these condensation events by removing forgotten events and inserting summaries" (default >10 events, keeps first 2). **~2× API cost reduction, no performance loss.**
- **SecurityAnalyzer / ConfirmRisky:** rates each tool call low/medium/high/unknown; blocks actions above a configurable threshold (default high) — a per-action gate *distinct from* a full audit.
Sources: <https://arxiv.org/html/2511.03690v1> · <https://docs.openhands.dev/sdk/guides/context-condenser>

### 1.5 Aider — Architect/Editor split with per-phase model routing (2-0)

Reason-about-the-change and apply-the-edit are separate phases routed to **different models** — reasoning models "strong at reasoning, but often fail to output properly formatted code editing instructions". 85.0% benchmark SOTA at publication.
→ Existence proof for per-phase CLI/model routing ([[project_advisor_owns_per_phase_cli]] direction).
Source: <https://aider.chat/2024/09/26/architect.html>

### 1.6 Classify-then-route — the canonical advisor mechanism (3-0)

Anthropic ("Building Effective Agents") and LangGraph both define Routing as: classify an input first, then direct it to a context-specific specialized task.
→ Validates a goal-type classification step driving recipe selection in the advisor.
Sources: <https://www.anthropic.com/research/building-effective-agents> · <https://docs.langchain.com/oss/python/langgraph/workflows-agents>

### 1.7 Google SRE — risk-gated staged/canary rollout (2-0)

"Expand exponentially until all clusters are updated"; multi-day region-interleaved rollouts for sensitive infra; "fit the deployment process to the risk profile."
→ Post-ship verification and rollout pacing are risk-routed control phases.
Source: <https://sre.google/sre-book/release-engineering/>

### 1.8 Verification caveats (honest read of the kill list)

- 10 claims were "killed", but most died **0-0 with 3 abstains** — verifier infrastructure failure (subagents failed to emit structured output), *not* refutation. Treat as plausible-unverified: CrewAI manager-delegation pattern, Google build→branch→test→package stage split, canary sub-step decomposition, code-review-size effect (r = −0.42…−0.33), test-case prioritization.
- One genuine refutation (0-2): "Orchestrator-Worker is the advisor mechanism" — dynamic task decomposition ≠ phase routing; the advisor is a router, not an orchestrator-worker.
- **LLM-generated test suites are documented-unreliable** (most contain ≥1 error/smell) → any test-generation phase must feed *signals into existing gates* (tdd/audit), never act as independent ship authority.

---

## 2. Round-2 findings: bugfix micro-phases

### 2.1 Agentless (arXiv 2407.01489)

"A simplistic three-phase process of **localization, repair, and patch validation**, without letting the LLM decide future actions or operate with complex tools." Fixed pipeline beat agentic scaffolds: **32.00% SWE-bench Lite (96/300) at $0.70/issue** — best open-source performance + lowest cost at publication.

- **Hierarchical fault localization** (no embeddings): repo-tree → top-N suspicious files → skeleton (declarations only) → element selection → full-code edit locations.
- **Repair**: search/replace diffs; 1 greedy + ~20 sampled completions → up to ~42 candidate patches.
- **Validation & ranking**: regression filtering against existing tests ("any patches which failed the existing tests can be filtered out"); Agentless 1.5 adds LLM-generated reproduction tests; majority voting selects the final patch.

### 2.2 Reproduce-first pattern (SWE-agent, OpenHands, quantified lift)

Top SWE-bench submissions share 4 stages: localization → **reproduction-script generation (fails before any edit)** → patch → regression validation. SWE-bench itself encodes the two oracles: FAIL_TO_PASS (reproduction) + PASS_TO_PASS (regression).

Quantified evidence that reproduce/regression-first raises precision:
- **TestPrune** (arXiv 2510.18270): +6.2–9.0% relative issue-reproduction (Otter), **+9.4–12.9% relative issue-resolution (Agentless)**.
- **SWE-Tester** (arXiv 2601.13713): up to +10% success, +21% change coverage on SWE-bench Verified.
- e-Otter++/TDD-Bench Verified: 63% avg fail-to-pass rate for generated reproduction tests.

### 2.3 AutoCodeRover (arXiv 2404.05427)

Splits localization into **AST-aware structured code search** (stateful class/method search APIs) + **spectrum-based fault localization** (SBFL, *conditional on an existing test suite*). 19% SWE-bench Lite at $0.43/issue.

### 2.4 RepairAgent (arXiv 2403.17134)

First autonomous LLM repair agent; FSM-guided tool loop interleaving gather-info / collect-ingredients / **validate-fixes**. Defects4J: **164 bugs fixed, 39 never fixed by any prior technique**, ~$0.14/bug. Confirms the generate-and-validate paradigm: patch validation is a distinct phase from generation.

### 2.5 Bugfix design takeaways

1. `reproduce-bug` is the **highest-leverage single addition** — quantified lift and the differentiator between bugfix and feature recipes.
2. SBFL and regression-filter must be routing-gated on test-suite presence.
3. Agentless vs RepairAgent: the *phases* are stable across both; the *control flow between them* (fixed vs agentic) is the configurable dimension — exactly the advisor's job.
4. Patch sampling + majority-vote are build-internal enhancements, not separate catalog phases (wave-1 scope call).

---

## 3. Round-2 findings: refactoring + performance micro-phases

### 3.1 `smell-scan`

Fowler's catalog is the taxonomy; **SonarQube's "code smells" ≠ Fowler's** (only 4 of 22 map). Adoption: SonarQube **>85K organizations**. LLM-based detection is competitive (arXiv 2504.16027; "Beyond Strict Rules" 2601.09873 — 76-developer ground truth): LLMs capture intent-level smells (Feature Envy) rule engines miss, at lower precision → **LLM detect + deterministic policy gate**.

### 3.2 `behavior-lock` (golden-master verification)

Feathers' characterization tests: capture input→output baseline pre-refactor, re-run and diff post-refactor. Research frontier for machine-checked equivalence: DiffKemp (ICST'21, LLVM IR), REM 2.0 (Coq proofs for Extract Method), RefactoringMiner 3.0 (TOSEM, refactoring-aware semantic diff). Practical default: test/golden-master gate; semantic equivalence as opt-in escalation.
**Pipeline implication: the phase must straddle the edit → implement as a `behavior-baseline` (pre-build) + `behavior-compare` (post-build) pair.**

### 3.3 `mutation-gate`

Google "Practical Mutation Testing at Scale" (IEEE TSE 2021): **>24,000 developers, >1,000 projects, 2B LOC**. Feasibility keys that transfer: (1) **incremental** — mutate only changed code; (2) filter irrelevant mutants, cap per line; (3) select operators by historical performance. Tooling: PIT (JVM), Stryker (JS/TS, native score thresholds), go-mutesting. Motivating failure mode: a real case reported **93% line coverage but 34% mutation score** — exactly the weakness of LLM-authored tests.

### 3.4 `perf-gate` (statistical benchmark gate)

Continuous-benchmarking practice. Three threshold philosophies: statistical (Go `benchstat` p-values — the default for noisy environments); fixed-percentage (**CodSpeed: 1.5% gate achievable on instrumented runners vs ~7% needed on noisy CI for the same <1% false-positive rate** — gate tightness is bounded by measurement environment, not policy); configurable statistical tests (Bencher's 7 threshold types, no universal default).
Sources: <https://bencher.dev/docs/explanation/thresholds/> · <https://codspeed.io/blog/benchmarks-in-ci-without-noise>

### 3.5 `cleanup-sweep`

JS/TS: **knip** (mark-and-sweep reachability + unused deps/files/members; ts-prune is in maintenance mode and recommends knip). Go: **`golang.org/x/tools/cmd/deadcode`** (Rapid Type Analysis from `main`; `-whylive`; whole-program only — can't scope to a library package). Detection and deletion must be separate sub-steps; removal goes through `behavior-lock`.

### 3.6 Refactor/perf design takeaways

- Detection (`smell-scan`, `cleanup-sweep`) = evaluate; verification gates (`behavior-lock`, `mutation-gate`, `perf-gate`) = control wrapping a deterministic measurement. None are pure build — refactor *edits* stay in the build phase, keeping these side-effect-free and re-runnable.
- Refactor recipe invariant: `smell-scan → behavior-baseline → build → behavior-compare → mutation-gate → cleanup-sweep → perf-gate`.
- All heavy phases **diff-scoped** (Google's incremental lesson).
- **Novelty:** no published agent system composes mutation-/equivalence-gated autonomous refactoring as discrete pipeline phases — evidence supports each phase individually; the composition is novel.

---

## 4. Round-2 findings: security / release / docs / learning micro-phases

### 4.1 `threat-model` (STRIDE, lightweight per-change)

STRIDE (Kohnfelder & Garg 1999; Microsoft SDL; Shostack) is explicitly usable by non-security practitioners → viable as a lightweight per-PR phase. Threat-Modeling-as-Code validates YAML/JSON models in CI per-PR. LLM-driven: STRIDE GPT (DFD/OpenAPI/IaC → threats + mitigations); ThreatModeling-LLM (arXiv 2411.17058); arXiv 2408.07537 (DFD+LLM threat validation); arXiv 2506.06478 (STRIDE on CI/CD pipelines themselves).

### 4.2 `fuzz-probe` + `fuzz-target-gen`

OSS-Fuzz: **>13,000 vulnerabilities and >50,000 bugs across ~1,000 projects** (May 2025); empirical study of 23,907 bugs/316 projects (arXiv 2103.11518). ClusterFuzzLite = per-PR short-budget continuous fuzzing in CI. Go has native fuzzing since 1.18 (`FuzzXxx`, `go test -fuzz`). **OSS-Fuzz-Gen** (LLM-generated harnesses): +29% line coverage vs human targets, >370K new lines covered across 272 C/C++ projects, 26 new bugs human harnesses missed; includes a 5-attempt build-failure auto-repair loop.

### 4.3 `rollback-plan` + progressive delivery

SRE practice: declare rollback mechanism + blast radius pre-deploy; confirm known-good version; verify rollback in staging; on-call revert <5 min. Feature flags = soft/runtime rollback (flag-off, not redeploy). DORA: elite teams deploy on demand with **<5% change-failure rate** vs up to 64% for low performers; change-failure-rate + failed-deployment-recovery-time are core metrics.

### 4.4 `changelog-sync`

Conventional Commits → semantic-release / release-please / release-drafter derive next semver + changelog + release **deterministically** — keep in tooling, minimal LLM (Core Rule 5).

### 4.5 Learning/memory phases (distinct from per-cycle retro)

- **`skill-extract`** — Voyager (arXiv 2305.16291): ever-growing **skill library of executable code**, retrieved by embedding; skills are "temporally extended, interpretable, and compositional"; trigger = self-verification passes. Writes durable procedural artifacts — distinct from retro's diagnosis.
- **`lesson-extract` (corpus-level)** — ExpeL (arXiv 2308.10144, AAAI-24): mines natural-language insights/rules across a *collection* of trajectories, no parametric updates. Architecturally distinct from a single-cycle retrospective.
- **`memory-manage`** — MemGPT (arXiv 2310.08560)/Letta: OS-style tiered Core/Recall/Archival memory with paging; trigger = context-window pressure.

### 4.6 `spec-design` / `contract-verify`

Contract-first: provider-owned OpenAPI spec as single source of truth, authored *before* implementation (plan archetype). Conformance: Pact consumer-driven contracts and/or schema-first provider validation as an evaluate gate on API changes.

---

## 5. Consolidated catalog and recipes

The full phase-by-phase spec sketches (inputs/outputs/signals, `insert_when`, classify rules) and the advisor goal-type recipe table live in the design doc: [docs/architecture/micro-phase-catalog.md](../../docs/architecture/micro-phase-catalog.md). Summary:

- **Wave 1 (goal-defining):** `reproduce-bug`, `fault-localization`, `behavior-baseline`/`behavior-compare`, `smell-scan`, `threat-model`, `test-amplification`
- **Wave 2 (deterministic-tooling gates):** `mutation-gate`, `benchmark-gate` (perf-profile upgrade), `fuzz-probe`, `cleanup-sweep`, `rollback-plan`
- **Wave 3 (release/feature/memory):** `changelog-sync`, `post-ship-monitor`, `api-contract-design`, `context-condense` (+ enhancements: AlphaCodium-style reflection/ranking cards; ExpeL corpus lesson-extract)

## 6. Cross-cutting principles distilled from the research

1. **One responsibility per phase** — every effective system studied (Agentless, AlphaCodium, AgentCoder, Aider) wins by splitting responsibilities, not by smarter monoliths.
2. **LLM judges, tooling gates** — detection/enumeration is LLM judgment; pass/fail authority comes from deterministic measurements (mutation score, benchstat p-value, reachability, FAIL_TO_PASS/PASS_TO_PASS).
3. **Anti-bias isolation** — test designers must not see the implementation (AgentCoder); auditors ≠ builders (already an evolve-loop cross-family floor).
4. **Phases stable, control flow configurable** — Agentless (fixed) vs RepairAgent (agentic) use the same phases; the advisor owns the control-flow dimension.
5. **Diff-scoping is the feasibility key** for heavy phases (Google mutation testing, ClusterFuzzLite, incremental benchmarks).
6. **Generated tests are signals, not oracles.**

## 7. Open questions for future research

- Controlled ablations isolating marginal value of individual micro-phases (vs full-pipeline deltas) — largely absent from the literature.
- Does goal-conditioned phase selection empirically beat a fixed pipeline? (Agentless suggests fixed-per-goal-type may be enough; the advisor's recipe table is effectively fixed-per-goal-type with LLM classification.)
- What validation/anchoring sub-phase should gate `test-amplification` before its tests are trusted? (Candidate: `mutation-gate` on the generated tests themselves.)
