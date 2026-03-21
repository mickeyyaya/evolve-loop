# Changelog

All notable changes to this project will be documented in this file.

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
