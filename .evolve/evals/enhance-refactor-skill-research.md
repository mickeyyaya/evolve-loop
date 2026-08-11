# Eval: enhance-refactor-skill-research

## Graders

- `test -f skills/refactor/SKILL.md && [ $(wc -l < skills/refactor/SKILL.md) -ge 800 ]`
- `grep -q "Cognitive Complexity Scoring" skills/refactor/SKILL.md`
- `grep -q "Architecture Analysis" skills/refactor/SKILL.md`
- `grep -q "Speed Optimizations" skills/refactor/SKILL.md`
- `grep -q "Code Smell Detection Catalog" skills/refactor/SKILL.md`
