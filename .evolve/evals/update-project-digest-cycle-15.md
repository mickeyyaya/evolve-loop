# Eval: update-project-digest-cycle-15

## Code Graders (bash commands that must exit 0)

- `grep -q "Generated Cycle 15" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`
- `grep -q "eval-grader-best-practices.md" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`
- `grep -q "evolve-builder.md\|evolve-auditor.md\|evolve-operator.md\|evolve-scout.md" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`
- `grep -q "^## Hotspots" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`
- `grep -q "^## Recent History" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`
- `grep -c "^##" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md | awk '{exit ($1 < 5)}'`
- `! grep -q "Generated Cycle 10\b" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`

## Regression Evals (full test suite)

- `test -f /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`

## Acceptance Checks (verification commands)

- `grep -q "Generated Cycle 15" /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md`
- `wc -l < /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/run-17739016583N-3840/workspace/project-digest.md | awk '{exit ($1 > 150)}'`

## Thresholds
- All checks: pass@1 = 1.0
