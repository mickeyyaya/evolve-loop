# Eval: stop-criterion-behavior-retro-phases

## Code Graders (bash commands that must exit 0)

- `[code]` `grep -c "STOP CRITERION" agents/evolve-behavior-baseline.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-behavior-compare.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-retrospective.md | awk '{exit ($1 >= 1) ? 0 : 1}'`

## Negative Cases (must NOT be broken)

- `[code]` `grep -c "STOP CRITERION" agents/evolve-builder.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-orchestrator.md | awk '{exit ($1 >= 1) ? 0 : 1}'`

## Regression Evals

- `[code]` `cd go && go test ./... -count=1 -timeout 120s 2>&1 | tail -5 | grep -qv "FAIL"`

## Acceptance Checks

- `[code]` `grep -A3 "STOP CRITERION" agents/evolve-behavior-baseline.md | grep -qi "gate\|complete\|written\|halt\|stop"`
- `[code]` `grep -A3 "STOP CRITERION" agents/evolve-retrospective.md | grep -qi "gate\|complete\|written\|halt\|stop"`

## Edge/OOD Cases

- `[code]` `grep -c "STOP CRITERION" agents/evolve-behavior-baseline.md | awk '{exit ($1 == 1) ? 0 : 1}'`

## Thresholds

- All checks: pass@1 = 1.0
