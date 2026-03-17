# Cycle 22 Scout Report

## Discovery Summary
- Scan mode: full (cycle 22, mode override to full — no prior digest in context)
- Files analyzed: 18 (all agent definitions, skill files, docs, instincts, state)
- Research: performed (cooldown expired, 3 queries executed)
- Instincts applied: 2 (inst-013: progressive-disclosure, inst-015: signal-to-action wiring)
- **instinctsApplied:** [inst-013 (guided task scoping — avoid inline duplication), inst-015 (signal-to-action wiring — each new mechanism must connect to a decision trigger)]

## Key Findings

### Architecture — MEDIUM
- `skills/evolve-loop/phases.md` is at 651 lines (81% of the 800-line threshold) with 31 commits — highest churn in the codebase. It is the single largest blast-radius file. Growth continues as more mechanisms are added. No split has been proposed yet.
- The Scout task selection algorithm is purely priority-based (linear ranking). There is no feedback loop from task *outcome* to future task *selection probability*. The loop learns instincts but doesn't directly adjust which task types it prefers based on reward history. This is a missing adaptive mechanism.

### Features — HIGH (goal-driven: inspiration)
- **Multi-armed bandit task selection** — The current task prioritization is static: the Scout ranks by priority tier and picks top 2-4. There is no mechanism that tracks which *types* of tasks (features, stability, security, techdebt) have historically yielded high process-reward scores and biases future selection toward high-reward arms. This is directly inspired by MAB/Thompson Sampling research.
- **Counterfactual cycle log** — When the Scout defers a task, there is no record of what the cycle *would have* looked like had that task been selected. Adding a lightweight "shadow run" field to deferred tasks (estimated complexity, predicted reward) would enable the loop to test whether its deferrals were good decisions — closing a causal reasoning gap.
- **Semantic task crossover** — The planCache stores reusable templates per task type, but tasks are always selected independently. Inspired by LLM crossover research (2025), the Scout could *recombine* attributes from two successful past tasks to generate novel task proposals that inherit traits from both parents — a primitive genetic crossover at the task specification level.

### Code Quality — LOW
- `phases.md` approaching 800-line ceiling. Monitor; no immediate split needed this cycle.
- `instinctSummary` in state.json is an empty array despite 14+ instinct files existing. This is a pre-existing gap, not introduced this cycle. The instinct summary should be populated so agents can apply learned patterns without reading full YAML files.

### Introspection Pass
Reviewing evalHistory delta metrics (cycles 17-21):
- `instinctsExtracted`: 1, 1, 1, 1, 2 — stable, minor uptick cycle 21
- `auditIterations`: 1.0 across all 5 cycles — Builder is highly efficient, no guidance needed
- `successRate`: 1.0 across all 5 cycles — proficient mastery confirmed, ready for novel/ambitious tasks
- `stagnationPatterns`: 0 across all 5 cycles — no stagnation
- `pendingImprovements`: empty — no remediation needed

**Introspection heuristic results:**
- instinctsExtracted >= 1 each cycle: no enrichment task triggered
- auditIterations == 1.0 avg: no Builder guidance needed
- stagnationPatterns == 0: no diversity broadening triggered
- successRate == 1.0: no task-sizing regression
- pendingImprovements empty: no remediation tasks

**Capability gap scanner:**
- inst-013 (confidence 0.65, not graduated): cited in cycle 18 and 21, active — not dormant
- inst-015 (confidence 0.7): cited in cycle 19, applied this scan — not dormant
- inst-016 (confidence 0.7): cited in cycle 20 — not dormant
- inst-017 (confidence 0.6): new in cycle 21 — no deferred tasks with passed revisitAfter dates found

No capability-gap candidates triggered. All introspection heuristics clear.

## Research

**Query 1:** "self-improving AI agent architecture novel mechanisms 2025 2026 emergent capabilities"
Key finding: Comprehensive taxonomy reveals "what to evolve" axes include model parameters, prompts, memory, toolsets, or *agent populations*. Population-based and reward-based algorithms provide different exploration tradeoffs. Meta-evolution of strategy prompts and workflow graphs is an emerging trajectory. (source: [arxiv.org/abs/2508.07407](https://arxiv.org/abs/2508.07407))

**Query 2:** "population-based agent evolution genetic algorithms prompt mutation crossover LLM 2025"
Key finding: LLMs can serve as the crossover and mutation operator in genetic algorithms, generating semantically valid offspring that inherit traits from parent solutions (Language Model Crossover, Autodesk Research 2023; EVOPROMPT 2025). CycleQD uses model merging as crossover for quality-diversity evolution. (source: [arxiv.org/pdf/2309.08532](https://arxiv.org/pdf/2309.08532), [proceedings.iclr.cc](https://proceedings.iclr.cc/paper_files/paper/2025/file/755acd0c7c07180d78959b6d89768207-Paper-Conference.pdf))

**Query 3:** "multi-armed bandit exploration exploitation AI agent task selection adaptive scheduling 2025"
Key finding: MAT-Agent (2025) deploys MAB algorithms per agent for adaptive task scheduling, using both ε-greedy and UCB strategies. Adaptive Budgeted Bandits integrate real-time constraint satisfaction with theoretical sublinear regret guarantees. LLMs can predict reward drift and adjust exploration-exploitation tradeoffs dynamically. (source: [openreview.net/forum?id=YDWRTYgR79](https://openreview.net/forum?id=YDWRTYgR79))

**Query 4:** "counterfactual reasoning AI agent what-if planning alternative history simulation 2025"
Key finding: Agentic systems are beginning to run multiple "what if" simulations before acting. Stanford HAI research shows counterfactual reasoning is critical for judging causation and assigning responsibility in complex systems. Counterfactual learning enhances resilience in autonomous agent systems by training on missed opportunities. (source: [frontiersin.org](https://www.frontiersin.org/journals/artificial-intelligence/articles/10.3389/frai.2023.1212336/full))

## Selected Tasks

### Task 1: add-bandit-task-selector
- **Slug:** add-bandit-task-selector
- **Type:** feature
- **Complexity:** M
- **Rationale:** The evolve-loop has 21 cycles of reward data (evalHistory, processRewards) but never uses it to bias future task selection. Adding a multi-armed bandit mechanism to the Scout's task selection — tracking per-task-type reward history and applying Thompson Sampling-style weighting — would make the loop genuinely adaptive rather than purely priority-based. This is the most direct path from current architecture to a self-improving selection algorithm. Connects to inst-015 (signal-to-action wiring): process rewards exist, now wire them to selection probability.
- **Acceptance Criteria:**
  - [ ] `state.json` schema documented to include `taskArms` with per-type reward history (`feature`, `stability`, `security`, `techdebt`, `performance`)
  - [ ] Scout task selection section in `SKILL.md` describes bandit weighting: tasks of historically high-reward types receive a priority boost (max +1 level)
  - [ ] `memory-protocol.md` documents `taskArms` schema with fields: `type`, `pulls`, `totalReward`, `avgReward`
  - [ ] `docs/architecture.md` updated to include bandit selection in self-improvement infrastructure section
- **Files to modify:** `skills/evolve-loop/SKILL.md`, `skills/evolve-loop/memory-protocol.md`, `docs/architecture.md`
- **Eval:** written to `evals/add-bandit-task-selector.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "bandit\|UCB\|Thompson\|exploration\|exploitation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md` → expects exit 0 (count >= 1)
  - `grep -c "taskArms\|armRewards\|banditState\|explorationRate" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects exit 0 (count >= 1)
  - `grep -c "Multi-Armed Bandit\|bandit" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md` → expects exit 0 (count >= 1)

### Task 2: add-counterfactual-cycle-log
- **Slug:** add-counterfactual-cycle-log
- **Type:** feature
- **Complexity:** S
- **Rationale:** When the Scout defers a task it records `decision: "deferred"` but loses all reasoning about what the cycle would have looked like had it been selected. Adding a lightweight counterfactual annotation to deferred tasks — predicted complexity, estimated reward, alternate acceptance criteria — creates a permanent record that allows the loop to retrospectively evaluate whether its deferrals were optimal. This closes a causal reasoning gap: the loop currently only learns from what it *did*, never from what it *chose not to do*. Directly inspired by counterfactual learning research showing it enhances resilience in autonomous systems.
- **Acceptance Criteria:**
  - [ ] `memory-protocol.md` documents `counterfactual` field on deferred evaluatedTask entries: `{predictedComplexity, estimatedReward, alternateApproach, deferralReason}`
  - [ ] Scout agent definition (`evolve-scout.md`) updated: when deferring a task, populate the counterfactual annotation
  - [ ] `phases.md` Phase 5 LEARN step updated to optionally review counterfactual accuracy (were deferred tasks later completed? did actual reward match prediction?)
- **Files to modify:** `skills/evolve-loop/memory-protocol.md`, `agents/evolve-scout.md`, `skills/evolve-loop/phases.md`
- **Eval:** written to `evals/add-counterfactual-cycle-log.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "counterfactual\|what-if\|alternate\|shadow" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects exit 0 (count >= 1)
  - `grep -c "counterfactual\|what-if" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects exit 0 (count >= 1)
  - `grep -c "counterfactualLog\|shadowRun\|alternateTask\|counterfactual" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects exit 0 (count >= 1)

### Task 3: add-semantic-task-crossover
- **Slug:** add-semantic-task-crossover
- **Type:** feature
- **Complexity:** S
- **Rationale:** The Scout currently selects tasks independently from the backlog. Inspired by genetic crossover research for LLMs (2025), the Scout could synthesize *novel* task proposals by recombining attributes from two high-performing past tasks from the planCache: take the "what" from one successful task and the "how" from another. For example: crossover `add-section-to-file` (type: documentation) with `add-process-rewards-remediation-loop` (type: feedback-wiring) to produce a novel task like "add reward-driven section pruning to docs". This is not just reusing templates — it's generating genuinely new task proposals the human designer never specified.
- **Acceptance Criteria:**
  - [ ] Scout agent definition updated: after initial task selection, if planCache has 4+ entries with `successCount >= 2`, attempt one crossover proposal by combining attributes from two high-performing cache entries
  - [ ] Crossover proposal is labeled `source: "crossover"` and competes in normal task selection (not automatically selected)
  - [ ] `memory-protocol.md` documents `crossoverLog` — a list of crossover-generated task slugs with their parent slugs and actual outcome (if built)
- **Files to modify:** `agents/evolve-scout.md`, `skills/evolve-loop/memory-protocol.md`
- **Eval:** written to `evals/add-semantic-task-crossover.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "crossover\|recombine\|offspring\|parent" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects exit 0 (count >= 1)
  - `grep -c "crossoverLog\|crossoverEnabled\|semanticCrossover\|crossover" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects exit 0 (count >= 1)

## Deferred
- **phases.md split** — At 651 lines, the file is approaching the 800-line ceiling but is not there yet. The risk is MEDIUM, not critical. Deferring until it exceeds 700 lines or a specific section becomes independently useful.
- **instinctSummary population** — The empty `instinctSummary` array in state.json is a gap, but fixing it requires runtime state updates during Phase 5 (not a Scout concern). Deferring to a dedicated repair cycle or manual update.
- **Island model activation** — Documented in `docs/island-model.md` but never activated. Would require a new `/evolve-loop --island` argument parser and multi-state file management. Complexity L — deferred until mastery level is confirmed stable for 3+ more meta-cycles.
