# Eval: Add Context Window Handoff Guide

## Code Graders (bash commands that must exit 0)
- `grep -qi "handoff\|stop.hook\|context.*window\|60%" docs/token-optimization.md`
- `grep -c "^##" docs/token-optimization.md | awk '{exit ($1 < 6)}'`
- `wc -l < docs/token-optimization.md | awk '{exit ($1 < 130)}'`

## Regression Evals (full test suite)
- `test -f docs/token-optimization.md`
- `grep -q "Model Routing" docs/token-optimization.md`
- `grep -q "Research Cooldown" docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `grep -qi "handoff\.md\|resume\|session" docs/token-optimization.md` → handoff file format is documented
- `grep -qi "60%\|capacity\|threshold\|context" docs/token-optimization.md` → trigger threshold is documented

## Thresholds
- All checks: pass@1 = 1.0
