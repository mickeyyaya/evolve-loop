# Eval: processRewards History Window in State Schema

## Code Graders (bash commands that must exit 0)
- `grep -n "processRewardsHistory\|rewardsHistory" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -n "processRewardsHistory\|rewardsHistory" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -n "rolling\|last 3\|window\|keep.*3\|trim.*3\|3 entries\|3-entry\|three" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -n "cycle.*score\|per.*cycle\|cycle.*reward" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Thresholds
- All checks: pass@1 = 1.0
