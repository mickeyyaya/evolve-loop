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

## Adding a New Agent

1. Create the agent file in `agents/evolve-<role>.md` with full frontmatter (`name`, `description`, `tools`, `model`)
2. Define clear inputs (JSON context block), responsibilities, and outputs
3. Assign exactly one workspace file
4. Add the agent path to `.claude-plugin/plugin.json` agents array
5. Update `skills/evolve-loop/phases.md` with when the agent runs
6. Update `skills/evolve-loop/memory-protocol.md` with the new workspace file
7. Update `skills/evolve-loop/SKILL.md` with the agent table entry
8. Update `CHANGELOG.md`
