# Eval: update-install-sh-version-string

## Code Graders (bash commands that must exit 0)
- `grep -q "Evolve Loop v6" /Users/danleemh/ai/claude/evolve-loop/install.sh && echo OK`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -v "Evolve Loop v4" /Users/danleemh/ai/claude/evolve-loop/install.sh | wc -l | xargs -I{} test {} -gt 0 && echo OK`

## Thresholds
- All checks: pass@1 = 1.0
