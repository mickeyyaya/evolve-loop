# Eval: add-agent-sandboxing-patterns-doc

> Graders for the agent sandboxing patterns documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/agent-sandboxing-patterns.md` | exit 0 |
| 2 | bash | `grep -q "isolation\|Isolation" docs/agent-sandboxing-patterns.md` | exit 0 |
| 3 | bash | `grep -q "capability\|permission\|Permission" docs/agent-sandboxing-patterns.md` | exit 0 |
| 4 | bash | `grep -q "container\|Container\|gVisor\|microVM" docs/agent-sandboxing-patterns.md` | exit 0 |
| 5 | bash | `grep -q "network\|Network" docs/agent-sandboxing-patterns.md` | exit 0 |
| 6 | bash | `grep -q "escape\|Escape\|breakout" docs/agent-sandboxing-patterns.md` | exit 0 |
| 7 | bash | `wc -l < docs/agent-sandboxing-patterns.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 8 | bash | `grep -c "^|" docs/agent-sandboxing-patterns.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 9 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/agent-sandboxing-patterns.md` | exit 0 |
| 10 | bash | `grep -q "Scout\|Builder\|Auditor" docs/agent-sandboxing-patterns.md` | exit 0 — maps to evolve-loop agents |
