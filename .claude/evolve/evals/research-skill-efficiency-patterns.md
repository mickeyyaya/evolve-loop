# Eval: research-skill-efficiency-patterns

## Code Graders (bash commands that must exit 0)
- `test -f /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/workspace/skill-efficiency-research.md`
- `wc -l /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/workspace/skill-efficiency-research.md | awk '{if ($1 >= 50) exit 0; else exit 1}'`
- `grep -q "## Findings" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/workspace/skill-efficiency-research.md`
- `grep -q "## Recommendations" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/workspace/skill-efficiency-research.md`
- `grep -qi "token" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/workspace/skill-efficiency-research.md`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -c "##" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/workspace/skill-efficiency-research.md | awk '{if ($1 >= 3) exit 0; else exit 1}'`
- `grep -q "source\|Source\|http" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/workspace/skill-efficiency-research.md`

## Thresholds
- All checks: pass@1 = 1.0
