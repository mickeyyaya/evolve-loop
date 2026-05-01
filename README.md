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

### Setup (one-time, auto-runs at Phase 0)

The orchestrator automatically runs `scripts/setup-skill-inventory.sh` at the start of each session to build a deterministic map of every installed skill — project-local, user-global (`~/.claude/skills/`), and plugin cache (`~/.claude/plugins/cache/*/skills/`). The output lives at `.evolve/skill-inventory.json` and is cached for 1 hour.

You can also run it manually:

```bash
# Fresh scan (default; cache-hit if <1h old)
bash scripts/setup-skill-inventory.sh

# Force rebuild (ignores cache)
bash scripts/setup-skill-inventory.sh --force
```

This replaces the legacy LLM-side parsing of the session's skill listing with a zero-token filesystem scan. Scout and Builder look up skills via the cached index; the ecc:e2e skill, for example, is automatically routed as a primary skill for any UI task without the orchestrator having to re-discover it every session.

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

| Metric | Start (v3.0) | Current (v8.16) |
|--------|-------------|-----------------|
| Agents | 11 (bloated) | 3 (lean) + inline Operator |
| Phases | 3 | 6 (+ meta-cycle every 5) |
| Skills | 1 | 5 (`/evolve-loop` + `/refactor` + `/code-review-simplify` + `/inspirer` + `/evaluator`) |
| Cycles completed | 0 | 176+ |
| Tasks shipped | 0 | 128+ |
| Commits | 1 | 410+ |
| Benchmark score | N/A | 89.9 / 100 |
| Consecutive successes | 0 | 74 |
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
| v8.4 | Mar 30 | Search routing — Smart for deep research, Default for quick lookups |
| v8.5 | Mar 30 | Beyond-the-Ask divergence trigger — proactive insight generation |
| v8.6 | Mar 31 | External skill discovery and routing — agents leverage installed plugins |
| v8.6.5 | Apr 4 | `/refactor` skill — research-backed refactoring pipeline with 22-smell catalog, 66-technique Fowler catalog, LLM safety protocols, and 10 reference files |
| v8.7 | Apr 6 | `/code-review-simplify` skill — unified code review + simplification with hybrid pipeline+agentic architecture, 4-dimension scoring, adaptive depth routing |
| v8.8 | Apr 6 | `/inspirer` skill — standalone creative divergence engine with 12 provocation lenses, web-grounded research, scored Inspiration Cards, and evolve-loop integration |
| v8.9 | Apr 6 | `/evaluator` skill — independent evaluation engine with 6-dimension scoring, EST anti-gaming defenses, self-improving criteria lifecycle, and strategic direction guidance |
| v8.10 | Apr 9 | `ecc:e2e` first-class integration (Scout routing → Builder generation → Auditor D.5 grounding → phase-gate ship block), deterministic `setup-skill-inventory.sh` (replaces LLM parsing, 281 skills indexed), phases renumbered to eliminate `Phase 0.5` (now 0-7 linear) with aligned filenames |
| v8.11 | Apr 20 | Added `autoresearch` strategy for testing hypotheses against fixed metrics, decriminalizing failure and overriding budget constraints for deep out-of-the-box exploration. Added dynamic context scaling (2M tokens for Gemini CLI) and cross-platform support. |
| v8.12 | Apr 27 | **Subagent subprocess isolation hardening** — phase agents now invoked via `scripts/subagent-run.sh` with per-agent CLI permission profiles (`.evolve/profiles/*.json`), per-invocation challenge tokens, tamper-evident SHA256 ledger entries, and OS-level sandboxing (`sandbox-exec` on macOS, `bwrap` on Linux). Mutation-testing pre-flight (`scripts/mutate-eval.sh`) blocks tautological evals at the discover→build gate (kill-rate ≥ 0.8). Adversarial Auditor mode (default-on) requires positive evidence per acceptance criterion. CLI adapters for Claude / Gemini / Codex enable provider-agnostic dispatch. |
| v8.13 | Apr 27 | **Atomic ship-gate via canonical `scripts/ship.sh` allowlist** (v8.13.0) — replaces v8.12.x's parser-bypass arms race with a single canonical ship path. The PreToolUse hook (`scripts/guards/ship-gate.sh`) allowlists exactly one realpath-resolved script for `git commit`/`git push`/`gh release create`. ship.sh enforces audit-first contract internally: TOFU self-SHA verification, audit verdict + report-SHA check, cycle-binding (HEAD + tree-state SHA must match what auditor audited), atomic commit/push/release. Breaking change: raw `git commit`/`git push`/`gh release create` denied unless via `bash scripts/ship.sh "<msg>"`. **v8.13.1 — trust boundary completed:** added `role-gate.sh` (path-allowlist on Edit/Write per active phase), `phase-gate-precondition.sh` (sequence-allowlist on `subagent-run.sh` invocations), `cycle-state.sh` helpers + `.evolve/cycle-state.json` runtime state, `run-cycle.sh` declarative cycle driver, and `agents/evolve-orchestrator.md` subagent prompt. 69/69 unit tests across the three gates. The orchestrator can no longer edit source code outside the build worktree, run phases out of order, or commit without going through ship.sh — all enforced at the kernel hook layer, not LLM cooperation. |
| v8.14 | Apr 29 | TBD — fill in via release-pipeline.sh + changelog-gen.sh |

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

## Included Skills

### `/evolve-loop` — Autonomous Improvement Pipeline

The core skill. Runs autonomous improvement cycles on your codebase. See [Quick Start](#quick-start) above.

### `/refactor` — Research-Backed Refactoring Pipeline

A comprehensive refactoring orchestrator built from academic research (arXiv papers, SonarQube, OpenRewrite, Rector, tree-sitter).

```bash
/refactor                   # Full pipeline on specified files
/refactor scan              # Detect and report only — no changes
/refactor arch              # Architecture analysis — circular deps, boundaries
/refactor complexity        # Cognitive complexity report
/refactor health            # Composite health score per function
/refactor diff              # Scan only changed files
/refactor hotspots          # Find high-churn, high-smell files
/refactor auto              # Fully autonomous — no confirmations
```

**5-phase workflow:** Scan → Prioritize → Plan & Partition → Execute (parallel worktrees) → Merge & Verify

**What it includes:**

| Capability | Source |
|-----------|--------|
| 22-smell detection catalog with numeric thresholds | refactoring.guru |
| 66-technique Fowler catalog with detection signals | Fowler 2nd edition |
| Cognitive complexity scoring algorithm | SonarQube |
| Architecture analysis (circular deps, fan-in/out, orphans) | dependency-cruiser, graph theory |
| LLM safety protocols (RefactoringMirror pattern) | arXiv:2411.04444 |
| Prompt specificity ladder (15.6% → 86.7% identification) | arXiv:2411.04444 |
| Multi-metric composite smell scoring | Research synthesis |
| Graph-based critical pair analysis for parallel execution | Graph transformation theory |
| Language-specific guides (TS/JS, Python, Go, Java) | — |

All reference material is in `skills/refactor/reference/` — loaded on demand, not at startup.

---

## Architecture

### Phase Details

**Phase 0 — CALIBRATE** (once per session)
Initialize workspace, load state, run benchmark if stale.

**Phase 2 — DISCOVER** (Scout agent)
Scan the codebase, read operator brief from last cycle, select 2-4 tasks with eval definitions. Uses multi-armed bandit for task type selection, novelty scoring to avoid over-touched files.

**Phase 3 — BUILD** (Builder agent)
Implement each task in an isolated git worktree. Self-verify against eval definitions. Max 3 attempts per task. Supports parallel builds for independent tasks.

**Phase 4 — AUDIT** (Auditor agent)
Single-pass review covering code quality, security, pipeline integrity, and eval checks. Blocks on MEDIUM+ severity. Failed audits trigger Builder retry.

**Phase 5 — SHIP**
Commit and push changes. Auto-increment patch version.

**Phase 6 — LEARN** (Operator agent)
Extract instincts from the cycle. Run health checks. Detect stagnation patterns. Write operator brief for next cycle.

**Phase 7 — META-CYCLE** (every 5 cycles)
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
├── skills/evolve-loop/          # Autonomous pipeline skill
│   ├── SKILL.md                 # Entry point (orchestrator)
│   ├── phases.md                # Phase sequencing
│   ├── phase0-calibrate.md      # Benchmark calibration
│   ├── phase3-build.md          # Build orchestration
│   ├── phase5-ship.md           # Commit and push
│   ├── phase6-learn.md          # Instinct extraction
│   ├── phase7-meta.md      # Meta-cycle self-improvement
│   ├── memory-protocol.md       # State and ledger schema
│   ├── eval-runner.md           # Eval gate mechanics
│   └── benchmark-eval.md        # 8-dimension scoring
├── skills/refactor/             # Refactoring pipeline skill
│   ├── SKILL.md                 # Workflow (653 lines)
│   └── reference/               # On-demand reference files (10)
│       ├── code-smells.md       # 22-smell catalog
│       ├── refactoring-techniques.md  # 66-technique Fowler catalog
│       ├── complexity-scoring.md      # Cognitive complexity algorithm
│       ├── architecture-analysis.md   # Circular deps, fan-in/out
│       ├── health-scoring.md          # Multi-metric composite scoring
│       ├── safety-protocols.md        # RefactoringMirror, re-prompting
│       ├── prompt-engineering.md      # Specificity ladder
│       ├── smell-to-technique-map.md  # Smell → fix lookup
│       ├── language-notes.md          # TS/JS, Python, Go, Java
│       └── worked-example.md          # Full pipeline walkthrough
├── scripts/                     # Safety scripts (not LLM-controlled)
│   ├── phase-gate.sh            # Mandatory phase transition checks
│   ├── cycle-health-check.sh    # Stagnation detection
│   └── eval-quality-check.sh    # Eval validation
├── docs/                        # Research and reference docs (52)
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
- [Research index](docs/research-index.md) — 52 research documents
- [Changelog](CHANGELOG.md)
- [Releases](https://github.com/mickeyyaya/evolve-loop/releases)
