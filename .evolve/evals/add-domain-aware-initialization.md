# Eval: add-domain-aware-initialization

## Code Graders (bash commands that must exit 0)
- `grep -q "domain.json" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -q "projectContext.domain" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -q "evalMode" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -q "buildIsolation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `grep -q "shipMechanism" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Regression Evals (full test suite)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md | awk '{exit ($1 < 10)}'`

## Acceptance Checks (verification commands)
- `grep -q "domain.json" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`
- `bash -c 'wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md | awk "{exit (\$1 > 360)}"'`

## Thresholds
- All checks: pass@1 = 1.0
