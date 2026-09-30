# Comment history: `acs/cycle440`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle440/predicates_test.go:3` — above `package cycle440`

```text
// Package cycle440 materialises the cycle-440 acceptance criteria for
// model_routing MR4 + MR5 (goal: b17855662370871400e2d044ab4c104dc7d19c940d872b63892330b0bca98049).
//
// Three ## top_n tasks, strictly sequenced A → B → C (triage-report.md):
//
//	Task A (mr4b-basecli-single-source, S): consolidate policy.baseCLI +
//	  bridge.baseCLIName into ONE exported policy.BaseCLI, and normalize the
//	  CLI base name before router.ClampPlanModelRouting's catalog lookup
//	  (F2/F3 — a suffixed "claude-tmux" currently misses the catalog, which
//	  is keyed on the base family "claude").
//	Task B (mr4-projection-apply-degrade, M/L): wire the UNWIRED
//	  router.ClampPlanModelRouting into the cycle-start plan path, thread a
//	  phase→{cli,tier} soft-overlay proposal into PhaseRequest, apply it in
//	  phases/runner/runner.go with pin==nil (soft, not absolute), and degrade
//	  to profile-static whenever no plan (or no per-phase proposal) exists.
//	Task C (mr4d-default-model-routing-auto, S): once B lands, default
//	  model_routing to auto in the checked-in registry (config-driven, not a
//	  Go literal); static/off remain the escape hatch.
//
// AC map (1:1, R9.3 floor-binding; predicates for the ## top_n tasks only):
//
//	Task A AC1 suffixed CLI honored via base name (positive)     → C440_001
//	Task A AC2 single exported source, dup removed (negative)    → C440_002 + C440_003
//	Task A AC3 apicover naming floor (regression)                 → C440_004
//	Task A AC4 genuine catalog miss still clamps (edge/OOD)       → C440_005 (pre-existing GREEN pin)
//	Task B AC1 auto applies a clamped overlay (positive)          → C440_006
//	Task B AC2 advisory logs, does not apply (mode semantics)     → C440_007
//	Task B AC3 static is a noop (regression floor)                → C440_008
//	Task B AC4 nil-plan degrades to profile-static (negative/OOD) → C440_009
//	Task B AC5 benched overlay primary still falls back (edge)    → C440_010
//	Task B AC6 no regression across touched packages              → C440_011
//	Task C AC1 checked-in registry declares model_routing=auto    → C440_012
//	Task C AC2 Go zero-value stays static (regression floor)      → C440_013
//	Task C AC3 checked-in registry loads as auto (config-default) → C440_014
//	Task C AC4 escape hatch (static/off) honored (negative)       → C440_015
//
// 1:1 enforcement: predicate=15 → total AC = 14 (Task A AC4 gets two
// predicates: the pre-existing-GREEN pin plus its share of C440_004's
// compile-and-pass regression sweep) ✓ every AC ≥1 predicate, none double-
// counted as a DIFFERENT AC.
//
// *** IMPORTANT DISCREPANCY — Task C's target file (read this before auditing
// C440_012/C440_014) ***
//
// The scout report, api-contract.md, and eval mr4d-default-model-routing-
// auto.md all say the default flip belongs in ".evolve/policy.json". Reading
// the ACTUAL producer (go/internal/config/config.go's registryDoc, whose
// model_routing field is bound to registryPath's `config.model_routing` key)
// and every real call site that builds registryPath (cmd/evolve/cmd_cycle.go,
// internal/cli/phasecmd/phase_verify.go, internal/router/policy.go) shows
// model_routing is parsed EXCLUSIVELY from
// docs/architecture/phase-registry.json. .evolve/policy.json is a SEPARATE
// file (policy.Load) that feeds Policy.Pins/MandatoryPhases/ShipFloor/etc. —
// it never reaches RoutingConfig.ModelRouting. config.go's own comments
// (lines 294/598/600) are stale/wrong on this point (a pre-existing
// documentation bug, out of this cycle's scope to fix elsewhere). C440_012 /
// C440_014 target the file the code actually reads (Rule 8: read first, don't
// invent an API from context) rather than the eval's literal (incorrect)
// grep target. See internal/config/model_routing_default_test.go's doc
// comment on TestCheckedInPolicyDefaultsModelRoutingAuto for the full trail.
// Builder/Auditor: edit docs/architecture/phase-registry.json's `config`
// object, NOT .evolve/policy.json, to satisfy Task C.
//
// *** Task A landmine for Builder: an existing test calls the doomed helper
// directly ***
//
// internal/bridge/catalog_overlay_test.go:78 calls the unexported
// baseCLIName(...) directly. AC2 requires that helper GONE — Builder must
// update/remove that assertion (route it through policy.BaseCLI, mirroring
// applyCatalogTierMap's own call-site fix) as part of the Task A removal, not
// leave it as a dangling compile error.
//
// RED strategy: C440_001/004/006-011 are compile-fail RED today (they
// reference policy.BaseCLI, llmroute.Overlay/ApplySoftOverlay,
// core.PhaseRequest.ModelRoutingCLI/Tier, and core.WithModelCatalogLookup —
// none exist yet on main). C440_002/003 are grep-based structural RED (the
// duplication still exists). C440_012/014 are behaviorally RED (the checked-
// in registry has no model_routing key yet, so it parses to static, not
// auto). C440_005/013/015 are documented pre-existing GREEN pins (the
// underlying behavior is already correct and unaffected by this cycle's
// change — see doc comments on the referenced tests).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C440_002 (duplication genuinely removed, not merely papered
//	            over — a source-region scan defeats a redundant helper, not
//	            just a passing value check), C440_009 (advisor outage must not
//	            silently apply a stale/garbage overlay), C440_015 (escape
//	            hatch must still work once the default flips)
//	Edge/OOD:   C440_005 (a genuine catalog miss, not just a suffix mismatch),
//	            C440_010 (benched overlay primary — health-state edge case)
//	Semantic:   C440_007 (advisory RECORDS the proposal but does not DISPATCH
//	            it — a distinct property from "computes the right value")
```

### `go/acs/cycle440/predicates_test.go:177` — above `func TestC440_004_ApicoverNamingFloor(t *testing.T) {`

```text
// TestC440_004_ApicoverNamingFloor (Task A AC3, regression): router + policy
// + bridge must compile and pass with the new exported symbol referenced in
// a _test.go (apicover -enforce naming floor, ADR-0069 CI-parity).
```
