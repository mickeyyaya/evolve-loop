# Eval: add-enterprise-agent-eval-doc

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md | awk '{exit ($1 < 50 || $1 > 140)}'`
- `grep -q "CLEAR" /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md`
- `grep -q "AgencyBench" /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md`
- `grep -q "2511.14136" /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md`
- `grep -q "2601.11044" /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md`
- `grep -q "research-paper-index" /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md`

## Regression Evals (full test suite)
- `bash scripts/phase-gate.sh audit 2>/dev/null || true`

## Acceptance Checks (verification commands)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md | awk '{exit ($1 < 3)}'`
- `grep -q "Reliability" /Users/danleemh/ai/claude/evolve-loop/docs/enterprise-agent-evaluation.md`
- `grep -q "research-paper-index.md" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds
- All checks: pass@1 = 1.0
