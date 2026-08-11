# Eval: Add Skill/Policy Design Guide Doc

## Code Graders (bash commands that must exit 0)

- `test -f docs/policy-design.md`
- `grep -qi "policy\|rule\|skill" docs/policy-design.md`
- `wc -l < docs/policy-design.md | awk '{exit ($1 < 30)}'`

## Regression Evals (full test suite)

- `test -d docs`
- `grep -q "token-optimization" docs/token-optimization.md`
- `test -f skills/evolve-loop/SKILL.md`

## Acceptance Checks (verification commands)

- `grep -q "policy-design\|policy_design" README.md || grep -q "policy-design\|policy_design" docs/architecture.md` → new doc referenced from an existing doc
- `grep -c "^##" docs/policy-design.md | awk '{exit ($1 < 3)}'` → at least 3 sections
- `grep -qi "guardrail\|boundary\|minimal\|specific\|actionable" docs/policy-design.md` → contains key best-practice terms

## Thresholds

- All checks: pass@1 = 1.0
