# Eval: add-ci-workflow

## Code Graders (bash commands that must exit 0)
- `bash -c 'test -f /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml'`
- `bash -c 'command -v python3 && python3 -c "import yaml; yaml.safe_load(open(\"/Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml\"))" 2>/dev/null || bash -c "grep -q \"on:\" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml"'`

## Regression Evals (full test suite)
- `./install.sh`

## Acceptance Checks (verification commands)
- `grep -n "install.sh" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml`
- `grep -n "on:" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml`
- `bash -c 'test $(grep -c "push\|pull_request" /Users/danleemh/ai/claude/evolve-loop/.github/workflows/ci.yml) -gt 0'`

## Thresholds
- All checks: pass@1 = 1.0
