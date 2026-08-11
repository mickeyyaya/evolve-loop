# Eval: add-autonomous-experiment-loop-doc

> Graders for the autonomous experiment loop documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/autonomous-experiment-loops.md` | exit 0 |
| 2 | bash | `grep -q "hypothesis" docs/autonomous-experiment-loops.md` | exit 0 |
| 3 | bash | `grep -q "experiment" docs/autonomous-experiment-loops.md` | exit 0 |
| 4 | bash | `grep -q "autoresearch" docs/autonomous-experiment-loops.md` | exit 0 |
| 5 | bash | `grep -q "Scout\|BUILD\|Audit" docs/autonomous-experiment-loops.md` | exit 0 |
| 6 | bash | `wc -l < docs/autonomous-experiment-loops.md \| awk '{exit ($1 > 400)}'` | exit 0 |
| 7 | bash | `grep -c "^|" docs/autonomous-experiment-loops.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 8 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/autonomous-experiment-loops.md` | exit 0 |
