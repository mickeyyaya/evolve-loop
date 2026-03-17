# Eval: Extract instincts from cycles 22-29

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml`
- `grep -c "^- id:" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml | xargs -I{} test {} -ge 5`

## Regression Evals (full test suite)
- `grep -q "pattern:" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml`
- `grep -q "confidence:" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml`
- `grep -q "category:" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml`

## Acceptance Checks (verification commands)
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); assert s.get('instinctCount',0) >= 23, f'instinctCount not updated: {s.get(\"instinctCount\")}'; print('PASS')"`
- `grep -q "source:.*cycle-2[2-9]" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-22-29-instincts.yaml`

## Thresholds
- All checks: pass@1 = 1.0
