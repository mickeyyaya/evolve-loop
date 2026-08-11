# Eval: add-multi-agent-coordination-doc

## Code Graders (bash commands that must exit 0)

- `test -f docs/multi-agent-coordination.md`
- `wc -l < docs/multi-agent-coordination.md | awk '{exit ($1 < 60 || $1 > 130)}'`
- `grep -q "AdaptOrch" docs/multi-agent-coordination.md`
- `grep -q "topology" docs/multi-agent-coordination.md`
- `grep -q "DAG\|dependency graph\|directed acyclic" docs/multi-agent-coordination.md`
- `grep -q "2602.16873" docs/multi-agent-coordination.md`
- `grep -q "sequential\|parallel\|hierarchical\|hybrid" docs/multi-agent-coordination.md`

## Regression Evals (full test suite)

- `grep -rq "multi-agent-coordination" docs/research-paper-index.md`

## Acceptance Checks (verification commands)

- `grep -q "multi-agent-coordination.md" docs/research-paper-index.md`

## Thresholds
- All checks: pass@1 = 1.0
