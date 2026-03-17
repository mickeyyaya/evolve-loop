# Eval: add-memory-hierarchy-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`
- `grep -c "Layer [0-9]\|Layer [0-9]:" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`
- `grep -c "episodic\|semantic\|procedural" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`
- `grep -c "consolidat\|abstraction\|promotion" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`

## Regression Evals (full test suite)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`

## Acceptance Checks (verification commands)
- `grep -l "state.json\|ledger\|instinct" /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/memory-hierarchy.md`

## Thresholds
- All checks: pass@1 = 1.0
