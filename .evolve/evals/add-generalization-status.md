# Eval: Add Generalization Status section

## Code Graders (bash commands that must exit 0)

- `grep -q "## Generalization Status\|# Generalization Status" /Users/danleemh/ai/claude/evolve-loop/README.md || grep -q "## Generalization Status\|# Generalization Status" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md` → Section exists in either file
- `grep -A 5 "Generalization Status" /Users/danleemh/ai/claude/evolve-loop/README.md 2>/dev/null | grep -q "domain\|writing\|research\|design" || grep -A 5 "Generalization Status" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md 2>/dev/null | grep -q "domain\|writing\|research\|design"` → Section mentions supported domains
- `grep -q "remains\|TODO\|next cycle" /Users/danleemh/ai/claude/evolve-loop/README.md 2>/dev/null | grep -A 5 "Generalization Status" || grep -q "remains\|TODO\|next cycle" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md 2>/dev/null | grep -A 5 "Generalization Status"` → Section lists what remains to generalize

## Regression Evals (full test suite)

- `grep -q "## Generalization Status\|# Generalization Status" /Users/danleemh/ai/claude/evolve-loop/README.md || grep -q "## Generalization Status\|# Generalization Status" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md` → Section is findable
- `grep -q "Domain Generalization" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md` → Existing domain docs remain intact

## Acceptance Checks (verification commands)

- `grep -c "writing\|research\|design" /Users/danleemh/ai/claude/evolve-loop/README.md 2>/dev/null | xargs test $(grep -A 10 "Generalization Status" /Users/danleemh/ai/claude/evolve-loop/README.md 2>/dev/null | grep -c "writing\|research\|design") -ge 1 || test $(grep -A 10 "Generalization Status" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md 2>/dev/null | grep -c "writing\|research\|design") -ge 1` → Section mentions at least one non-coding domain
- `grep -q "Phase\|domain\|Cycle" /Users/danleemh/ai/claude/evolve-loop/README.md 2>/dev/null | grep -A 15 "Generalization Status" || grep -q "Phase\|domain\|Cycle" /Users/danleemh/ai/claude/evolve-loop/docs/architecture.md 2>/dev/null | grep -A 15 "Generalization Status"` → Section includes technical depth (phases, cycles, etc.)

## Thresholds

- All checks: pass@1 = 1.0
