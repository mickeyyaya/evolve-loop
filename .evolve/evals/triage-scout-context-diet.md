# Eval: triage-scout-context-diet

## Code Graders (bash commands that must exit 0)

- `[code]` `grep -c "Context Diet\|read only.*Selected\|Selected Tasks.*Key Findings\|scout.*sections\|skip.*sections" agents/evolve-triage.md | awk '{exit ($1 >= 1) ? 0 : 1}'`

## Negative Cases (must NOT be broken)

- `[code]` `grep -qi "carryoverTodos\|top_n\|deferred\|dropped" agents/evolve-triage.md`
- `[code]` `grep -qi "scout-report" agents/evolve-triage.md`

## Regression Evals

- `[code]` `cd go && go test ./... -count=1 -timeout 120s 2>&1 | tail -5 | grep -qv "FAIL"`

## Acceptance Checks

- `[code]` `grep -c "STOP CRITERION" agents/evolve-triage.md | awk '{exit ($1 >= 1) ? 0 : 1}'`

## Edge/OOD Cases (reading-protocol does not remove required inputs)

- `[code]` `grep -qi "triage-decision\|top_n" agents/evolve-triage.md`

## Thresholds

- All checks: pass@1 = 1.0
