# Eval: add-self-learning-skill-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -c "instinct\|Instinct" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -c "bandit\|Bandit\|reward\|Reward" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -c "LLM-as-a-Judge\|llm.judge\|self.eval" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -c "consolidat\|episodic\|semantic\|procedural" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Regression Evals (full test suite)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Acceptance Checks (verification commands)
- `grep -l "self-improvement\|self.improvement\|feedback.loop\|feedback loop" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Thresholds
- All checks: pass@1 = 1.0
