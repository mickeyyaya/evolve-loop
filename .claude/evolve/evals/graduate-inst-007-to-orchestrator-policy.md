# Eval: graduate-inst-007-to-orchestrator-policy

## Code Graders (bash commands that must exit 0)
- `grep -q "orchestrator.*builder\|builder.*orchestrator\|inline.*implement\|implement.*inline" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Regression Evals (full test suite)
- `CI=true ./install.sh`

## Acceptance Checks (verification commands)
- `grep -q "S.*complexity\|small.*task\|S-complexity" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -c "Anti-Pattern\|anti-pattern\|policy\|Policy" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md | grep -v "^0$"`

## Thresholds
- All checks: pass@1 = 1.0
