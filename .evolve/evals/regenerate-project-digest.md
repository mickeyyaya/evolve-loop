# Eval: Regenerate Project Digest

## Code Graders (bash commands that must exit 0)

- `grep -q "accuracy-self-correction.md" /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md`
- `grep -q "performance-profiling.md" /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md`
- `grep -q "security-considerations.md" /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md`
- `grep -q "phase5-learn.md" /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md`
- `grep -q "Generated Cycle 10" /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md`
- `grep -q "## Recent History" /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md`

## Regression Evals (full test suite)

- `grep -q "## Hotspots" /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md`

## Acceptance Checks (verification commands)

- `wc -l < /Users/danleemh/ai/claude/evolve-loop/.evolve/workspace/project-digest.md | awk '{exit ($1 < 60)}'`

## Thresholds
- All checks: pass@1 = 1.0
