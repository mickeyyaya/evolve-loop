# Eval: Add Skill/Instinct Trust Governance Section

## Code Graders (bash commands that must exit 0)
- `grep -qi "trust.*tier\|tier.*trust\|instinct.*trust\|trust.*instinct" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`
- `grep -qi "2602.12430\|Agent Skills" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md | awk '{exit ($1 > 200)}'`

## Regression Evals (full test suite)
- `grep -q "Eval Tamper Detection" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`
- `grep -q "Prompt Injection" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`
- `grep -q "Summary" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`

## Acceptance Checks (verification commands)
- `grep -qi "26.1\|vulnerabilit" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`
- `grep -qi "community\|external.*instinct\|instinct.*external\|global.*promot" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`

## Thresholds
- All checks: pass@1 = 1.0
