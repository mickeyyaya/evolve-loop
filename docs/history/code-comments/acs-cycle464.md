# Comment history: `acs/cycle464`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle464/predicates_test.go:3` — above `package cycle464`

```text
// Package cycle464 materialises the cycle-464 acceptance criteria for the
// two triage-committed tasks (## top_n only): S1 of the FLEET-AS-POLICY
// operator-priority goal (backlog deferred by explicit operator order —
// see scout-report.md Carryover Decisions).
//
//	fleet-policy-block (P1, go/internal/policy/policy.go)  → C464_001..005
//	fleet-policy-docs  (P2, docs-only, depends on P1)      → C464_006..009
//
// 1:1 AC-materialization: 9 predicates + 1 manual+checklist (the
// fleet-policy-docs [model] "Table accuracy" grader, disposed in
// test-report.md — no automated predicate stands in for a model grader)
// + 0 removed = 10 ACs total (5 in evals/fleet-policy-block.md,
// 5 in evals/fleet-policy-docs.md), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): C464_001-004
// fail because go/internal/policy/fleet_config_param_test.go references
// policy.FleetPolicy/FleetConfig/FleetConfig(), which do not exist yet —
// the internal/policy package fails to COMPILE, so every test that touches
// it (including the whole-package regression C464_003 and the apicover
// naming sweep C464_004, which needs a green coverage profile) is red for
// the same root cause. C464_005 is a PRE-EXISTING GREEN config-check (no
// production reader exists yet for the not-yet-invented env names — this
// predicate exists to prevent their future introduction, mirroring the
// cycle-22 dead-flag contract). C464_006-008 fail because neither
// docs/operations/runtime-reference.md nor docs/architecture/control-flags.md
// mentions the fleet block yet. C464_009 is PRE-EXISTING GREEN for the same
// reason as C464_005 (nothing to leak yet).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C464_002 (unknown plan_source must fail safe to "manual",
//	            not pass through unchanged — kills a vocab-blind getter),
//	            C464_005/C464_009 (no new EVOLVE_FLEET_* env var may appear
//	            in production code or docs — config-driven only)
//	Edge/OOD:   C464_001 (count 0/negative clamp; concurrency 0 follows the
//	            RESOLVED count, not the raw input — the zero/negative-lane
//	            edge cases a hardcoded-defaults getter cannot fake)
//	Semantic:   C464_006 vs C464_007 (the runtime-reference KEY TABLE is a
//	            distinct requirement from control-flags.md's CLOSED-VOCAB
//	            documentation — both must hold independently; a docs stub
//	            mentioning "fleet" once satisfies neither)
```

### `go/acs/cycle464/predicates_test.go:124` — above `func TestC464_004_ApicoverNamingEnforced(t *testing.T) {`

```text
// TestC464_004_ApicoverNamingEnforced (AC4, CI-parity): mirrors
// .github/workflows/go.yml's "api-coverage enforce" step scoped to
// internal/policy, EXACTLY the eval fleet-policy-block.md grader command —
// every new exported symbol (FleetPolicy/FleetConfig/FleetConfig) must be
// named by a test AST AND show >0% executed coverage. Kills the cycle-413
// gaming class (a new exported symbol shipped without a naming test breaks
// main CI's repo-wide apicover -enforce).
```

### `go/acs/cycle464/predicates_test.go:145` — above `func TestC464_005_NoNewFleetEnvFlags(t *testing.T) {`

```text
// TestC464_005_NoNewFleetEnvFlags (AC5, negative, config-check): the fleet
// block is config-driven ([[no_feature_flags_use_design_patterns]]) — no
// production Go file may read EVOLVE_FLEET_COUNT/CONCURRENCY/PLAN via
// os.Getenv. PRE-EXISTING GREEN today (the names don't exist yet); this
// predicate is the standing contract that prevents their future
// introduction (mirrors the cycle-22 dead-flag pattern).
//
// acs-predicate: config-check
```
