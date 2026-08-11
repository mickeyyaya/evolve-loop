# Eval: Add Domain Adapter Abstraction

## Code Graders (bash commands that must exit 0)
- `test -f docs/domain-adapters.md`
- `grep -c "## " docs/domain-adapters.md | awk '{exit ($1 < 4)}'`
- `grep -q "ship" docs/domain-adapters.md`
- `grep -q "eval" docs/domain-adapters.md`
- `wc -l < docs/domain-adapters.md | awk '{exit ($1 > 120)}'`

## Regression Evals (full test suite)
- N/A — no test runner. Regression check: `grep -rq "domain-adapters" docs/architecture.md`

## Acceptance Checks (verification commands)
- `grep -q "domain-adapters" docs/architecture.md`
- `grep -qi "domain.specific" docs/domain-adapters.md || grep -qi "domain-specific" docs/domain-adapters.md`
- `grep -qi "build isolation\|worktree\|isolation" docs/domain-adapters.md`
- `grep -qi "auto.detect\|detection" docs/domain-adapters.md`

## Thresholds
- All checks: pass@1 = 1.0
