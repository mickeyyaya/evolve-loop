# Eval: Add Self-Learning Methods Validation Protocol

## Code Graders (bash commands that must exit 0)

- `grep -q 'Method Attribution' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qiE 'validated|inconclusive|validation rubric' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -c 'stepwise\|CSI\|MUSE\|calibration\|taxonomy\|experience' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{exit ($1 < 5)}'`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{exit ($1 > 300)}'`
- `grep -q 'self-learning.md' /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`

## Regression Evals (full test suite)

- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/meta-cycle.md`

## Acceptance Checks (verification commands)

- `grep -q '## Method Attribution' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qiE 'adoption cycle|benchmark dimension|score delta' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi 'stepwise' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi 'coefficient.*self-improvement\|CSI' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi 'confidence.correctness\|calibration' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`
- `grep -qi 'self-evolving\|taxonomy' /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md`

## Thresholds

- All checks: pass@1 = 1.0
