# Eval: add-instinct-graduation-spec

## Code Graders (bash commands that must exit 0)
- `grep -qi "graduation\|graduate" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "confidence" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "graduated.*true\|true.*graduated" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`

## Acceptance Checks (verification commands)
- `grep -q "## Instinct Graduation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{exit ($1 > 440)}'`

## Thresholds
- All checks: pass@1 = 1.0
