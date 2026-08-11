# Eval: add-failure-pattern-analysis

## Code Graders (bash commands that must exit 0)
- `grep -q "failurePatterns\|failure_patterns\|FailurePattern" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md && echo "PASS: failure pattern analysis documented"`
- `grep -q "root.cause\|rootCause\|root_cause" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md && echo "PASS: root cause categorization present"`
- `grep -c "DGM\|Darwin.Godel\|archive.based\|novelty.bonus" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{if ($1 >= 2) {print "PASS: DGM-inspired concepts referenced"; exit 0} else {print "FAIL: insufficient DGM references"; exit 1}}'`

## Regression Evals (full test suite)
- `bash /Users/danleemh/ai/claude/evolve-loop/scripts/eval-quality-check.sh`

## Acceptance Checks (verification commands)
- `test -f /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md && wc -l < /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{if ($1 >= 170) {print "PASS: file expanded"; exit 0} else {print "FAIL: file not expanded enough"; exit 1}}'`
- `grep -q "suggested.alternative\|alternativeApproach\|alternative_approach" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md && echo "PASS: alternative approach tracking present"`

## Thresholds
- All checks: pass@1 = 1.0
