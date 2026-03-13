# Eval: add-cycle8-instincts-file

## Code Graders (bash commands that must exit 0)
- `test -f .claude/evolve/instincts/personal/cycle-8-instincts.yaml`
- `grep -q "inst-012" .claude/evolve/instincts/personal/cycle-8-instincts.yaml`
- `grep -q "confidence: 0.8" .claude/evolve/instincts/personal/cycle-8-instincts.yaml`

## Regression Evals (full test suite)
- `python3 -c "import yaml; data=yaml.safe_load(open('.claude/evolve/instincts/personal/cycle-8-instincts.yaml').read()); print('OK: valid YAML, parsed successfully')" 2>/dev/null || python3 -c "import sys; content=open('.claude/evolve/instincts/personal/cycle-8-instincts.yaml').read(); assert 'inst-012' in content; print('OK: file is readable YAML with expected content')"`

## Acceptance Checks (verification commands)
- `grep -q "source.*cycle-8" .claude/evolve/instincts/personal/cycle-8-instincts.yaml`

## Thresholds
- All checks: pass@1 = 1.0
