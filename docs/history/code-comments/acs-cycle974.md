# Comment history: `acs/cycle974`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle974/predicates_test.go:3` — above `package cycle974`

```text
// Package cycle974 materializes the cycle-974 acceptance criteria for this
// fleet lane's single scoped id `scan-phase-fast-tier-envelopes`, which scout
// resolved to a real phase-contract-drift defect plus a tightly-coupled
// inert-API defect in the SAME model-tier-override mechanism. Triage committed
// TWO coherent top_n tasks (Task 2 dependsOn Task 1); predicates are authored
// for BOTH (both are top_n, not deferred — the deferred
// profile-authoring-lint-schema-validator gets ZERO predicates, R9.3
// floor-binding).
//
//	T1 envelope-floor-guard-model-tier-overrides — every model_tier_overrides
//	   value in every profile MUST rank within its OWN
//	   model_tier_envelope [min,max] on the canonical ladder
//	   fast<balanced<deep<top. Currently violated (see RED note below), AND a
//	   permanent regression guard TestModelTierOverridesWithinEnvelope must
//	   exist in package profiles so future drift is caught in normal CI.
//	T2 wire-or-remove-model-tier-overrides-consumer — the ModelTierOverrides
//	   map has ZERO production consumers (only *_test.go read it; phaseconfig.go
//	   names it in a comment only). "No inert API" is a hard goal constraint:
//	   Builder must EITHER wire a real production consumer OR remove the inert
//	   override entries. Leaving it as-discovered (declared but unread) is not
//	   an acceptable outcome — the disjunction predicate below red-fails it.
//
// PREDICATE STYLE (cycle-85 rule): go/internal/profiles + modelcatalog are
// importable from go/acs, so C974_001 EXERCISES the live profiles.Loader (the
// SUT) against the real config and asserts on the typed
// ModelTierEnvelope/ModelTierOverrides fields — a magic-string source edit
// cannot satisfy it, the JSON must change. C974_002 runs the real guard test
// via a `go test` subprocess. C974_003 asserts a structural deliverable-state
// disjunction (a real production consumer exists, OR the inert entries are
// gone) — not a "does source contain text X" grep: neither branch can be faked
// without actually wiring a consumer or deleting the dead config.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE → C974_001 red-fails on the CURRENT drifted config (the strongest
//	           anti-no-op: an empty/no-op change leaves violations in place);
//	           C974_003 red-fails on the leave-as-is (inert) outcome.
//	POSITIVE → C974_002 requires the permanent guard test to be PRESENT and
//	           PASS (a positive signal a no-op cannot fake).
//	EDGE     → C974_001 enforces BOTH envelope bounds (below-min AND above-max),
//	           catching over-max overrides scout's prose undercounted.
//	SEMANTIC → floor-guard (T1) and inert-API resolution (T2) are DISTINCT
//	           outcomes, asserted by separate predicates.
//
// RED note (surfaced per Core Rule 3 — no silent change): scout's prose
// estimated "6 violations". C974_001 encodes the AC as literally stated
// ("between min and max"), which also catches OVER-max overrides scout's
// min-direction scan missed: tester (ultrathink/m_complex/audit_retry=deep >
// max=balanced) and orchestrator (cycle_1_or_low_goal=deep > max=balanced).
// The true pre-fix violation count is higher than 6; the predicate is faithful
// to the stated invariant, not the undercount.
//
// AC map (1:1 with test-report.md AC-Materialization table):
//
//	T1-a permanent envelope guard test exists & passes    → C974_002 (predicate)
//	T1-b every override within its own [min,max] envelope  → C974_001 (predicate)
//	T2   ModelTierOverrides consumed OR removed (no inert)  → C974_003 (predicate)
```
