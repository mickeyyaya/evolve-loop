# Eval: add-readme-context-management-section

## Code Graders (bash commands that must exit 0)
- `grep -n "Context Management" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -n "60%" /Users/danleemh/ai/claude/evolve-loop/README.md`
- `grep -n "handoff" /Users/danleemh/ai/claude/evolve-loop/README.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `bash -c 'grep -A 4 "### Context Management" /Users/danleemh/ai/claude/evolve-loop/README.md | grep -q "handoff"'`

## Thresholds
- All checks: pass@1 = 1.0
