# Writing Custom Agents

Guide for creating new evolve-loop agents or modifying existing ones.

## Agent File Format

```markdown
---
name: evolve-<role>
description: <one-line description of what this agent does>
tools: ["Read", "Grep", "Glob", "Bash"]  # tools this agent needs
model: sonnet  # or opus
---

# Agent Name

<core instructions — what the agent does>

## Inputs

You will receive a JSON context block with:
- `cycle`: current cycle number
- `workspacePath`: path to `.claude/evolve/workspace/`
- `ledgerPath`: path to `.claude/evolve/ledger.jsonl`
- <additional context fields>

## Responsibilities

<what the agent does, step by step>

## Output

### Workspace File: `workspace/<filename>.md`
<output format>

### Ledger Entry
```json
{"ts":"<ISO-8601>","cycle":<N>,"role":"<role>","type":"<type>","data":{...}}
```
```

## Rules

1. **One workspace file per agent.** Each agent writes exactly one file. If you need to split responsibilities, create separate agents.

2. **READ-ONLY for auditors.** The Auditor must not modify source code. It can only write to its workspace file.

3. **Ledger entry required.** Every agent must append one JSONL entry to the ledger with ts, cycle, role, type, and structured data.

4. **Context block as input.** Agents receive their inputs via a JSON context block in the prompt.

5. **Self-evolution focus.** Every instruction should serve the evolution goal. Skip generic best practices — be specific about what matters for autonomous improvement.

## Current Agents

| Agent | File | Purpose |
|-------|------|---------|
| Scout | `evolve-scout.md` | Discovery + analysis + task selection |
| Builder | `evolve-builder.md` | Design + implement + self-test |
| Auditor | `evolve-auditor.md` | Review + security + eval gate |
| Operator | `evolve-operator.md` | Loop health monitoring |

## Efficiency Guidelines

Agent and skill prompts consume tokens on every invocation. Efficient prompts reduce cost and improve instruction adherence.

### 1. Target 150 Lines Max Per Agent

Frontier LLMs follow ~150-200 discrete instructions with reasonable consistency. Beyond that, compliance degrades. Keep agent files concise by focusing on role-specific behavior, not generic best practices the model already handles.

### 2. Use Progressive Disclosure

Tell agents *where to find* information rather than embedding it inline. Pass references via the context block instead of duplicating content across agent files.

**Do:** `"strategy": "harden"` in context — agent reads strategy name and adapts
**Don't:** Copy the full strategy definitions table into every agent file

### 3. Eliminate Cross-Agent Duplication

Shared concepts (strategy definitions, output formats, ledger schemas) should be defined once and referenced. If 4 agents each contain the same 20-line section, extract it to a shared reference.

### 4. Order Context Blocks for Cache Reuse

Structure agent context with **static → semi-stable → dynamic** ordering:
- **Static:** Project context, workspace paths, goal, strategy (stable across cycles)
- **Semi-stable:** Instinct summary, state.json extracts (changes every few cycles)
- **Dynamic:** Cycle number, changed files, recent notes (changes every cycle)

This ordering maximizes prefix-cache hits when the model sees similar context across invocations.

### 5. Compress Output Templates

Output template sections (workspace file format, ledger entry schemas) are verbose. Use minimal field lists rather than full markdown examples — agents can infer formatting from a compact specification.

### 6. Pass Only Relevant Context

When launching an agent, extract only the sections it needs. Don't pass the full scout-report to the Builder — pass only the specific task object. Don't pass full state.json — pass the relevant slices (instinctSummary, mastery, tokenBudget).

### 7. Measure and Track

Use `skillMetrics` in state.json to track prompt overhead over time. After any agent prompt change, re-measure line counts and estimated tokens. Regressions in prompt size should be justified by corresponding gains in effectiveness.

## Adding a New Agent

1. Create the agent file in `agents/evolve-<role>.md` with full frontmatter (`name`, `description`, `tools`, `model`)
2. Define clear inputs (JSON context block), responsibilities, and outputs
3. Assign exactly one workspace file
4. Add the agent path to `.claude-plugin/plugin.json` agents array
5. Update `skills/evolve-loop/phases.md` with when the agent runs
6. Update `skills/evolve-loop/memory-protocol.md` with the new workspace file
7. Update `skills/evolve-loop/SKILL.md` with the agent table entry
8. Update `CHANGELOG.md`
