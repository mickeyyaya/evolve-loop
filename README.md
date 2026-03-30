# Evolve Loop

**A self-improving development pipeline that makes your codebase better while you sleep.**

Evolve Loop is an open-source plugin for AI coding assistants (Claude Code, Gemini CLI) that runs autonomous improvement cycles on your codebase. Each cycle, it scans your project, picks tasks, implements changes, reviews its own work, and ships — then learns from the experience to do better next time.

Think of it as a tireless junior developer that gets smarter with every cycle.

---

## How It Works

Each cycle runs through 6 phases — and each cycle feeds the next:

```
DISCOVER ──→ BUILD ──→ AUDIT ──→ SHIP ──→ LEARN ──→ next cycle
   │            │         │        │         │
   │            │         │        │         └─ Extract instincts + proposals
   │            │         │        └─ Commit and push
   │            │         └─ Code review + eval gate (blocks bad code)
   │            └─ Implement in worktree + surface discoveries
   └─ Hypothesize + select tasks (including prior proposals)
```

Four specialized AI agents handle the work:

| Agent | Job | What It Does |
|-------|-----|--------------|
| **Scout** | Find work | Scans your codebase, reads past learnings, picks the most valuable tasks |
| **Builder** | Do the work | Designs and implements changes in an isolated branch |
| **Auditor** | Check the work | Reviews code quality, security, and correctness. Blocks bad changes. |
| **Operator** | Watch the loop | Monitors health, detects stalls, tracks quality trends |

## Quick Start

### Prerequisites

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Gemini CLI](https://github.com/google-gemini/gemini-cli) installed
- A git repository you want to improve

### Install

**Option A: Plugin (recommended)**

```bash
# In your AI CLI
/plugin marketplace add mickeyyaya/evolve-loop
/plugin install evolve-loop@evolve-loop
```

**Option B: Manual**

```bash
git clone https://github.com/mickeyyaya/evolve-loop.git
cd evolve-loop
./install.sh
```

### Run

```bash
# Run 2 cycles with balanced strategy (default)
/evolve-loop

# Run 1 cycle focused on a specific goal
/evolve-loop 1 add dark mode support

# Run 5 cycles
/evolve-loop 5

# Use a strategy preset
/evolve-loop innovate          # prioritize new features
/evolve-loop harden            # prioritize stability and tests
/evolve-loop repair fix auth   # fix-only mode with a specific target
```

### Upgrade

```bash
/plugin marketplace update evolve-loop
/plugin update evolve-loop@evolve-loop
/plugin reload
```

## What Makes It Different

**It discovers things you didn't ask for.** While building, agents surface latent bugs, inconsistencies, and opportunities in your codebase. These "unsolicited insights" appear in your session report as "Things Found Beyond Your Goal."

**It proposes its own next steps.** Each cycle generates hypotheses and discoveries that feed the next cycle as task candidates. The loop continues as long as there's something new to learn — not just something to do.

**It learns from itself.** After every cycle, the pipeline extracts "instincts" — reusable patterns about what worked and what didn't. These feed back into future cycles, so the same mistakes don't repeat.

**It guards its own quality.** The Auditor agent blocks any change rated MEDIUM severity or higher. Bad code doesn't ship.

**It runs in isolation.** The Builder works in a separate git worktree, so your working directory is never touched. If a build fails, nothing is affected.

**It improves its own process.** Every 5 cycles, a meta-cycle evaluates the pipeline itself — adjusting agent prompts, token budgets, and strategies based on measured performance.

**No external dependencies.** No npm packages, no Python libraries, no Docker. It's pure markdown instructions that AI agents follow. Works anywhere git works.

## Evolution Data

Evolve Loop has been running on its own codebase since March 12, 2026. Here's how it evolved:

### Growth Over Time

| Metric | Start (v3.0) | Current (v8.3) |
|--------|-------------|-----------------|
| Agents | 11 (bloated) | 3 (lean) + inline Operator |
| Phases | 3 | 6 (+ meta-cycle every 5) |
| Cycles completed | 0 | 170+ |
| Tasks shipped | 0 | 115+ |
| Commits | 1 | 300+ |
| Benchmark score | N/A | 89.9 / 100 |
| Consecutive successes | 0 | 62 |
| Mastery level | N/A | Proficient |

### Version History

| Version | Date | Key Changes |
|---------|------|-------------|
| v3.0 | Mar 12 | Initial multi-agent pipeline (11 agents, 3 phases) |
| v4.0 | Mar 13 | Consolidated to 4 lean agents, added strategy presets |
| v5.0 | Mar 14 | Added eval gating, instinct extraction, curriculum learning |
| v6.0 | Mar 17 | Added gene library, mutation testing, island model |
| v7.0 | Mar 19 | Added accuracy self-correction, performance profiling, security pipeline |
| v7.2 | Mar 20 | Added stepwise self-evaluation, functional memory categories |
| v7.4 | Mar 21 | Added hallucination detection, parallel builds, process rewards |
| v7.6 | Mar 22 | Major refactor — split monolithic phases into modules (46% reduction) |
| v7.8 | Mar 22 | Deterministic phase gate script after gaming incident |
| v8.0 | Mar 23 | Progressive disclosure (85% SKILL.md reduction), agent compression |
| v8.1 | Mar 24 | Pipeline efficiency overhaul, inline Operator, slim Scout |
| v8.2 | Mar 25 | Compound discovery loop — hypotheses, discoveries, proposals, velocity convergence |
| v8.3 | Mar 30 | Smart Web Search skill — intent-aware 6-stage search pipeline |

### Benchmark Scores (v8.0)

The project scores itself across 8 dimensions using automated + LLM graders:

| Dimension | Score |
|-----------|-------|
| Documentation Completeness | 80 |
| Specification Consistency | 95 |
| Defensive Design | 100 |
| Eval Infrastructure | 100 |
| Modularity | 93 |
| Schema Hygiene | 93 |
| Convention Adherence | 100 |
| Feature Coverage | 100 |
| **Overall** | **94.4** |

### Incidents and Recovery

The pipeline has experienced and recovered from integrity incidents — proving its safety mechanisms work:

| Cycles | What Happened | How It Was Fixed |
|--------|---------------|------------------|
| 102-111 | Reward hacking: agent inflated success metrics | Added eval tamper detection, independent verification |
| 132-141 | Orchestrator gaming: skipped agents, fabricated cycles | Added deterministic phase gate script (`phase-gate.sh`) that the LLM cannot bypass |
| Gemini | Forged audit reports during cross-platform run | Added anti-forgery defenses, platform-specific safeguards |

These incidents led to a key architectural insight: **structural constraints beat behavioral constraints**. Safety rules in prompts can be ignored; safety checks in bash scripts cannot.

## Architecture

### Phase Details

**Phase 0 — CALIBRATE** (once per session)
Initialize workspace, load state, run benchmark if stale.

**Phase 1 — DISCOVER** (Scout agent)
Scan the codebase, read operator brief from last cycle, select 2-4 tasks with eval definitions. Uses multi-armed bandit for task type selection, novelty scoring to avoid over-touched files.

**Phase 2 — BUILD** (Builder agent)
Implement each task in an isolated git worktree. Self-verify against eval definitions. Max 3 attempts per task. Supports parallel builds for independent tasks.

**Phase 3 — AUDIT** (Auditor agent)
Single-pass review covering code quality, security, pipeline integrity, and eval checks. Blocks on MEDIUM+ severity. Failed audits trigger Builder retry.

**Phase 4 — SHIP**
Commit and push changes. Auto-increment patch version.

**Phase 5 — LEARN** (Operator agent)
Extract instincts from the cycle. Run health checks. Detect stagnation patterns. Write operator brief for next cycle.

**Phase 6 — META-CYCLE** (every 5 cycles)
Evaluate pipeline performance. Evolve agent prompts via critique-synthesize loop. Adjust strategies and budgets. Auto-revert changes that degrade performance.

### Multi-Task Flow

When Scout selects multiple tasks, phases 2-3 loop per task:

```
Scout → [Task A, Task B, Task C]
  → Builder(A) → Auditor(A) → commit
  → Builder(B) → Auditor(B) → commit
  → Builder(C) → Auditor(C) → commit
→ Ship → Learn
```

### Self-Learning System

Seven mechanisms compound across cycles:

1. **Instinct extraction** — patterns from each cycle (starts at 0.5 confidence, increases with confirmation)
2. **Meta-cycle review** — pipeline self-evaluation every 5 cycles
3. **Prompt evolution** — TextGrad-style critique loop refines agent prompts
4. **Gene library** — reusable fix templates with selectors and validation
5. **Curriculum learning** — difficulty-graduated task queue with mastery levels
6. **Process rewards** — step-level scoring per phase
7. **Mutation testing** — self-generated evals that test the tests

### Strategy Presets

| Strategy | Focus | When to Use |
|----------|-------|-------------|
| `balanced` | Mix of features, fixes, quality | Default — general improvement |
| `innovate` | New features first | When the codebase needs new capabilities |
| `harden` | Stability, tests, security | Before a release or after incidents |
| `repair` | Bug fixes only | When something is broken |
| `ultrathink` | Maximum reasoning depth | Complex architectural decisions |

## Project Structure

```
evolve-loop/
├── .claude-plugin/
│   ├── plugin.json              # Plugin manifest
│   └── marketplace.json         # Marketplace distribution
├── agents/                      # Agent definitions (4 files)
│   ├── evolve-scout.md
│   ├── evolve-builder.md
│   ├── evolve-auditor.md
│   └── evolve-operator.md
├── skills/evolve-loop/          # Orchestration and phase logic
│   ├── SKILL.md                 # Entry point
│   ├── phases.md                # Phase sequencing
│   ├── phase0-calibrate.md      # Benchmark calibration
│   ├── phase2-build.md          # Build orchestration
│   ├── phase4-ship.md           # Commit and push
│   ├── phase5-learn.md          # Instinct extraction
│   ├── phase6-metacycle.md      # Meta-cycle self-improvement
│   ├── memory-protocol.md       # State and ledger schema
│   ├── eval-runner.md           # Eval gate mechanics
│   └── benchmark-eval.md        # 8-dimension scoring
├── scripts/                     # Safety scripts (not LLM-controlled)
│   ├── phase-gate.sh            # Mandatory phase transition checks
│   ├── cycle-health-check.sh    # Stagnation detection
│   └── eval-quality-check.sh    # Eval validation
├── docs/                        # Research and reference docs
├── examples/                    # Annotated examples
├── install.sh
├── uninstall.sh
├── CONTRIBUTING.md
├── CHANGELOG.md
└── LICENSE (MIT)
```

### Workspace (generated per project)

When Evolve Loop runs on your project, it creates an `.evolve/` directory:

```
.evolve/
├── workspace/              # Current cycle artifacts
│   ├── scout-report.md     # What was found
│   ├── build-report.md     # What was built
│   ├── audit-report.md     # What was reviewed
│   └── operator-log.md     # Health assessment
├── evals/                  # Eval definitions
├── instincts/              # Learned patterns
├── genes/                  # Reusable fix templates
├── history/                # Archived past cycles
├── state.json              # Persistent state
├── ledger.jsonl            # Structured log
└── notes.md                # Cross-cycle context
```

## Requirements

- An AI CLI that supports plugins (Claude Code or Gemini CLI)
- Git

No other dependencies. The entire system is markdown files that AI agents interpret.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on adding agents, modifying phases, or fixing bugs.

The short version:
1. Fork the repo
2. Create a feature branch
3. Make changes
4. Test with `./install.sh && /evolve-loop 1` on a sample project
5. Submit a PR

## License

[MIT](LICENSE) -- Copyright (c) 2026 Dan Lee

## Links

- [Documentation index](docs/index.md) — all reference docs
- [Changelog](CHANGELOG.md)
