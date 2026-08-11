# Eval: Wire Multi-Stage Verification into Auditor Agent

## Code Graders (bash commands that must exit 0)
- `grep -ci "segment\|verify\|reflect\|multi.stage\|MSV" agents/evolve-auditor.md | awk '{exit ($1 < 2)}'`
- `grep -q "complexity.*M\|M.*complexity\|M-complexity\|M complexity" agents/evolve-auditor.md`
- `wc -l < agents/evolve-auditor.md | awk '{exit ($1 > 230)}'`

## Regression Evals (full test suite)
- `grep -q "^---" agents/evolve-auditor.md`
- `grep -q "tools:" agents/evolve-auditor.md`
- `grep -q "PASS\|FAIL\|WARN" agents/evolve-auditor.md`

## Acceptance Checks (verification commands)
- `grep -qi "segment\|multi.stage" agents/evolve-auditor.md`
- `grep -q "Eval Gate\|### E\." agents/evolve-auditor.md`
- `grep -c "^###\|^##" agents/evolve-auditor.md | awk '{exit ($1 < 5)}'`

## Thresholds
- All checks: pass@1 = 1.0
