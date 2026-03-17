# Eval: Self-Building Capability Gap Scanner in Scout

## Code Graders (bash commands that must exit 0)
- `grep -n "Capability Gap\|capability.gap\|capability gap" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -n "revisitAfter\|deferred.*task\|task.*deferred" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -n "dormant.*instinct\|instinct.*dormant\|graduated" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -n "source.*capability-gap\|capability-gap.*source" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Thresholds
- All checks: pass@1 = 1.0
