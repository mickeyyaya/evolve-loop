# Changelog

All notable changes to this project will be documented in this file.

## [3.0.0] - 2026-03-12

### Added
- **Loop Operator agent** (`evolve-operator.md`) — 3 invocation modes: pre-flight, checkpoint, post-cycle
- **E2E Runner agent** (`evolve-e2e.md`) — ECC wrapper for end-to-end testing
- **Security Reviewer agent** (`evolve-security.md`) — ECC wrapper for OWASP security scanning
- **Eval Runner** (`eval-runner.md`) — Phase 5.5 hard gate with code graders, regression evals, acceptance checks
- **Phase 0: MONITOR-INIT** — Loop Operator pre-flight checks
- **Phase 4.5: CHECKPOINT** — Loop Operator mid-cycle assessment
- **Phase 5.5: EVAL** — Hard eval gate with retry protocol (max 3 attempts)
- **Instinct extraction** in Phase 7 — continuous learning from cycle artifacts
- **Instinct reading** in Developer and Planner agents
- **Eval definition output** from Planner (writes to `.claude/evolve/evals/`)
- 3 new anti-patterns: skip eval gate, ignore instincts, ignore HALT
- `costBudget`, `evalHistory`, `instinctCount`, `operatorWarnings` in state.json

### Changed
- **Architect agent** → ECC wrapper (adds ADRs, testing strategy)
- **Developer agent** → ECC wrapper (adds eval-driven TDD, instinct reading)
- **Reviewer agent** → ECC wrapper (adds design compliance check, confidence filtering)
- **Phase 5: VERIFY** expanded from 2 parallel agents to 3 (reviewer + e2e + security)
- **Phase 7: LOOP** expanded to LOOP+LEARN (instinct extraction + operator post-cycle)
- Deployer now checks eval-report.md as primary ship gate
- Memory protocol updated with 4 new workspace files, Layers 4-5

### Removed
- **QA agent** (`evolve-qa.md`) — replaced by evolve-e2e + evolve-security

## [2.0.0] - 2026-03-12

### Added
- Initial 9-agent pipeline (PM, Researcher, Scanner, Planner, Architect, Developer, Reviewer, QA, Deployer)
- 7-phase cycle (DISCOVER → PLAN → DESIGN → BUILD → VERIFY → SHIP → LOOP)
- Parallel execution in DISCOVER (3 agents) and VERIFY (2 agents)
- JSONL ledger + markdown workspace shared memory protocol
- Goal-directed and autonomous discovery modes
- Worktree isolation for BUILD phase
- State.json for cross-cycle persistence
