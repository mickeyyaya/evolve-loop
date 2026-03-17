# Eval: populate-plan-cache-templates

## Code Graders (bash commands that must exit 0)
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); assert len(s.get('planCache', [])) >= 3, f'planCache has {len(s.get(\"planCache\",[]))} entries, need 3+'; print('OK')" && exit 0 || exit 1`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); pc=s.get('planCache',[]); assert all('slug' in t and 'pattern' in t and 'template' in t for t in pc), 'Missing required fields in planCache entries'; print('OK')"`
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); pc=s.get('planCache',[]); slugs=[t['slug'] for t in pc]; print(slugs)"`

## Thresholds
- All checks: pass@1 = 1.0
