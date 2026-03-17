# Eval: add-instinct-extraction-trigger

## Code Graders (bash commands that must exit 0)
- `grep -c "instinctsExtracted\|extraction trigger\|instinct extraction" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`
- `grep -c "consecutive.*zero\|0.*consecutive\|extraction.*stall\|no new instincts" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `grep -c "Phase 5" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Acceptance Checks (verification commands)
- `grep -l "instinct.*extraction.*block\|force.*extraction\|instinct.*prompt" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Thresholds
- All checks: pass@1 = 1.0
