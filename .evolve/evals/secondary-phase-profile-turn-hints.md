# Eval: secondary-phase-profile-turn-hints

## Purpose
Verify that 6 secondary-phase profile JSON files that previously lacked `turn_budget_hint` now have
a non-null integer value for that field. This implements the P-NEW-23 preemptive budget hint pattern
for codex-tmux and claude-tmux phases not covered in the original P-NEW-23 rollout.

## Code Graders (bash commands that must exit 0)

### tdd-engineer.json: turn_budget_hint must be set (non-null integer)
- `[code]` `jq -e '.turn_budget_hint != null and (.turn_budget_hint | type) == "number"' .evolve/profiles/tdd-engineer.json`

### adversarial-review.json: turn_budget_hint must be set
- `[code]` `jq -e '.turn_budget_hint != null and (.turn_budget_hint | type) == "number"' .evolve/profiles/adversarial-review.json`

### architecture-design.json: turn_budget_hint must be set
- `[code]` `jq -e '.turn_budget_hint != null and (.turn_budget_hint | type) == "number"' .evolve/profiles/architecture-design.json`

### behavior-baseline.json: turn_budget_hint must be set
- `[code]` `jq -e '.turn_budget_hint != null and (.turn_budget_hint | type) == "number"' .evolve/profiles/behavior-baseline.json`

### behavior-compare.json: turn_budget_hint must be set
- `[code]` `jq -e '.turn_budget_hint != null and (.turn_budget_hint | type) == "number"' .evolve/profiles/behavior-compare.json`

### retrospective.json: turn_budget_hint must be set
- `[code]` `jq -e '.turn_budget_hint != null and (.turn_budget_hint | type) == "number"' .evolve/profiles/retrospective.json`

### turn_budget_hint must be within reasonable range (≥4 and ≤ max_turns)
- `[code]` `jq -e '.turn_budget_hint >= 4 and .turn_budget_hint <= .max_turns' .evolve/profiles/tdd-engineer.json`
- `[code]` `jq -e '.turn_budget_hint >= 4 and .turn_budget_hint <= .max_turns' .evolve/profiles/adversarial-review.json`
- `[code]` `jq -e '.turn_budget_hint >= 4 and .turn_budget_hint <= .max_turns' .evolve/profiles/architecture-design.json`
- `[code]` `jq -e '.turn_budget_hint >= 4 and .turn_budget_hint <= .max_turns' .evolve/profiles/behavior-baseline.json`
- `[code]` `jq -e '.turn_budget_hint >= 4 and .turn_budget_hint <= .max_turns' .evolve/profiles/behavior-compare.json`
- `[code]` `jq -e '.turn_budget_hint >= 4 and .turn_budget_hint <= .max_turns' .evolve/profiles/retrospective.json`

## Negative Cases (must NOT be broken by the change)

### Previously-configured profiles must still have their hints
- `[code]` `jq -e '.turn_budget_hint == 12' .evolve/profiles/scout.json`
- `[code]` `jq -e '.turn_budget_hint == 20' .evolve/profiles/builder.json`
- `[code]` `jq -e '.turn_budget_hint == 30' .evolve/profiles/auditor.json`
- `[code]` `jq -e '.turn_budget_hint == 12' .evolve/profiles/triage.json`

## Regression Evals

- `[code]` `cd go && go test ./... -count=1 -timeout 120s 2>&1 | tail -5 | grep -qv "FAIL"`

## Edge/OOD Cases

### Profiles without turn_budget_hint (not in scope) must not be accidentally set
- `[code]` `jq -e '.turn_budget_hint == null' .evolve/profiles/security-scan.json`

## Thresholds

- All checks: pass@1 = 1.0
