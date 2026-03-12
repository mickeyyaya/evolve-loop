# Evolve Loop

A self-evolving development pipeline for [Claude Code](https://docs.anthropic.com/en/docs/claude-code). Orchestrates 4 specialized AI agents across 5 lean phases to autonomously discover, build, audit, and ship improvements to any codebase.

Optimized for fast iteration — diverse small/medium tasks per cycle, worktree isolation, 12hr research cooldown, and single-pass auditing.

## Features

- **4 specialized agents** — Scout, Builder, Auditor, Operator
- **5 lean phases** — DISCOVER → BUILD → AUDIT → SHIP → LEARN
- **Multi-task per cycle** — 2-4 small tasks built and audited sequentially
- **Worktree isolation** — Builder works in isolated git worktrees
- **Eval hard gate** — Auditor runs code graders and acceptance checks before shipping
- **Continuous learning** — instinct extraction after each cycle with deep reasoning
- **Loop monitoring** — Operator detects stalls, quality degradation, and repeated failures
- **No external dependencies** — fully self-contained Claude Code plugin

## Quick Start

### Prerequisites

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI installed
- A git repository to evolve

### Installation

**Option A: As a Claude Code plugin (recommended)**

In Claude Code, run:
```
/plugin marketplace add mickeyyaya/evolve-loop
/plugin install evolve-loop@evolve-loop
```

The skill and agents load automatically.

**Option B: Manual install**

```bash
git clone https://github.com/mickeyyaya/evolve-loop.git
cd evolve-loop
./install.sh
```

### Usage

```bash
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
Phase 1:   DISCOVER ─── [Scout]     scan + research + task selection
Phase 2:   BUILD ────── [Builder]   design + implement + self-test (worktree)
Phase 3:   AUDIT ────── [Auditor]   review + security + eval gate
Phase 4:   SHIP ──────── orchestrator   commit + push
Phase 5:   LEARN ──────── orchestrator + [Operator]   instincts + health check
```

For multiple tasks per cycle, Phase 2-3 loop:
```
Scout → [Task A, Task B, Task C]
  → Builder(A) → Auditor(A) → commit
  → Builder(B) → Auditor(B) → commit
  → Builder(C) → Auditor(C) → commit
→ Ship → Learn
```

### Data Flow

```
Phase 1: Scout ──→ scout-report.md + evals/<task>.md
              |
Phase 2: Builder ──→ build-report.md  (per task, in worktree)
              |
Phase 3: Auditor ──→ audit-report.md  [GATE: MEDIUM+ blocks]
              |
Phase 4: Orchestrator ── git commit + push
              |
Phase 5: Orchestrator ── instincts + archive
         Operator ──→ operator-log.md
```

## Agents

| Role | File | Model | Purpose |
|------|------|-------|---------|
| Scout | `evolve-scout.md` | sonnet | Discovery + analysis + task selection |
| Builder | `evolve-builder.md` | sonnet | Design + implement + self-test |
| Auditor | `evolve-auditor.md` | sonnet | Review + security + eval gate |
| Operator | `evolve-operator.md` | sonnet | Loop health monitoring |

## Key Mechanics

### Scout (Phase 1)
- **Cycle 1:** Full codebase scan + optional web research
- **Cycle 2+:** Incremental scan (only what changed) + research cooldown (12hr)
- Outputs 2-4 small/medium tasks with eval definitions
- Reads instincts to avoid repeating mistakes

### Builder (Phase 2)
- Designs and implements in a single pass (no architect → developer handoff)
- Works in isolated worktree
- Self-verifies against eval definitions before declaring done
- Max 3 attempts per task, then logs failure and moves on

### Auditor (Phase 3)
- Single-pass review: code quality + security + pipeline integrity + eval checks
- Blocks on MEDIUM+ severity findings
- Assesses blast radius, reversibility, and convergence
- WARN or FAIL triggers Builder retry (max 3 iterations)

### Instinct Extraction (Phase 5)
- Deep reasoning about what worked, what failed, and why
- Specific actionable patterns, not generic advice
- Confidence scoring: starts at 0.5, increases with confirmation
- After 5+ cycles, high-confidence instincts promote to global scope

### Operator (Phase 5)
- Post-cycle health assessment
- Stall detection (2+ consecutive no-ship cycles)
- Quality trend tracking
- HALT protocol: pauses loop for human attention

## Project Structure

```
evolve-loop/
├── .claude-plugin/
│   ├── plugin.json             # Plugin manifest (agents, skills, metadata)
│   └── marketplace.json        # Marketplace distribution config
├── agents/                     # 4 agent definition files
│   ├── evolve-scout.md        # Discovery + task selection
│   ├── evolve-builder.md      # Design + implement
│   ├── evolve-auditor.md      # Review + security + eval
│   └── evolve-operator.md     # Loop monitoring
├── skills/
│   └── evolve-loop/
│       ├── SKILL.md           # Entry point + orchestrator
│       ├── phases.md          # Phase-by-phase instructions
│       ├── memory-protocol.md # Workspace, ledger, state schema
│       └── eval-runner.md     # Eval gate instructions
├── docs/
│   ├── architecture.md        # Detailed architecture docs
│   ├── configuration.md       # Configuration reference
│   └── writing-agents.md      # Guide for creating agents
├── install.sh                 # Installation script
├── uninstall.sh               # Uninstallation script
├── README.md
├── CONTRIBUTING.md
├── LICENSE
└── CHANGELOG.md
```

## Workspace Layout (per project)

```
.claude/evolve/
├── workspace/           # Current cycle (overwritten each cycle)
│   ├── scout-report.md
│   ├── build-report.md
│   ├── audit-report.md
│   └── operator-log.md
├── evals/               # Eval definitions (created by Scout)
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

## License

[MIT](LICENSE)
