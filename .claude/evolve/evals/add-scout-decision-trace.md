# Eval: add-scout-decision-trace

## Code Graders (bash commands that must exit 0)
- `grep -c "Decision Trace\|decisionTrace\|decision.*trace" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -c "decisionTrace\|Decision Trace\|decision.*trace" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`

## Regression Evals (full test suite)
- `grep -c "counterfactual" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` (must remain >= 1 — counterfactual schema must not be removed)
- `grep -c "crossover\|crossoverLog" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` (must remain >= 1 — crossoverLog must not be removed)

## Acceptance Checks (verification commands)
- `grep -c "finalDecision\|signals\|direction" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects >= 1 (structured trace fields documented)
- `grep -c "Novelty Critic\|novelty.*critic\|meta.*cycle.*trace\|decisionTrace" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects >= 1 (meta-cycle consumer documented)
- `grep -c "## Decision Trace\|Decision Trace" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects >= 1 (section header present in scout-report template)

## Thresholds
- All checks: pass@1 = 1.0
