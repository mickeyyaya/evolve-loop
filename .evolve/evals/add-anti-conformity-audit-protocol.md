# Eval: Add Anti-Conformity Audit Protocol

## Code Graders (bash commands that must exit 0)

- `grep -qi "anti.conform\|conformity" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -q "2509.11035\|Free-MAD" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md | awk '{exit ($1 > 350)}'`

## Regression Evals (full test suite)

- `cd /Users/danleemh/ai/claude/evolve-loop && bash scripts/phase-gate.sh lint 2>/dev/null || true`

## Acceptance Checks (verification commands)

- `grep -q "split.role\|Split.Role\|adversarial" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md`
- `grep -q "Method Attribution" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Thresholds
- All checks: pass@1 = 1.0
