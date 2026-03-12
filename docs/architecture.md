# Architecture

## Overview

Evolve Loop is a multi-agent pipeline that orchestrates 13 specialized AI agents across 8 phases. The orchestrator (the main Claude Code session) coordinates agents via the Agent tool, managing data flow through a shared workspace and JSONL ledger.

## Design Principles

### 1. Agent Isolation
Each agent has a single responsibility and owns exactly one workspace file. Agents communicate through workspace files, not direct messaging.

### 2. Parallel Where Possible
Agents with no data dependencies run in parallel (Phase 1: 3 agents, Phase 5: 3 agents). Sequential phases have explicit dependencies.

### 3. Hard Gates
Two gate types prevent bad code from shipping:
- **User approval gate** (Phase 2) — user confirms task selection
- **Eval hard gate** (Phase 5.5) — automated quality checks must pass

### 4. Continuous Learning
Each cycle extracts instincts (patterns) that future cycles can read. This creates a feedback loop where the system improves over time.

### 5. Safe Autonomy
The Loop Operator monitors for stalls, cost drift, and repeated failures. It can HALT the loop, requiring human intervention.

## Phase Dependencies

```
Phase 0 → Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 4.5 → Phase 5 → Phase 5.5 → Phase 6 → Phase 7
                                                                    ↑                       |
                                                                    └───── retry loop ──────┘
```

The retry loop (Phase 5/5.5 → Phase 4 → Phase 5/5.5) runs up to 3 times if the eval gate fails.

## Shared Memory Architecture

### Layer 1: JSONL Ledger
Append-only structured log. Every agent appends one entry per invocation. Used for timing analysis, cost tracking, and audit trail.

### Layer 2: Markdown Workspace
Human-readable files overwritten each cycle. Each agent reads upstream files and writes its own output file.

### Layer 3: Persistent State
`state.json` persists across cycles: research cache, task history, failed approaches, eval history, instinct count.

### Layer 4: Eval State
Eval definitions (created by Planner) and baseline results (for regression comparison).

### Layer 5: Instincts
Extracted patterns from completed cycles. YAML files with confidence scoring that evolve over time.

## ECC Integration

Five agents delegate to ECC via `subagent_type` (the Operator uses `general-purpose` since no ECC subagent_type exists):

| Evolve Agent | ECC subagent_type |
|-------------|-------------------|
| Architect | `everything-claude-code:architect` |
| Developer | `everything-claude-code:tdd-guide` |
| Reviewer | `everything-claude-code:code-reviewer` |
| E2E Runner | `everything-claude-code:e2e-runner` |
| Security | `everything-claude-code:security-reviewer` |

**Context overlay pattern:** Each evolve agent file (~40 lines) contains only evolve-specific context — inputs, outputs, workspace ownership, ledger format. The ECC agent's full instructions load automatically via `subagent_type`. The orchestrator reads the overlay and passes it as the prompt.

Benefits over the v3.0 "copy ECC content" approach:
- **Always current** — ECC updates apply immediately, no manual sync
- **Small files** — ~40 lines instead of ~150-200, saves context window
- **No duplication** — users with ECC installed don't get instructions twice
- **Clean separation** — ECC owns expertise, evolve owns workflow integration
