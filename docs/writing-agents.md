# Writing Custom Agents

Guide for creating new evolve-loop agents or modifying existing ones.

## Agent File Format

```markdown
---
model: sonnet  # or opus
---

# Agent Name

<core instructions — what the agent does>

## ECC Source (if wrapping an ECC agent)

Copied from: `everything-claude-code/agents/<name>.md`
Sync date: <date>

---

## Evolve Loop Integration

<evolve-specific instructions>

### Inputs

You will receive a JSON context block with:
- `cycle`: current cycle number
- `workspacePath`: path to `.claude/evolve/workspace/`
- `ledgerPath`: path to `.claude/evolve/ledger.jsonl`
- <additional context fields>

### Output

#### Workspace File: `workspace/<filename>.md`
<output format>

#### Ledger Entry
```json
{"ts":"<ISO-8601>","cycle":<N>,"role":"<role>","type":"<type>","data":{...}}
```
```

## Rules

1. **One workspace file per agent.** Each agent writes exactly one file. If you need to split responsibilities, create separate agents.

2. **READ-ONLY for reviewers.** Agents in the VERIFY phase (reviewer, e2e, security) must not modify source code. They can only write to their workspace file.

3. **Ledger entry required.** Every agent must append one JSONL entry to the ledger with timestamp, cycle, role, type, and structured data.

4. **Context block as input.** Agents receive their inputs via a JSON context block in the prompt, not through file paths in frontmatter.

5. **Verdict for verification agents.** Agents in VERIFY/EVAL phases must output a verdict: PASS, WARN, or FAIL.

## Creating an ECC Wrapper (Context Overlay Pattern)

ECC wrappers are **thin context overlays**, not full copies. The ECC agent's content is loaded automatically via `subagent_type` delegation — your wrapper only adds evolve-loop-specific context on top.

To create an ECC wrapper:

1. Create a new file `agents/evolve-<role>.md`
2. Add the context overlay header referencing the ECC agent:
   ```markdown
   # Evolve <Role> — Context Overlay

   > Launched via `subagent_type: "everything-claude-code:<ecc-agent-name>"`.
   > This file provides evolve-loop-specific context layered on top of the ECC agent.
   ```
3. Add an `## Evolve Loop Integration` section with:
   - Input context block schema (cycle, workspacePath, ledgerPath, etc.)
   - Workspace file ownership (one file per agent)
   - Evolve-specific responsibilities beyond the ECC agent's base behavior
   - Output format (workspace file + ledger entry)
4. Do **NOT** copy the ECC agent's content into the file. The `subagent_type` field in the orchestrator handles delegation automatically.

See `agents/evolve-reviewer.md` or `agents/evolve-e2e.md` for examples of this pattern.

## Adding a New Phase

If your agent requires a new phase:

1. Determine where it fits in the pipeline (what it depends on, what depends on it)
2. Add the agent file to `agents/`
3. Update `skills/evolve-loop/phases.md` with the new phase instructions
4. Update `skills/evolve-loop/memory-protocol.md` with the new workspace file
5. Update `skills/evolve-loop/SKILL.md` with the updated architecture diagram and agent table
6. Update `CHANGELOG.md`
