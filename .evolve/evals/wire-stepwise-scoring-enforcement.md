# Eval: Wire Stepwise Scoring Enforcement

## Code Graders (bash commands that must exit 0)
- `grep -q "MANDATORY" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "MUST enumerate" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "2511.07364" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "wired" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Regression Evals (full test suite)
- `grep -oP '\]\(([^)]+\.md)\)' /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | wc -l | awk '{exit ($1 > 8)}'`

## Acceptance Checks (verification commands)
- `grep -c "NOTE — Stepwise Evidence Gathering" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{exit ($1 > 0)}'`
- `grep -q "1a\." /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{exit ($1 > 450)}'`

## Thresholds
- All checks: pass@1 = 1.0
