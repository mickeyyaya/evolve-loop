# Eval: add-changelog-cycle3-plugin-packaging

## Code Graders (bash commands that must exit 0)
- `bash -c 'grep -q "\[4\.1\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md && echo OK'`
- `bash -c 'grep -q "plugin\.json" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md && echo OK'`
- `bash -c 'grep -q "frontmatter\|agents.*array\|skills.*array" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md && echo OK'`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -n "4.1.0" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -A 10 "\[4\.1\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`

## Thresholds
- All checks: pass@1 = 1.0
