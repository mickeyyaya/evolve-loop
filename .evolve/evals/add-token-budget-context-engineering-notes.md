# Eval: Add Context Engineering Best Practices to Token Optimization Doc

## Code Graders (bash commands that must exit 0)

- `grep -qi "context engineering\|just.in.time\|static.*before.*dynamic\|KV.cache" docs/token-optimization.md`
- `grep -qi "sub.agent\|compaction\|handoff" docs/token-optimization.md`

## Regression Evals (full test suite)

- `grep -q "Model Routing" docs/token-optimization.md`
- `grep -q "Incremental Scan" docs/token-optimization.md`
- `grep -q "Research Cooldown" docs/token-optimization.md`
- `grep -q "Plan Caching" docs/token-optimization.md`

## Acceptance Checks (verification commands)

- `wc -l < docs/token-optimization.md | awk '{exit ($1 < 96)}'` → file grew (new content added)
- `grep -c "^##" docs/token-optimization.md | awk '{exit ($1 < 8)}'` → at least 8 sections (was 7)

## Thresholds

- All checks: pass@1 = 1.0
