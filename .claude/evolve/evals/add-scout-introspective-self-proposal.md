# Eval: Scout Introspective Self-Proposal Step

## Code Graders (bash commands that must exit 0)
- `grep -n "Introspection Pass\|introspection pass\|Introspection" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -n "instinctsExtracted\|auditIterations\|stagnationPatterns" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -n "source.*introspection\|introspection.*source\|introspection" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -c "instinctsExtracted\|auditIterations\|stagnationPatterns\|consecutive\|threshold" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -n "evalHistory\|delta\|pendingImprovements" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`

## Thresholds
- All checks: pass@1 = 1.0
- `grep -c` acceptance check: expects count >= 3
