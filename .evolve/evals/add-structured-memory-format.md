# Eval: add-structured-memory-format

## Code Graders (bash commands that must exit 0)
- `grep -q "exchange_core" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "files_touched" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "specific_context" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "thematic_assignments" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `grep -q "Structured.*Distillation\|structured memory\|4-field\|compound object" /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Regression Evals (full test suite)
- `test -f /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md`
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{exit ($1 > 520)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/token-optimization.md | awk '{exit ($1 > 430)}'`
- `grep -c "exchange_core\|specific_context\|thematic_assignments\|files_touched" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phase5-learn.md | awk '{exit ($1 < 4)}'`

## Thresholds
- All checks: pass@1 = 1.0
