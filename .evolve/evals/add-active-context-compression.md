# Eval: Add Active Context Compression Pattern

## Code Graders (bash commands that must exit 0)
- `awk '/Active Context Compression/{count++} END{exit (count < 2)}' docs/token-optimization.md`
- `awk '/22\.7|Focus Agent/{found=1} END{exit !found}' docs/token-optimization.md`
- `[ $(wc -l < docs/token-optimization.md) -gt 320 ]`

## Regression Evals (full test suite)
- `awk '/Model Routing/{found=1} END{exit !found}' docs/token-optimization.md`
- `awk '/Dynamic Turn/{found=1} END{exit !found}' docs/token-optimization.md`
- `awk '/accuracy-self-correction/{found=1} END{exit !found}' docs/token-optimization.md`

## Acceptance Checks (verification commands)
- `awk '/compress_context/{found=1} END{exit !found}' docs/token-optimization.md`
- `awk '/2601.07190/{found=1} END{exit !found}' docs/token-optimization.md`

## Thresholds
- All checks: pass@1 = 1.0
