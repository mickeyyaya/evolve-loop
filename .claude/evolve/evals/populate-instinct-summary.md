# Eval: Populate instinctSummary in state.json

## Code Graders (bash commands that must exit 0)
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); assert 'instinctSummary' in s, 'instinctSummary missing'; assert isinstance(s['instinctSummary'], list), 'not a list'; assert len(s['instinctSummary']) >= 10, f'too few entries: {len(s[\"instinctSummary\"])}'; print('PASS')"`
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); [assert('id' in e and 'pattern' in e and 'confidence' in e, f'missing field in {e}') for e in s['instinctSummary']]; print('PASS')"`

## Regression Evals (full test suite)
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); assert s.get('instinctCount',0) >= 18, 'instinctCount not preserved'; print('PASS')"`

## Acceptance Checks (verification commands)
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); ids=[e['id'] for e in s['instinctSummary']]; assert 'inst-007' in ids, 'inst-007 missing'; assert 'inst-013' in ids, 'inst-013 missing'; print('PASS')"`
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); entries=[e for e in s['instinctSummary'] if e.get('graduated')]; assert len(entries) >= 1, 'no graduated instinct marked'; print('PASS')"`

## Thresholds
- All checks: pass@1 = 1.0
