# Eval: add-runtime-guardrails-doc

## Code Graders (bash commands that must exit 0)

- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md | awk '{exit ($1 < 40 || $1 > 120)}'`
- `grep -q "AgentSpec" /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md`
- `grep -q "AEGIS" /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md`
- `grep -q "phase-gate" /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md`
- `grep -q "trigger" /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md`
- `grep -q "research-paper-index" /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md`
- `grep -q "runtime-guardrails" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)

- `grep -q "research-paper-index" /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md`

## Acceptance Checks (verification commands)

- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/docs/runtime-guardrails.md | awk '{exit ($1 < 3)}'`

## Thresholds

- All checks: pass@1 = 1.0
