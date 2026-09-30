# Comment history: `acs/cycle340`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle340/predicates_test.go:3` — above `package cycle340`

```text
// Package cycle340 materializes the cycle-340 acceptance criteria for the two
// committed top_n tasks (driver-agnostic model-routing campaign):
//
//	T1  substitutability-at-parity-acceptance-test — add TestSpineSubstitutabilityAtParity
//	    to go/internal/profiles/driver_agnostic_test.go; fixture covers codex/agy/ollama
//	    × fast/balanced/deep tiers; asserts non-empty Lookup() for each spine phase at
//	    its canonical tier; uses t.Errorf (not t.Skip) for lookup failures.
//
//	T2  fix-profiles-agents-md-vendor-tier-docs — update .evolve/profiles/AGENTS.md
//	    "Model selection" table row for model_tier_default to replace legacy vendor names
//	    (haiku/sonnet/opus) with canonical tiers (fast/balanced/deep); add prohibition note.
//
// Predicates are BEHAVIORAL where possible (cycle-85 lesson). C340_001 runs the
// real test subprocess. C340_002 and C340_003 are config-check waivers on the
// test contract file (the test file IS the deliverable). C340_004 and C340_005
// are config-check waivers on a documentation file.
//
// AC map (1:1 with triage-report.md top_n items):
//
//	T1.pass     TestSpineSubstitutabilityAtParity passes in profiles pkg     → C340_001
//	T1.drivers  fixture covers codex, agy, ollama drivers                    → C340_002
//	T1.errorf   function uses t.Errorf not t.Skip for lookup failures        → C340_003
//	T2.no-vendor no haiku/sonnet/opus in model_tier_default row              → C340_004
//	T2.canonical canonical tiers (fast/balanced/deep) appear in row          → C340_005
```
