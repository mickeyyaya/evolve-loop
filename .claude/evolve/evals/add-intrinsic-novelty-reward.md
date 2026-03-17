# Eval: add-intrinsic-novelty-reward

## Code Graders (bash commands that must exit 0)
- `grep -c "fileExplorationMap\|explorationMap\|novelty" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md`
- `grep -c "novelty\|under-explored\|exploration.*boost\|boost.*exploration" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md`
- `grep -c "novelty\|under-explored\|fileExploration" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md`

## Regression Evals (full test suite)
- `grep -c "taskArms\|bandit" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/SKILL.md` (must remain >= 1 — bandit section must not be removed)
- `grep -c "crossover\|planCache" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` (must remain >= 1 — crossover logic must not be removed)

## Acceptance Checks (verification commands)
- `grep -c "fileExplorationMap" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects >= 1 (schema documented)
- `grep -c "novelty" /Users/danleemh/ai/claude/evolve-loop/agents/evolve-scout.md` → expects >= 1 (novelty rule in Scout task selection)
- `grep -c "lastTouchedCycle\|lastTouched\|exploration" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/memory-protocol.md` → expects >= 1 (map entry schema with cycle tracking)

## Thresholds
- All checks: pass@1 = 1.0
