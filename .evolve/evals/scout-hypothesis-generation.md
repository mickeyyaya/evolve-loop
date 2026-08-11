# Eval: scout-hypothesis-generation
## Graders
- `grep -c "Hypothesis Generation" agents/evolve-scout.md | grep -qv "^0$"`
- `grep -c "## Hypotheses" agents/evolve-scout.md | grep -qv "^0$"`
- `grep -c "architecture-improvement" agents/evolve-scout.md | grep -qv "^0$"`
- `grep -c "priorHypotheses" skills/evolve-loop/phases.md | grep -qv "^0$"`
