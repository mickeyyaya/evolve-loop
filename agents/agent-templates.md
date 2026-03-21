# Agent Templates — Shared Schemas

Shared input/output schemas for evolve-loop agents. Each agent references this file instead of duplicating boilerplate. Agent-specific fields are documented in the individual agent files.

## Shared Context Block

All agents receive a JSON context block with these common fields:

| Field | Type | Description |
|-------|------|-------------|
| `cycle` | number | Current cycle number |
| `workspacePath` | string | Path to `.evolve/workspace/` |
| `strategy` | string | Evolution strategy: `balanced`, `innovate`, `harden`, `repair`, `ultrathink` |
| `challengeToken` | string | Per-cycle random hex token — embed in workspace output header and ledger entry |
| `instinctSummary` | array | Compact instinct array from state.json (inline) |

Agent-specific additions (e.g., `task`, `buildReport`, `mode`, `projectContext`) are documented in each agent file.

## Strategy Handling

Adapt behavior based on the active `strategy` from context. See SKILL.md Strategy Presets table for definitions:

- **`balanced`** — standard approach, mixed focus
- **`innovate`** — prefer additive changes, relaxed on style
- **`harden`** — defensive coding, strict on all dimensions
- **`repair`** — fix-only, smallest diff, strict on regressions
- **`ultrathink`** — maximum reasoning budget, stepwise confidence

Each agent applies strategy to its own domain:
- **Scout:** adapts discovery scope and task selection priorities
- **Builder:** adapts implementation approach and risk tolerance
- **Auditor:** adapts audit strictness and checklist depth

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
