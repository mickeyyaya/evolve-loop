# Eval: Add Reference Links to Architecture Reference Documents Section

## Code Graders (bash commands that must exit 0)
- `grep -q "genes\.md" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "meta-cycle\.md" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "island-model\.md" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Regression Evals (full test suite)
- `grep -q "Reference Documents" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "Operator Brief" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`
- `grep -q "Run Isolation" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md`

## Acceptance Checks (verification commands)
- `grep -c "\[.*\](.*\.md)" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | awk '{exit ($1 < 7)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md | awk '{exit ($1 > 230)}'`

## Thresholds
- All checks: pass@1 = 1.0
