# Eval: Update README Docs Index

## Code Graders (bash commands that must exit 0)

- `grep -q "accuracy-self-correction.md" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "performance-profiling.md" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "security-considerations.md" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "domain-adapters.md" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "skill-building.md" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "policy-design.md" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -q "phase5-learn.md" /Users/danleemh/ai/claude/evolve-loop/README.md`

## Regression Evals (full test suite)

- `grep -c "^- " /Users/danleemh/ai/claude/evolve-loop/README.md | awk '{exit ($1 < 40)}'`

## Acceptance Checks (verification commands)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/README.md | awk '{exit ($1 > 340)}'`

## Thresholds
- All checks: pass@1 = 1.0
