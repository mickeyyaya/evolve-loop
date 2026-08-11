# Eval: add-swe-ci-regression-tracking

## Code Graders (bash commands that must exit 0)
- `grep -qi "SWE-CI\|regression.*eval\|EvoScore\|zero.regression" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`
- `grep -qi "technical.*debt\|long.term.*mainten\|prior.*behavior" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`
- `grep -qi "regression.*eval\|preserve.*prior\|Regression Evals" /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/eval-runner.md`

## Regression Evals (full test suite)
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md | awk '{exit ($1 > 200)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/skills/evolve-loop/eval-runner.md | awk '{exit ($1 > 280)}'`

## Acceptance Checks (verification commands)
- `grep -q "security-considerations.md" /Users/danleemh/ai/claude/evolve-loop/docs/accuracy-self-correction.md || grep -q "security-considerations" /Users/danleemh/ai/claude/evolve-loop/docs/security-considerations.md`

## Thresholds
- All checks: pass@1 = 1.0
