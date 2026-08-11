# Eval: Extract Instincts from Cycles 17-19

## Code Graders (bash commands that must exit 0)

- `test $(ls /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/ | grep -cE 'inst-01[2-9]|inst-02') -ge 3`
- `grep -q 'pattern' /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/inst-012.yaml`
- `grep -q 'confidence' /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/inst-012.yaml`
- `grep -q 'category' /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/inst-012.yaml`

## Regression Evals (full test suite)

- No automated regression suite for this project. Verify file structure only.

## Acceptance Checks (verification commands)

- `test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/inst-012.yaml`
- `test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/inst-013.yaml`
- `test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/inst-014.yaml`
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/state.json')); assert s['instinctCount'] >= 14, f'instinctCount={s[\"instinctCount\"]} < 14'"`
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.evolve/state.json')); ids=[i['id'] for i in s['instinctSummary']]; assert 'inst-012' in ids, 'inst-012 missing from instinctSummary'"`
- `grep -qiE 'grep|anchor|regex|eval.grader' /Users/danleemh/ai/claude/evolve-loop/.evolve/instincts/personal/inst-012.yaml`

## Thresholds

- All checks: pass@1 = 1.0
