# Eval: Sync instinctSummary with inst-018 through inst-023

## Code Graders (bash commands that must exit 0)
- `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert len(s['instinctSummary']) == 23, f'expected 23 entries, got {len(s[\"instinctSummary\"])}'"` 
- `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); ids=[e['id'] for e in s['instinctSummary']]; missing=[f'inst-0{i:02d}' for i in range(18,24) if f'inst-0{i:02d}' not in ids]; assert not missing, f'missing: {missing}'"`

## Regression Evals (full test suite)
- `bash .claude/evolve/eval-runner.sh` (if present) or verify JSON is valid: `python3 -c "import json; json.load(open('.claude/evolve/state.json'))"`

## Acceptance Checks (verification commands)
- `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert s['instinctCount'] == 23, f'instinctCount mismatch: {s[\"instinctCount\"]}'"`
- `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); entry=next(e for e in s['instinctSummary'] if e['id']=='inst-022'); assert entry['pattern'] == 'meta-cycle-extraction-stall'"`

## Thresholds
- All checks: pass@1 = 1.0
