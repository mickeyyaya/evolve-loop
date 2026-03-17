# Eval: add-ci-skill-file-validation

## Code Graders (bash commands that must exit 0)
- `grep -c "SKILL.md" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml`
- `grep -c "phases.md" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml`
- `grep -c "memory-protocol.md" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml`
- `grep -c "eval-runner.md" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml`

## Regression Evals (full test suite)
- `CI=true /Users/danleemh/ai/claude/evolve-loop/install.sh`

## Acceptance Checks (verification commands)
- `grep -A5 "skill" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml | grep -c "skills/evolve-loop"`

## Thresholds
- All checks: pass@1 = 1.0
