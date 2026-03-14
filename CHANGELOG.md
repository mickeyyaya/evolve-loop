# Changelog

All notable changes to this project will be documented in this file.

## [6.5.0] - 2026-03-14

### Added
- **Self-improvement feedback loops** — three interconnected mechanisms that enable the loop to automatically detect, diagnose, and propose fixes for its own performance issues
- **Process rewards remediation loop** — per-cycle check that auto-generates `pendingImprovements` entries when any reward dimension scores below 0.7 for 2+ consecutive cycles, with dimension-specific suggested tasks
- **Scout introspection pass** — new step in Scout responsibilities that reviews `evalHistory` delta metrics and proposes pipeline self-improvement tasks using 5 concrete heuristics (instinct enrichment, builder guidance, task diversity, task sizing, pending improvements)
- **`processRewardsHistory`** — rolling 3-entry array in state.json enabling trend detection for sustained degradation vs one-off dips
- **`pendingImprovements`** — state.json field for auto-generated remediation tasks, read by Scout as high-priority candidates
- **Capability gap scanner** — extends Scout introspection to detect dormant instincts and overdue deferred tasks, proposing them as `source: "capability-gap"` candidates

### Changed
- Scout responsibilities expanded from 5 to 6 sections (added Introspection Pass before Task Selection)
- Scout task prioritization now includes `pendingImprovements` at priority level 2

## [6.4.0] - 2026-03-14

### Added
- **Skill efficiency guidelines** — new "Efficiency Guidelines" section in `docs/writing-agents.md` with 7 research-backed practices: progressive disclosure, 150-line target, context ordering, cross-agent deduplication, output template compression, relevant-context-only passing, and measurement tracking
- **Skill metrics baseline** — `skillMetrics` field in state.json tracks line counts and estimated tokens for all skill and agent files (1,798 lines / 26,970 tokens baseline)
- **Skill efficiency research** — documented findings from CodeAgents, OPTIMA, CLAUDE.md optimization research with 7 actionable recommendations (R1-R7)
- **Plan cache templates** — 4 reusable task templates (`add-section-to-file`, `docs-update`, `version-bump`, `fix-stale-references`) populated from 42 historical tasks, activating the plan cache mechanism designed in v6.0.0
- **`skillEfficiency` process reward** — new dimension in process rewards rubric tracks prompt token changes relative to baseline (1.0 = decreased, 0.5 = stable, 0.0 = increased)

### Changed
- **Agent prompt deduplication** — Strategy Handling sections in scout, builder, and auditor reduced from full strategy descriptions to 2-line SKILL.md references, saving 18 lines / ~270 tokens across agent files
- Agent file line counts: scout 240→235, builder 152→147, auditor 148→143

## [6.3.0] - 2026-03-14

### Added
- **Token optimization for multi-cycle runs** — 7 optimizations reducing per-cycle token usage by 40-65% at cycle 50+
- **Convergence short-circuit** — skips Scout when `nothingToDoCount >= 2`, runs lightweight confirmation at `== 1` with forced web research
- **Project digest** — `project-digest.md` generated on cycle 1, Scout reads digest instead of full codebase scan on cycle 2+
- **Notes compression** — rolling window caps notes.md at ~5KB with pre-compression memory flush to state.json
- **Ledger summary** — `ledgerSummary` in state.json so agents never read full ledger.jsonl
- **Instinct summary** — `instinctSummary` in state.json replaces reading all instinct YAML files
- **Inline eval graders** — Scout embeds eval criteria in task specs, Builder reads scout-report.md only
- **Pre-computed context** — orchestrator pre-reads growing files, passes inline slices to agents
- **Hotspot analysis** — project digest includes fan-in, size, and churn hotspots for Scout prioritization
- **Context block ordering** — static → semi-stable → dynamic ordering in all agent context blocks

### Changed
- Agent context blocks no longer include `ledgerPath`, `notesPath`, or `instinctsPath` — replaced with inline data
- `evalHistory` in state.json trimmed to last 5 entries (older data captured by `ledgerSummary`)
- Operator supports `convergence-check` mode for when Scout is skipped

### Removed
- **Runtime state from git tracking** — `.claude/evolve/` state files (history, evals, instincts, workspace, ledger, notes, state.json) added to `.gitignore`

## [6.2.0] - 2026-03-13

### Added
- **Example files** — annotated `examples/instinct-example.yaml` and `examples/gene-example.yaml` for contributor onboarding
- **CI skill validation** — CI now verifies all 4 required skill files exist (SKILL.md, phases.md, memory-protocol.md, eval-runner.md)
- **Uninstall CI mode** — `uninstall.sh` supports `--ci` dry-run mode, matching install.sh
- **Examples cross-links** — README, docs, and eval-runner now link to example files

### Fixed
- **Stale templates** — bug report, feature request, and PR templates updated to v6 phase names and agent names
- **Process rewards** — state.json processRewards now contains real scores instead of all zeros
- **Instinct provenance** — added missing cycle-8-instincts.yaml

## [6.1.0] - 2026-03-13

### Added
- **Meta-cycle documentation** — standalone `docs/meta-cycle.md` reference for the meta-cycle review process (trigger, split-role critique, mutation testing, topology review)
- **Process rewards scoring rubric** — deterministic scoring formula in phases.md with 3 score levels (1.0, 0.5, 0.0) per phase dimension
- **Context Management section** — README Key Mechanics now documents the 60% context handoff pattern
- **Global instinct promotion** — phases.md and docs/instincts.md now include concrete promotion steps and criteria
- **Memory consolidation trigger** — explicit `cycle % 3` check step in Phase 5

### Changed
- **Architecture docs** — fully rewritten for v6 (strategy presets, stagnation detection, mastery graduation, gene library, model routing)
- **CI workflow** — added docs validation step for v6 required docs
- **State schema docs** — memory-protocol.md now documents mastery, processRewards, planCache, synthesizedTools
- **Instinct path** — replaced `homunculus` references with `~/.claude/instincts/personal/`

### Fixed
- **Operator model** — README agents table corrected from sonnet to haiku
- **install.sh usage** — updated to include `[strategy]` parameter

## [6.0.0] - 2026-03-13

### Added
- **Stop-hook context reset** — proactive context management at cycle boundaries. Writes handoff.md with session state and resume command when context exceeds 60%. Enables indefinite runtime across sessions.
- **Dynamic model routing** — selects model per phase based on complexity: haiku for routine tasks (Operator, incremental scans), sonnet for standard work, opus for meta-cycle reasoning. Reduces cost while maintaining quality.
- **Plan template caching** — caches successful build plans as reusable templates. Matches new tasks by type and file patterns. ~30-50% cost reduction on similar tasks. Auto-evicts unused templates after 10 cycles.
- **Memory integrity & eval tamper detection** — instinct provenance verification, state.json schema validation, eval checksum tracking, objective hacking detection. Builder cannot modify eval criteria without authorization.
- **Recursive memory consolidation** — every 3 cycles, clusters similar instincts into abstractions, applies temporal decay (0.1/pass), archives stale memories (<0.3 confidence). Entropy gating prevents duplicates.
- **Difficulty-graduated task queue** — curriculum learning with mastery levels: novice (S-only), competent (S+M), proficient (all). Advances on 3+ consecutive 100% success cycles.
- **Split-role critique personas** — three critic perspectives during meta-cycles: efficiency, correctness, novelty. Reduces blind spots in self-assessment.
- **Gene/Capsule library** — structured fix templates with pattern-matching selectors and pre/post validation. Capsules bundle multiple genes into composite workflows. See docs/genes.md.
- **Process rewards** — step-level scoring for each phase (0.0-1.0). Enables targeted agent improvement based on which phase underperforms.
- **Self-generated mutation testing** — during meta-cycles, generates code mutations and verifies evals catch them. Tracks mutation kill rate; <60% triggers eval improvement.
- **Island model evolution** — maintain 3-5 independent configurations evolving in parallel with periodic migration of best traits. See docs/island-model.md.
- **Workflow topology review** — during meta-cycles, evaluates phase ordering for optimization: skipping, merging, addition, parallelization. Proposals require human approval.
- **TextGrad prompt optimization** — generates textual gradients during prompt evolution: observed → desired → specific change → expected impact.
- **Capability gap detection & tool synthesis** — Builder identifies missing capabilities, searches for existing tools, synthesizes reusable scripts in .claude/evolve/tools/.
- **MAP-Elites fitness scoring** — Operator scores cycles across four dimensions: speed, quality, cost, novelty. Recommends strategy changes targeting the weakest dimension.

### Changed
- **Operator default model** — changed from sonnet to haiku (sufficient for routine post-cycle checks)
- **state.json schema** — new fields: `planCache`, `mastery`, `synthesizedTools`
- **Directory structure** — new directories: `instincts/archived/`, `genes/`, `tools/`
- **Meta-cycle expanded** — now includes split-role critique, mutation testing, topology review, and TextGrad optimization

## [5.0.0] - 2026-03-13

### Added
- **Strategy presets** — `innovate`, `harden`, `repair`, `balanced` (default) steer cycle intent without full goal strings. Each agent adapts discovery, building, and auditing behavior based on active strategy.
- **Token budgets** — soft limits per task (80K) and per cycle (200K) to prevent runaway costs. Scout sizes tasks within budget, Operator recommends adjustments if exceeded.
- **Pattern-based stagnation detection** — three detection patterns: same-file churn, same-error repeat, diminishing returns. Replaces simple `nothingToDoCount`. 3+ active patterns trigger Operator HALT.
- **Rich failed-attempt memory** — failed approaches now include root cause reasoning, files affected, and cycle number alongside error and alternative.
- **Meta-cycle self-improvement** — every 5 cycles, the orchestrator evaluates its own pipeline effectiveness (success rates, agent efficiency, stagnation) and may propose changes.
- **Automated prompt evolution** — during meta-cycles, uses critique-synthesize loop to refine agent prompts. Max 2 edits per meta-cycle, auto-reverts on degradation.
- **Multi-type instinct memory** — instincts categorized as episodic (what happened), semantic (domain knowledge), or procedural (how-to) for targeted agent retrieval.
- **Delta evaluation metrics** — each cycle records quantitative metrics (success rate, audit iterations, tasks shipped) enabling trend analysis across cycles.

### Changed
- **state.json schema** — new fields: `strategy`, `tokenBudget`, `stagnation` (replaces flat `nothingToDoCount`)
- **Argument parsing** — now accepts strategy as optional second argument: `/evolve-loop [cycles] [strategy] [goal]`
- **All agent context blocks** — now include `strategy` field
- **evalHistory entries** — now include `delta` object with quantitative metrics
- **Instinct schema** — new `category` field (episodic/semantic/procedural) and new types (domain, technique)

## [4.2.0] - 2026-03-13

### Added
- **Denial-of-wallet guardrails** — `maxCyclesPerSession` (default 10) and `warnAfterCycles` (default 5) prevent runaway sessions; enforced in SKILL.md initialization and phases.md per-cycle check
- **Orchestrator Policies** section in SKILL.md — graduated instincts (inst-004, inst-007) formalized as default pipeline behavior
- **inst-010** instinct — deferred security tasks escalate to CRITICAL after 3 cycles

### Changed
- **Instinct consolidation** — inst-004 and inst-007 consolidated to confidence 0.9 with supersedes metadata
- **state.json** — now includes `maxCyclesPerSession` and `warnAfterCycles` fields

## [4.1.0] - 2026-03-13

### Changed
- **Plugin packaging** — `plugin.json` now declares `agents` array (explicit file paths) and `skills` array for proper plugin system registration
- **Agent frontmatter** — all 4 agents now include `name`, `description`, and `tools` fields required by the plugin system
- **CI workflow** — validates plugin.json schema, marketplace.json, and agent frontmatter fields
- **install.sh** — CI mode validates plugin structure without copying; manual mode shows plugin install as preferred
- **README** — plugin install via `/plugin marketplace add` + `/plugin install` is now primary method

### Removed
- 10 legacy v3 agent files from installed agents (evolve-architect, evolve-deployer, evolve-developer, evolve-e2e, evolve-planner, evolve-pm, evolve-researcher, evolve-reviewer, evolve-scanner, evolve-security)

## [4.0.0] - 2026-03-13

### Added
- **Scout agent** (`evolve-scout.md`) — combines PM, Researcher, Scanner, and Planner into one agent
- **Builder agent** (`evolve-builder.md`) — combines Architect and Developer with self-evolution principles
- **Auditor agent** (`evolve-auditor.md`) — single-pass review covering code quality, security, pipeline integrity, and eval gate
- Multi-task per cycle — 2-4 small/medium tasks built and audited sequentially
- 12hr research cooldown with cached results
- Incremental discovery (cycle 2+ only scans what changed)
- Self-evolution principles: minimal change, reversibility, compound thinking, blast radius assessment

### Changed
- **Pipeline simplified** — 5 phases (down from 8): DISCOVER → BUILD → AUDIT → SHIP → LEARN
- **Operator simplified** — single post-cycle invocation (down from 3)
- **Audit threshold** — MEDIUM+ blocks shipping (was only FAIL)
- **Ship phase** — orchestrator inline (no Deployer agent)
- **Learn phase** — orchestrator inline for instincts (deeper reasoning)
- **Token usage** — ~60-70% reduction per cycle
- **No external dependencies** — removed ECC requirement

### Removed
- **10 agents** — PM, Researcher, Scanner, Planner, Architect, Developer, Reviewer, E2E Runner, Security Reviewer, Deployer
- **ECC dependency** — all agents are now self-contained
- **3 phases** — MONITOR-INIT (pre-flight inline), CHECKPOINT (removed), DESIGN (merged into BUILD)
- `costBudget` field from state.json (simplified)

## [3.1.0] - 2026-03-12

### Changed
- ECC wrapper agents refactored from full content copies to thin context overlays using `subagent_type` delegation
- Net reduction of 607 lines across 5 agent files
- Added Claude Code plugin support (`.claude-plugin/` manifests)

## [3.0.0] - 2026-03-12

### Added
- Loop Operator agent with 3 invocation modes
- E2E Runner and Security Reviewer agents (ECC wrappers)
- Eval Runner with hard gate and retry protocol
- Phase 0 (MONITOR-INIT), Phase 4.5 (CHECKPOINT), Phase 5.5 (EVAL)
- Instinct extraction in Phase 7
- Eval definition output from Planner

### Changed
- Architect, Developer, Reviewer → ECC wrappers
- Phase 5 expanded from 2 to 3 parallel agents
- Phase 7 expanded to LOOP+LEARN

### Removed
- QA agent (replaced by E2E + Security)

## [2.0.0] - 2026-03-12

### Added
- Initial 9-agent pipeline
- 7-phase cycle
- Parallel execution, JSONL ledger, workspace protocol
- Goal-directed and autonomous modes
