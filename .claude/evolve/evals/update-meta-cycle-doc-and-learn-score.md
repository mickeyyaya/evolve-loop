# Eval: Update meta-cycle doc with LLM-as-a-Judge and fix learn score

## Code Graders (bash commands that must exit 0)
- `grep -q "LLM-as-a-Judge\|self-evaluation" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`
- `grep -q "self-learning.md" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`

## Regression Evals (full test suite)
- `python3 -c "import json; s=json.load(open('/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/state.json')); pr=s.get('processRewards',{}); learn=pr.get('learn',None); assert learn is not None and learn != 0.5, f'learn score still stale 0.5: {learn}'; print('PASS')"`
- `grep -q "Split-Role Critique\|split-role" /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md | xargs -I{} test {} -ge 80`

## Thresholds
- All checks: pass@1 = 1.0
