# Eval: Fix Tier System Relative Paths

## Code Graders (bash commands that must exit 0)

- `grep -q '\[configuration.md\](docs/configuration.md)' skills/evolve-loop/SKILL.md` — verify new correct path exists in SKILL.md
- `grep -q '\[docs/genes.md\](docs/genes.md)' skills/evolve-loop/phase5-learn.md` — verify new correct path exists in phase5-learn.md
- `! grep '(../../docs/' skills/evolve-loop/SKILL.md` — verify old relative path pattern is gone from SKILL.md
- `! grep '(../../docs/' skills/evolve-loop/phase5-learn.md` — verify old relative path pattern is gone from phase5-learn.md
- `grep -c "^\- See \[" skills/evolve-loop/SKILL.md | awk '{exit ($1 >= 8)}'` — verify at least 8 reference links remain (document structure intact)

## Regression Evals (full test suite)

- All links in the modified files should resolve (verified via `test -f` checks from repo root):
  - `test -f docs/configuration.md && test -f docs/genes.md` — both files exist
  - Manual spot-check: SKILL.md line 329 and phase5-learn.md line 133 should now use repo-root-relative paths

## Acceptance Checks (verification commands)

- `grep 'configuration.md\](docs/' skills/evolve-loop/SKILL.md | wc -l` — should output 1 (exactly one corrected reference)
- `grep 'genes.md\](docs/' skills/evolve-loop/phase5-learn.md | wc -l` — should output 1 (exactly one corrected reference)
- `grep -E '\[.*\]\(\.\./\.\./docs/' skills/evolve-loop/SKILL.md | wc -l` — should output 0 (no incorrect patterns remain)

## Thresholds
- All code graders: pass@1 = 1.0
- Regression evals: pass@1 = 1.0
- Acceptance checks: manual verification
