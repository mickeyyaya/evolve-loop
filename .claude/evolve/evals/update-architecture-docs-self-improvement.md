# Eval: Update docs/architecture.md for Self-Improvement Infrastructure

## Code Graders (bash commands that must exit 0)
- `grep -n "processRewardsHistory" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -n "[Ii]ntrospection" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Thresholds
- All checks: pass@1 = 1.0
