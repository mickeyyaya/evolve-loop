# Eval: Add Domain Detection Extension Guide

## Code Graders (bash commands that must exit 0)
- `grep -q "## Domain Detection" docs/configuration.md`
- `grep -q "domain.json" docs/configuration.md`
- `grep -q "writing" docs/configuration.md`
- `grep -q "research" docs/configuration.md`

## Regression Evals (full test suite)
- N/A — no test runner. Regression check: existing sections of configuration.md must be intact.
- `grep -q "## Configuration Reference" docs/configuration.md || grep -q "## " docs/configuration.md`

## Acceptance Checks (verification commands)
- `grep -c "## " docs/configuration.md | awk '{exit ($1 < 2)}'`
- `grep -qi "detection\|detect" docs/configuration.md`
- `grep -qi "coding\|code" docs/configuration.md`

## Thresholds
- All checks: pass@1 = 1.0
