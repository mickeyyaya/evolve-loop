# Eval: add-changelog-cycle10-meta-cycle

## Code Graders (bash commands that must exit 0)
- `grep -q "6\.1\.0\|6.1.0" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "meta-cycle\|Meta-Cycle\|meta_cycle" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -q "6\.1\.0" /Users/danleemh/ai/claude/evolve-loop/.claude-plugin/plugin.json`
- `grep -q "6\.1\.0" /Users/danleemh/ai/claude/evolve-loop/.claude-plugin/marketplace.json`

## Thresholds
- All checks: pass@1 = 1.0
