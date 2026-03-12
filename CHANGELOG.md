# Changelog

All notable changes to this project will be documented in this file.

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
