# Eval: add-process-rewards-to-config-and-state

## Code Graders (bash commands that must exit 0)
- `grep -n "processRewards" /Users/danleemh/ai/claude/evolve-loop/docs/configuration.md`
- `grep -n "processRewards" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); assert 'processRewards' in s, 'processRewards missing from state.json'"`

## Acceptance Checks (verification commands)
- `bash -c 'grep -A 5 "processRewards" /Users/danleemh/ai/claude/evolve-loop/docs/configuration.md | grep -q "discover\|build\|audit"'`

## Thresholds
- All checks: pass@1 = 1.0
