# Eval: stop-criterion-anchor-phrase-repair

## Purpose
Verify that 7 persona files received `## STOP CRITERION` sections containing the exact anchor phrases required by the cycle contract (the phrases predicate 007 of cycle 253 found absent).

## Code Graders (bash commands that must exit 0)

### Structural gate: STOP CRITERION heading present
- `[code]` `grep -c "STOP CRITERION" agents/evolve-tdd-engineer.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-adversarial-review.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-architecture-design.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-triage.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-behavior-baseline.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-behavior-compare.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-retrospective.md | awk '{exit ($1 >= 1) ? 0 : 1}'`

### Anchor phrase: tdd-engineer must contain "criteria mapping" (case-insensitive)
- `[code]` `grep -i "criteria mapping" agents/evolve-tdd-engineer.md | grep -qi "." && exit 0 || exit 1`

### Anchor phrase: tdd-engineer must contain "RED confirmation" (case-insensitive)
- `[code]` `grep -i "RED confirmation" agents/evolve-tdd-engineer.md | grep -qi "." && exit 0 || exit 1`

### Anchor phrase: adversarial-review must contain "findings classification" (case-insensitive)
- `[code]` `grep -i "findings classification" agents/evolve-adversarial-review.md | grep -qi "." && exit 0 || exit 1`

### Anchor phrase: architecture-design must contain "current-state mapping" (case-insensitive)
- `[code]` `grep -i "current-state mapping" agents/evolve-architecture-design.md | grep -qi "." && exit 0 || exit 1`

### Anchor phrase: architecture-design must contain "design decision" (case-insensitive, inside STOP CRITERION)
- `[code]` `awk '/## STOP CRITERION/{found=1} found && /design decision/{exit 0} END{exit 1}' agents/evolve-architecture-design.md`

### Anchor phrase: triage must contain "scoped input reading" (case-insensitive)
- `[code]` `grep -i "scoped input reading" agents/evolve-triage.md | grep -qi "." && exit 0 || exit 1`

### Anchor phrase: behavior-baseline must contain "target scoping" (case-insensitive)
- `[code]` `grep -i "target scoping" agents/evolve-behavior-baseline.md | grep -qi "." && exit 0 || exit 1`

### Anchor phrase: behavior-baseline must contain "baseline report writing" (case-insensitive)
- `[code]` `grep -i "baseline report writing" agents/evolve-behavior-baseline.md | grep -qi "." && exit 0 || exit 1`

### Anchor phrase: behavior-compare must contain "input reading" (case-insensitive, inside STOP CRITERION)
- `[code]` `awk '/## STOP CRITERION/{found=1} found && /input reading/{exit 0} END{exit 1}' agents/evolve-behavior-compare.md`

### Anchor phrase: retrospective must contain "lesson YAML verification" (case-insensitive)
- `[code]` `grep -i "lesson YAML verification" agents/evolve-retrospective.md | grep -qi "." && exit 0 || exit 1`

## Negative Cases (must NOT be broken by the change)

- `[code]` `grep -c "STOP CRITERION" agents/evolve-builder.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-auditor.md | awk '{exit ($1 >= 1) ? 0 : 1}'`
- `[code]` `grep -c "STOP CRITERION" agents/evolve-scout.md | awk '{exit ($1 >= 1) ? 0 : 1}'`

## Regression Evals

- `[code]` `cd go && go test ./... -count=1 -timeout 120s 2>&1 | tail -5 | grep -qv "FAIL"`

## Edge/OOD Cases

- `[code]` `grep -c "STOP CRITERION" agents/evolve-tdd-engineer.md | awk '{exit ($1 == 1) ? 0 : 1}'`

## Thresholds

- All checks: pass@1 = 1.0
