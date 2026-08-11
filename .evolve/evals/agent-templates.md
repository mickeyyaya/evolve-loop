# Eval: Consolidate agent preamble into shared templates

## Code Graders (bash commands that must exit 0)
- `[code]` `test -f agents/agent-templates.md`
- `[code]` `grep -q "agent-templates" agents/evolve-scout.md`
- `[code]` `grep -q "agent-templates" agents/evolve-builder.md`

## Acceptance Checks
- `[code]` `bash -c 'test $(wc -l < agents/agent-templates.md) -gt 30'`
- `[code]` `grep -q "Inputs\|Input Schema\|Context Block" agents/agent-templates.md`

## Thresholds
- All checks: pass@1 = 1.0
