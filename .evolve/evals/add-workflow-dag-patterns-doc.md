# Eval: add-workflow-dag-patterns-doc

> Graders for the workflow DAG patterns documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/workflow-dag-patterns.md` | exit 0 |
| 2 | bash | `grep -q "state.machine\|state machine\|State Machine" docs/workflow-dag-patterns.md` | exit 0 |
| 3 | bash | `grep -q "DAG\|directed acyclic" docs/workflow-dag-patterns.md` | exit 0 |
| 4 | bash | `grep -q "conditional\|branching\|Conditional" docs/workflow-dag-patterns.md` | exit 0 |
| 5 | bash | `grep -q "checkpoint\|Checkpoint\|resume" docs/workflow-dag-patterns.md` | exit 0 |
| 6 | bash | `grep -q "state.delta\|state delta\|minimal context" docs/workflow-dag-patterns.md` | exit 0 |
| 7 | bash | `wc -l < docs/workflow-dag-patterns.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 8 | bash | `grep -c "^|" docs/workflow-dag-patterns.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 9 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/workflow-dag-patterns.md` | exit 0 |
| 10 | bash | `grep -q "Scout\|Builder\|Auditor" docs/workflow-dag-patterns.md` | exit 0 — maps to evolve-loop agents |
