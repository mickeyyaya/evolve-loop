# Eval: add-skill-efficiency-guidelines

## Code Graders (bash commands that must exit 0)
- `grep -q "## Efficiency Guidelines\|## Prompt Efficiency\|## Token Efficiency" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md`
- `grep -qi "token" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md`
- `grep -qi "compres\|concise\|overhead\|redundan" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `wc -l /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md | awk '{if ($1 > 68) exit 0; else exit 1}'`
- `grep -c "^-\|^[0-9]\." /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md | awk '{if ($1 >= 5) exit 0; else exit 1}'`

## Thresholds
- All checks: pass@1 = 1.0
