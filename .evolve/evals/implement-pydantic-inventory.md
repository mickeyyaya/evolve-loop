# Eval: implement-pydantic-inventory
## Code Graders (bash commands that must exit 0)
- `[code]` grep -q "BaseModel" scripts/setup_skill_inventory.py
- `[code]` python3 scripts/setup_skill_inventory.py --out .evolve/skill-inventory.json
## Regression Evals (full test suite)
- `[code]` bash .evolve/calibrate.sh
## Acceptance Checks
- `[code]` test -f .evolve/skill-inventory.json
## Thresholds
- All checks: pass@1 = 1.0