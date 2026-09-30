# Comment history: `acs/cycle341`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle341/predicates_test.go:3` — above `package cycle341`

```text
// Package cycle341 materializes the cycle-341 acceptance criteria for the two
// committed top_n tasks (driver-agnostic model-routing campaign, continuation):
//
//	T1  all-profiles-substitutability-parity-test — add TestAllProfilesSubstitutabilityAtParity
//	    to go/internal/profiles/profile_model_routing_amplification_test.go; uses real bridge
//	    manifests via loadSwappableManifests (not a synthetic Catalog{}); iterates all profiles
//	    via loader.List(); checks default tier AND overrides AND envelope; uses t.Errorf not t.Skip.
//
//	T2  policy-doc-substitutability-reference — update docs/architecture/model-routing-policy.md
//	    "Substitutability acceptance test" paragraph to cite TestAllProfilesSubstitutabilityAtParity
//	    and document allowed_clis dispatch exceptions for builder/tdd-engineer/tester.
//
// Predicates follow the cycle-85 lesson: BEHAVIORAL where possible; config-check waivers only
// where the deliverable IS the test/doc contract file.
//
// AC map (1:1 with triage-report.md top_n items):
//
//	T1.pass         TestAllProfilesSubstitutabilityAtParity passes           → C341_001 (BEHAVIORAL)
//	T1.real-manifests test uses loadSwappableManifests not synthetic Catalog → C341_002 (config-check)
//	T1.loader-list  test iterates via loader.List() not hardcoded list       → C341_003 (config-check)
//	T1.errorf       test uses t.Errorf not t.Skip for misses                 → C341_004 (config-check)
//	T1.regression   full go test ./internal/profiles/... passes (≥24 tests) → C341_005 (BEHAVIORAL)
//	T2.citation     policy doc cites TestAllProfilesSubstitutabilityAtParity → C341_006 (config-check)
//	T2.allowed-clis policy doc documents allowed_clis dispatch exceptions    → C341_007 (config-check)
//	T2.no-vendor    policy doc substitutability section has no bare vendor   → C341_008 (config-check, pre-existing GREEN)
//	                model names (haiku/sonnet/opus) as tier values
```
