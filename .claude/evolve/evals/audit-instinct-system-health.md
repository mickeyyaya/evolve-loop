# Eval: audit-instinct-system-health

## Code Graders (bash commands that must exit 0)
- `ls /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-4-instincts.yaml`

## Regression Evals (full test suite)
- `CI=true ./install.sh`

## Acceptance Checks (verification commands)
- `grep -q "inst-" /Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/cycle-4-instincts.yaml`
- `bash -c 'python3 -c "import yaml; [yaml.safe_load(open(f)) for f in __import__(\"glob\").glob(\"/Users/danleemh/ai/claude/evolve-loop/.claude/evolve/instincts/personal/*.yaml\")]" 2>/dev/null && echo "YAML valid" || echo "YAML invalid"' | grep -q "YAML valid"`

## Thresholds
- All checks: pass@1 = 1.0
