# Eval: Implement Dynamic Turn Budgets Per Phase

## Code Graders (bash commands that must exit 0)
- `awk '/Turn Budget/{found=1} END{exit !found}' docs/token-optimization.md`
- `awk '/24-68/{found=1} END{exit !found}' docs/token-optimization.md`
- `awk '/future improvement/{found=1} END{exit found}' docs/token-optimization.md`
- `[ $(wc -l < docs/token-optimization.md) -gt 300 ]`

## Regression Evals (full test suite)
- `awk '/Dynamic Turn/{found=1} END{exit !found}' docs/token-optimization.md`
- `awk '/Model Routing/{found=1} END{exit !found}' docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `awk '/Scout.*5.*turn|5.*turn.*Scout/{found=1} END{exit !found}' docs/token-optimization.md`
- `awk '/Auditor.*3.*turn|3.*turn.*Auditor/{found=1} END{exit !found}' docs/token-optimization.md`

## Thresholds
- All checks: pass@1 = 1.0
