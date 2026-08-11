# Eval: Add Skill Building Lifecycle Doc

## Code Graders (bash commands that must exit 0)
- `test -f docs/skill-building.md`
- `wc -l < docs/skill-building.md | awk '{exit ($1 < 40)}'`
- `grep -c "^##" docs/skill-building.md | awk '{exit ($1 < 3)}'`

## Regression Evals (full test suite)
- `test -f skills/evolve-loop/SKILL.md`
- `test -f docs/policy-design.md`
- `test -d docs`

## Acceptance Checks (verification commands)
- `grep -qi "confidence\|lifecycle\|instinct\|graduation\|policy" docs/skill-building.md` → contains skill-building concepts
- `grep -qi "0\.5\|0\.8\|0\.9\|threshold" docs/skill-building.md` → includes concrete thresholds
- `grep -q "skill-building\|skill_building" docs/architecture.md || grep -q "skill-building\|skill_building" docs/policy-design.md` → new doc referenced from an existing doc

## Thresholds
- All checks: pass@1 = 1.0
