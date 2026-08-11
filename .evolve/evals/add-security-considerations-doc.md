# Eval: add-security-considerations-doc

## Code Graders (bash commands that must exit 0)
- `test -f docs/security-considerations.md`
- `wc -l < docs/security-considerations.md | awk '{exit ($1 < 40)}'`
- `grep -qi 'eval.*tamper\|tamper.*detect\|state\.json.*validat' docs/security-considerations.md`
- `grep -qi 'prompt.*inject\|inject' docs/security-considerations.md`

## Regression Evals (full test suite)
- `grep -c '^## ' docs/security-considerations.md | awk '{exit ($1 < 3)}'`

## Acceptance Checks (verification commands)
- `wc -l < docs/security-considerations.md | awk '{exit ($1 > 150)}'`
- `grep -qi 'rollback\|revert' docs/security-considerations.md`

## Thresholds
- All checks: pass@1 = 1.0
