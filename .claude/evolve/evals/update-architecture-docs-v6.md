# Eval: update-architecture-docs-v6

## Code Graders (bash commands that must exit 0)
- `grep -q "v6" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "genes" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "synthesized\|tools" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "mastery\|curriculum" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -v "v4 architecture reduces token usage" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | grep -qv "v3 (11 agents)"`
- `grep -c "Layer" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | awk '{exit ($1 < 5) ? 1 : 0}'`

## Thresholds
- All checks: pass@1 = 1.0
