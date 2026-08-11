# Eval: add-writing-domain-showcase

## Code Graders (bash commands that must exit 0)
- `grep -q "writing" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`
- `grep -q "Rubric Grader" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`
- `grep -q "domain.json" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`
- `grep -q "file-save" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`

## Regression Evals (full test suite)
- `bash -c 'wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md | awk "{exit (\$1 > 420)}"'`

## Acceptance Checks (verification commands)
- `grep -q "Writing Domain" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`
- `grep -q "rubric" /Users/danleemh/ai/claude/evolve-loop/docs/showcase.md`

## Thresholds
- All checks: pass@1 = 1.0
