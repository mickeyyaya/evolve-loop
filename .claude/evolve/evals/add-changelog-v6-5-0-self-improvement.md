# Eval: CHANGELOG v6.5.0 for Self-Improvement Infrastructure

## Code Graders (bash commands that must exit 0)
- `grep -n "^\#\# \[6\.5\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -n "processRewardsHistory" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -n "[Ii]ntrospection" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -A2 "^\#\# \[6\.5\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md | grep -q "2026-03-14"`

## Thresholds
- All checks: pass@1 = 1.0
