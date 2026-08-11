# Eval: add-agentassay-eval-protocol

## Code Graders (bash commands that must exit 0)

- `grep -q "AgentAssay\|behavioral fingerprint\|INCONCLUSIVE\|three-valued" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -q "non-deterministic\|flaky\|behavioral drift" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -q "arXiv:2603.02601\|AgentAssay" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Regression Evals (full test suite)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md | awk '{exit ($1 > 250)}'`

## Acceptance Checks (verification commands)

- `grep -c "AgentAssay\|behavioral fingerprint" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md | awk '{exit ($1 < 1)}'`
- `grep -q "Cycle 144" /Users/danleemh/ai/claude/evolve-loop/docs/research-paper-index.md`

## Thresholds

- All checks: pass@1 = 1.0
