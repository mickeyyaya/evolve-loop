# Eval: Add Instinct Citation Tracking

## Code Graders (bash commands that must exit 0)
- `grep -n "instinctsApplied" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -n "instinctsApplied" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`
- `grep -n "instinctsApplied\|citation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -n "cited\|citation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -A5 "instinctsApplied" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -A5 "instinctsApplied" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-builder.md`

## Thresholds
- All checks: pass@1 = 1.0
