# Evolve Loop

A self-evolving development pipeline for [Claude Code](https://docs.anthropic.com/en/docs/claude-code). Orchestrates 13 specialized AI agents across 8 phases to autonomously discover, plan, design, build, verify, and ship improvements to any codebase.

Built on top of [Everything Claude Code](https://github.com/anthropics/everything-claude-code) (ECC) battle-tested agents for architecture, TDD, code review, E2E testing, security review, and loop operation.

## Features

- **13 specialized agents** — each with a focused role and clear workspace ownership
- **8-phase pipeline** — MONITOR → DISCOVER → PLAN → DESIGN → BUILD → CHECKPOINT → VERIFY → EVAL → SHIP → LEARN
- **Parallel execution** — 3 agents run simultaneously in DISCOVER and VERIFY phases
- **Eval hard gate** — code graders, regression evals, and acceptance checks must pass before deploy
- **Continuous learning** — instinct extraction after each cycle, promoting patterns with high confidence
- **Loop monitoring** — operator agent with pre-flight checks, mid-cycle checkpoints, and post-cycle assessment
- **Goal-directed or autonomous** — focus on a specific goal or let agents discover improvements

## Quick Start

### Prerequisites

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI installed
- A git repository to evolve

### Installation

```bash
# Clone this repo
git clone https://github.com/danleemh/evolve-loop.git

# Run the installer
cd evolve-loop
./install.sh
```

This copies agents to `~/.claude/agents/` and the skill to `~/.claude/skills/evolve-loop/`.

### Usage

```bash
# In any git project directory:

# Autonomous mode — 2 cycles, agents discover what to improve
/evolve-loop

# Goal-directed — 1 cycle focused on a specific feature
/evolve-loop 1 add dark mode support

# Multiple autonomous cycles
/evolve-loop 5

# Goal-directed with default 2 cycles
/evolve-loop add user authentication
```

## Architecture

```
Phase 0:   MONITOR-INIT ── sequential ──── [Loop Operator] pre-flight
Phase 1:   DISCOVER ────── 3 PARALLEL ──── [PM] [Researcher] [Scanner]
Phase 2:   PLAN ────────── sequential ──── [Planner] + user gate + eval defs
Phase 3:   DESIGN ─────── sequential ──── [Architect (ECC)]
Phase 4:   BUILD ────────── sequential ──── [Developer (ECC tdd-guide)] (worktree)
Phase 4.5: CHECKPOINT ──── sequential ──── [Loop Operator] mid-cycle
Phase 5:   VERIFY ─────── 3 PARALLEL ──── [Code-Reviewer] [E2E-Runner] [Security-Reviewer]
Phase 5.5: EVAL ────────── sequential ──── [Eval Harness] HARD GATE
Phase 6:   SHIP ────────── sequential ──── [Deployer] (only if eval PASS)
Phase 7:   LOOP+LEARN ──── sequential ──── archive + instinct extraction + operator post-cycle
```

### Data Flow

```
                    PHASE 0: Loop Operator → loop-operator-log.md (pre-flight)
                              |
                    PHASE 1: DISCOVER (3 parallel)
PM ─────────→ briefing.md ──────────┐
Researcher ──→ research-report.md ──┤
Scanner ─────→ scan-report.md ──────┘
                                     |
                    PHASE 2: PLAN    v
                    Planner → backlog.md + evals/<task>.md
                                     |
                    PHASE 3: DESIGN  v
                    Architect → design.md
                                     |
                    PHASE 4: BUILD   v
                    Developer → impl-notes.md
                                     |
                    PHASE 4.5: Loop Operator → checkpoint
                                     |
                    PHASE 5: VERIFY (3 parallel)
Reviewer ────→ review-report.md ─────────┐
E2E Runner ──→ e2e-report.md ────────────┤
Security ────→ security-report.md ───────┘
                                          |
                    PHASE 5.5: EVAL       v
                    Eval Runner → eval-report.md [HARD GATE]
                                          |
                    PHASE 6: SHIP         v (only if PASS)
                    Deployer → deploy-log.md
                                          |
                    PHASE 7: LOOP+LEARN   v
                    Archive + Instincts + Operator post-cycle
```

## Agents

| Role | File | Source | Model | Workspace File |
|------|------|--------|-------|----------------|
| Operator | `evolve-operator.md` | ECC wrapper | sonnet | `loop-operator-log.md` |
| PM | `evolve-pm.md` | Custom | sonnet | `briefing.md` |
| Researcher | `evolve-researcher.md` | Custom | sonnet | `research-report.md` |
| Scanner | `evolve-scanner.md` | Custom | sonnet | `scan-report.md` |
| Planner | `evolve-planner.md` | Custom | opus | `backlog.md` + `evals/*.md` |
| Architect | `evolve-architect.md` | ECC wrapper | opus | `design.md` |
| Developer | `evolve-developer.md` | ECC wrapper | sonnet | `impl-notes.md` |
| Reviewer | `evolve-reviewer.md` | ECC wrapper | sonnet | `review-report.md` |
| E2E Runner | `evolve-e2e.md` | ECC wrapper | sonnet | `e2e-report.md` |
| Security | `evolve-security.md` | ECC wrapper | sonnet | `security-report.md` |
| Eval Runner | (orchestrator) | eval-runner.md | — | `eval-report.md` |
| Deployer | `evolve-deployer.md` | Custom | sonnet | `deploy-log.md` |

**ECC wrapper agents** contain the full [Everything Claude Code](https://github.com/anthropics/everything-claude-code) agent content plus an `## Evolve Loop Integration` section for workspace ownership, ledger format, and context inputs. Self-contained — no symlinks.

## Key Mechanics

### Eval Hard Gate (Phase 5.5)

The eval gate is the primary quality bar. It runs code-based graders, regression evals, and acceptance checks defined by the Planner in Phase 2.

- All checks must pass (pass@1 = 1.0)
- If FAIL → re-launch Developer → re-run VERIFY + EVAL (max 3 attempts)
- If still FAIL after 3 → log as failed approach, skip deploy

### Continuous Learning (Phase 7)

After each cycle, the orchestrator extracts instincts — patterns observed during the cycle:

- Successful patterns (approaches, tools, libraries that worked)
- Failed patterns (anti-patterns, pitfalls to avoid)
- Repeated workflows worth encoding
- Domain knowledge specific to the project

Instincts start at confidence 0.5 and increase with repetition. After 5+ cycles, high-confidence instincts (>= 0.8) promote to global scope.

### Loop Operator (3 checkpoints)

- **Pre-flight (Phase 0):** Verify quality gates, eval baseline, rollback path, cost budget
- **Checkpoint (Phase 4.5):** Check timing, detect stalls, flag cost drift
- **Post-cycle (Phase 7):** Progress assessment, stall detection, recommendations
- Returns `HALT` if issues found — orchestrator must pause and present to user

## Project Structure

```
evolve-loop/
├── agents/                      # 11 agent definition files
│   ├── evolve-operator.md       # Loop monitoring (ECC wrapper)
│   ├── evolve-pm.md             # Project manager (custom)
│   ├── evolve-researcher.md     # External intelligence (custom)
│   ├── evolve-scanner.md        # Code scanner (custom)
│   ├── evolve-planner.md        # Task selection + eval defs (custom)
│   ├── evolve-architect.md      # System design (ECC wrapper)
│   ├── evolve-developer.md      # TDD implementation (ECC wrapper)
│   ├── evolve-reviewer.md       # Code review (ECC wrapper)
│   ├── evolve-e2e.md            # E2E testing (ECC wrapper)
│   ├── evolve-security.md       # Security review (ECC wrapper)
│   └── evolve-deployer.md       # Ship + CI (custom)
├── skills/
│   └── evolve-loop/
│       ├── SKILL.md             # Entry point + orchestrator overview
│       ├── phases.md            # Phase-by-phase instructions
│       ├── memory-protocol.md   # Workspace, ledger, state schema
│       └── eval-runner.md       # Eval hard gate instructions
├── docs/
│   ├── architecture.md          # Detailed architecture docs
│   ├── configuration.md         # Configuration reference
│   └── writing-agents.md        # Guide for creating custom agents
├── examples/
│   └── eval-definition.md       # Example eval definition
├── install.sh                   # Installation script
├── uninstall.sh                 # Uninstallation script
├── README.md
├── CONTRIBUTING.md
├── LICENSE
└── CHANGELOG.md
```

## Workspace Layout (per project)

When evolve-loop runs, it creates this structure in the target project:

```
.claude/evolve/
├── workspace/           # Current cycle workspace (overwritten each cycle)
│   ├── loop-operator-log.md
│   ├── briefing.md
│   ├── research-report.md
│   ├── scan-report.md
│   ├── backlog.md
│   ├── design.md
│   ├── impl-notes.md
│   ├── review-report.md
│   ├── e2e-report.md
│   ├── security-report.md
│   ├── eval-report.md
│   └── deploy-log.md
├── evals/               # Eval definitions (created by Planner)
├── instincts/
│   └── personal/        # Extracted patterns from cycles
├── history/
│   └── cycle-N/         # Archived workspace per cycle
├── state.json           # Persistent cycle state
├── ledger.jsonl         # Append-only structured log
└── notes.md             # Cross-cycle context (append-only)
```

## Requirements

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI
- Git (for worktree isolation)
- A project with test infrastructure (npm test, pytest, go test, etc.)

## Configuration

### Cost Budget

Set a per-cycle cost budget in `state.json`:

```json
{
  "costBudget": 5.00
}
```

The Loop Operator will flag cost drift exceeding 120% of average cycle cost.

### Goal Modes

| Mode | Usage | Behavior |
|------|-------|----------|
| Autonomous | `/evolve-loop` | Broad discovery across all dimensions |
| Directed | `/evolve-loop add auth` | All agents focus on the specified goal |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on adding agents, modifying phases, or improving the eval system.

## License

[MIT](LICENSE)

## Acknowledgments

- [Everything Claude Code](https://github.com/anthropics/everything-claude-code) — battle-tested agents used as wrappers
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — the AI coding assistant powering the agents
