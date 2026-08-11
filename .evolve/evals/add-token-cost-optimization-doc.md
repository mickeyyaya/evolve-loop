# Eval: add-token-cost-optimization-doc

> Graders for the token cost optimization documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/token-cost-optimization.md` | exit 0 |
| 2 | bash | `grep -q "semantic caching\|Semantic Caching\|prompt caching" docs/token-cost-optimization.md` | exit 0 |
| 3 | bash | `grep -q "output token\|Output Token" docs/token-cost-optimization.md` | exit 0 |
| 4 | bash | `grep -q "speculative decoding\|Speculative Decoding" docs/token-cost-optimization.md` | exit 0 |
| 5 | bash | `grep -q "dynamic.*loop\|turn limit\|loop control" docs/token-cost-optimization.md` | exit 0 |
| 6 | bash | `wc -l < docs/token-cost-optimization.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 7 | bash | `grep -c "^|" docs/token-cost-optimization.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 8 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/token-cost-optimization.md` | exit 0 |
| 9 | bash | `grep -q "Scout\|Builder\|Auditor" docs/token-cost-optimization.md` | exit 0 — maps to evolve-loop agents |
