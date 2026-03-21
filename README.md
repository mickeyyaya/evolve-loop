# Evolve Loop

A self-evolving development pipeline for AI CLIs (like Gemini CLI, Claude Code). Orchestrates 4 specialized AI agents across 5 lean phases to autonomously discover, build, audit, and ship improvements to any codebase.

Optimized for fast iteration — diverse small/medium tasks per cycle, worktree isolation, 12hr research cooldown, and single-pass auditing.

## Features

- **4 specialized agents** — Scout, Builder, Auditor, Operator
- **5 lean phases** — DISCOVER → BUILD → AUDIT → SHIP → LEARN
- **Multi-task per cycle** — 2-4 small tasks built and audited sequentially
- **Worktree isolation** — Builder works in isolated git worktrees
- **Eval hard gate** — Auditor runs code graders and acceptance checks before shipping
- **Continuous learning** — instinct extraction after each cycle with deep reasoning
- **Loop monitoring** — Operator detects stalls, quality degradation, and repeated failures
- **Strategy presets** — `innovate`, `harden`, `repair`, `ultrathink`, `balanced` steer cycle intent
- **Token budgets** — soft limits per task and per cycle prevent runaway costs
- **Stagnation detection** — pattern-based detection of same-file churn, error repeats, diminishing returns
- **Meta-cycle self-improvement** — every 5 cycles, the pipeline evaluates and improves itself
- **Automated prompt evolution** — TextGrad-style critique-synthesize loop refines agent prompts
- **Delta evaluation** — quantitative trend tracking across cycles
- **Multi-type instinct memory** — episodic, semantic, and procedural categories for targeted retrieval
- **Dynamic model routing** — 3-tier model abstraction (tier-1/tier-2/tier-3) works across any LLM provider
- **Plan template caching** — reuse successful build plans for ~30-50% cost reduction
- **Gene/Capsule library** — structured fix templates with selectors and validation
- **Memory consolidation** — cluster, decay, and archive instincts to prevent unbounded growth
- **Curriculum learning** — difficulty-graduated task queue with mastery levels
- **Process rewards** — step-level scoring per phase for targeted improvement
- **Mutation testing** — self-generated evals that test the tests themselves
- **Safety & integrity** — eval tamper detection, memory provenance, rollback protocol
- **Accuracy self-correction** — chain-of-thought verification, multi-stage output correction
- **Performance profiling** — cost-bottleneck analysis, per-phase token attribution
- **Security pipeline integrity** — eval tamper detection, prompt injection defense, provenance tracking
- **Island model** — parallel configuration evolution with trait migration (advanced)
- **Capability gap detection** — synthesize new tools when existing ones can't handle a task
- **MAP-Elites fitness** — multi-dimensional scoring (speed, quality, cost, novelty)
- **LLM-as-a-Judge self-evaluation** — structured rubric scores each cycle on correctness, quality, and safety before shipping
- **Self-learning architecture** — 7 mechanisms (instinct extraction, meta-cycle, prompt evolution, gene library, curriculum, process rewards, mutation testing) compound across cycles
- **Stop-hook context reset** — indefinite runtime via session handoff
- **Cost awareness** — soft warning threshold for long-running sessions
- **Multi-armed bandit task selection** — Thompson Sampling biases Scout toward historically high-reward task types
- **Semantic task crossover** — recombines successful plan templates to generate novel task proposals
- **Intrinsic novelty reward** — priority boost for tasks touching files not modified in 3+ cycles
- **Decision trace** — structured audit trail of Scout selection signals for interpretability
- **Prerequisite task graph** — dependency-aware scheduling with auto-deferral for unmet prerequisites
- **Counterfactual annotations** — deferred tasks annotated with predicted outcomes for retrospective analysis
- **Builder retrospective** — cross-cycle learning via file fragility observations and recommendations
- **Auditor adaptive strictness** — reduced checklist for proven task types, full scrutiny for new ones
- **Agent mailbox** — typed cross-agent messaging for coordination across pipeline phases
- **Operator next-cycle brief** — closed-loop feedback from monitoring to task selection
- **Session narrative** — human-readable story synthesis of what the loop learned each cycle
- **No external dependencies** — fully self-contained AI CLI plugin

## Quick Start

### Prerequisites

- An AI CLI (like Gemini CLI or Claude Code) installed
- A git repository to evolve

### Installation

**Option A: As an AI CLI plugin (recommended)**

In your AI CLI, run:
```
/plugin marketplace add mickeyyaya/evolve-loop
/plugin install evolve-loop@evolve-loop
```

The skill and agents load automatically.

**Upgrading to the latest version:**

```
/plugin marketplace update evolve-loop
/plugin update evolve-loop@evolve-loop
```

Then reload in your current session:
```
/plugin reload
```

**Option B: Manual install**

```bash
git clone https://github.com/mickeyyaya/evolve-loop.git
cd evolve-loop
./install.sh
```

### Usage

```bash
# Autonomous mode — 2 cycles, balanced strategy
/evolve-loop

# Goal-directed — 1 cycle focused on a specific feature
/evolve-loop 1 add dark mode support

# Strategy presets — steer cycle intent
/evolve-loop innovate                    # feature-first mode
/evolve-loop 3 harden                    # stability-first for 3 cycles
/evolve-loop repair fix broken auth      # fix-only with directed goal

# Multiple autonomous cycles
/evolve-loop 5
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

| Role | File | Default Tier | Purpose |
|------|------|-------------|---------|
| Scout | `evolve-scout.md` | tier-2 | Discovery + analysis + task selection |
| Builder | `evolve-builder.md` | tier-2 | Design + implement + self-test |
| Auditor | `evolve-auditor.md` | tier-2 | Review + security + eval gate |
| Operator | `evolve-operator.md` | tier-3 | Loop health monitoring |

## Showcase

See [docs/showcase.md](docs/showcase.md) for an annotated walkthrough of a complete cycle — Scout decision trace with bandit/novelty/crossover signals, mailbox exchange between Builder and Auditor, Builder retrospective notes, extracted instinct, and Operator next-cycle brief and session narrative.

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
- Post-cycle health assessment with delta metrics
- Stagnation detection (same-file churn, error repeats, diminishing returns)
- Quality trend tracking via quantitative delta analysis
- HALT protocol: pauses loop for human attention

### Context Management
- The orchestrator runs continuously through all requested cycles without stopping — it never pauses for user input
- A `handoff.md` checkpoint is written after each cycle as a safety measure for external interruptions
- If a session is interrupted, the next `/evolve-loop` invocation reads the handoff to continue seamlessly

### Meta-Cycle (every 5 cycles)
- Evaluates pipeline success rates, agent efficiency, stagnation
- Automated prompt evolution via critique-synthesize loop
- May adjust strategy, token budgets, or agent prompts
- Auto-reverts prompt changes that degrade performance

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
│       ├── eval-runner.md     # Eval gate instructions
│       ├── phase5-learn.md    # LEARN phase instinct extraction
│       └── benchmark-eval.md  # Benchmark evaluation framework
├── docs/
│   ├── accuracy-self-correction.md  # CoT + multi-stage verification
│   ├── architecture.md        # Detailed architecture docs
│   ├── configuration.md       # Configuration reference
│   ├── domain-adapters.md     # Domain-specific adapter patterns
│   ├── generalization-status.md  # Cross-domain generalization tracking
│   ├── genes.md               # Gene/Capsule library docs
│   ├── instincts.md           # Instinct system docs
│   ├── island-model.md        # Island model evolution docs
│   ├── memory-hierarchy.md    # Memory hierarchy guide (layers 0-6, access matrix)
│   ├── meta-cycle.md          # Meta-cycle review docs
│   ├── performance-profiling.md  # Cost-bottleneck analysis + token attribution
│   ├── policy-design.md       # Agent policy design patterns
│   ├── security-considerations.md  # Pipeline integrity + prompt injection defense
│   ├── self-learning.md       # Self-learning architecture (7 mechanisms)
│   ├── showcase.md            # Annotated cycle walkthrough
│   ├── skill-building.md      # Skill authoring guide
│   ├── token-optimization.md  # Token optimization strategies
│   └── writing-agents.md      # Guide for creating agents
├── examples/
│   ├── eval-definition.md     # Annotated eval definition example
│   ├── gene-example.yaml      # Annotated gene/capsule example
│   └── instinct-example.yaml  # Annotated instinct example
├── install.sh                 # Installation script
├── uninstall.sh               # Uninstallation script
├── README.md
├── CONTRIBUTING.md
├── LICENSE
└── CHANGELOG.md
```

## Workspace Layout (per project)

```
.evolve/
├── workspace/           # Current cycle (overwritten each cycle)
│   ├── scout-report.md
│   ├── build-report.md
│   ├── audit-report.md
│   └── operator-log.md
├── evals/               # Eval definitions (created by Scout)
├── instincts/
│   ├── personal/        # Extracted patterns from cycles
│   └── archived/        # Superseded/stale instincts
├── genes/               # Reusable fix templates (Gene/Capsule library)
├── tools/               # Synthesized tools (capability gap detection)
├── history/
│   └── cycle-N/         # Archived workspace per cycle
├── state.json           # Persistent cycle state
├── ledger.jsonl         # Append-only structured log
└── notes.md             # Cross-cycle context (append-only)
```

## Research & Optimization

<!-- challenge: 2d1190ca57c390ec -->

The loop's design is grounded in applied research. Over 30 papers were surveyed covering context optimization, reward hacking detection, and agent efficiency to inform the pipeline's architecture and self-improvement mechanisms.

Key techniques applied from that research:

- **Phase isolation** — separate context windows per agent phase to prevent cross-contamination
- **Dynamic turn budgets** — per-task token limits adjusted by complexity and strategy
- **Compression** — instinct clustering, memory decay, and plan template caching to reduce redundancy
- **Model routing** — 3-tier abstraction (tier-1/tier-2/tier-3) matching task complexity to model capability
- **Graph exploration** — prerequisite task graphs with dependency-aware scheduling

Human-readable research reports are in `docs/`:

- [docs/research-applied-context-optimization.md](docs/research-applied-context-optimization.md) — main report: techniques surveyed, what was applied, and measured impact
- [docs/human-learning-guide.md](docs/human-learning-guide.md) — plain-language guide to understanding what the loop learns and why it makes the decisions it does

## Requirements

- AI CLI (like Gemini CLI or Claude Code)
- Git (for worktree isolation)

## License

[MIT](LICENSE)
