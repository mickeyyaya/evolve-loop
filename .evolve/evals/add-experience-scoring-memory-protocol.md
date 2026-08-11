# Eval: Add Experience Scoring to Memory Protocol and Frontmatter

## Code Graders (bash commands that must exit 0)

- `head -1 skills/evolve-loop/memory-protocol.md | grep -q '^---'`
- `grep -q 'name:' skills/evolve-loop/memory-protocol.md`
- `grep -q 'description:' skills/evolve-loop/memory-protocol.md`
- `grep -qi 'quality scoring\|historicalSuccessRate\|success.rate' skills/evolve-loop/memory-protocol.md`
- `wc -l < skills/evolve-loop/memory-protocol.md | awk '{exit ($1 > 430)}'`

## Regression Evals (full test suite)

- `grep -q 'Instinct\|instinct' skills/evolve-loop/memory-protocol.md`
- `grep -rl "^---" skills/ --include="*.md" | grep -v SKILL.md | wc -l | tr -d ' ' | awk '{exit ($1 < 5)}'`

## Acceptance Checks (verification commands)

- `head -5 skills/evolve-loop/memory-protocol.md | grep -q '^---'`
- `grep -qi 'Instinct Quality Scoring\|quality score' skills/evolve-loop/memory-protocol.md`

## Thresholds
- All checks: pass@1 = 1.0
