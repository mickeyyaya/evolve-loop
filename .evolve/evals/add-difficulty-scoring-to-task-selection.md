# Eval: add-difficulty-scoring-to-task-selection

## Code Graders (bash commands that must exit 0)
- `grep -q "difficulty.*scor\|difficultyScore\|difficulty_score\|1-10.*difficulty" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md && echo "PASS: difficulty scoring documented"`
- `grep -q "task.type.*difficulty\|per.type.*success\|taskTypeDifficulty" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md && echo "PASS: per-task-type difficulty tracking present"`
- `grep -c "DAAO\|difficulty.aware\|cost.performance.*feedback\|token.*cost.*delta" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md | awk '{if ($1 >= 1) {print "PASS: DAAO-inspired concepts present"; exit 0} else {print "FAIL: no DAAO references"; exit 1}}'`

## Regression Evals (full test suite)
- `bash /Users/danleemh/ai/claude/evolve-loop/scripts/eval-quality-check.sh`

## Acceptance Checks (verification commands)
- `grep -q "cost.performance\|token.*budget.*actual\|estimated.*actual" /Users/danleemh/ai/claude/evolve-loop/docs/self-learning.md && echo "PASS: cost-performance feedback documented"`

## Thresholds
- All checks: pass@1 = 1.0
