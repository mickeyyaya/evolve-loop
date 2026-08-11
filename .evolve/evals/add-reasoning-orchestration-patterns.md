# Eval: add-reasoning-orchestration-patterns

> Graders for the reasoning orchestration patterns documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/reasoning-orchestration-patterns.md` | exit 0 |
| 2 | bash | `grep -q "chain-of-thought\|Chain-of-Thought\|CoT" docs/reasoning-orchestration-patterns.md` | exit 0 |
| 3 | bash | `grep -q "tree-of-thought\|Tree-of-Thought\|ToT" docs/reasoning-orchestration-patterns.md` | exit 0 |
| 4 | bash | `grep -q "planning\|Planning" docs/reasoning-orchestration-patterns.md` | exit 0 |
| 5 | bash | `grep -q "CPO\|preference" docs/reasoning-orchestration-patterns.md` | exit 0 |
| 6 | bash | `wc -l < docs/reasoning-orchestration-patterns.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 7 | bash | `grep -c "^|" docs/reasoning-orchestration-patterns.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 8 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/reasoning-orchestration-patterns.md` | exit 0 |
| 9 | bash | `grep -q "Scout\|Builder\|Auditor" docs/reasoning-orchestration-patterns.md` | exit 0 — maps to evolve-loop agents |
