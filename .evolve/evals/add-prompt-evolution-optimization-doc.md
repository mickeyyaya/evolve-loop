# Eval: add-prompt-evolution-optimization-doc

> Graders for the prompt evolution and optimization documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/prompt-evolution-optimization.md` | exit 0 |
| 2 | bash | `grep -q "MIPROv2\|SIMBA\|GEPA" docs/prompt-evolution-optimization.md` | exit 0 |
| 3 | bash | `grep -q "Prochemy\|prochemy" docs/prompt-evolution-optimization.md` | exit 0 |
| 4 | bash | `grep -q "meta.prompt\|meta-prompt\|Meta-prompt" docs/prompt-evolution-optimization.md` | exit 0 |
| 5 | bash | `grep -q "evolutionary\|evolution\|Evolution" docs/prompt-evolution-optimization.md` | exit 0 |
| 6 | bash | `grep -q "adaptive\|Adaptive" docs/prompt-evolution-optimization.md` | exit 0 |
| 7 | bash | `wc -l < docs/prompt-evolution-optimization.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 8 | bash | `grep -c "^|" docs/prompt-evolution-optimization.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 9 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/prompt-evolution-optimization.md` | exit 0 |
| 10 | bash | `grep -q "instinct\|gene\|Instinct\|Gene" docs/prompt-evolution-optimization.md` | exit 0 — maps to evolve-loop prompt systems |
