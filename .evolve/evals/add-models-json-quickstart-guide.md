# Eval: Add Models.json Reference Guide

## Code Graders (bash commands that must exit 0)

- `test -f docs/models-quickstart.md` — verify new file was created
- `wc -l < docs/models-quickstart.md | awk '{exit ($1 < 40 || $1 > 60)}'` — verify file is 40-60 lines (compact reference guide)
- `grep -q 'cost optimization' docs/models-quickstart.md` — scenario 1 present
- `grep -q 'provider switching' docs/models-quickstart.md` — scenario 2 present
- `grep -q 'thinking mode' docs/models-quickstart.md` — scenario 3 present
- `grep -q 'models-quickstart' docs/configuration.md` — cross-reference added to configuration.md
- `grep -q 'models-quickstart' skills/evolve-loop/SKILL.md` — cross-reference added to SKILL.md

## Regression Evals (full test suite)

- Documentation files should remain intact:
  - `grep -c '^##' docs/models-quickstart.md | awk '{exit ($1 >= 3)}'` — verify at least 3 sections (one per scenario)
  - `grep -c '^#' docs/configuration.md | awk '{exit ($1 >= 5)}'` — configuration.md structure intact
  - `grep -c '^#' skills/evolve-loop/SKILL.md | awk '{exit ($1 >= 5)}'` — SKILL.md structure intact
- All JSON examples should be properly formatted:
  - `grep -c '```json' docs/models-quickstart.md | awk '{exit ($1 < 3)}'` — at least 3 JSON code blocks (one per scenario)

## Acceptance Checks (verification commands)

- `grep -i 'example' docs/models-quickstart.md` — verify practical examples present
- `grep -c 'provider' docs/models-quickstart.md` — should be ≥3 (multiple provider mentions)
- `grep 'models-quickstart' docs/configuration.md | grep -i reference` — verify meaningful cross-reference (not just a link)
- Head check: `head -5 docs/models-quickstart.md` — should have clear title/intro

## Thresholds
- All code graders: pass@1 = 1.0
- Regression evals: pass@1 = 1.0
- Acceptance checks: manual verification that guide is practical and useful
