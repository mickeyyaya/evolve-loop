# Eval: Unify Models.json Examples Across Docs

## Code Graders (bash commands that must exit 0)

- `grep -A10 '"provider": "anthropic"' skills/evolve-loop/SKILL.md | grep -q 'thinkingMode'` — verify thinkingMode field now present in SKILL.md example
- `grep -c '"thinkingMode"' docs/configuration.md` — should output ≥1 (thinkingMode documented)
- `grep -q 'SKILL.md' docs/configuration.md` — verify cross-reference to SKILL.md added to configuration.md
- `diff <(grep -A15 '"provider"' docs/configuration.md | head -15) <(grep -A15 '"provider"' skills/evolve-loop/SKILL.md | head -15) | grep -q "thinkingMode"` — verify both examples now reference thinkingMode
- `grep -c '```json' skills/evolve-loop/SKILL.md | awk '{exit ($1 >= 2)}'` — verify both code examples present (json and at least one other format)

## Regression Evals (full test suite)

- Both files should remain syntactically valid Markdown:
  - `grep -c '^#' skills/evolve-loop/SKILL.md | awk '{exit ($1 >= 5)}'` — verify heading structure intact
  - `grep -c '^#' docs/configuration.md | awk '{exit ($1 >= 5)}'` — verify heading structure intact
- All code blocks should be properly formatted:
  - `grep '```json' skills/evolve-loop/SKILL.md` — should have json blocks
  - `grep '```json' docs/configuration.md` — should have json blocks

## Acceptance Checks (verification commands)

- `grep -A3 'thinkingMode' skills/evolve-loop/SKILL.md` — verify thinkingMode example is formatted like configuration.md
- `grep -c 'overrides' skills/evolve-loop/SKILL.md` — should output ≥1 (overrides field present)
- `grep 'SKILL.md' docs/configuration.md | grep -i config` — verify relevant cross-reference exists

## Thresholds
- All code graders: pass@1 = 1.0
- Regression evals: pass@1 = 1.0
- Acceptance checks: manual verification of unified schema
