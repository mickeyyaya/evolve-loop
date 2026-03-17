# Eval: add-prerequisite-task-graph

## Code Graders (bash commands that must exit 0)
- `grep -c "prerequisites\|prerequisite" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -c "prerequisites\|prerequisite" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -c "prerequisites\|prerequisite" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md`

## Regression Evals (full test suite)
- `grep -c "counterfactual" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` (must remain >= 1 — counterfactual annotation in Deferred section must not be removed)
- `grep -c "bandit\|taskArms" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` (must remain >= 1 — taskArms schema must not be removed)
- `grep -c "Convergence Short-Circuit\|nothingToDoCount" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` (must remain >= 1 — Phase 1 convergence logic must not be removed)

## Acceptance Checks (verification commands)
- `grep -c "prerequisites.*completed\|unmet.*prerequisite\|prerequisite.*not met\|deferralReason.*prerequisite" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects >= 1 (auto-deferral logic documented)
- `grep -c "prerequisites" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects >= 1 (field in evaluatedTask schema)
- `grep -c "prerequisite" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/phases.md` → expects >= 1 (Phase 1 orchestrator check documented)

## Thresholds
- All checks: pass@1 = 1.0
