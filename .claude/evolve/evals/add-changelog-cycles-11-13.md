# Eval: add-changelog-cycles-11-13

## Code Graders (bash commands that must exit 0)
- `grep -q "\[6\.2\.0\]" CHANGELOG.md`
- `python3 -c "import json; p=json.load(open('.claude-plugin/plugin.json')); assert p['version']=='6.2.0', f'Expected 6.2.0, got {p[\"version\"]}'"` 
- `python3 -c "import json; m=json.load(open('.claude-plugin/marketplace.json')); assert m['plugins'][0]['version']=='6.2.0', f'Expected 6.2.0, got {m[\"plugins\"][0][\"version\"]}'"` 

## Regression Evals (full test suite)
- `CI=true ./install.sh`

## Acceptance Checks (verification commands)
- `grep -n "6\.2\.0" CHANGELOG.md | head -3`
- `python3 -c "import json; p=json.load(open('.claude-plugin/plugin.json')); print('plugin.json version:', p['version'])"`
- `python3 -c "import json; m=json.load(open('.claude-plugin/marketplace.json')); print('marketplace.json version:', m['plugins'][0]['version'])"`

## Thresholds
- All checks: pass@1 = 1.0
