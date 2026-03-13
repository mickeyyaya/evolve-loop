# Eval: update-writing-agents-frontmatter-template

## Code Graders (bash commands that must exit 0)
- `bash -c 'grep -n "^name:" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md | grep -q "." && echo OK'`
- `bash -c 'grep -n "^description:" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md | grep -q "." && echo OK'`
- `bash -c 'grep -n "^tools:" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md | grep -q "." && echo OK'`
- `bash -c 'grep -n "^model:" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md | grep -q "." && echo OK'`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -c "name:" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md`
- `grep -c "description:" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md`
- `grep -c "tools:" /Users/danleemh/ai/claude/evolve-loop/docs/writing-agents.md`

## Thresholds
- All checks: pass@1 = 1.0
