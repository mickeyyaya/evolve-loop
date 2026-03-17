# Eval: add-v6-dirs-to-readme-workspace-layout

## Code Graders (bash commands that must exit 0)
- `grep -q "genes/" /Users/danleemh/ai/claude/evolve-loop/README.md && echo OK`
- `grep -q "tools/" /Users/danleemh/ai/claude/evolve-loop/README.md && echo OK`
- `grep -q "archived/" /Users/danleemh/ai/claude/evolve-loop/README.md && echo OK`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -A 30 "Workspace Layout" /Users/danleemh/ai/claude/evolve-loop/README.md | grep -q "genes/" && echo OK`

## Thresholds
- All checks: pass@1 = 1.0
