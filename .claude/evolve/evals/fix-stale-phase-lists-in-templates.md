# Eval: fix-stale-phase-lists-in-templates

## Code Graders (bash commands that must exit 0)
- `grep -q "DISCOVER" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/feature_request.md`
- `grep -q "DISCOVER" /Users/danleemh/ai/claude/evolve-loop/.github/PULL_REQUEST_TEMPLATE.md`
- `grep -qv "MONITOR-INIT" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/feature_request.md`
- `grep -qv "MONITOR-INIT" /Users/danleemh/ai/claude/evolve-loop/.github/PULL_REQUEST_TEMPLATE.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -c "Phase" /Users/danleemh/ai/claude/evolve-loop/.github/ISSUE_TEMPLATE/feature_request.md`
- `grep -c "Phase" /Users/danleemh/ai/claude/evolve-loop/.github/PULL_REQUEST_TEMPLATE.md`

## Thresholds
- All checks: pass@1 = 1.0
