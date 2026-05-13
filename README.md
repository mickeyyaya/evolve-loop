# Evolve Loop

**A self-improving development pipeline that makes your codebase better while you sleep.**

Evolve Loop is an open-source plugin for AI coding assistants (Claude Code, Gemini CLI, Codex CLI) that runs autonomous improvement cycles on your codebase. Each cycle scans your project, picks tasks, implements changes, reviews its own work, ships only what passes, and then learns from the experience to do better next time.

Think of it as a tireless junior developer that gets smarter with every cycle — and whose phases are enforced by shell hooks at the OS layer, not by prompt-level promises.

---

## How It Works

Each cycle runs eight phases, plus a meta-cycle every five cycles. The full pipeline is enforced structurally — a trust kernel of three PreToolUse shell hooks denies any deviation before the LLM can act.

```
INTENT ─→ SCOUT ─→ TRIAGE ─→ [PLAN-REVIEW] ─→ BUILD ─→ AUDIT ─→ SHIP ─→ RETROSPECTIVE
   │        │        │            │             │        │       │           │
   │        │        │            │             │        │       │           └─ Extract lessons + carryover todos; on FAIL/WARN, auto-fires (v8.45.0+)
   │        │        │            │             │        │       └─ Commit + push (only on audit PASS or WARN; canonical via ship.sh)
   │        │        │            │             │        └─ Adversarial cross-check; verdict PASS / WARN / FAIL
   │        │        │            │             └─ Implement in per-cycle git worktree (isolation)
   │        │        │            └─ (opt-in) 4-lens plan review: CEO / Eng / Design / Security
   │        │        └─ Bound this cycle's scope; top_n / deferred / dropped (default-on v8.59.0+)
   │        └─ Find work + write evals; cite research; fan-out optional
   └─ Structure the vague goal: 8 fields + AwN classifier + ≥1 challenged premise (v8.19.1+)
```

**Trust kernel** — three PreToolUse shell hooks block deviations structurally:

| Hook | Watches | Denies |
|---|---|---|
| `phase-gate-precondition.sh` | every subagent-run.sh invocation | Out-of-order phases |
| `role-gate.sh` | every Edit/Write | Writes outside the active phase's allowlist |
| `ship-gate.sh` | every Bash with git/gh verbs | Anything that isn't `scripts/lifecycle/ship.sh` |

Plus a tamper-evident SHA-chained `.evolve/ledger.jsonl` (every entry records `prev_hash`; `bash scripts/observability/verify-ledger-chain.sh` walks the chain).

**Four specialized agents (+ inline orchestrator):**

| Agent | Job | Output |
|-------|-----|--------|
| **Scout** | Find work, cite research, write evals | `scout-report.md` |
| **Triage** | Bound this cycle's scope (top_n / deferred / dropped) | `triage-decision.md` |
| **Builder** | Implement in an isolated git worktree | `build-report.md` |
| **Auditor** | Adversarial cross-check (Opus by default — different family from Builder's Sonnet) | `audit-report.md` |

## Quick Start

### Prerequisites

- One of:
  - [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (tier-1, primary)
  - [Gemini CLI](https://github.com/google-gemini/gemini-cli) (tier-1-hybrid — delegates to `claude` binary at runtime)
  - [Codex CLI](https://github.com/openai/codex) (tier-1-hybrid since v8.51.0)
- A git repository you want to improve

### Install

**Option A: Plugin (recommended)**

```bash
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

The v9.1.0 syntax is **budget-first** (cost-driven), with cycle-count and resume as alternatives:

```bash
# Budget mode (recommended) — run cycles until cumulative spend ≥ $N
/evolve-loop --budget-usd 5 "improve test coverage"

# Cycle mode — run exactly N cycles regardless of cost
/evolve-loop --cycles 3 "add dark mode"

# Resume a previously paused cycle (v9.1.0+)
/evolve-loop --resume

# Strategy presets (positional, after flags)
/evolve-loop --budget-usd 10 innovate "explore concurrency primitives"
/evolve-loop --cycles 5 harden                    # stability + tests
/evolve-loop --cycles 3 repair "fix auth bug"     # fix-only, smallest diff
/evolve-loop --cycles 1 ultrathink "refactor X"   # tier-1 forced
/evolve-loop --cycles 5 autoresearch              # hypothesis testing, embraces failure
```

> Legacy positional integer (`/evolve-loop 5`) still parses as cycles with a deprecation WARN — v10.0.0 candidate will consider flipping bare-positional to dollars.

### Resume after a pause (v9.1.0+)

If a cycle is checkpointed (Claude Code subscription quota wall, batch cap near, or operator-requested), the dispatcher preserves the worktree + cycle-state on disk. Recover with:

```bash
/evolve-loop --resume
```

The dispatcher locates the most recent paused cycle, validates state (git HEAD unchanged, worktree still exists), and re-spawns the orchestrator from the paused phase boundary. The trust kernel holds across resume — phase-gate, role-gate, ship-gate enforce the same invariants. See [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md) for the full protocol.

### Strategy presets

| Strategy | Focus | Approach | Strictness |
|----------|-------|----------|------------|
| `balanced` | Broad discovery | Standard | MEDIUM+ blocks |
| `innovate` | New features, gaps | Additive | Relaxed style |
| `harden` | Stability, tests | Defensive | Strict all |
| `repair` | Bugs, broken tests | Fix-only, smallest diff | Strict regressions |
| `ultrathink` | Complex refactors | tier-1 forced | Strict + confidence |
| `autoresearch` (v8.11+) | Hypothesis testing | Fixed metrics, embraces failure | Divergent, unpenalized |

### Operator commands (read-only, safe mid-cycle)

```
./bin/status                            current cycle + recent ledger summary
./bin/cost <cycle>                      per-cycle token + cost breakdown (--json available)
./bin/health <cycle> <workspace>        anomaly fingerprint for any past cycle
./bin/verify-chain                      tamper-evident ledger chain check
./bin/preflight                         full pipeline dry-run (regression + simulate + release-pipeline dry-run)
./bin/check-caps [cli]                  show resolved capability tier per adapter
bash scripts/observability/show-context-monitor.sh <cycle>   per-cycle context usage (v9.1.0+)
bash scripts/observability/show-context-monitor.sh --watch   live-tail latest cycle (3s refresh)
```

## What Makes It Different

Unlike `/goal`, `/mission`, and other long-running-LLM skills that orchestrate agents via prompt instructions alone, **evolve-loop enforces its pipeline at the OS layer** — three PreToolUse kernel hooks fire *before* the agent can act, not after.

**Phases enforced by shell, not promises.** Most long-running-LLM skills sequence phases via prompt instructions; agents can and do skip them. evolve-loop's `phase-gate-precondition.sh` fires as a kernel hook before every subagent dispatch and denies the call if phases are out of order. A 2026-04-29 flow audit found 12 consecutive cycles where Scout and Builder were silently skipped by an orchestrator following prompt rules; the shell gate eliminated the pattern structurally.

**Every artifact is SHA-chained and forgery-resistant.** Each phase embeds a per-invocation challenge token and records its SHA256 in a hash-chained ledger. Before any commit, `ship.sh` verifies the auditor's artifact SHA against the file on disk — not what the agent *claims* it produced. A cross-CLI forgery attempt (a fabricated PASS audit report) was detected and blocked by this chain (see `docs/incidents/gemini-forgery.md`).

**Failures become structured lessons, not repeated mistakes.** On every FAIL or WARN verdict, the Retrospective agent fires inline (v8.45.0+, Reflexion / Shinn et al. 2023) and writes a structured YAML lesson merged into `state.json:instinctSummary[]`. The next cycle's Scout reads that lesson before choosing tasks. A deterministic shell script (`failure-adapter.sh`) — not an LLM — classifies failure patterns and blocks cycles that have exhausted recoverable paths.

**It survives resource walls (v9.1.0).** Two paired capabilities:
- **Checkpoint-resume** — when cumulative cost crosses 95% of budget (or a phase exits with the quota-exhaustion signature: rc=1 + empty stderr + cost in danger zone), the dispatcher signals a graceful pause at the next clean phase boundary. The worktree + cycle-state are preserved on disk; `--resume` picks up where it left off.
- **Context-window control** — per-phase `context-monitor.json` tracks input tokens cumulatively; `EVOLVE_CONTEXT_AUTOTRIM=1` enables head-60% / tail-35% prompt trim when a phase exceeds the cap. Same threshold model as cost — WARN at 80%, CRITICAL at 95% (which sets the checkpoint signal). See [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md) and [docs/architecture/context-window-control.md](docs/architecture/context-window-control.md).

**Bounded scope via Triage (v8.59.0+ default-on).** A separate phase between Scout and Build refuses to over-commit. Triage picks 1-3 top_n items for this cycle, defers the rest to next cycle's carryoverTodos, and drops with-a-reason anything that shouldn't have been in the backlog. The phase-gate blocks on `cycle_size_estimate=large` — split required before re-entry. Closes the failure mode where Scout returned 8 items and Builder shipped 2 half-done.

**Adversarial Auditor (default-on).** The auditor is prompted to require positive evidence per acceptance criterion — saying "I see no problems" is not 0.85 confidence. Default model is Opus (different family from Builder's Sonnet) to break same-family-judge sycophancy per Sharma et al. 2024.

**It runs in isolation.** Builder works in a per-cycle git worktree. Your working tree is never touched. If a build fails, nothing main-branch is affected.

**It improves its own process.** Every 5 cycles, a meta-cycle evaluates the pipeline itself — adjusting agent prompts, token budgets, and strategies based on measured performance.

**No external dependencies.** No npm packages, no Python libraries, no Docker. It's markdown instructions + bash scripts. Works anywhere git works.

## Knowledge Base Stewardship (v9.1.x+)

**The rule** — for all research learned, applied, or verified in any cycle, the source material must be persisted to the knowledge base for future reference.

The pipeline has two distinct content surfaces:

| Surface | Path | Loaded into agent context? | Audience |
|---|---|---|---|
| Runtime context | `docs/research/` (5 active refs), `agents/`, `skills/`, `scripts/`, `docs/architecture/` | YES — agents read during cycles | Agents + contributors |
| Developer knowledge base | `docs/private/research/` (42 archived refs + future additions) | **NO** — explicitly excluded across all CLIs | Contributors only |

**How it's used in the skill:**

1. **Scout** cites research in `scout-report.md` (publication + GitHub repo). If the cited paper / repo / pattern is novel (not previously archived), Scout adds a corresponding `docs/private/research/<slug>.md` summarizing the source's relevance, the section it informed, and a link to the cycle's report.
2. **Builder** applies the cited research and records the application in `build-report.md` with a cross-reference to the knowledge-base entry. If the application reveals new sub-references (e.g., a transitive citation), Builder adds those to `docs/private/research/` too.
3. **Auditor** verifies that the cited research is real (non-fabricated) and that the knowledge-base entry has been created. WARNs on missing entries (low-severity, awareness-only); FAILs on fabricated citations.
4. **Retrospective** on FAIL/WARN cross-links the lesson YAML to the knowledge-base entry that informed the failed approach — so future cycles see both "what we tried" and "what research informed it."

**Why we need this:**

- **Avoid re-research.** Cycles routinely re-discover the same papers and repos. Without persistence, the same Lost-in-the-Middle / Self-Refine / Reflexion patterns get re-grepped every few weeks.
- **Audit trail for design decisions.** Six months later, "why did we adopt pattern X in cycle N?" should be answerable in <30 seconds. The knowledge-base entry is that pointer.
- **Cross-cycle learning.** Lessons in `state.json:instinctSummary[]` are compact; they don't carry the full context. The knowledge-base entry holds the expanded context that the lesson abbreviates.
- **Stop hollow citations.** Without a persistent surface, agents can cite papers they haven't read. Requiring the knowledge-base entry forces the agent to summarize the source — which catches fabrication.
- **Survives runtime exclusion.** `docs/private/` is invisible to agents during cycles (Liu et al. 2023 "Lost in the Middle" context-noise mitigation), so persistence doesn't re-introduce the noise problem cycle 13 fixed.

See [docs/architecture/private-context-policy.md](docs/architecture/private-context-policy.md) for the full convention and the three-layer runtime-exclusion mechanism (OS sandbox + adapter passthrough + Layer-B context-builder filter).

## Architecture

### Phase Details

For the complete walkthrough (per-phase: who runs, when, inputs, what it checks/does, outputs, model selection, version-specific behavior, kernel-hook integration), see [docs/architecture/phase-architecture.md](docs/architecture/phase-architecture.md). For the research citations that motivated each design decision (Reflexion, Voyager, Constitutional AI, Self-Refine, METR reward-hacking, Crosby & Wallach tamper-evident logging, IETF Agent Audit Trail draft, etc.), see [docs/architecture/phase-architecture-citations.md](docs/architecture/phase-architecture-citations.md).

Brief per-phase summaries:

**Phase 0 — CALIBRATE** (once per `/evolve-loop` invocation)
Compute `projectBenchmark` across 8 dimensions (documentation, defensive design, eval infrastructure, modularity, schema hygiene, convention adherence, feature coverage, specification consistency). Hybrid automated + LLM probes. Grounded in ISO/IEC 25010 + Goodhart's-Law mitigation via composite scoring.

**Phase 0b — INTENT** (v8.19.1+, always-on for `/evolve-loop`)
The "Intent Architect" persona converts vague user goals into a structured `intent.md` (8 fields + AwN classifier `IMKI/IMR/IwE/IBTC/CLEAR` + ≥1 challenged_premise). Closes the 56% "missing key information" gap (arXiv 2409.00557) before Scout starts. The mandatory ≥1 challenged_premise rule is the anti-sycophancy mechanism at the structuring layer.

**Phase 1 — DISCOVER** (Scout agent)
Scan the codebase, identify highest-leverage tasks (1-3 per cycle), write evals (acceptance graders) Builder is measured against. Cite at least one publication + one GitHub repo (with URL + star count). Add corresponding knowledge-base entries for novel citations. Mode-aware: `full` (cycle 1, full scan), `incremental` (cycle 2+, diff-only), `convergence-confirmation` (no work to do). Optional Pattern-3 fan-out splits Scout into codebase / research / evals sub-personas.

**Phase 1b — TRIAGE** (v8.59.0+ default-on)
Bound this cycle's scope. Read scout-report backlog + `state.json:carryoverTodos[]`. Emit `triage-decision.md` with `top_n[]` (commit), `deferred[]` (next cycle), `dropped[]` (with reasons), and `cycle_size_estimate: {small,medium,large}`. The phase-gate fails on `large` — split required. Opt out via `EVOLVE_TRIAGE_DISABLE=1`.

**Phase 1c — PLAN-REVIEW** (opt-in via `EVOLVE_PLAN_REVIEW=1`)
4-lens fan-out review: CEO / Engineering / Design / Security. Each lens scores on its dimension; aggregator emits PROCEED / REVISE / ABORT. Inspired by gstack `/autoplan`. Phase-gate `gate_plan_review_to_tdd` enforces the verdict.

**Phase 2 — BUILD** (Builder agent)
Implement Triage's top_n in an isolated git worktree on branch `evolve/cycle-N`. Profile-scoped permissions (writes only to worktree path). Verify isolation → read instincts/genes → research → design → implement → self-verify (run eval graders) → commit → report. Genes (`.evolve/genes/<id>.yaml`) are reusable action recipes accumulated from prior cycles (Voyager-style skill library).

**Phase 3 — AUDIT** (Auditor agent)
Cross-checks Builder's narrative against ground truth via the Single-Pass Review Checklist (Code Quality, Security, Hallucination Detection, Pipeline Integrity, Eval Integrity, E2E Grounding). Verdict: PASS / WARN / FAIL. Default model: Opus (different family from Builder's Sonnet) — breaks same-family-judge sycophancy per Sharma et al. 2024; v8.35.0 auto-downgrades to Sonnet for trivial diffs. Adversarial framing requires *positive evidence* for PASS (Constitutional AI / Bai et al. 2022). Cycle binding (git_head + tree_state_sha + ledger SHA + v8.37.0 hash chain) makes the verdict structurally unforgeable post-hoc.

**Phase 4 — SHIP / RECORD**
Two paths driven by audit verdict:
- **PASS or WARN**: `scripts/lifecycle/ship.sh` (canonical entry point allowlisted by `ship-gate.sh`) commits in the worktree, fast-forward merges into main, pushes. WARN ships by default since v8.28.0 (set `EVOLVE_STRICT_AUDIT=1` for legacy strict).
- **FAIL**: `record-failure-to-state.sh` appends to `state.json:failedApproaches[]` with structured classification (9 classes, each with severity + age-out + retry policy). Next cycle's `failure-adapter.sh` reads non-expired entries and emits a deterministic decision the orchestrator follows verbatim.

**Phase 5 — RETROSPECTIVE** (v8.45.0+, fires on FAIL or WARN)
Extract a reusable lesson (one per *root cause*, not one per defect). Write `.evolve/instincts/lessons/<id>.yaml` with `description` + `preventiveAction` + `contradicts` + `relatedInstincts`. Cross-link to knowledge-base entries that informed the failed approach. Future cycles' Scout/Builder/Auditor read `instinctSummary` from state.json. Reflexion-style verbal-RL loop (Shinn et al. 2023). Double-loop learning (Argyris & Schon) demands the *underlying assumption that turned out to be wrong*. Disable via `EVOLVE_DISABLE_AUTO_RETROSPECTIVE=1`.

**Phase 6 — META** (every 5 cycles, optional)
Self-improvement of the loop itself. Pattern detection across last 5 cycles → proposals in `state.json:proposals[]` (operator-reviewable). Operator approval gate prevents runaway self-modification (Asilomar AI Principles). Promptbreeder-style evolutionary improvement (Fernando et al. 2023).

### Cross-cutting structural integrity

Three Tier-1 kernel hooks fire as PreToolUse in every cycle:

| Hook | Watches | Denies |
|---|---|---|
| `phase-gate-precondition.sh` | `subagent-run.sh` invocations | Out-of-order phases |
| `role-gate.sh` | Edit/Write tool calls | Writes outside the active phase's allowlist |
| `ship-gate.sh` | Bash with git/gh verbs | Anything except `scripts/lifecycle/ship.sh` doing commit/push/release |

Plus the v8.37.0 tamper-evident hash-chained ledger (`prev_hash` chains every entry; `.evolve/ledger.tip` detects truncation; `bash scripts/observability/verify-ledger-chain.sh` walks the chain). All grounded in Saltzer & Schroeder 1975's complete-mediation principle and Crosby & Wallach 2009's tamper-evident logging.

### Self-Learning System

Seven mechanisms compound across cycles:

1. **Instinct extraction** — patterns from each cycle (starts at 0.5 confidence, increases with confirmation)
2. **Meta-cycle review** — pipeline self-evaluation every 5 cycles
3. **Prompt evolution** — TextGrad-style critique loop refines agent prompts
4. **Gene library** — reusable fix templates with selectors and validation
5. **Curriculum learning** — difficulty-graduated task queue with mastery levels
6. **Process rewards** — step-level scoring per phase
7. **Knowledge-base persistence** — every cycle's research citations land in `docs/private/` for future cross-cycle reference (see § Knowledge Base Stewardship above)

## Evolution Data

evolve-loop has been running on its own codebase since March 12, 2026. Selected milestones:

### Current state — Current (v10.1), post cycle 14

| Metric | Value |
|--------|-------|
| Agents | 4 (Scout, Triage, Builder, Auditor) + inline orchestrator + Retrospective |
| Phases | 8 + meta-cycle every 5 |
| Skills | 8+ (`/evolve-loop`, `/scout`, `/build`, `/audit`, `/ship`, `/retro`, `/refactor`, `/code-review-simplify`, etc.) |
| Cycles completed | 200+ |
| Trust kernel test suites | swarm-architecture (41 assertions), role-gate (23), checkpoint-roundtrip (19), preemptive-checkpoint (18), reactive-quota-classify (15), resume-cycle (28), orchestrator-resume-mode (23), context-window-control (22), private-context-exclusion (20) |
| Active research refs (`docs/research/`) | 5 |
| Archived research refs (`docs/private/research/`) | 42 |

### Version History (highlights)

| Version | Date | Key Changes |
|---------|------|-------------|
| v3.0 | Mar 12 | Initial multi-agent pipeline (11 agents, 3 phases) |
| v4.0 | Mar 13 | Consolidated to 4 lean agents, added strategy presets |
| v5.0 | Mar 14 | Eval gating, instinct extraction, curriculum learning |
| v6.0 | Mar 17 | Gene library, mutation testing, island model |
| v7.0 | Mar 19 | Accuracy self-correction, performance profiling, security pipeline |
| v7.6 | Mar 22 | Major refactor — split monolithic phases (46% reduction) |
| v7.8 | Mar 22 | Deterministic phase gate script after gaming incident |
| v8.0 | Mar 23 | Progressive disclosure (85% SKILL.md reduction), agent compression |
| v8.10 | Apr 9 | `ecc:e2e` first-class integration, deterministic `setup-skill-inventory.sh` |
| v8.11 | Apr 20 | `autoresearch` strategy; dynamic context scaling (2M tokens for Gemini) |
| v8.12 | Apr 27 | Subagent subprocess isolation hardening: per-agent CLI permission profiles, challenge tokens, tamper-evident SHA256 ledger, OS-level sandboxing, mutation-testing pre-flight, Adversarial Auditor (default-on) |
| v8.13 | Apr 27 | Atomic ship-gate via canonical `ship.sh` allowlist; role-gate + phase-gate-precondition; 69/69 unit tests across three gates |
| v8.15 | May 1 | Cross-CLI deployment matrix (Claude Code / Gemini / Codex), hybrid adapter pattern |
| v8.19 | May 3 | Intent phase always-on for `/evolve-loop`; AwN classifier; ≥1 challenged_premise mandatory |
| v8.21 | May 4 | Privileged-shell worktree provisioning; deny-list orchestrator from `git worktree` |
| v8.22 | May 4 | Deterministic failure-adapter; orchestrator reads JSON, never interprets markdown rules |
| v8.25 | May 5 | Three-Tier Strictness Model; explicit ship.sh `--class` (cycle/manual/release) |
| v8.28 | May 6 | Fluent-WARN policy: WARN ships by default; legacy strict via `EVOLVE_STRICT_AUDIT=1` |
| v8.35 | May 8 | Adaptive auditor model selection (Opus / Sonnet / Haiku by diff complexity) |
| v8.37 | May 8 | Tamper-evident hash-chained ledger (`prev_hash` + `entry_seq` + `.evolve/ledger.tip`) |
| v8.42 | May 8 | `.agents/skills/` symlink convention for cross-CLI standard |
| v8.45 | May 9 | Auto-retrospective on FAIL/WARN (closes the Reflexion verbal-RL loop) |
| v8.50 | May 9 | `./bin/preflight` full pipeline dry-run; opt-in to release pipeline |
| v8.51 | May 9 | Codex CLI hybrid adapter; cross-CLI capability tier resolver |
| v8.55 | May 9 | Sequential-write discipline codified in profile JSON; parallel_eligible enforcement |
| v8.56 | May 9 | Layer B `role-context-builder.sh` per-phase prompt assembly |
| v8.57 | May 10 | PASS-cycle memo (Layer P); carryoverTodos with `defer_count` tracking |
| v8.58 | May 10 | Per-batch cumulative cost cap with tripwire |
| v8.59 | May 10 | Triage default-on (Layer C); opt-out via `EVOLVE_TRIAGE_DISABLE=1` |
| v8.60 | May 10 | Budget-driven dispatch (`--budget-usd N`); cycle→cost migration begins |
| v9.0.0 | May 11 | Four-tier token-optimization rebuild: invocation context, cycle digest, role-filtered context, persona Layer 1/3 split |
| v9.0.1-5 | May 11 | Per-phase token fixes: intent 7→≤2 turns, scout 49→≤8-12, builder 58→≤15-20; cycle→cost doc closure |
| **v9.1.0** | **May 11** | **Checkpoint-resume + context-window control. `--resume` flag; per-cycle context-monitor.json; autotrim opt-in; reactive quota-likely classification; pre-emptive 80/95% thresholds; orchestrator resume-mode protocol** |
| v9.1.x | May 11 | Knowledge-base content model: separate runtime context (`docs/research/`) from developer-only reference (`docs/private/research/`); 42 archived files restored; three-layer cross-CLI exclusion; end-to-end resume bug fix (collision check + WORKTREE_PATH reset) |

### Incidents and recovery

| Cycles | What Happened | How It Was Fixed |
|--------|---------------|------------------|
| 102-111 | Reward hacking: agent inflated success metrics | Eval tamper detection, independent verification |
| 132-141 | Orchestrator gaming: skipped agents, fabricated cycles | Deterministic `phase-gate-precondition.sh` (kernel hook) |
| Gemini run | Forged audit reports during cross-platform run | Hash-chained ledger; ship.sh verifies artifact SHA, not LLM claim |
| Cycle 11 (this session) | Quota wall mid-build — entire worktree lost pre-v9.1.0 | v9.1.0 checkpoint-resume; survive subscription quota walls |
| v9.1.0 resume positive-path (post-ship) | INTEGRITY-FAIL + empty WORKTREE_PATH on `--resume` | v9.1.x patch: collision check and WORKTREE_PATH reset gated by `EVOLVE_RESUME_MODE`; end-to-end verification added |

These incidents led to the project's key architectural conviction: **structural constraints beat behavioral constraints**. Safety rules in prompts can be ignored; safety checks in bash scripts cannot.

## Project Structure

```
evolve-loop/
├── .claude-plugin/
│   ├── plugin.json              # Plugin manifest
│   └── marketplace.json         # Marketplace distribution
├── agents/                      # Agent personas (Scout, Triage, Builder, Auditor, Retrospective, Orchestrator)
├── skills/evolve-loop/          # Autonomous pipeline skill (symlinked from .agents/skills/)
├── skills/refactor/             # Refactoring pipeline skill
├── scripts/
│   ├── dispatch/                # Dispatcher, run-cycle, resume-cycle, subagent-run, CLI adapters
│   ├── lifecycle/               # ship.sh, cycle-state.sh, phase-gate.sh, role-context-builder.sh
│   ├── guards/                  # PreToolUse hooks: ship-gate, role-gate, phase-gate-precondition
│   ├── observability/           # show-cycle-cost.sh, show-context-monitor.sh, verify-ledger-chain.sh
│   └── tests/                   # Trust kernel + per-feature regression suites
├── docs/                        # Single doc root (v9.1.x+ consolidation)
│   ├── README.md                # Layout + distinguishing principle
│   ├── architecture/            # checkpoint-resume.md, context-window-control.md, private-context-policy.md, ...
│   ├── reference/               # Per-agent technique manuals
│   ├── guides/                  # How-to (operational tasks, publishing-releases.md)
│   ├── research/                # 5 ACTIVE research refs (agent-accessible on demand)
│   ├── operations/              # Release archive + release-notes/
│   ├── incidents/               # Forensic post-mortems
│   ├── reports/                 # Eval results, benchmarks
│   ├── private/                 # v9.1.x+: AGENT-CONTEXT-EXCLUDED research backlog
│   │   ├── README.md            # Convention + how runtime exclusion works
│   │   └── research/            # 42 ARCHIVED research refs (NOT loaded into agent context)
│   └── MOVED.md                 # (transitional) old→new path map; removed in v9.2.x or v9.3.x
├── bin/                         # Read-only operator commands
├── examples/                    # Annotated examples
├── install.sh
├── uninstall.sh
├── AGENTS.md                    # Cross-CLI source-of-truth (canonical)
├── CLAUDE.md                    # Claude Code overlay
├── GEMINI.md                    # Gemini CLI overlay
├── CONTRIBUTING.md
├── CHANGELOG.md
└── LICENSE (MIT)
```

### Workspace (generated per project on first cycle)

```
.evolve/
├── runs/cycle-N/                # Per-cycle artifacts (gitignored)
│   ├── intent.md
│   ├── scout-report.md
│   ├── triage-decision.md
│   ├── build-report.md
│   ├── audit-report.md
│   ├── context-monitor.json     # v9.1.0+ per-phase token usage
│   └── orchestrator-report.md
├── profiles/                    # Agent CLI permission profiles (shipped with plugin)
├── evals/                       # Eval definitions (gitignored)
├── instincts/lessons/           # Reflexion-style lessons (gitignored)
├── genes/                       # Reusable fix templates
├── state.json                   # Persistent state
├── ledger.jsonl                 # SHA-chained tamper-evident log
└── cycle-state.json             # Current cycle's phase state (or absent between cycles)
```

## Two-folder Content Model (v9.1.x+)

evolve-loop maintains two distinct content surfaces:

| Surface | Folder | Loaded into agent context? | Audience |
|---|---|---|---|
| Runtime context | `docs/research/`, `agents/`, `skills/`, `scripts/`, `docs/architecture/` | YES — agents read these | Agents + contributors |
| Developer knowledge base | `docs/private/` | NO — kernel-blocked across all CLIs | Contributors only |

The runtime exclusion uses three layers (OS sandbox + adapter permission-mode + Layer-B context-builder filter). See [docs/architecture/private-context-policy.md](docs/architecture/private-context-policy.md) for the architecture and [docs/private/README.md](docs/private/README.md) for the contributor-facing convention.

## Requirements

- An AI CLI that supports plugins (Claude Code, Gemini CLI, or Codex CLI)
- Git
- `jq` (used by the kernel scripts; install via `brew install jq` / `apt install jq`)

No other dependencies. The entire system is markdown + bash that AI agents interpret.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, including the "where to file research?" rule (active runtime ref → `docs/research/`; archived reference → `docs/private/research/`).

## Community

- **Code of Conduct**: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — adopts [Contributor Covenant 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/)
- **Security policy**: [SECURITY.md](SECURITY.md) — vulnerability reporting via GitHub Security Advisories
- **Cross-CLI standard**: skills live at both `skills/<name>/` (Claude Code primary) and `.agents/skills/<name>/` (Codex/Gemini open standard via symlinks). Edit either path; both resolve to the same SKILL.md.
- **AI agent instructions**: [AGENTS.md](AGENTS.md) is the canonical cross-CLI source-of-truth. CLI-specific overlays at [CLAUDE.md](CLAUDE.md) and [GEMINI.md](GEMINI.md).

## License

[MIT](LICENSE) — Copyright (c) 2026 Dan Lee

## Links

- [Documentation index](docs/index.md) — all reference docs
- [Research index](docs/research-index.md) — 5 active refs + 42 archived
- [Architecture docs](docs/architecture/) — phase architecture, checkpoint-resume, context-window-control, private-context-policy
- [Changelog](CHANGELOG.md)
- [Releases](https://github.com/mickeyyaya/evolve-loop/releases)
