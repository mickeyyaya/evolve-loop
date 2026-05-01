---
name: agent-templates
description: Base schemas and context blocks for all agents.
tools: []
---

# Agent Templates — Shared Schemas

Shared input/output schemas for evolve-loop agents. Each agent references this file instead of duplicating boilerplate. Agent-specific fields are documented in the individual agent files.

## Agent Definition Schema

Every agent file MUST include these frontmatter fields (in addition to `name`, `description`, `model`, `capabilities`, `tools`):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `perspective` | string | **yes** | The evaluative lens through which this agent interprets every input. Sets the default bias for all judgments. Example: `"adversarial reviewer seeking failure modes"`. |
| `output-format` | string | **yes** | Canonical structure of this agent's primary artifact — file name + section list. Enables deterministic merging in parallel fan-out scenarios. Example: `"audit-report.md — Verdict, Defect Table, Eval Gate result"`. |

**Why these fields matter:** In parallel builder fan-out (multiple specialist builders per cycle), the Orchestrator uses `output-format` to identify merge semantics automatically. The `perspective` field is injected verbatim into the agent's system prompt preamble, replacing fragile inline persona instructions.

---

## Shared Context Block

All agents receive a JSON context block with these common fields:

| Field | Type | Description |
|-------|------|-------------|
| `cycle` | number | Current cycle number |
| `workspacePath` | string | Path to `.evolve/workspace/` |
| `strategy` | string | Evolution strategy: `balanced`, `innovate`, `harden`, `repair`, `ultrathink` |
| `challengeToken` | string | Per-cycle random hex token — embed in workspace output header and ledger entry |
| `instinctSummary` | array | Compact instinct array from state.json (inline) |

| `budgetRemaining` | object | Token/cycle budget awareness — see Budget-Aware Behavior below |

Agent-specific additions (e.g., `task`, `buildReport`, `mode`, `projectContext`) are documented in each agent file.

## Budget-Aware Behavior

Every agent receives a `budgetRemaining` object in context. Agents should adapt their behavior based on remaining resources — this is **not** a hard limit, but a signal for self-regulation. (Research basis: BATS framework [arXiv:2511.17006] — budget-aware agents self-regulate without additional training.)

```json
{
  "budgetRemaining": {
    "cyclesLeft": 7,
    "estimatedTokensLeft": 140000,
    "budgetPressure": "low|medium|high"
  }
}
```

| Pressure | Meaning | Agent Behavior |
|----------|---------|----------------|
| **low** | >60% budget remaining | Explore broadly, full analysis, comprehensive output |
| **medium** | 30-60% remaining | Focus on highest-priority items, trim verbose output |
| **high** | <30% remaining | Minimal output, skip optional sections, fastest path to completion |

The orchestrator computes `budgetPressure` at cycle start:
- `low`: `cyclesLeft / totalCycles > 0.6`
- `medium`: `cyclesLeft / totalCycles` between 0.3 and 0.6
- `high`: `cyclesLeft / totalCycles < 0.3`

Agents should **not** refuse to work under high pressure — they should work more efficiently. For example, Scout under high pressure selects 1-2 tasks instead of 3-4. Builder under high pressure skips alternative analysis in the design step.

## Strategy Handling

Adapt behavior based on the active `strategy` from context. See SKILL.md Strategy Presets table for definitions:

- **`balanced`** — standard approach, mixed focus
- **`innovate`** — prefer additive changes, relaxed on style
- **`harden`** — defensive coding, strict on all dimensions
- **`repair`** — fix-only, smallest diff, strict on regressions
- **`ultrathink`** — maximum reasoning budget, stepwise confidence

Each agent applies strategy to its own domain:
- **Scout:** adapts discovery scope and task selection priorities
- **TDD Engineer:** adapts test depth and coverage threshold enforcement
- **Builder:** adapts implementation approach and risk tolerance
- **Auditor:** adapts audit strictness and checklist depth

## Skill Awareness

Agents may receive `recommendedSkills` in their task context — a compact list of external skills (from installed plugins) that the orchestrator or Scout matched to the current task.

**Schema:**

```json
"recommendedSkills": [
  {"name": "everything-claude-code:security-review", "priority": "primary", "rationale": "security-type task"},
  {"name": "python-review-patterns", "priority": "supplementary", "rationale": "Python codebase"}
]
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Skill name as registered (e.g., `"everything-claude-code:security-review"`) |
| `priority` | string | `"primary"` (strongly relevant — invoke before design) or `"supplementary"` (nice to have — invoke only if needed) |
| `rationale` | string | Why this skill is recommended (under 50 chars) |

**Rules:**
- 0-3 skills per task (compact — adds ~200 tokens to context)
- Agents invoke via the `Skill` tool — invocation is **optional**, based on agent judgment
- Under budget pressure (medium/high): invoke at most 1 primary skill
- Skip supplementary skills if an applied instinct already covers the pattern
- Each invocation costs ~2-5K tokens

**Cross-platform & Fallbacks:** Use your platform's native skill invocation method (e.g., the `Skill` tool on Claude Code). On Gemini CLI / generic platforms, read the skill's `SKILL.md` file directly if available at the path in the skill inventory. If a specific recommended skill (e.g., `everything-claude-code:security-review`) is not found in the inventory, search the inventory for the closest available alternative in the same category.

## Shared Output Conventions

### Challenge Token

Embed `challengeToken` from context in your workspace output file header as an HTML comment:
```markdown
<!-- Challenge: {challengeToken} -->
```
Also include the token in your ledger entry under `data.challenge`.

### Ledger Entry

Every agent writes a structured ledger entry on completion. Common fields:

```json
{
  "ts": "<ISO-8601>",
  "cycle": "<N>",
  "role": "<scout|builder|auditor>",
  "type": "<discovery|build|audit>",
  "data": {
    "challenge": "<challengeToken>",
    "prevHash": "<hash of previous ledger entry>",
    "...": "<agent-specific fields>"
  }
}
```

Agent-specific `data` fields are defined in each agent file's Ledger Entry section.

### Mailbox Protocol

- **On start:** Read `workspace/agent-mailbox.md` for messages addressed to you (by role name) or `to: "all"`. Apply any hints, flags, or persistent warnings from prior agents.
- **On completion:** Post messages for other agents if you identified concerns worth carrying forward (e.g., fragile files, recurring smells, follow-up suggestions).
- Use `persistent: true` only for concerns spanning multiple cycles.

---

## Pipeline Agents

The full evolve-loop pipeline and the agent responsible for each phase:

| Phase | Agent | File | Output Artifact |
|-------|-------|------|-----------------|
| Calibrate | Orchestrator | `evolve-orchestrator.md` | cycle-state.json |
| Research / Discover | Scout | `evolve-scout.md` | `scout-report.md` |
| Test Contract (TDD) | TDD Engineer | `evolve-tdd-engineer.md` | `test-report.md` |
| Build | Builder | `evolve-builder.md` | `build-report.md` |
| Audit | Auditor | `evolve-auditor.md` | `audit-report.md` |
| Ship | Orchestrator / ship.sh | `evolve-orchestrator.md` | commit SHA |
| Learn | Retrospective | `evolve-retrospective.md` | instinct entries |

**TDD Engineer contract:** Runs after Scout selects a task and before Builder implements. Writes failing tests that encode acceptance criteria (RED phase). Builder must make those tests pass without modifying them. See [evolve-tdd-engineer.md](evolve-tdd-engineer.md) for the full workflow.

**Phase sequence enforcement:** `phase-gate-precondition.sh` blocks out-of-order agent invocations. The TDD engineer phase (`tdd`) must be advanced via `cycle-state.sh advance tdd tdd-engineer` before Builder can be invoked.
