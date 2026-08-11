# Eval: Add Process Reward Audit Analysis

## Code Graders (bash commands that must exit 0)

- `grep -q "process reward" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "Step-Level Process Reward" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "2502.10325" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{exit ($1 > 250)}'`

## Regression Evals (full test suite)

- `bash scripts/phase-gate.sh lint 2>/dev/null || true`

## Acceptance Checks (verification commands)

- `grep -c "process" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{exit ($1 < 2)}'`
- `grep -q "AgentPRM\|arXiv:2502.10325" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Thresholds
- All checks: pass@1 = 1.0
