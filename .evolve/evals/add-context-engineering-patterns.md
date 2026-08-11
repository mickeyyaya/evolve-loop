# Eval: add-context-engineering-patterns

> Graders for the context engineering patterns documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/context-engineering-patterns.md` | exit 0 |
| 2 | bash | `grep -q "selection" docs/context-engineering-patterns.md` | exit 0 |
| 3 | bash | `grep -q "compression" docs/context-engineering-patterns.md` | exit 0 |
| 4 | bash | `grep -q "ordering" docs/context-engineering-patterns.md` | exit 0 |
| 5 | bash | `grep -q "isolation" docs/context-engineering-patterns.md` | exit 0 |
| 6 | bash | `grep -q "format" docs/context-engineering-patterns.md` | exit 0 |
| 7 | bash | `wc -l < docs/context-engineering-patterns.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 8 | bash | `grep -c "^|" docs/context-engineering-patterns.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 9 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/context-engineering-patterns.md` | exit 0 |
| 10 | bash | `grep -q "Scout\|Builder\|Auditor" docs/context-engineering-patterns.md` | exit 0 — maps to evolve-loop agents |
