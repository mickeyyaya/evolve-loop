# Changelog

All notable changes to this project will be documented in this file.

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
