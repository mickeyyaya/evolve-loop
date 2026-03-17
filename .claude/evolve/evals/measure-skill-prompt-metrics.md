# Eval: measure-skill-prompt-metrics

## Code Graders (bash commands that must exit 0)
- `grep -q "skillMetrics" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json`
- `grep -q "SKILL.md" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json`
- `grep -q "phases.md" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json`
- `grep -q "lineCount" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json`
- `grep -q "estimatedTokens" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `python3 -c "import json; d=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); m=d['skillMetrics']; assert len(m['files']) >= 4, 'Must have at least 4 files tracked'"`
- `python3 -c "import json; d=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); m=d['skillMetrics']; assert m['totalLines'] > 0 and m['totalEstimatedTokens'] > 0"`

## Thresholds
- All checks: pass@1 = 1.0
