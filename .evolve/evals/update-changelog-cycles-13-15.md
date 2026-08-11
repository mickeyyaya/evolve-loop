# Eval: update-changelog-cycles-13-15

## Code Graders (bash commands that must exit 0)

- `grep -q "^\[7\.1\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "wire-cot-builder-agent\|chain-of-thought\|CoT" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "wire-msv-auditor-agent\|multi-stage verification\|M-complexity" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "mutation testing\|eval-runner\|mutation" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "eval-grader-best-practices\|grader best practices\|eval grader" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "token budget\|performance tracking\|Token Budget" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -q "benchmark.*brief\|operator.*benchmark\|brief.*benchmark" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `! grep -q "TODO\|FIXME\|PLACEHOLDER" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`

## Regression Evals (full test suite)

- `grep -c "^## \[" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md | awk '{exit ($1 < 8)}'`

## Acceptance Checks (verification commands)

- `grep -q "^\[7\.1\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md`
- `grep -A5 "\[7\.1\.0\]" /Users/danleemh/ai/claude/evolve-loop/CHANGELOG.md | grep -q "Added\|Changed\|Fixed"`

## Thresholds
- All checks: pass@1 = 1.0
