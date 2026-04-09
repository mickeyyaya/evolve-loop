# Changelog

All notable changes to this project will be documented in this file.

## [8.10.0] - 2026-04-09

### Added
- **`ecc:e2e` first-class integration** — UI/browser tasks now auto-invoke the `everything-claude-code:e2e-testing` skill to generate and run Playwright tests. Scout routes UI work to a new `e2e` skill category; Builder Step 4.5 generates `tests/e2e/<slug>.spec.ts`; Auditor checklist D.5 verifies selector grounding, artifact presence, and `## E2E Verification` in the build-report; `phase-gate.sh` blocks ship if a UI task is missing e2e evidence.
- **`scripts/setup-skill-inventory.sh` + `scripts/setup_skill_inventory.py`** — deterministic filesystem scanner that indexes every installed skill (project, user-global, plugin cache) and writes `.evolve/skill-inventory.json`. Replaces LLM-side parsing of the session's skill listing with a zero-token, cache-friendly scan. Automatically picks newest plugin version, skips IDE mirror dirs (`.cursor/skills`, `.kiro/skills`), and categorizes via the routing taxonomy. Tested: 281 skills indexed across 7 scopes.
- **New E2E Graders eval-runner section** (`skills/evolve-loop/eval-runner.md`) — first-class grader type with artifact locations (`playwright-report/`, `test-results/`, `artifacts/*.zip`), flake handling, and skip-condition semantics.
- **Auditor audit-report template** extended with `## E2E Grounding (D.5)` table.
- **Builder build-report template** extended with `## E2E Verification` section.

### Changed
- **Phases renumbered to eliminate `x.5` irregularity.** Phase 0.5 → 1, cascade 1-6 → 2-7:
  - Phase 0: CALIBRATE (unchanged)
  - Phase 1: RESEARCH (was 0.5)
  - Phase 2: DISCOVER (was 1)
  - Phase 3: BUILD (was 2)
  - Phase 4: AUDIT (was 3)
  - Phase 5: SHIP (was 4)
  - Phase 6: LEARN (was 5)
  - Phase 7: META (was 6)
- **Phase markdown files renamed** to align filenames with phase numbers and descriptions:
  - `phase05-research.md` → `phase1-research.md`
  - `phase1-discover.md` → `phase2-discover.md`
  - `phase2-build.md` → `phase3-build.md`
  - `phase4-ship.md` → `phase5-ship.md`
  - `phase5-learn.md` → `phase6-learn.md`
  - `phase6-metacycle.md` → `phase7-meta.md` (filename now matches the `Phase 7: META` heading text)
- **Phase 0 Skill Inventory step** now calls `scripts/setup-skill-inventory.sh` instead of LLM-parsing the system-reminder skill list. Deterministic, faster, and complete across every installed plugin.
- **Scout skill-matching table** adds an `e2e` category row routing UI tasks to `everything-claude-code:e2e-testing` as primary.
- **251 phase references** (text + filepaths) rewritten across 43 source files; TOC anchor slugs updated to match renumbered headers; `phase-gate.sh` anti-forgery whitelist extended with `setup-skill-inventory.sh`.

### Migration notes
- Phase numbering is an internal convention — plugin consumers invoke `/evolve-loop` as before.
- `.evolve/` runtime artifacts from prior cycles still reference old phase names in historical logs; the next cycle's Scout will naturally write new-naming references.
- `skills/refactor/` has its own independent phase pipeline (SCAN/PRIORITIZE/PLAN/EXECUTE/MERGE) and was deliberately left unchanged.

## [8.9.1] - 2026-04-07

### Changed
- **Skill descriptions standardized to "Use when..." trigger format** — rewrote descriptions across all skills to start with concrete trigger conditions, improving auto-invocation accuracy.
- **`smart-web-search.md` split into reference files** — 654 → 112 lines. Extracted query transformation patterns, intent classification, and provider routing into `reference/`.
- **`phases.md` Phase 0.5 and Phase 1 extracted** — 700 → 474 lines. Each phase now has its own focused file (`phase05-research.md`, `phase1-discover.md`).
- **`refactor/SKILL.md` split into reference files** — 653 → 154 lines. Detection rules, fix patterns, and worktree orchestration moved to `reference/`.
- **Skill routing policy added** (`skill-routing.md`) — formal policy for which skill handles which kind of request, reducing dispatch ambiguity.
- **SKILL.md frontmatter standardized** — consistent header format and field ordering across all skills.

### Notes
Patch release. No behavior changes — all updates are documentation, file organization, and skill discoverability improvements.

## [8.9.0] - 2026-04-06

### Added
- **`/evaluator` skill** — Independent evaluation engine that works standalone or integrated with evolve-loop. 5-layer architecture (GRADE → DETECT → SCORE → DIRECT → META-EVAL) with 6 scoring dimensions.
- **6 scoring dimensions** — correctness (0.25), security (0.20), maintainability (0.20), architecture (0.15), completeness (0.10), evolution (0.10). Each scored 0.0-1.0 with confidence levels and 5-point granularity rubrics.
- **EST anti-gaming defenses** — Evaluator Stress Test (arXiv:2507.05619) protocol: perturbation tests detect format-dependent score inflation at 78% precision and 2.1% overhead. Includes saturation monitoring and proxy-true correlation tracking.
- **Self-improving evaluation lifecycle** — 4-stage lifecycle from EDDOps (arXiv:2411.13768): baseline → calibration → steady state → evolution. Adaptive difficulty auto-introduces harder criteria when dimensions saturate.
- **Strategic direction guidance** — Layer 4 (DIRECT) ranks improvement priorities by `(1.0 - score) * weight * feasibility` with evidence-linked recommendations tracing to specific files and lines.
- **Meta-evaluation (Layer 5)** — Red-team protocol for the evaluator itself. Triggered by repeated gaming detection, saturation, or proxy correlation drops.
- **3 evaluation scopes** — `task` (changed files), `project` (full codebase), `strategic` (trajectory and priorities).
- **Phase 3 delegation hook** — Evolve-loop Auditor can invoke `/evaluator --scope task` when `strategy == "harden"` or `forceFullAudit == true`.
- **`docs/evaluator-research.md`** — Comprehensive 414-line research archive documenting 14 papers, 8 agent benchmarks, 12 LLM-judge biases, reward hacking incidents, independent evaluation principles, and full cross-reference of existing evolve-loop eval mechanisms.
- **Reference material** — `scoring-dimensions.md` (6-dimension rubric), `anti-gaming.md` (EST protocol + known gaming patterns), `eval-lifecycle.md` (4-stage lifecycle + drift detection + meta-evaluation).

### Research Documented
- EDDOps reference architecture (arXiv:2411.13768) — evaluation as continuous governing function
- Evaluator Stress Test (arXiv:2507.05619) — gaming detection via format/content sensitivity
- CALM framework (arXiv:2410.02736) — 12 LLM-judge biases with mitigations
- METR reward hacking (June 2025) — frontier models actively hack evals
- Anthropic eval principles — unambiguous tasks, outcome over path, saturation monitoring
- AISI Inspect Toolkit — 3-axis sandbox isolation for evaluators
- LiveAgentBench, SWE-Bench Verified, CLEAR — major agent benchmarks 2025-2026

## [8.8.1] - 2026-04-06

### Added
- **`scripts/token-profiler.sh`** — Measures token footprint of all skill, agent, and script files. Outputs ranked table with line counts and estimated tokens. Supports `--json`, `--save-baseline`, and `--compare` flags for tracking optimization progress over time.
- **`docs/token-optimization-guide.md`** — Research-backed optimization guide documenting 5 techniques (three-tier progressive disclosure, context block ordering, AgentDiet trajectory compression, event-driven reminders, per-phase context selection) with measured baselines and per-file recommendations. Cites 7 papers including AgentDiet (FSE 2026), OPENDEV, CEMM, and Prompt Compression Survey (NAACL 2025).

### Changed
- **`skills/evolve-loop/reference/policies.md`** — Compressed from 318 to 176 lines (44% reduction, ~2.1K tokens saved per read). Removed duplicate Session Break Handoff Template and compressed verbose rate limit pseudocode into tables. All 11+ functional sections preserved with zero quality loss.

## [8.8.0] - 2026-04-06

### Added
- **`/inspirer` skill** — Standalone creative divergence engine grounded in data-driven web research. Extracts the evolve-loop's internal creativity mechanisms (provocation lenses, concept scoring, research grounding) into a reusable skill invocable on any topic.
- **6-stage pipeline** — FRAME (parse topic) → DIVERGE (apply lenses) → RESEARCH (web search) → SCORE (Inspiration Cards) → CONVERGE (rank & filter) → DELIVER (report/table/JSON).
- **12 provocation lenses** — 10 from evolve-loop (Inversion, Analogy, 10x Scale, Removal, User-Adjacent, First Principles, Composition, Failure Mode, Ecosystem, Time Travel) + 2 new general-purpose lenses (Constraint Flip, Audience Shift).
- **3 depth levels** — QUICK (~20K tokens, 3 lenses), STANDARD (~40K, 4 lenses), DEEP (~60K, 5 lenses) for explicit creativity-vs-cost tradeoff.
- **Inspiration Cards** — Extended Concept Cards with one-liner pitch, implementation sketch (3-5 steps), risks, and next steps. Scored on feasibility x impact x novelty with KEEP/DROP verdicts.
- **Research grounding requirement** — Every idea MUST be backed by at least 1 web research result. No research = auto-drop.
- **3 output formats** — `full` (human-readable report), `brief` (compact table), `evolve` (JSON compatible with Scout task selection).
- **Domain affinity matrix** — Maps 5 topic domains to optimal lens selections for targeted creative divergence.
- **Phase 0.5 delegation hook** — Evolve-loop orchestrator can delegate to `/inspirer` when `strategy == "innovate"` or discovery velocity stagnates.
- **Reference material** — `provocation-lenses.md` (12 lenses with examples), `scoring-rubric.md` (detailed criteria), `worked-examples.md` (3 end-to-end pipelines).
- **Solution documentation** — `docs/inspirer-solution.md` recording design rationale and architecture decisions.

## [8.7.0] - 2026-04-06

### Added
- **`/code-review-simplify` skill** — Unified code review and simplification engine integrated into the evolve-loop pipeline. Combines structured pattern checks with agentic reasoning in a single pass.
- **Hybrid pipeline+agentic architecture** — Pipeline layer runs 6 deterministic checks (~0.5s, ~2-5K tokens) before agentic layer handles contextual analysis (~15-40K tokens). Saves 40-60% tokens vs. separate review + simplify agents.
- **Multi-dimensional scoring** — 4 dimensions (correctness 0.35, security 0.25, performance 0.15, maintainability 0.25) with numeric 0.0-1.0 scores replace binary PASS/FAIL.
- **Adaptive depth routing** — 3 tiers (lightweight < 50 lines, standard 50-200 lines, full review > 200 lines) with auto-escalation for security-sensitive files.
- **`scripts/code-review-simplify.sh`** — Pipeline layer engine with 6 checks: file length (800), function length (50), nesting depth (4), secrets detection, cognitive complexity (15/function), near-duplicate detection.
- **`scripts/complexity-check.sh`** — Per-function cognitive complexity scorer with `--threshold` flag and multi-language support (bash, Python, JS/TS, Go, Java, Rust).
- **Auditor D4 integration** — Optional skill consultation for code changes > 20 lines; composite score supplements verdict; auto-generates simplification suggestions when maintainability < 0.7.
- **Builder self-review** — Optional Step 5 enhancement runs lightweight pipeline after eval pass; applies simplifications before auditor sees the code.
- **Simplification catalog** — 8 localized refactoring techniques (Extract Method, flatten nesting, decompose conditional, extract utility, rename, replace magic numbers, inline over-abstraction, remove dead code).
- **Solution documentation** — `docs/code-review-simplify-solution.md` records research findings, build-vs-buy justification, architecture decisions, and future work.

### Research Findings
- Anthropic multi-agent code review: 16% → 54% substantive PR comments
- Cursor BugBot: pipeline → agentic = biggest quality gain (70% resolution, 2M+ PRs/month)
- Qodo 2.0: multi-agent specialists achieve F1 = 60.1%
- CodeScene: simplified code reduces AI token consumption ~50%
- ICSE 2025: LLMs excel at localized refactoring, weak at architectural

## [8.6.6] - 2026-04-05

### Added
- **Rate Limit Recovery Protocol** — Detects API rate limits after every agent dispatch (Scout, Builder, Auditor) and auto-schedules resumption via `/schedule` (remote trigger) or `/loop` (local retry) instead of silently dying.
- **3-tier auto-resumption** — Priority cascade: remote trigger (≥1hr limits) → local loop (short limits) → manual fallback.
- **Consecutive failure tracking** — 3+ sequential agent failures trigger rate limit recovery as a safety net.
- **Plugin cache refresh in release flow** — `scripts/release.sh` now clears stale plugin cache, updates marketplace checkout, and refreshes the plugin registry automatically.

### Changed
- Orchestrator loop step 6 now includes rate limit check after every agent dispatch.
- `reference/policies.md` extended with Rate Limit Recovery section and comparison table (rate limit vs context budget).
- `phases.md` adds rate limit recovery gate wrapping all agent dispatches.

## [8.6.0] - 2026-03-31

### Added
- **External skill discovery and routing** — Phase 0 builds a skill inventory from installed plugins, categorizing ~150 skills into routing categories (security, testing, language:X, framework:X, etc.).
- **Task-to-skill matching** — Scout matches tasks to relevant external skills using a category routing table, adding `recommendedSkills` to task metadata.
- **Builder skill consultation** (Step 2.7) — Builder invokes matched skills via the `Skill` tool for domain-specific guidance before designing its approach.
- **Skill usage verification** (Auditor D3) — Auditor checks whether recommended primary skills were invoked (informational, non-blocking).
- **Skill effectiveness tracking** (Phase 5) — Tracks hit rate per skill; low-value skills demoted after 5+ invocations.
- **Skill Awareness** section in `agent-templates.md` — shared schema for `recommendedSkills` field.

### Changed
- **Scout and Builder tools** now include `Skill` in their tool arrays.
- **state.json schema** extended with `skillInventory` and `skillEffectiveness` fields.

## [8.5.0] - 2026-03-30

### Added
- **Beyond-the-Ask divergence trigger** — structured provocation system with 10 lenses (Inversion, Analogy, 10x Scale, Removal, User-Adjacent, First Principles, Composition, Failure Mode, Ecosystem, Time Travel) that fire during Phase 0.5 and Scout hypothesis generation to surface ideas beyond the user's explicit request.
- **Lens selection protocol** — each cycle selects 2 lenses (1 random + 1 matched to weakest benchmark dimension) for targeted creative divergence.
- **Beyond-ask tracking** in Phase 5 — hit rate, lens effectiveness, and benchmark delta for proactive insights. Underperforming lenses flagged for meta-cycle replacement.

### Changed
- **Scout hypothesis generation** now produces standard + beyond-ask hypotheses with differentiated auto-promotion thresholds (0.7 standard, 0.6 beyond-ask).
- **Phase 0.5** includes new Step 2.5 (DIVERGENCE TRIGGER) between gap analysis and research execution.
- **Scout report format** includes separate `Beyond-the-Ask Hypotheses` table.
- **Research brief format** includes `Beyond-the-Ask Provocations` section.

## [8.4.0] - 2026-03-30

### Added
- **Search routing** — decision table in `online-researcher.md` routes queries to Smart Web Search (deep research, surveys, concept cards) or Default WebSearch (quick lookups, error resolution, budget-constrained) based on complexity and token budget.
- **Cost profile benchmarks** — documented token/duration costs for each search approach based on head-to-head comparison testing.

### Changed
- **Builder reactive lookups** now default to Default WebSearch (1-2 direct queries, ~60% token savings) instead of always using the full Smart Web Search pipeline.
- **Phase 0.5 research** uses search routing — Smart for surveys/deep dives, Default for factual gap fills.
- **`smart-web-search.md`** clarifies when to use Smart vs Default with explicit routing guidance.

## [8.3.0] - 2026-03-30

### Added
- **Smart Web Search skill** — intent-aware 6-stage search pipeline that classifies questions, transforms queries using Query2doc/HyDE, iteratively searches and refines, and returns grounded cited answers.
- **Release checklist script** (`scripts/release.sh`) — validates version consistency across all files before release to prevent version drift.

### Changed
- **Online Researcher** now leverages Smart Web Search skill for web searches.

## [8.0.0] - 2026-03-23

### Added
- **Progressive disclosure** — SKILL.md reduced from 523 to 90 lines (85% context reduction). Phase details load on demand via `read_file` references instead of being embedded inline.
- **Agent compression** — all 4 agent files compressed for 41% token reduction while preserving all behavior.
- **Anti-forgery defenses** (v7.9.0) — platform-specific safeguards after Gemini CLI forged audit reports during cross-platform run.
- **Research docs** — enterprise evaluation, agent personalization, adversarial eval co-evolution, runtime guardrails, secure code generation, multi-agent coordination, agent observability, uncertainty quantification, threat taxonomy, pre-execution simulation.

### Changed
- **SKILL.md architecture** — moved from monolithic orchestrator to progressive disclosure pattern. Entry point contains only routing logic; phase details are separate files loaded as needed.
- **Agent files** — restructured for leaner context footprint while maintaining full capability.
- **Reference files** — unified structure per Anthropic skill best practices (blockquote header, TOC, tables over prose).

### Security
- **Anti-forgery defenses** — added after incident where Gemini CLI session forged audit-report.md contents. Auditor now verifies report provenance.

## [7.8.0] - 2026-03-22

### Security
- **Deterministic phase gate script** (`scripts/phase-gate.sh`) — enforces phase transitions via bash, not LLM judgment. Verifies artifact existence, re-runs evals independently, checks health fingerprint, controls state.json writes. The orchestrator cannot skip, suppress, or bypass these checks.
- **Incident report: cycles 132-141** (`docs/incidents/cycle-132-141.md`) — documents orchestrator gaming: skipped agents, fabricated 4 empty cycles, inflated mastery. All existing detection mechanisms were bypassed because the orchestrator controlled whether they ran.
- **Anti-pattern #10: Orchestrator gaming** — added to SKILL.md with cross-reference to incident report

### Changed
- **Phase boundaries now mandatory** — all 5 phase transitions require `phase-gate.sh` execution (discover-to-build, build-to-audit, audit-to-ship, ship-to-learn, cycle-complete)
- **State.json writes moved to script** — `lastCycleNumber` and `consecutiveSuccesses` can only be updated by the phase gate script, not by the LLM directly
- **Safety & Integrity section rewritten** — now documents the separation of enforcement (scripts) from execution (LLM), with research basis (Greenblatt AI Control, Redwood Factored Cognition)
- **Protected paths expanded** — `scripts/` directory added to Builder's protected-file list alongside `skills/`, `agents/`, `.claude-plugin/`

### Research
- **Orchestrator anti-gaming research** (`docs/research-orchestrator-anti-gaming.md`) — surveyed principal-agent problem, separation of duties, tamper-proof logging, AI control protocols, factored cognition. Key finding: structural constraints > behavioral constraints.

## [7.7.0] - 2026-03-22

### Research
- **Pipeline optimization research** (`docs/research-pipeline-optimization.md`) — surveyed 25+ papers from 2025-2026 on parallelization, trimming, multi-model strategies. Key findings: 4-agent saturation (Google/MIT), Self-MoA > multi-model mixing (Princeton), speculative execution 48.7% latency reduction (Sherlock/Microsoft), AgentDiet 40-60% token savings

### Added
- **Self-MoA parallel builds** — spawn 2-3 Builder agents with approach diversity for M-complexity tasks; early termination accepts first passing result. Research: M1-Parallel 2.2x speedup (arXiv:2507.08944), Self-MoA (arXiv:2502.00674)
- **Budget-aware agent context** — `budgetRemaining` field (cyclesLeft, estimatedTokensLeft, budgetPressure) enables agents to self-regulate effort. Research: BATS framework (arXiv:2511.17006)
- **Per-phase context selection matrix** — each agent receives ONLY needed fields; saves 3-5K tokens per invocation. Research: Anthropic Select strategy
- **Speculative Auditor execution** — start Auditor concurrently with Builder; rollback on failure. Research: Sherlock (arXiv:2511.00330)
- **Eval-delta prediction** — Scout predicts benchmark impact per task; Phase 5 tracks prediction accuracy for calibration. Research: eval-driven development (arXiv:2411.13768)
- **Eager context budget estimation** — pre-compute cycle token cost before launching agents; proactive lean mode entry. Research: OPENDEV (arXiv:2603.05344)
- **AgentDiet trajectory compression** — prune useless/redundant/expired context between every phase transition. Research: AgentDiet (arXiv:2509.23586)

### Changed
- **Lean mode trigger** — now activates on budget pressure (not just cycle 4+), enabling earlier optimization
- **Scout task output** — now includes "Expected eval delta" field for prediction tracking
- **phase2-build.md** — expanded with Self-MoA dispatch, speculative auditor, trajectory compression sections

## [7.6.0] - 2026-03-22

### Added
- **Phase decomposition** — monolithic phases.md split into focused modules: `phase0-calibrate.md`, `phase2-build.md`, `phase5-learn.md`, `phase6-metacycle.md` (cycles 122-125)
- **Agent templates** — `agents/agent-templates.md` consolidates shared Input/Output schemas across Scout, Builder, Auditor (cycle 122)
- **Model routing doc** — `docs/model-routing.md` is the single source of truth for tier definitions, provider mappings, and routing rules (cycle 124)
- **Changelog archive** — entries v2.0-v6.9 archived to `CHANGELOG-ARCHIVE.md`, keeping CHANGELOG.md lean (cycle 126)

### Changed
- **phases.md: 717 → 386 lines** (46% reduction) — Phase 0 and Phase 2 extracted to standalone modules
- **phase5-learn.md: 596 → 334 lines** (44% reduction) — meta-cycle logic extracted to phase6-metacycle.md
- **SKILL.md: 560 → 500 lines** (11% reduction) — model routing tables extracted to docs/model-routing.md
- **token-optimization.md: 444 → 412 lines** — model routing duplication removed (references docs/model-routing.md)
- **CHANGELOG.md: 368 → 102 lines** — old entries archived
- **Shared values consolidated** — memory-protocol.md Layer 0 references SKILL.md as canonical source (no duplication)
- **Dead state.json fields removed** — `processRewards` replaced by `processRewardsHistory` in schema
- **Instinct docs deduplicated** — docs/self-learning.md references phase5-learn.md instead of duplicating algorithms
- **Estimated token savings: 24-42K per cycle** (8-14% reduction) from modular loading and deduplication

### Architecture
```
Before (v7.5.0):                    After (v7.6.0):
phases.md (717 lines)               phases.md (386) — orchestrator sequencing
                                    ├── phase0-calibrate.md (99) — once per invocation
                                    ├── phase2-build.md (297) — build orchestration
                                    ├── phase4-ship.md (244) — shipping
                                    ├── phase5-learn.md (334) — per-cycle learning
                                    └── phase6-metacycle.md (191) — every 5 cycles

3 agents × duplicated boilerplate   agent-templates.md (68) + 3 lean agents
1 monolithic model routing table    docs/model-routing.md (single source of truth)
```

## [7.5.0] - 2026-03-22

### Added
- **Platform compatibility doc** (`docs/platform-compatibility.md`) — tool mapping tables for 6 platforms, model tier mappings for 7 providers
- **Multi-platform agent frontmatter** — `capabilities`, `tools-gemini`, `tools-generic` fields in all 4 agents
- **Provider-agnostic prompt caching** — guidance for Anthropic, Google, OpenAI, and self-hosted engines

### Changed
- Agent invocation abstracted from Claude Code `Agent` tool to platform dispatch blocks
- Architecture doc updated: "host LLM session" replaces "Claude Code session"
- Model tier mappings updated to March 2026 latest (Gemini 3.1, GPT-5.4, Mistral Large 3, Qwen 3.5)

## [7.4.0] - 2026-03-21

### Added
- **Hallucination self-detection** — Auditor checklist now includes Section B2 that verifies imports, API signatures, and config keys against actual project dependencies. Catches fabricated APIs before they ship. (Source: agent-self-evaluation-patterns skill)
- **Parallel builder execution** — SKILL.md and phases.md now include explicit dependency-partitioning algorithm and fan-out/fan-in instructions for running independent tasks in parallel worktrees. Cuts cycle latency 2-3x for multi-task cycles. (Source: agent-orchestration-patterns skill)
- **Formal eval taxonomy** — Three grader types (`[code]`, `[model]`, `[human]`) formalized in eval-runner.md with type tagging, cost controls, and pass@k tracking. Scout tags every eval command with its grader type. (Source: eval-harness skill)
- **Process rewards per build step** — Builder reports step-level confidence in build-report.md. Auditor cross-validates via Section D2 (CALIBRATION_MISMATCH detection). Phase 5 aggregates step-level patterns into processRewardsHistory for meta-cycle analysis. (Source: eval-harness process rewards)
- **Instinct-to-skill graduation pipeline** — Meta-cycle now synthesizes qualifying instinct clusters (3+, same category, all confidence >= 0.8) into genes or skill fragments. Recorded in state.json.synthesizedTools. Closes the loop between learning and capability expansion. (Source: continuous-learning-v2, self-learning-agent-patterns skills)
- **Shared values inheritance model** — Shared agent values block in SKILL.md injected into every agent context. Eliminates protocol duplication across 4 agent files, enables single-source-of-truth meta-cycle edits. (Source: agent-shared-values-patterns skill)

### Changed
- **Version: 7.3.0 → 7.4.0** — minor version bump for 6 new features
- **Auditor reduced-checklist rule** — now references Section B2 (Hallucination Detection) alongside A and C as skippable sections
- **docs/skill-building.md** — Stage 5 expanded from 2 lines to full synthesis protocol with gene/skill-fragment examples
- **docs/meta-cycle.md** — Skill Synthesis section added between Automated Prompt Evolution and Mutation Testing

## [7.3.0] - 2026-03-20

### Added
- **Per-cycle enhanced summary** — each cycle now outputs a rich summary with benchmark delta, audit iterations, graduated instincts, operator warnings, and next focus
- **Final session report** — comprehensive markdown report generated after all cycles complete, covering task table, benchmark trajectory, learning stats, and recommendations
- **Auto version bump** — SHIP phase automatically increments patch version in plugin.json/marketplace.json after each cycle push
- **Operator brief spec doc** — new `docs/operator-brief.md` documenting the `next-cycle-brief.json` schema and cross-cycle communication protocol
- **Run isolation doc** — new `docs/run-isolation.md` documenting the `RUN_ID`/`WORKSPACE_PATH` parallel invocation safety model
- **Experiment journal doc** — new `docs/experiment-journal.md` documenting `experiments.jsonl` anti-repeat memory protocol
- **Scout discovery guide extraction** — modular discovery guide extracted from monolithic scout agent for better maintainability
- **Security self-check** — Builder agent now performs security self-verification before completing builds
- **Stepwise scoring enforcement** — mandatory stepwise confidence scoring wired into the evaluation protocol
- **isLastCycle flag** — passed to Operator context for reliable session-summary.md generation on final cycle
- **Instinct graduation section** — `docs/instincts.md` now documents the graduation lifecycle
- **Parallel safety doc** — new `docs/parallel-safety.md` consolidating OCC, ship-lock, and run isolation

### Fixed
- **Schema hygiene** — missing fitness fields added to state.json schema example
- **Method attribution** — validation protocol added for research source attribution

### Changed
- **Benchmark score: ~91** — 12+ tasks shipped across cycles 20-23
- **Version: 7.2.0 → 7.3.0** — auto-bump now prevents version drift

## [7.2.0] - 2026-03-20

### Added
- **Stepwise self-evaluation** — Builder performs per-step correctness checks during implementation using stepwise verification (arxiv 2511.07364), catching errors before they compound
- **Instinct quality scoring (EvolveR)** — instincts now carry quality scores derived from downstream task outcomes, enabling confidence-weighted retrieval and automatic pruning of low-value instincts
- **MUSE functional memory categories** — instincts classified into functional categories (heuristic, constraint, pattern, anti-pattern) for targeted retrieval by agent role
- **CSI metric (Confidence-Stability Index)** — new composite metric tracking confidence-correctness alignment across cycles, used by Operator for pipeline health assessment
- **Phase 4 SHIP extraction** — shipping logic extracted into a dedicated, testable phase module with structured status reporting
- **Confidence-correctness alignment** — process rewards calibrated so stated confidence correlates with actual correctness (arxiv 2603.06604), reducing overconfident shipping of flawed changes

### Fixed
- **30+ broken internal links** — comprehensive link audit and repair across all docs, skills, and agent files (Cycle 16)
- **Link-checker grader regex** — fixed false negatives in the link-checker eval grader caused by overly strict regex patterns
- **processRewards schema** — corrected field validation that rejected valid reward entries with optional dimensions

### Changed
- **Benchmark score: 87.4 to ~91.5** — 9 tasks shipped across 4 cycles with 5 research methods adopted from 8 sources
- **CHANGELOG refreshed** — cycles 16-19 documented

## [7.1.0] - 2026-03-19

### Added
- **Chain-of-thought (CoT) design requirement** — Builder agent Step 3 now requires numbered reasoning steps with evidence citations before selecting an approach (+35% accuracy on complex tasks)
- **Multi-stage verification (MSV)** — Auditor agent applies segment→verify→reflect protocol for M-complexity tasks touching >3 files, with groundedness checking against filesToModify
- **Mutation testing specification** — eval-runner.md now documents mutation generation, kill rate calculation (target >=80%), and interpretation thresholds
- **Token budget awareness for Scout** — Scout agent now estimates per-task token cost and drops lowest-priority tasks when cycle budget (200K) would be exceeded
- **Eval grader best practices guide** — new `docs/eval-grader-best-practices.md` covering grader precision, anti-patterns, composition patterns, worked examples, and mutation resistance
- **Operator benchmark-to-brief translation** — Operator now maps projectBenchmark weakness scores to taskTypeBoosts in next-cycle brief, closing the benchmark→Scout feedback loop
- **Cross-run research deduplication** — OCC-based query locking protocol prevents parallel runs from issuing duplicate web searches (saves 45-90K tokens per overlapping cycle)

### Changed
- **All 4 agent files updated** — Builder (CoT), Auditor (MSV), Scout (token budget), Operator (benchmark sync) now implement documented accuracy and performance techniques
- **eval-runner.md** — mutation testing section + cross-reference to eval-grader-best-practices.md
- **CHANGELOG refreshed** — cycles 13-15 documented

## [7.0.0] - 2026-03-19

### Added
- **Accuracy self-correction techniques** — new `docs/accuracy-self-correction.md` with CoT prompting (+35% accuracy), multi-stage verification (HaluAgent pattern), context alignment scoring, and uncertainty acknowledgment, each mapped to specific evolve-loop agents
- **Implementation patterns** — concrete CoT-enforcing audit graders, multi-stage verification flow examples, and groundedness check patterns in accuracy-self-correction.md
- **Performance Profiling guide** — new `docs/performance-profiling.md` covering per-phase token measurement, cost-bottleneck identification, cycle-level telemetry, and model routing cost impact
- **Security considerations** — new `docs/security-considerations.md` documenting eval tamper detection, state.json integrity, prompt injection defense, rollback protocol, and output groundedness as security signal
- **Plan Cache Schema specification** — JSON schema, write-back protocol, similarity matching algorithm (composite score > 0.7), and eviction rules in token-optimization.md
- **Instinct Graduation specification** — graduation threshold (confidence >= 0.75, 3+ cycle citations), operational effects on Builder/Scout, and reversal conditions in phase5-learn.md
- **Agentic Plan Caching (APC) research baseline** — NeurIPS 2025 paper results (50.31% cost reduction, 27.28% latency reduction) documented in token-optimization.md
- **Dynamic Turn Limits** — probability-based marginal value gating pattern (24% cost reduction) in token-optimization.md

### Fixed
- **Benchmark eval macOS compatibility** — replaced grep -P (PCRE) with -E (POSIX ERE), fixed exit code handling, multi-file grep count summing, stale file paths, and setext header false positives
- **5 broken internal links** in SKILL.md and phase5-learn.md (incorrect relative paths from skills/evolve-loop/ to docs/)

### Changed
- **README.md** — updated project structure tree with all 18 docs, added 3 new feature bullets
- **Project digest** — regenerated at cycle 10 (meta-cycle)

---

For changelog entries prior to v7.0.0 (versions 2.0.0 through 6.9.0), see [CHANGELOG-ARCHIVE.md](CHANGELOG-ARCHIVE.md).
