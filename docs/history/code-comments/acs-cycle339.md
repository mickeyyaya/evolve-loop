# Comment history: `acs/cycle339`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle339/predicates_test.go:3` — above `package cycle339`

```text
// Package cycle339 materializes the cycle-339 acceptance criteria for the two
// committed top_n tasks (driver-agnostic model-routing campaign):
//
//	T1  migrate-domain-profiles-to-canonical-tiers — all 79 domain/optional
//	    profiles must use canonical capability tiers (fast/balanced/deep) instead
//	    of vendor model names (sonnet/opus/haiku), including overrides and
//	    envelope fields. 4 envelope-driven exceptions apply (memo/evaluator→fast,
//	    plan-reviewer/retrospective→deep).
//	T2  widen-driver-agnostic-acceptance-test — TestAllProfilesAreDriverAgnostic
//	    dynamically enumerates all profiles via os.ReadDir and passes.
//
// Predicates are BEHAVIORAL (cycle-85 lesson): they invoke the system under
// test — the live profiles.Loader for T1, and a go test subprocess for T2.
// No load-bearing assertion is "does source file contain text X".
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.defaults   no vendor names in model_tier_default (79 profiles)   → C339_001
//	T1.overrides  no vendor names in overrides/envelope (14 overrides)  → C339_002
//	T1.exceptions envelope-driven 4 profiles get correct special tiers  → C339_003
//	T2.test       TestAllProfilesAreDriverAgnostic exists and passes     → C339_004
```
