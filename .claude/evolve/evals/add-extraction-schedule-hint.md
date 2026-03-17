# Eval: Add next instinct extraction schedule hint to state.json

## Code Graders (bash commands that must exit 0)
- `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert 'nextExtractionCycle' in s or any(p.get('type')=='instinct-extraction' for p in s.get('pendingImprovements',[])), 'no extraction schedule found'"`

## Regression Evals (full test suite)
- `python3 -c "import json; json.load(open('.claude/evolve/state.json'))"` (JSON must remain valid)

## Acceptance Checks (verification commands)
- `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); v=s.get('nextExtractionCycle') or next((p for p in s.get('pendingImprovements',[]) if 'extraction' in str(p).lower()),None); assert v, 'extraction schedule entry missing'; print('found:', v)"`

## Thresholds
- All checks: pass@1 = 1.0
