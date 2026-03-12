---
name: evolve-loop
description: "Orchestrates a self-evolving development cycle with 13 role-specialized agents (Operator, PM, Researcher, Scanner, Planner, Architect, Developer, Reviewer, E2E Runner, Security Reviewer, Eval Runner, Deployer) across 8 phases. Integrates ECC battle-tested agents, eval-based quality gating, continuous learning via instinct extraction, and loop monitoring. Use when running autonomous improvement loops on a codebase."
argument-hint: "[cycles] [goal]"
disable-model-invocation: true
---

# Evolve Loop v3

Orchestrates 13 specialized agents through 8 phases per cycle, maximizing parallel execution. Integrates Everything Claude Code (ECC) agents for architecture, TDD, code review, E2E testing, security review, and loop operation. Features eval-based hard gating, instinct extraction for continuous learning, and loop operator monitoring.

**Usage:** `/evolve-loop [cycles] [goal]`

## Argument Parsing

Parse `$ARGUMENTS` as follows:
- If the first token is a number → use it as `cycles`, remainder is `goal`
- If the first token is NOT a number → `cycles` defaults to 2, entire input is `goal`
- If empty → `cycles` = 2, `goal` = null (autonomous discovery mode)

Examples:
- `/evolve-loop` → cycles=2, goal=null
- `/evolve-loop 3` → cycles=3, goal=null
- `/evolve-loop 1 add dark mode support` → cycles=1, goal="add dark mode support"
- `/evolve-loop add user authentication` → cycles=2, goal="add user authentication"

## Goal Modes

**With goal (directed mode):** All discovery agents (PM, Researcher, Scanner) and the Planner focus their work toward achieving the goal. The PM assesses what's relevant to the goal, the Researcher searches for approaches and best practices related to the goal, the Scanner identifies code areas the goal will touch, and the Planner selects tasks that advance the goal.

**Without goal (autonomous mode):** Agents perform broad discovery — the PM evaluates all 8 dimensions, the Researcher searches for general trends, the Scanner does a full codebase audit, and the Planner picks the highest-impact work. This is the original behavior.

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

## Initialization (once per session)

1. Ensure directories exist:
   ```bash
   mkdir -p .claude/evolve/workspace .claude/evolve/history .claude/evolve/evals .claude/evolve/instincts/personal
   ```

2. Read `.claude/evolve/state.json` if it exists. If not, initialize:
   ```json
   {"lastUpdated":"<now>","costBudget":null,"research":{"queries":[]},"evaluatedTasks":[],"failedApproaches":[],"evalHistory":[],"instinctCount":0,"operatorWarnings":[],"nothingToDoCount":0}
   ```

3. Auto-detect project context (language, framework, test commands, domain). Store as `projectContext`.

## Orchestrator Loop

You are the orchestrator. For each cycle:
1. Parse arguments into `cycles` and `goal` (see Argument Parsing above)
2. Pass `goal` (or null) in the context block to every agent
3. Launch agents via the Agent tool — **maximize parallel launches**
4. Collect results and verify workspace files
5. Handle gates (operator HALT, user approval, eval gate, fix loops, exit conditions)

**Key rule:** When agents have no data dependencies, launch them in a **single message with multiple Agent tool calls**.

For detailed phase-by-phase instructions, see [phases.md](phases.md).
For the shared memory protocol (ledger format, workspace conventions, state.json schema), see [memory-protocol.md](memory-protocol.md).
For the eval hard gate instructions, see [eval-runner.md](eval-runner.md).

## Dependencies

**Required:** [Everything Claude Code](https://github.com/anthropics/everything-claude-code) (ECC) plugin must be installed. Five agents use ECC subagent_types at runtime:

| ECC subagent_type | Used by |
|-------------------|---------|
| `everything-claude-code:architect` | Phase 3: Architect |
| `everything-claude-code:tdd-guide` | Phase 4: Developer |
| `everything-claude-code:code-reviewer` | Phase 5: Reviewer |
| `everything-claude-code:e2e-runner` | Phase 5: E2E Runner |
| `everything-claude-code:security-reviewer` | Phase 5: Security |

## Agent Definitions

Agent files live in `~/.claude/agents/`. There are two types:

**Custom agents** — full self-contained instructions, launched with `subagent_type: general-purpose`:

| Role | Agent File | Model | Workspace File |
|------|-----------|-------|----------------|
| Operator | `evolve-operator.md` | sonnet | `loop-operator-log.md` |
| PM | `evolve-pm.md` | sonnet | `briefing.md` |
| Researcher | `evolve-researcher.md` | sonnet | `research-report.md` |
| Scanner | `evolve-scanner.md` | sonnet | `scan-report.md` |
| Planner | `evolve-planner.md` | opus | `backlog.md` + `evals/*.md` |
| Deployer | `evolve-deployer.md` | sonnet | `deploy-log.md` |

**ECC context overlays** — thin files (~40 lines) that add evolve-specific context on top of ECC agents. Launched with the corresponding ECC `subagent_type`:

| Role | Agent File | ECC subagent_type | Model | Workspace File |
|------|-----------|-------------------|-------|----------------|
| Architect | `evolve-architect.md` | `everything-claude-code:architect` | opus | `design.md` |
| Developer | `evolve-developer.md` | `everything-claude-code:tdd-guide` | sonnet | `impl-notes.md` |
| Reviewer | `evolve-reviewer.md` | `everything-claude-code:code-reviewer` | sonnet | `review-report.md` |
| E2E Runner | `evolve-e2e.md` | `everything-claude-code:e2e-runner` | sonnet | `e2e-report.md` |
| Security | `evolve-security.md` | `everything-claude-code:security-reviewer` | sonnet | `security-report.md` |

**Eval Runner** — not an agent; orchestrator-executed instructions in [eval-runner.md](eval-runner.md). Writes `eval-report.md`.

**Context overlay pattern:** The orchestrator reads the overlay file and passes its content as the prompt. The ECC agent's built-in instructions are loaded automatically via `subagent_type`. The overlay adds only evolve-specific concerns: workspace file ownership, input context, output format, ledger entry. No ECC content is duplicated.

## Anti-Patterns

1. **Serializing independent agents** — PM, Researcher, Scanner have NO data dependencies → always launch all 3 in parallel
2. **Proceeding before reviewers finish** — WAIT for all parallel agents
3. **Same context for author and reviewer** — Use separate Agent calls
4. **No cross-iteration context** — Always read/write notes.md
5. **Retrying the same failure** — Log in state.json, try alternative next cycle
6. **Skipping the review barrier** — All 3 verifiers (Reviewer + E2E + Security) must pass
7. **Agents writing outside their workspace file** — Each agent owns one file
8. **Skipping the eval gate** — Phase 5.5 is a HARD gate. Never proceed to SHIP without PASS.
9. **Ignoring instincts** — Developer and Planner MUST read instincts when available
10. **Ignoring HALT** — When Loop Operator returns HALT, the orchestrator MUST pause and present issues to user
