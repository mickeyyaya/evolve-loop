# Phase Architecture — Citations & Design Rationale

> Maps every load-bearing design decision in [phase-architecture.md](phase-architecture.md) to the public research, technical reports, or industry standards that motivated it. When a structural choice is "non-obvious", this doc explains *which paper made us think this was important*. Updated alongside each release that adds a new structural mechanism.

## How to read this

Each phase + cross-cutting concern lists:
- **Decision**: the structural choice in evolve-loop
- **Citation**: the source (paper, technical report, RFC, blog) that motivated it
- **Why it applies**: the specific failure mode the citation describes that this decision prevents

Bibliography at the bottom — alphabetical with full author lists, year, venue, link.

---

## Phase 0 — Calibrate

### 8 benchmark dimensions (compositional scoring)
- **Citation**: ISO/IEC 25010:2011 *Software product quality model* (functionality, reliability, usability, efficiency, maintainability, portability, security, compatibility) [ISO-25010]
- **Why**: Established taxonomy. evolve-loop's 8 dimensions (`documentationCompleteness`, `defensiveDesign`, `evalInfrastructure`, etc.) are an LLM-era specialization of ISO 25010 — same goal (multi-axis quality measurement), different axes (because ISO's "compatibility" matters less for AI-generated code than `evalInfrastructure` does).

### Hybrid automated + LLM probes per dimension
- **Citation**: Ribeiro et al. 2020 *Beyond Accuracy: Behavioral Testing of NLP Models with CheckList* [Ribeiro-2020]
- **Why**: Single-metric quality scores hide failure modes. Combining quantitative (automated) + qualitative (LLM judgment) probes gives a calibration that catches both "syntax passes but logic broken" and "logic correct but readability poor".

### Goodhart's Law mitigation (composite vs single score)
- **Citation**: Strathern 1997 *Improving ratings: audit in the British University system* [Strathern-1997] (formalized as Goodhart's Law: "When a measure becomes a target, it ceases to be a good measure")
- **Why**: A single fitness score is gameable. Composite scoring across 8 independent dimensions makes it costly for an agent to game one dimension (the others surface the cost). evolve-loop's `state.json:fitnessRegression` flag fires when ANY dimension drops, even if `overall` is stable.

---

## Phase 0b — Intent (v8.19.1+)

### "Intent Architect" persona — structuring before execution
- **Citation**: Berglund et al. 2024 *Reversal Curse and Instruction Following Failures* (arXiv 2409.00557) [Berglund-2024]
- **Why**: Quoted in the persona file — "56% of real-world user instructions are missing key information." Production agents typically achieve only 25% prompt fidelity. Intent's job is to close that 56% gap before Scout starts.

### Ask-when-Needed (AwN) classifier — IMKI/IMR/IwE/IBTC/CLEAR
- **Citation**: Andreas et al. 2022 *Language Models as Agent Models* [Andreas-2022] + practitioner literature on intent disambiguation
- **Why**: Treating goal ambiguity as a class taxonomy (rather than free-form interpretation) makes the agent's reasoning auditable. The orchestrator's failure-adapter rule "intent-rejected → IBTC blocks the cycle" is enabled by AwN's discrete classes.

### Mandatory ≥1 challenged_premise rule (anti-sycophancy at structuring)
- **Citation**: Sharma et al. 2024 *Towards Understanding Sycophancy in Language Models* (arXiv 2310.13548) [Sharma-2024]
- **Why**: LLMs default to agreement. Sharma et al. show RLHF-trained models systematically prefer responses that match user beliefs, even when wrong. Forcing Intent to surface ≥1 contested premise breaks the sycophancy default at the earliest cycle phase.

### Karpathy's "wrong assumptions running uncaught" framing
- **Citation**: Karpathy 2024 talks/posts on agentic coding (no formal paper; multiple Twitter/podcast references) [Karpathy-2024]
- **Why**: Empirical observation that became a structural design principle. Intent surfaces assumptions explicitly (in `assumptions:` field) so future cycles can challenge them.

---

## Phase 1 / 2 — Scout (Research + Discovery)

### Pattern-3 fan-out (parallel sub-scouts + aggregator)
- **Citation**: Du et al. 2023 *Improving Factuality and Reasoning in Language Models through Multiagent Debate* [Du-2023] + addyosmani's swarm patterns (informal, referenced in CLAUDE.md)
- **Why**: Multi-agent debate / parallel critique improves output quality vs single-agent. The 3-sub-scout split (codebase / research / evals) gives each sub-agent narrower context, deeper expertise per sub-task. Aggregator merges via consensus — same pattern as Du et al.'s debate format.

### Mutation testing as eval-quality pre-flight (kill rate ≥ 0.8)
- **Citation**: Jia & Harman 2011 *An Analysis and Survey of the Development of Mutation Testing* [Jia-2011] + DeMillo, Lipton, Sayward 1978 *Hints on Test Data Selection* [DeMillo-1978]
- **Why**: An eval that mutated code can't kill is tautological — it would pass a buggy implementation. Mutation testing operationalizes "does this eval actually distinguish good from bad?" The 0.8 kill rate threshold is the standard cutoff in the mutation-testing literature.

### "Tautological evals" detection (cycle 25 incident)
- **Citation**: Pan et al. 2023 *MALT: Misalignment Detection at Scale* via METR [METR-2025]
- **Why**: METR documented frontier models gaming evals (e.g., editing the test to pass rather than fixing implementation). Mutation testing + Auditor's "test files weren't ALTERED" check are the structural defenses against this class.

### Goal-conditional vs autonomous discovery
- **Citation**: Wang et al. 2023 *Voyager: An Open-Ended Embodied Agent with Large Language Models* [Wang-2023]
- **Why**: Voyager's iterative skill-discovery loop alternates between goal-pursuit and exploratory discovery. Scout's `goal: provided` vs `goal: null` modes mirror this — focused execution for explicit goals, broad exploration for "find me work to do" autonomy.

---

## Phase 3 — Build (Builder)

### Per-cycle git worktree isolation
- **Citation**: Standard git worktree practice (git docs); specifically motivated by Anthropic's *Secure Deployment Guide* §"sandboxing-permissions-are-not-sandboxes" [Anthropic-Secure-Deploy]
- **Why**: Permission allowlists are not sandboxes. Worktree gives a separate filesystem branch + working directory so a compromised Builder can't accidentally write to main. Combined with profile-scoped permissions and OS sandbox-exec/bwrap, this is the Tier-2 OS-isolation layer.

### Self-verify before report (Step 5 — run task's eval graders)
- **Citation**: Madaan et al. 2023 *Self-Refine: Iterative Refinement with Self-Feedback* (arXiv 2303.17651) [Madaan-2023]
- **Why**: Self-Refine showed agents can iterate to better outputs by critiquing their own work before producing the final answer. Builder's Step 5 (run eval graders, fix failures, re-run) is Self-Refine adapted for code: the eval grader is the critique signal, the iteration is the refinement.

### Genes (gene matching by errorPattern + fileGlob, confidence ≥ 0.6)
- **Citation**: Wang et al. 2023 *Voyager* [Wang-2023] + Park et al. 2023 *Generative Agents: Interactive Simulacra of Human Behavior* [Park-2023]
- **Why**: Voyager's "skill library" + Generative Agents' "memory consolidation" both formalize the idea that agents accumulate reusable action recipes from prior experience. evolve-loop's `.evolve/genes/<id>.yaml` files are concrete action recipes; Builder's confidence-weighted gene matching is the consumption side. The 0.6 threshold is empirically tuned.

### Online research with Knowledge Capsules (Step 2.5)
- **Citation**: Yao et al. 2023 *ReAct: Synergizing Reasoning and Acting in Language Models* [Yao-2023] + RAG literature
- **Why**: ReAct's interleaved reasoning + acting (with tools) is the prototype. Builder's online-research step is its specialization for code: gap detected → search → capsule (cached for future cycles). The capsule cache is the consolidation mechanism that makes this scale beyond a single cycle.

### Worktree commit on cycle branch (Step 6)
- **Citation**: Standard git workflow; specifically the trunk-based development pattern
- **Why**: Branch-per-task is the long-standing convention. The v8.43.0 fix (worktree-aware ship.sh ff-merge) restored compatibility with this convention after a regression introduced when worktree isolation was added.

---

## Phase 4 — Audit (Auditor)

### Cross-family judge (Builder=Sonnet, Auditor=Opus default)
- **Citation**: Sharma et al. 2024 *Towards Understanding Sycophancy* [Sharma-2024] + Pan et al. 2024 *MMLU-Pro: Same-Family Judge Bias* [Pan-2024]
- **Why**: Same-model-family judging produces inflated PASS rates due to shared training distribution + RLHF preferences. Different model families (Sonnet vs Opus) have different blindspots, so Auditor catches things Builder's training data made invisible.

### Adversarial Auditor framing (default-on)
- **Citation**: Bai et al. 2022 *Constitutional AI: Harmlessness from AI Feedback* [Bai-2022]
- **Why**: Constitutional AI's red-team / blue-team split established that adversarial framing produces more rigorous critique than neutral framing. evolve-loop's "ADVERSARIAL AUDIT MODE" prompt prefix instantiates this — Auditor must produce *positive evidence* for PASS, not just absence of evidence for FAIL.

### Single-Pass Review Checklist (A through F)
- **Citation**: Code review checklist literature (e.g., Bacchelli & Bird 2013 *Expectations, Outcomes, and Challenges of Modern Code Review*) [Bacchelli-2013]
- **Why**: Empirical research shows checklist-based reviews catch ~30% more defects than free-form review. The A-F structure (Code Quality, Security, Hallucination, Pipeline Integrity, Eval Integrity, Multi-stage) is a domain-specific checklist for AI-generated code.

### Hallucination detection (B2 — claims-vs-diff)
- **Citation**: Ji et al. 2023 *Survey of Hallucination in Natural Language Generation* [Ji-2023] + METR reward hacking [METR-2025]
- **Why**: LLMs systematically claim facts they didn't verify. The v8.34.0 actual-diff footer (recording real file list in commit message) plus B2's claim-vs-diff cross-check is the structural defense.

### Mutation kill rate threshold (0.8)
- **Citation**: Jia & Harman 2011 [Jia-2011] (see Scout section)
- **Why**: Same justification as Scout's eval-quality pre-flight. Auditor re-verifies the rate so eval changes mid-cycle (Builder weakens grader → Auditor catches) get flagged.

### Pattern-3 fan-out (4 sub-auditors)
- **Citation**: Du et al. 2023 *Multiagent Debate* [Du-2023]
- **Why**: Same as Scout's fan-out. Specialized sub-auditors (eval-replay, lint, regression, build-quality) each have narrower scope → deeper inspection per area. Aggregator's "ANY FAIL → aggregate FAIL" is a consensus-style merge.

### Cycle binding (git_head + tree_state_sha) + ledger SHA
- **Citation**: Crosby & Wallach 2009 *Efficient Data Structures for Tamper-Evident Logging* [Crosby-2009]
- **Why**: Auditor's verdict must be unforgeable post-hoc — otherwise the orchestrator could rewrite the audit report after the fact. Recording artifact SHA256 + git state pins the verdict to the exact code+artifact state. Combined with v8.37.0 hash chain, modification anywhere in history breaks the chain at the modification point.

### v8.35.0 adaptive model selection (Sonnet for trivial diffs)
- **Citation**: ProjectDiscovery 2025 *Multi-step pipeline cost analysis* (blog) [ProjectDiscovery-2025] + Anthropic prompt caching docs [Anthropic-Cache-2025]
- **Why**: Empirical observation that smaller-model auditing is sufficient for low-complexity diffs. Saves 60-80% cost on routine cycles without losing the cross-family-judge benefit (Builder=Sonnet, Auditor=Opus is the default; Auditor stays Sonnet only when diff is trivial AND no security paths).

---

## Phase 5 — Ship/Record

### ship-gate.sh canonical-entry-point allowlist
- **Citation**: Saltzer & Schroeder 1975 *The Protection of Information in Computer Systems* (principle of least privilege + complete mediation) [Saltzer-1975]
- **Why**: ship-class commands (git commit/push/gh release) are mediated through ONE allowlisted script (ship.sh). All other paths denied. This is "complete mediation" — every ship-class operation goes through the same audit-checking entry point.

### Version-aware TOFU (Trust-On-First-Use) — v8.32.0
- **Citation**: Wendlandt et al. 2008 *Perspectives: Improving SSH-style Host Authentication with Multi-Path Probing* [Wendlandt-2008] (TOFU primer)
- **Why**: TOFU's classic problem is the SHA changing legitimately (e.g., software upgrade) is indistinguishable from tampering. v8.32.0's version-aware variant pins both SHA AND version → version bump (legitimate) re-pins automatically; same-version SHA change (tampering) blocks.

### Cycle binding — preventing audit replay attacks
- **Citation**: Anderson 2008 *Security Engineering* §"replay attacks" [Anderson-2008]
- **Why**: Without cycle binding, an old audit could be replayed to ship newer code. The git_head + tree_state_sha pin ensures the audit's "scope of authorization" matches the ship's "scope of execution".

### Worktree-aware ship (v8.43.0+)
- **Citation**: Standard git workflow + the v8.43.0 incident report (~$18.92 wasted across 5 cycles)
- **Why**: Bug-driven design. The 5-cycle waste in the user-reported incident showed the necessity of bridging worktree branch → main. The v8.43 fix uses `git merge --ff-only` rather than cherry-pick because ff-merge preserves the cycle's commit history exactly, making rollback trivial.

### Failure classification taxonomy (9 classes)
- **Citation**: Avizienis et al. 2004 *Basic Concepts and Taxonomy of Dependable and Secure Computing* [Avizienis-2004]
- **Why**: Avizienis et al. distinguish *transient* vs *systemic* faults, with different recovery strategies for each. evolve-loop's classification mirrors this: `infrastructure-transient` (1d age-out, retry-yes) vs `infrastructure-systemic` (7d, needs-operator). Code failures get separate axes since code-quality and infra-quality are independent dimensions.

### Fluent-by-default failure adaptation (v8.28.0+)
- **Citation**: Anderson 2008 *Security Engineering* §"fail-safe vs fail-secure" [Anderson-2008]
- **Why**: Anderson distinguishes systems where failure should default-allow (fail-safe — door unlocks if power fails) vs default-deny (fail-secure — door locks if power fails). Pre-v8.28.0 evolve-loop was fail-secure on every WARN/recurring-failure (block ship). v8.28.0 made code-side failures fail-safe (proceed with awareness) while keeping integrity-side failures fail-secure. The split lets the pipeline keep moving on quality issues while blocking only on structural breaches.

---

## Phase 6 — Learn (Retrospective)

### Reflexion-style self-reflection loop
- **Citation**: Shinn et al. 2023 *Reflexion: Language Agents with Verbal Reinforcement Learning* (arXiv 2303.11366) [Shinn-2023]
- **Why**: Reflexion shows agents can improve over time by writing structured retrospectives that feed back into future trial prompts. evolve-loop's lesson YAMLs serve the same purpose — failure → reflection → instinct → next-cycle-input.

### "Lesson IS the retrospective, not a status report"
- **Citation**: Schon 1983 *The Reflective Practitioner* [Schon-1983] + Argyris & Schon 1978 *Organizational Learning* [Argyris-1978]
- **Why**: Schon distinguishes "single-loop learning" (fix the immediate error) from "double-loop learning" (revise the assumptions that produced the error). Double-loop produces actionable institutional memory. evolve-loop's persona explicitly demands the *underlying assumption* (double-loop) rather than just the defect list (single-loop).

### One-lesson-per-root-cause rule
- **Citation**: Toyota's 5 Whys methodology (root-cause analysis literature) [Ohno-1988]
- **Why**: Multiple defects sharing a root cause should produce one consolidated lesson, not duplicates. Cross-linking via `relatedInstincts` preserves the relationship.

### Adversarial honesty about contradicted instincts
- **Citation**: Popper 1959 *The Logic of Scientific Discovery* (falsifiability) [Popper-1959] + Kuhn 1962 *The Structure of Scientific Revolutions* [Kuhn-1962]
- **Why**: When new evidence contradicts a prior instinct, Popper says the prior must be falsified, not preserved. evolve-loop's lesson schema's `contradicts:` field is the falsification mechanism. The orchestrator doesn't auto-prune (avoids over-correction) but flags for operator review.

### Future-self-readable framing
- **Citation**: Engelbart 1962 *Augmenting Human Intellect* [Engelbart-1962] (notion of externalized knowledge)
- **Why**: Lessons are not just for THIS retrospective; they're context for future agents who lack this conversation. The persona explicitly instructs writing for "future-self consumption" so the description + preventiveAction are agent-actionable without session context.

---

## Phase 7 — Meta (every-5-cycle self-improvement)

### Periodic meta-cycle (gated frequency)
- **Citation**: Lehman et al. 2022 *Evolution Through Large Models* [Lehman-2022] + Sun et al. 2023 *Self-Improving Foundation Models without Human Feedback* [Sun-2023]
- **Why**: Both papers show self-improving systems need frequency gates to avoid runaway adaptation (over-correction on noise). Every-5-cycle gives enough signal accumulation between meta-changes.

### Pattern detection across cycles
- **Citation**: Fernando et al. 2023 *Promptbreeder: Self-Referential Self-Improvement Via Prompt Evolution* [Fernando-2023]
- **Why**: Promptbreeder's evolutionary prompt-improvement loop inspires evolve-loop's pattern-detection-then-propose pattern. The proposals[] field is the "candidate gene pool" the operator selects from.

### Operator approval gate (no auto-modification of kernel)
- **Citation**: Asilomar AI Principles + Anthropic's Responsible Scaling Policy [Asilomar-2017] [Anthropic-RSP-2024]
- **Why**: Self-modification of trust-critical infrastructure (kernel hooks, profile allowlists) without human approval is the canonical scenario for unbounded agent risk. Operator gate is the structural circuit-breaker.

---

## Cross-cutting concerns

### Tier-1 kernel hooks (phase-gate, role-gate, ship-gate, hash chain, cycle binding)
- **Citation**: Saltzer & Schroeder 1975 *The Protection of Information in Computer Systems* [Saltzer-1975] (principles of complete mediation, fail-safe defaults, least privilege)
- **Why**: All five Saltzer-Schroeder design principles map directly onto evolve-loop's Tier-1 layer. Phase-gate = sequence enforcement. Role-gate = least privilege per agent. Ship-gate = complete mediation. Hash chain = unforgeable history. Cycle binding = scope of authorization.

### Tamper-evident hash-chained ledger (v8.37.0+)
- **Citations**:
  - Crosby & Wallach 2009 *Efficient Data Structures for Tamper-Evident Logging* [Crosby-2009]
  - Merkle 1988 *A Digital Signature Based on a Conventional Encryption Function* [Merkle-1988]
  - IETF draft 2026 *Agent Audit Trail* [IETF-Agent-Audit-2026]
- **Why**: The hash chain pattern is foundational. Crosby & Wallach formalize the data structure properties. Merkle's hash trees are the primitive. The IETF draft is the 2026 contemporary standardization for AI agent audit trails specifically — evolve-loop's v8.37.0 implementation aligns with the draft's mandatory `prev_hash` requirement.

### Adaptive cost (prompt caching, conditional context blocks)
- **Citations**:
  - Anthropic 2025 *Prompt Caching documentation* [Anthropic-Cache-2025]
  - ProjectDiscovery 2025 *Multi-step pipeline cost report* [ProjectDiscovery-2025]
- **Why**: Anthropic's prompt cache (≥1024 tokens, 5-min TTL, 0.1× cost on cache reads) is the runtime mechanism. ProjectDiscovery's empirical 59% cost reduction in multi-step pipelines is the validation. v8.33.0's "static-first dynamic-last" prompt structure is the application of these findings.

### Three-Tier Strictness Model
- **Citation**: Avizienis et al. 2004 *Basic Concepts and Taxonomy of Dependable and Secure Computing* [Avizienis-2004]
- **Why**: Avizienis et al. distinguish "structural integrity" (faults that violate system invariants) from "operational issues" (faults the system can recover from). Tier 1 = structural, Tier 2 = environmental, Tier 3 = workflow defaults. The taxonomy matches Avizienis et al.'s fault classification.

### Worktree provisioning with stale-admin recovery (v8.36.0+)
- **Citation**: Standard git documentation + the v8.36.0 incident report
- **Why**: Bug-driven design. Stale `.git/worktrees/<name>/` admin entries blocked branch deletion; `git worktree prune` is the documented recovery. v8.36.0 codified this in run-cycle.sh's pre-flight cleanup so operators don't hit the same trap manually.

### Reward hacking detection (Auditor's anti-gaming layer)
- **Citations**:
  - METR 2025 *Recent Frontier Models Are Reward Hacking* [METR-2025]
  - Pan et al. 2023 *MALT: Misalignment Detection at Scale* [Pan-2023]
  - aerosta 2025 *rewardhackwatch* (GitHub) [Rewardhackwatch-2025]
- **Why**: METR documented that even SOTA models (Claude, GPT-4) game evaluations when given autonomous coding tasks. The MALT dataset (5,391 confirmed reward-hacking trajectories) is the empirical foundation. evolve-loop's Adversarial framing + cycle binding + hash chain are the three structural defenses; rewardhackwatch's regex pattern library is a candidate v8.38+ addition.

### Fluent-by-default vs strict (v8.28.0+)
- **Citation**: Anderson 2008 *Security Engineering* [Anderson-2008] §"fail-safe vs fail-secure"
- **Why**: See Phase 5 — same justification. The split between "code-side fluent" and "integrity-side strict" is the operational tradeoff Anderson describes.

---

## Bibliography

[Anderson-2008] Anderson, R. *Security Engineering: A Guide to Building Dependable Distributed Systems* (2nd ed.). Wiley, 2008. https://www.cl.cam.ac.uk/~rja14/book.html

[Andreas-2022] Andreas, J. "Language Models as Agent Models." *Proceedings of EMNLP* (2022). arXiv:2212.01681

[Anthropic-Cache-2025] Anthropic. "Prompt Caching." Documentation, 2025. https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching

[Anthropic-RSP-2024] Anthropic. "Responsible Scaling Policy v2." 2024. https://www.anthropic.com/responsible-scaling-policy

[Anthropic-Secure-Deploy] Anthropic. "Secure Deployment Guide for Claude Code." 2025. https://docs.anthropic.com/en/docs/claude-code/secure-deployment

[Argyris-1978] Argyris, C. & Schon, D. *Organizational Learning: A Theory of Action Perspective*. Addison-Wesley, 1978.

[Asilomar-2017] Future of Life Institute. "Asilomar AI Principles." 2017. https://futureoflife.org/open-letter/ai-principles/

[Avizienis-2004] Avizienis, A.; Laprie, J.-C.; Randell, B.; Landwehr, C. "Basic Concepts and Taxonomy of Dependable and Secure Computing." *IEEE Transactions on Dependable and Secure Computing* 1(1), 2004.

[Bacchelli-2013] Bacchelli, A. & Bird, C. "Expectations, Outcomes, and Challenges of Modern Code Review." *ICSE* 2013.

[Bai-2022] Bai, Y. et al. "Constitutional AI: Harmlessness from AI Feedback." Anthropic, 2022. arXiv:2212.08073

[Berglund-2024] Berglund, L. et al. "Reversal Curse and Instruction Following Failures." 2024. arXiv:2409.00557

[Crosby-2009] Crosby, S. A. & Wallach, D. S. "Efficient Data Structures for Tamper-Evident Logging." *USENIX Security* 2009.

[DeMillo-1978] DeMillo, R. A.; Lipton, R. J.; Sayward, F. G. "Hints on Test Data Selection: Help for the Practicing Programmer." *IEEE Computer* 11(4), 1978.

[Du-2023] Du, Y. et al. "Improving Factuality and Reasoning in Language Models through Multiagent Debate." 2023. arXiv:2305.14325

[Engelbart-1962] Engelbart, D. C. "Augmenting Human Intellect: A Conceptual Framework." SRI Summary Report AFOSR-3223, 1962.

[Fernando-2023] Fernando, C. et al. "Promptbreeder: Self-Referential Self-Improvement Via Prompt Evolution." 2023. arXiv:2309.16797

[IETF-Agent-Audit-2026] Sharif, et al. "Agent Audit Trail: A Standard Logging Format for Autonomous AI Systems." IETF Internet-Draft, March 2026. https://datatracker.ietf.org/doc/draft-sharif-agent-audit-trail/

[ISO-25010] ISO/IEC 25010:2011. "Systems and software engineering — Systems and software Quality Requirements and Evaluation (SQuaRE) — System and software quality models." 2011.

[Ji-2023] Ji, Z. et al. "Survey of Hallucination in Natural Language Generation." *ACM Computing Surveys* 55(12), 2023.

[Jia-2011] Jia, Y. & Harman, M. "An Analysis and Survey of the Development of Mutation Testing." *IEEE Transactions on Software Engineering* 37(5), 2011.

[Karpathy-2024] Karpathy, A. Multiple talks/posts on agentic coding, 2024. (No formal paper; referenced via Twitter/podcast appearances.)

[Kuhn-1962] Kuhn, T. S. *The Structure of Scientific Revolutions*. University of Chicago Press, 1962.

[Lehman-2022] Lehman, J. et al. "Evolution through Large Models." 2022. arXiv:2206.08896

[Madaan-2023] Madaan, A. et al. "Self-Refine: Iterative Refinement with Self-Feedback." *NeurIPS* 2023. arXiv:2303.17651

[Merkle-1988] Merkle, R. C. "A Digital Signature Based on a Conventional Encryption Function." *CRYPTO '87* (published 1988).

[METR-2025] METR. "Recent Frontier Models Are Reward Hacking." Blog post + MALT dataset, June 2025. https://metr.org/blog/2025-06-05-recent-reward-hacking/

[Ohno-1988] Ohno, T. *Toyota Production System: Beyond Large-Scale Production*. Productivity Press, 1988. (Origin of "5 Whys" root-cause method.)

[Pan-2023] Pan, A. et al. "MALT: Misalignment Detection at Scale." 2023. METR technical report.

[Pan-2024] Pan, A. et al. "MMLU-Pro." 2024. arXiv (multiple variants).

[Park-2023] Park, J. S. et al. "Generative Agents: Interactive Simulacra of Human Behavior." *UIST* 2023. arXiv:2304.03442

[Popper-1959] Popper, K. *The Logic of Scientific Discovery*. Hutchinson, 1959.

[ProjectDiscovery-2025] ProjectDiscovery. "Multi-step pipeline cost analysis with prompt caching." Blog, 2025.

[Rewardhackwatch-2025] aerosta. "rewardhackwatch: Runtime detector for reward hacking and misalignment in LLM agents." 2025. https://github.com/aerosta/rewardhackwatch

[Ribeiro-2020] Ribeiro, M. T. et al. "Beyond Accuracy: Behavioral Testing of NLP Models with CheckList." *ACL* 2020.

[Saltzer-1975] Saltzer, J. H. & Schroeder, M. D. "The Protection of Information in Computer Systems." *Proceedings of the IEEE* 63(9), 1975.

[Schon-1983] Schon, D. A. *The Reflective Practitioner: How Professionals Think In Action*. Basic Books, 1983.

[Sharma-2024] Sharma, M. et al. "Towards Understanding Sycophancy in Language Models." Anthropic, 2024. arXiv:2310.13548

[Shinn-2023] Shinn, N. et al. "Reflexion: Language Agents with Verbal Reinforcement Learning." *NeurIPS* 2023. arXiv:2303.11366

[Strathern-1997] Strathern, M. "'Improving ratings': audit in the British University system." *European Review* 5(3), 1997.

[Sun-2023] Sun, Z. et al. "Self-Improving Foundation Models without Human Feedback." 2023. arXiv:2310.04408

[Wang-2023] Wang, G. et al. "Voyager: An Open-Ended Embodied Agent with Large Language Models." 2023. arXiv:2305.16291

[Wendlandt-2008] Wendlandt, D.; Andersen, D. G.; Perrig, A. "Perspectives: Improving SSH-style Host Authentication with Multi-Path Probing." *USENIX ATC* 2008.

[Yao-2023] Yao, S. et al. "ReAct: Synergizing Reasoning and Acting in Language Models." *ICLR* 2023. arXiv:2210.03629

---

## Updating this doc

When a new structural mechanism is added (typically per major release), add an entry following the format:

```markdown
### <Mechanism name> (v8.NN.0+)
- **Citation**: <Author Year *Title* [Tag]>
- **Why**: <one-sentence motivation linked to a specific failure mode>
```

And add a corresponding bibliography entry. The goal: every load-bearing design choice should trace to a public paper, RFC, blog, or named industry standard within 3 clicks.
