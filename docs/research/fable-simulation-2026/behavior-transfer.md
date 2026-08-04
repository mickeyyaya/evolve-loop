# Behavior Transfer via Prompting: Simulating a Stronger Model's Operating Discipline in Production Coding Agents (2025–2026 web research)

Scope: techniques for capturing a stronger model's decision policies as written procedures and running them on weaker/faster production coding agents (Claude/GPT/Gemini CLI-class harnesses) — **no fine-tuning**. Researched 2026-07-07. All entries linked; effect sizes quoted where published.

---

## 1. Behavioral cloning via system prompts / procedure files

### 1.1 SkillsBench — curated procedures lift weak models past unaugmented strong ones (headline result)
[SkillsBench (arXiv 2602.12670)](https://arxiv.org/html/2602.12670v1) — 84 tasks, 11 domains, deterministic verifiers; 3 harnesses (Claude Code, Gemini CLI, Codex CLI) × 7 models, 7,308 trajectories.
- Curated Skills: **+16.2pp average** (24.3% → 40.6% pass rate). Domain spread: healthcare +51.9pp, manufacturing +41.9pp, cybersecurity +23.2pp … software engineering only **+4.5pp** (pretraining already covers it).
- **Claude Haiku 4.5 + curated skills (27.7%) beat Claude Opus 4.5 with no skills (22.0%)** — direct evidence that written procedures partially substitute for model capacity.
- **Self-generated skills: −1.3pp vs baseline (21.0%)** — the weaker/receiving model cannot synthesize its own effective procedures; the knowledge must come from a stronger source (human expert or stronger model). This is the core justification for "capture strong-model behavior → run on cheap model."
- What works structurally: concrete procedures, step-by-step workflows, **worked examples**, domain specifics. "Detailed" (+18.8pp) and "compact" (+17.1pp) skills beat "comprehensive" (**−2.9pp** — exhaustive docs actively hurt). 2–3 skills per task optimal (+18.6pp); 4+ skills collapse to +5.9pp (guidance overload). 16/84 tasks showed negative deltas — conflicting guidance is a real failure mode.

### 1.2 SkillJuror — how procedure *organization* changes runtime behavior
[SkillJuror (arXiv 2606.11543)](https://arxiv.org/html/2606.11543) — isolates skill *structure* as controlled variable (82 tasks × 3 conditions × 5 trials = 1,230 trials, GPT-5.4 high-reasoning).
- Flat monolithic instruction file (42.0% pass) vs **progressive disclosure** (short root + on-demand reference files): 46.1% pass (**+4.1pp**), same cost per pass ($1.31 vs $1.28).
- PD changed behavior measurably: resource fanout 1.18 → 3.85 files/trajectory; effective resource uptake 1.33 → 3.92 events/trajectory; skill consultation spread across task phases instead of front-loaded.
- Heterogeneous by task type: gains on code/security, losses on numeric-tolerance/media tasks. Structure is not free — measure per domain.
- Methodology worth stealing: deterministic gating + rubric-based semantic audit (968 items) + "Effective Resource Uptake" (resource access only counts when consumed into observable implementation/validation/correction).

### 1.3 ACE — evolving playbooks distilled from execution traces
[Agentic Context Engineering (arXiv 2510.04618)](https://arxiv.org/html/2510.04618v1) (Stanford/SambaNova/Berkeley; builds on [Dynamic Cheatsheet, Suzgun et al. 2025](https://arxiv.org/pdf/2510.04618)) — treats context as an **evolving playbook**, not a static prompt. Generator (runs tasks) → Reflector (distills lessons from success/failure traces) → Curator (merges typed delta items with helpful/harmful counters, dedup, pruning).
- **+10.6% on AppWorld agent tasks, +8.6% finance reasoning, ~87% latency reduction** vs strong context-adaptation baselines.
- Key design lesson: itemized, structured bullets with provenance counters beat monolithic prompt rewrites ("context collapse" — LLM rewriters compress away hard-won detail). Directly applicable to maintaining a "strong-model behavior file" that a weaker executor consumes.

### 1.4 GEPA — reflective prompt evolution as the distillation mechanism
[GEPA (arXiv 2507.19457, ICLR 2026 oral)](https://arxiv.org/abs/2507.19457) — samples full trajectories, reflects in natural language on failures, evolves prompts genetically along a Pareto frontier.
- Beats GRPO (RL) by **+6pp avg, up to +19pp, with 35× fewer rollouts**; beats MIPROv2 by >10pp (+12pp AIME-2025).
- Relevance: the "capture" side of behavior transfer — a stronger reflector model can write the prompt scaffold that a weaker executor runs; language-level reflection extracts more per trajectory than scalar rewards.

### 1.5 Worked examples vs abstract rules
- Practitioner + survey evidence: LLMs are **pattern-followers more than rule-followers** — 2–10 demonstrations typically beat equivalent abstract instruction lists for format/style/procedure adherence ([Latitude](https://latitude.so/blog/how-examples-improve-llm-style-consistency), [Comet few-shot for agents](https://www.comet.com/site/blog/few-shot-prompting/)). Caveat: demonstrations can be neutral or harmful on knowledge-heavy tasks (MMLU-style), so use examples for *procedural/format* transfer, rules for *boundary conditions*.
- SkillsBench corroborates at agent scale: effective skills = concrete procedures + worked examples; abstract/comprehensive prose hurt (−2.9pp).
- [Anthropic's Agent Skills authoring guidance](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills): progressive disclosure (metadata → SKILL.md → reference files); bundle deterministic operations as **scripts, not prose** ("code as both tool and documentation"); split mutually-exclusive contexts into separate files; start from an eval of the capability gap; iterate from observed trajectories.
- Net: **decision trees/if-then playbooks + worked examples + executable scripts transfer best; long abstract rule lists transfer worst** (see also IFScale, §3.3).

---

## 2. Metacognitive scaffolding on weaker/faster models

### 2.1 Metacognitive Prompting (Wang & Zhao, NAACL 2024)
[Metacognitive Prompting Improves Understanding in LLMs](https://aclanthology.org/2024.naacl-long.106/) ([arXiv 2308.05342](https://arxiv.org/abs/2308.05342)) — 5-stage introspective scaffold (comprehension → preliminary judgment → critical evaluation → decision+confidence → reflection). Tested on Llama-2, PaLM-2, GPT-3.5, GPT-4 across 10 NLU datasets. MP consistently beat CoT and its variants; **PaLM-2 + MP approached GPT-4-level performance** — mid-tier models gain the most.

### 2.2 Flavell-framework monitoring and Meta-R1
[Before you \<think\>, monitor (arXiv 2510.16374)](https://arxiv.org/pdf/2510.16374) implements explicit monitor→control loops; **Meta-R1** adds a separate meta-process that plans, monitors, and enforces stopping for a reasoning model — metacognition as architectural add-on improves efficiency and sometimes accuracy ([survey context](https://arxiv.org/pdf/2505.19184)). Consistent finding across this literature: metacognitive abilities in LLMs are **partial and unstable**; scaffolds externalize what the model can't reliably do internally.

### 2.3 The self-critique ceiling (critical for weak receivers)
- [LLMs Cannot Self-Correct Reasoning Yet (Huang et al., arXiv 2310.01798)](https://arxiv.org/abs/2310.01798): intrinsic self-correction **without external feedback degrades performance** — correct answers get flipped. Oracles/verifiers drove earlier "self-correction works" claims.
- [Small Language Models Need Strong Verifiers to Self-Correct Reasoning (arXiv 2404.17140)](https://arxiv.org/pdf/2404.17140): sub-frontier models specifically cannot self-verify; pairing them with an **external verifier** recovers the gains.
- 2025-26 confirmation: self-critique degrades via false negatives + hallucinated feedback; gains come from **multiple attempts + reliable external verification**, not richer critique prompts ([metacognitive monitoring battery](https://arxiv.org/pdf/2604.15702), [self-assessment rethink](https://arxiv.org/pdf/2605.07806)). Scaffolding "tendency to hurt weaker models more often" was observed ([Alignment Forum overview](https://www.alignmentforum.org/posts/m5d4sYgHbTxBnFeat/human-like-metacognitive-skills-will-reduce-llm-slop-and-aid)).
- **Design consequence**: for Flash/mini-class receivers, prompt scaffolds should demand *checkable actions* (run test, diff, grep) rather than *introspective judgment* ("re-examine your reasoning"). Verify-with-tools transfers; verify-by-rethinking does not.

### 2.4 Confidence tagging
Verbalized confidence is **systematically overconfident and saturates at 0.9/1.0** across models and elicitation strategies ([Wired for Overconfidence, arXiv 2604.01457](https://arxiv.org/pdf/2604.01457); [systematic evaluation, arXiv 2510.20460](https://arxiv.org/html/2510.20460v1); [Xiong et al. 2306.13063](https://arxiv.org/html/2306.13063v2)). Partial fixes: top-K alternatives before scoring, sampling-based aggregation, multi-option probability distributions (DINCO). For an operating discipline: confidence tags are useful as **routing signals** (low-confidence → escalate/verify) but not as calibrated probabilities; prefer behavioral proxies (did tests pass) over self-reported certainty.

### 2.5 Plan-verify-execute
Plan-and-Solve-style scaffolds remain competitive on current models (e.g., 0.96 accuracy for GPT-4.1 PS vs 0.958 few-shot-CoT on ETHICS; [ProMoral-Bench, arXiv 2602.13274](https://arxiv.org/pdf/2602.13274)), but published Flash/mini-specific effect sizes for checklist prompting are thin; the strongest small-model evidence is SkillsBench's Haiku result (§1.1) — procedure files, not generic "plan first" exhortations, moved the needle.

---

## 3. Cross-model prompt portability

### 3.1 "Model Drifting" is quantified — direct transfer loses 5–30%
[PromptBridge (arXiv 2512.01420)](https://arxiv.org/html/2512.01420v1):
- GPT-4o's best prompt on other models: **−23.5% with Claude's template, −29.0% with Gemini's template**; GPT-5's optimal prompt on Llama-3.1-70B: 68.70% vs the model's own 79.47% optimum. GPT-4o-optimized prompt on a *stronger* target hits 92.27% vs achievable 98.37% on HumanEval — even upgrading models loses ground without prompt adaptation.
- Fixing transfer (calibrate-then-map adapter) recovers: **+13.3% (xCodeEval GPT-4o→o3), +27.4% (SWE-Bench o4-mini→o3), +39.4% (Terminal-Bench GPT-4o→o3)** relative gains.
- What breaks: role-tag/interface mismatches, tokenization, vendor RLHF criteria, reasoning-style expectations. What survives: core task intent + a *systematic* (learnable) transformation between vendor styles — "shared latent structure" in the edits.

### 3.2 Vendor formatting and instruction-hierarchy differences
- **Anthropic**: trained on XML tags; official guidance is explicit tag-delimited sections ([Claude prompting best practices](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices)). Claude 4 scored best of all models on respecting system-vs-user message conflicts in the [OpenAI–Anthropic cross-lab safety evaluation](https://openai.com/index/openai-anthropic-safety-evaluation/).
- **OpenAI**: formal instruction hierarchy (platform > developer > user, [Model Spec](https://model-spec.openai.com/2025-12-18.html)); prefers instructions-first + `###`/`"""` delimiters, markdown-leaning.
- **Gemini**: system instructions since 1.5; ordering + explicit constraints emphasized. ([Cross-API comparison, Steve Kinney](https://stevekinney.com/writing/prompt-engineering-frontier-llms))
- Formatting is not cosmetic: **XML-vs-markdown choice alone swings performance up to ~30% on some model/task pairs**, in opposite directions per model ([CinfyAI cross-model practices](https://www.blog.cinfy.ai/prompt-engineering-across-models-best-practices-when-you-compare-gpt-claude-gemini-more/), [model-specific prompting](https://www.joanmedia.dev/ai-blog/model-specific-prompting-how-claude-gpt-and-gemini-differ)). Portable baseline: behavioral instructions first, delimited context blocks, task last.

### 3.3 Rule-list length tolerance — IFScale
[How Many Instructions Can LLMs Follow at Once? (arXiv 2507.11538)](https://arxiv.org/abs/2507.11538) — 500 simultaneous instructions, 20 models, 7 providers:
- Best frontier models: only **68% adherence at 500 instructions**; earlier-position instructions favored (primacy bias).
- **Three decay regimes**: threshold decay (reasoning models: o3, gemini-2.5-pro — near-perfect until a cliff), linear decay (gpt-4.1, claude-sonnet-4), **exponential decay (mini/fast class: gpt-4o, llama-4-scout)**. Weaker receivers hit the wall soonest — long rule lists are the least portable scaffold form; reasoning effort/thinking mode buys back some density.
- Design consequence: cap always-on rules (dozens, not hundreds), push detail into progressively-disclosed files (§1.2), put the highest-stakes rules first.

---

## 4. Measuring whether a model follows an operating discipline

### 4.1 Bait-scenario / process-compliance evals
- **BS-Bench** ([The Compliance Gap, arXiv 2605.01771](https://arxiv.org/abs/2605.01771)) — first open process-compliance benchmark; 7 tool-call-log audit metrics + leaderboard. Plants shortcuts that produce correct outputs while violating prescribed procedure. Findings: six frontier models at **0% instruction compliance by default**; Claude Sonnet 4 *verbally agreed* to the procedure in 10/10 trials and violated it in 10/10. **Audit-trail requirement (log rationale per step) → 97% compliance; removing the tempting affordance (delegation tools) → 75% (Cohen's d = 2.47)**. Conclusion: the gap is environmental-affordance-driven, so measure from tool logs and enforce structurally.
- **Tempting Shortcut scenario** ([hierarchical safety principles benchmark, arXiv 2506.02357](https://arxiv.org/html/2506.02357v1)): introducing a principle dropped task success 100% → 68% even when compliant solutions existed — quantifies the "cost of compliance" you must budget for when adding discipline.
- **ImpossibleBench** ([arXiv 2510.20270](https://arxiv.org/html/2510.20270v1)): mutate unit tests to contradict the spec — any pass proves spec-violating shortcutting. **Stronger models cheat more**, with a taxonomy (assertion edits, special-casing, operator overloading, state recording). Companion benchmarks: [EvilGenie](https://arxiv.org/html/2511.21654), [RHB tool-use reward hacking](https://arxiv.org/html/2605.02964v1), [SpecBench long-horizon](https://arxiv.org/pdf/2605.21384).
- **SOPBench** ([arXiv 2503.08669](https://arxiv.org/pdf/2503.08669)): standard-operating-procedure adherence under constraints; code-verifiable compliance.

### 4.2 Instruction-following-under-agency benchmarks
[AgentIF (arXiv 2505.16944)](https://arxiv.org/pdf/2505.16944): realistic agentic system prompts (long, multi-constraint); **best models follow <30%** of full instruction sets. Key methodological stance shared with IFEval: **verify compliance by code, not judgment** wherever a constraint is mechanically checkable. [The Stability Trap (arXiv 2601.11783)](https://arxiv.org/pdf/2601.11783) warns LLM-judged adherence audits are themselves unreliable — prefer programmatic checks, use LLM judges only for semantics.

### 4.3 Rubric-scored trajectories and blind A/B skill evals
- **Claude Code Skills 2.0 built-in evals** ([MindStudio guide](https://www.mindstudio.ai/blog/claude-code-skills-2-evaluation-ab-testing), [whytryai how-to-test](https://www.whytryai.com/p/how-to-test-claude-skills)): blind A/B — independent Claude instance judges with-skill vs without-skill outputs without knowing which is which, using a rubric derived from the eval prompt + assertions; weighted-criteria rubrics (weights sum to 1).
- **SkillJuror's layered method** (§1.2): deterministic structural gates + rubric semantic audit + human edge-case review; runtime metrics (outcome, efficiency, paradigm realization, resource-routing quality).
- Trajectory evaluation practice: score the tool-call sequence, not just the final answer ([agent evaluation overview](https://medium.com/@vinodkrane/chapter-8-agent-evaluation-for-llms-how-to-test-tools-trajectories-and-llm-as-judge-788f6f3e0d52); [survey of skill eval frameworks, arXiv 2606.11435](https://arxiv.org/pdf/2606.11435)).
- Recipe for behavior-transfer evals: (1) bait tasks with a planted shortcut, compliance read from tool logs programmatically; (2) blind A/B with/without the behavior file, rubric-scored; (3) per-domain breakdown (SkillsBench showed 16/84 tasks regress); (4) 5+ trials/condition (high per-task variance everywhere in this literature).

---

## 5. Known ceilings of prompt-level transfer + published mitigations

### 5.1 What prompting demonstrably cannot close
1. **Intrinsic verification depth** — no prompt makes a weak model a reliable judge of its own reasoning ([2310.01798](https://arxiv.org/abs/2310.01798), [2404.17140](https://arxiv.org/pdf/2404.17140)). Mitigation: external verifiers, tests, deterministic gates.
2. **Process compliance under tempting affordances** — 0% default compliance even with verbal agreement ([BS-Bench](https://arxiv.org/abs/2605.01771)); harness-level enforcement (remove affordance, require audit trail) works where instructions fail.
3. **Long-horizon coherence** — the "coherence cliff": failure is self-conditioning + lost-in-the-middle + error cascades, not raw capability ([Sharad Jain](https://www.sharadja.in/blog/long-horizon-agents-coherence-cliff), [Long-Horizon Task Mirage, arXiv 2604.11978](https://arxiv.org/pdf/2604.11978)). METR: best model 50%-time-horizon ≈ **14.5h (Claude Opus 4.6, Feb 2026)**, doubling every ~4–7 months ([METR 2503.14499](https://arxiv.org/pdf/2503.14499), [AI Digest tracker](https://theaidigest.org/time-horizons), [half-life model 2505.05115](https://arxiv.org/pdf/2505.05115)) — weaker/faster models sit far below, and no prompt scaffold moves a model up this curve materially.
4. **Rule-density ceiling** — exponential decay for mini-class models under many simultaneous constraints ([IFScale](https://arxiv.org/abs/2507.11538)).
5. **Self-synthesis of expertise** — models can't write their own effective skills (SkillsBench −1.3pp); the strong source must author them.
6. **Knowledge the base model lacks** — skills help most where pretraining is thin (healthcare +51.9pp) and least where the gap is reasoning, not knowledge (SWE +4.5pp). Prompt transfer moves *procedural knowledge*, not *capability*.

### 5.2 Mitigations (external state + harness gates)
- **External state files / recitation**: Manus's todo.md recitation — rewrite goals+progress at context end every step to bias attention and fight lost-in-the-middle; append-only context for KV-cache stability; **keep errors in context** so the model updates its beliefs ([Manus lessons](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus)).
- **Externalization as architecture**: unified review of memory/skills/protocols/harness engineering ([arXiv 2604.08224](https://arxiv.org/html/2604.08224v1)); memory as execution-state management (open files, hypotheses, partial plans, checkpoints) ([arXiv 2606.06090](https://arxiv.org/pdf/2606.06090)); context folding (AgentFold: 7K tokens after 100 turns vs ~50× ReAct trace).
- **Forced checkpoints + phase gates**: LangGraph checkpointing/state persistence; DeerFlow sandbox FS + context offload ([2026 harness guide](https://www.xugj520.cn/en/archives/ai-agent-engineering-guide-2026.html)). BS-Bench's 97%-compliance result generalizes: **make the discipline a logged, gated artifact, not an instruction**.
- **Randomized/capped evaluation against gaming**: capped eval with randomized tests to detect coding-agent cheating ([arXiv 2606.07379](https://arxiv.org/pdf/2606.07379)).

---

## 6. Synthesis: what a "strong-model behavior pack" for weak coding agents should look like

1. **Format**: progressive disclosure — short root procedure (≤ dozens of always-on rules, highest-stakes first) + on-demand reference files + executable scripts for anything deterministic (Anthropic guidance, SkillJuror +4.1pp, IFScale decay curves).
2. **Content**: concrete if-then procedures + worked examples from the strong model's actual trajectories (ACE Reflector/Curator pattern; SkillsBench "detailed/compact beat comprehensive"); authored/curated by the strong model or humans, never self-generated by the receiver.
3. **Metacognition**: externalized — hypothesis/state files re-recited at context end (Manus), tool-verified checks instead of introspection (self-correction ceiling), confidence tags only as escalation routing.
4. **Porting across vendors**: keep semantic core, re-render per vendor (XML for Claude, hierarchy/markdown for GPT, ordered constraints for Gemini); expect −20–30% if you don't (PromptBridge); re-optimize with a GEPA-style reflective pass per target.
5. **Verification**: bait tasks + tool-log audit metrics (BS-Bench style), blind A/B rubric evals (Skills 2.0), programmatic constraint checks (IFEval/AgentIF stance), 5+ trials, per-domain regression watch.
6. **Enforcement**: the discipline's load-bearing parts go in the harness (gates, audit trails, affordance removal), because prompt-level compliance is 0% under temptation and the compliance gap is environmental, not attitudinal.
