# Cycle 1 Design

## Task 1: fix-wt-worktree-references

### ADR-001: Replace `wt` CLI with Claude Code built-in worktree tools

- **Context:** `evolve-developer.md` references `wt switch --create` and `evolve-deployer.md` references `wt merge`. The `wt` CLI does not exist in users' environments. Claude Code provides built-in `EnterWorktree` and `ExitWorktree` tools.
- **Decision:** Replace all `wt` CLI references with Claude Code's built-in tools. For merge workflow, use explicit standard git commands.
- **Alternatives:** Document `wt` as prerequisite (rejected: not a real tool), use raw `git worktree` (rejected: Claude Code tools are idiomatic).
- **Rationale:** Claude Code's tools handle worktree lifecycle automatically and are available in all Claude Code environments.

### File Changes
| Action | File | Description |
|--------|------|-------------|
| MODIFY | `agents/evolve-developer.md` | Replace `wt switch --create` with `EnterWorktree` tool reference |
| MODIFY | `agents/evolve-deployer.md` | Replace `wt merge` block with `ExitWorktree` + standard git merge commands |

### Implementation Order
1. Fix evolve-developer.md (line 33)
2. Fix evolve-deployer.md (lines 41-49, line 88)

---

## Task 2: fix-docs-and-ledger-consistency

### ADR-002: Rewrite ECC wrapper guidance to context-overlay pattern

- **Context:** `docs/writing-agents.md` instructs contributors to "Copy the full content of the ECC agent file" — contradicts v3.1 thin overlay architecture.
- **Decision:** Rewrite to describe context-overlay pattern with `subagent_type` delegation.
- **Rationale:** Contributors following old guidance create bloated, duplicated agent files.

### ADR-003: Normalize ledger field names to `"ts"`

- **Context:** Ledger entries use both `"timestamp"` and `"ts"`. Spec says `"ts"`.
- **Decision:** Normalize all entries to canonical `"ts"` schema.
- **Rationale:** Consistency for tooling that parses the ledger.

### File Changes
| Action | File | Description |
|--------|------|-------------|
| MODIFY | `docs/writing-agents.md` | Rewrite "Creating an ECC Wrapper" section |
| MODIFY | `.claude/evolve/ledger.jsonl` | Normalize entries to canonical schema |

### Implementation Order
1. Fix writing-agents.md (lines 58-69)
2. Normalize ledger.jsonl entries
