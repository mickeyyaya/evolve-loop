# Eval: bump-marketplace-version-to-6-0-0

## Code Graders (bash commands that must exit 0)
- `python3 -c "import json; d=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude-plugin/marketplace.json')); assert d['plugins'][0]['version'] == '6.0.0', f'Expected 6.0.0, got {d[\"plugins\"][0][\"version\"]}'; print('OK')"`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q '"version": "6.0.0"' /Users/danleemh/ai/claude/evolve-loop/.claude-plugin/marketplace.json && echo OK`

## Thresholds
- All checks: pass@1 = 1.0
