# Comment history: `acs/cycle463`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle463/predicates_test.go:3` — above `package cycle463`

```text
// Package cycle463 materialises the cycle-463 acceptance criteria for the
// three triage-committed tasks (## top_n only; T2 advisor-plan-measured-
// usage-inputs was deferred by triage and gets ZERO predicates per R9.3):
//
//	T1 advisor-plan-prompt-tier-elicitation      (P1) → C463_001..006
//	T3 runner-overlay-observability-dossier-model-source (P3) → C463_007..012
//	T4 routing-replay-clamp-golden-matrix        (P4) → C463_013..018
//
// Each predicate shells to the NAMED RED unit test(s) written this cycle in
// go/internal/{core,router,phases/runner,dossier} (requireTestsRan guards
// against a silent "no tests to run" false-green), so a predicate can never
// pass on unwritten or renamed work. 1:1 AC-materialization: 18 predicates +
// 0 manual + 0 removed = 18 ACs (6 per task, matching each task's eval file
// in .evolve/evals/), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): every
// predicate below currently FAILs — either the target package fails to
// compile (phasespec.PhaseSpec/router.Clamp/phasetiming.Entry/runner.Options
// carry no new fields yet; router.RejectionsFromClamps does not exist) or
// the named test asserts a behavior the production code does not implement
// yet (fast-below-min clamps to empty, not up to balanced). The "regression"
// predicates (C463_006/012/018) fail for the same reason: a package that
// doesn't compile cannot pass `go vet`/`go test` for the whole package.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C463_004 (absent cli/tier fields must never fabricate an
//	            overlay), C463_009 (pin must win over a non-empty overlay),
//	            C463_016 (no clamp relaxation for a persuasive justification)
//	Edge/OOD:   C463_005 (tier vocabulary confinement: high/top/raw model),
//	            C463_011 (legacy workspace with no model metadata),
//	            C463_017 (legacy cycle-459-shape response byte-identical)
//	Semantic:   C463_002 vs C463_014 (guardrail PROJECTION is distinct from
//	            guardrail RENDERING — both must hold), C463_013 vs C463_017
//	            (an in-bounds proposal overlays; a legacy one never does —
//	            only jointly satisfiable by correct gating on cli/tier
//	            presence)
```

### `go/acs/cycle463/predicates_test.go:288` — above `func TestC463_017_LegacyReplayByteIdentical(t *testing.T) {`

```text
// TestC463_017_LegacyReplayByteIdentical (AC5, edge): replaying the exact
// legacy (cycle-459-shape) response yields zero model-routing clamps and a
// dispatch byte-identical to the profile-static baseline.
```
