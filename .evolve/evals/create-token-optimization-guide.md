# Eval: Create Token Optimization Guide

## Code Graders (bash commands that must exit 0)
- `test -f docs/token-optimization-guide.md`
- `[ $(wc -l < docs/token-optimization-guide.md) -gt 50 ]`

## Regression Evals (existing docs untouched)
- `test -f docs/token-cost-optimization.md`
- `test -f docs/index.md`

## Acceptance Checks (required sections and content)
- `grep -q "Token Footprint" docs/token-optimization-guide.md`
- `grep -q "AgentDiet\|trajectory compression" docs/token-optimization-guide.md`
- `grep -q "progressive disclosure\|three-tier" docs/token-optimization-guide.md`
- `grep -qi "phases\.md\|policies\.md\|SKILL\.md" docs/token-optimization-guide.md`
- `grep -q "cache\|prefix" docs/token-optimization-guide.md`
- `grep -qE "actionable|recommendation|optimize" docs/token-optimization-guide.md`

## Thresholds
- All checks: pass@1 = 1.0
