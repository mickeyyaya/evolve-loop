# Agent Observability

Trajectory-level logging, state tracking, and diagnostic patterns for the evolve-loop pipeline. Based on research showing that **state tracking failures are 2.7x more prevalent in failed agent runs** and reduce task success probability by 49% (arXiv:2602.06841).

---

## Why Trajectory-Level Observability

Traditional per-step attribution (input→output mapping) fails to diagnose where multi-step agents break down. Agentic systems require **trajectory-level explainability** — examining sequences of decisions rather than isolated outputs.

The evolve-loop already captures primitive trajectory data across its agents. This document formalizes the observability patterns and identifies improvement opportunities.

---

## Two Explanation Artifact Types

### 1. Reasoning Traces

Step-by-step chain-of-thought logs from each agent. Currently captured in:
- `scout-report.md` — decision trace with signals per task
- `build-report.md` — design rationale, approach description, risk assessment
- `audit-report.md` — finding justification, chain-of-thought before verdict

**Improvement:** Structure reasoning traces as phase → action → outcome triplets:

```markdown
## Reasoning Trace
| Phase | Action | Outcome |
|-------|--------|---------|
| Design | Enumerated 3 target files | 2 files confirmed, 1 missing |
| Implement | Edited docs/self-learning.md | Added 45 lines |
| Verify | Ran 3 eval graders | 3/3 PASS |
```

### 2. Tool-Call Summaries

Structured records of every tool invoked during a phase. The ledger (`ledger.jsonl`) captures agent-level summaries but not individual tool calls within an agent's execution.

**Recommended format for builder-notes.md:**

```markdown
## Tool Calls — Cycle {N}
| Tool | Target | Result | Tokens |
|------|--------|--------|--------|
| Read | docs/self-learning.md | 460 lines | ~2K |
| Edit | docs/self-learning.md | 45 lines added | ~3K |
| Bash | eval grader 1 | PASS | ~1K |
```

---

## State Tracking Checkpoints

State tracking failures — where an agent loses track of what it has already done or what the current file state is — are the highest-value diagnostic signal for failure localization.

**Checkpoint protocol:** Before and after each major agent phase, assert state consistency:

| Checkpoint | When | What to Assert |
|-----------|------|----------------|
| Pre-build | Before Builder starts | `git status --porcelain` is clean |
| Mid-build | After each file edit | Changed files match `filesToModify` list |
| Pre-audit | Before Auditor starts | Worktree exists, build-report.md present |
| Post-audit | After verdict | Worktree state matches audit expectations |
| Pre-ship | Before commit | No unexpected file changes beyond task scope |

**State inconsistency detection:** If `git diff --name-only` shows files not in the task's `filesToModify`, flag as a state tracking failure. This catches the common case where the Builder edits a file it read for reference, confusing "reading for context" with "editing as part of the task."

---

## Failure Diagnosis Protocol

When a task fails audit, diagnose using trajectory-level signals before re-running:

1. **Check state tracking first** — did the Builder edit unexpected files? Did file state change between reads?
2. **Check reasoning trace consistency** — does the design phase mention files that the implement phase didn't touch?
3. **Check tool call efficiency** — did the Builder re-read the same file multiple times (possible context loss)?
4. **Only then** check the eval grader output for specific assertion failures

This ordering is based on the 2.7x correlation between state tracking failures and task failure. Fixing state tracking issues prevents cascading failures in subsequent retries.

---

## Observability Anti-Patterns

| Anti-Pattern | Description | Fix |
|-------------|-------------|-----|
| Over-logging | Capturing every tool call inflates token usage | Log summaries, not raw output |
| Under-logging | No reasoning trace → blind retries | Require phase→action→outcome triplets |
| Stale state assumptions | Agent assumes file state from a previous read | Re-read before edit, or use checkpoints |
| Post-hoc attribution | Only diagnosing after failure | Continuous state tracking catches issues mid-build |

---

## Research References

- "From Features to Actions" (arXiv:2602.06841): trajectory-level explainability, state tracking correlation
- AgentDebug (arXiv:2509.25370): 5-dimension error taxonomy complements trajectory diagnosis
- AgentAssay (arXiv:2603.02601): behavioral fingerprinting for detecting consistency drift

See [research-paper-index.md](research-paper-index.md) for the full citation index.
