# Eval: processRewards Per-Cycle Remediation Loop

## Code Graders (bash commands that must exit 0)
- `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -n "pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -n "dimension\|suggestedTask\|remediation" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -n "< 0.7\|below.*threshold\|pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -n "pendingImprovements\|priority.*task" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Thresholds
- All checks: pass@1 = 1.0
