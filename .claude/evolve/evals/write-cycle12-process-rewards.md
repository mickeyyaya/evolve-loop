# Eval: write-cycle12-process-rewards

## Code Graders (bash commands that must exit 0)
- `python3 -c "import json; d=json.load(open('.claude/evolve/state.json')); pr=d['processRewards']; assert all(v > 0.0 for v in pr.values()), f'All processRewards must be > 0.0, got {pr}'; print('OK: processRewards values are non-zero:', pr)"`

## Regression Evals (full test suite)
- `python3 -c "import json; d=json.load(open('.claude/evolve/state.json')); required=['discover','build','audit','ship','learn']; missing=[k for k in required if k not in d['processRewards']]; assert not missing, f'Missing keys: {missing}'; print('OK: all processRewards keys present')"`

## Acceptance Checks (verification commands)
- `grep -A 7 '"processRewards"' .claude/evolve/state.json | grep -v '"0\.0"'`

## Thresholds
- All checks: pass@1 = 1.0
