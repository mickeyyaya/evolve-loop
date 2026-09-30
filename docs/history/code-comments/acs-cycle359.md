# Comment history: `acs/cycle359`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle359/predicates_test.go:3` — above `package cycle359`

```text
// Package cycle359 materializes the cycle-359 acceptance criteria for the
// committed top_n task:
//
//   - remove-dead-platform-cli-hybrid-cluster — remove 5 dead Platform/CLI Hybrid
//     cluster flags (GEMINI_CLAUDE_PATH, GEMINI_REQUIRE_FULL, CODEX_CLAUDE_PATH,
//     ALLOW_INTERACTIVE_FALLBACK, FORCE_BARE) from flagregistry, update
//     cycle354/amplified_test.go, and regenerate control-flags.md (285 → 280 flags).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	remove-dead-platform-cli-hybrid-cluster:
//	  AC1  5 flags absent from flagregistry.Lookup                  → C359_001
//	  AC2  evolve flags check exits 0 (Generated Index in sync)     → C359_002 (pre-existing GREEN)
//	  AC3  cycle354 acs tests all pass (Amp_003 updated)            → C359_003 (pre-existing GREEN)
//	  AC4  5 flags absent from control-flags.md                     → C359_004
//	  AC5  0 production readers outside acs/ remain                 → C359_005
//	  [adversarial] live Platform/CLI Hybrid flags not over-removed → C359_006 (pre-existing GREEN)
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (BYPASS_SHIP_VERIFY, DISABLE_AUTO_RETROSPECTIVE, sandbox cluster) get zero predicates.
```
