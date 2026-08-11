# Eval: dynamic-routing-advisory-default

## Code Graders (bash commands that must exit 0)

- `[code]` `python3 -c "import json; d=json.load(open('/Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-registry.json')); assert d['config']['dynamic_routing'] == 'advisory', f\"got {d['config']['dynamic_routing']!r}, want 'advisory'\"; print('OK')"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/config/... -run TestRegistryAdvisoryDefault -count=1 2>&1 | grep -E "^ok|PASS|--- PASS"`

## Regression Evals

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/config/... -count=1 2>&1 | grep -E "^(ok|FAIL)" | grep -v "^ok" | wc -l | xargs -I{} test {} -eq 0`

## Acceptance Checks

- `[code]` `grep '"dynamic_routing"' /Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-registry.json | grep -q '"advisory"'`

## Negative Cases

- `[code]` `python3 -c "import json; d=json.load(open('/Users/danleemh/ai/claude/evolve-loop/docs/architecture/phase-registry.json')); assert d['config']['dynamic_routing'] != '0', 'dynamic_routing is still 0 — change not applied'; print('OK')"`

## Thresholds
- All checks: pass@1 = 1.0
