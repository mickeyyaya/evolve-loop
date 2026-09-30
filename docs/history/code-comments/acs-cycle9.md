# Comment history: `acs/cycle9`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle9/predicates_test.go:3` — above `package cycle9`

```text
// Package cycle9 materializes the cycle-9 acceptance criteria for two committed
// top_n tasks (current session: dossier ADR-0055 contracts) plus the prior-session
// fanout-consolidation predicates that have been corrected to match current state.
//
// AC map for current top_n tasks (fix-dossier-render-contracts,
// fix-core-apply-defects-blank-filter):
//
//	fix-dossier-render-contracts:
//	  AC-D1  RenderJSON(nil) returns error                     → C9_012 (behavioral)
//	  AC-D2  RenderJSON(invalid) calls Validate, returns error → C9_013 (behavioral)
//	  AC-D3  RenderJSON output ends with '\n'                  → C9_014 (behavioral)
//	  AC-D4  RenderMarkdown(invalid) returns error             → C9_015 (behavioral)
//	  AC-D5  Write(d,"",false) returns error                   → C9_016 (behavioral)
//	  AC-D6  Build({WorkspacePath:""}) returns error           → C9_017 (behavioral)
//	  AC-D7  Build({Goal:""}) returns error                    → C9_018 (behavioral)
//	  AC-D8  All 7 dossier gap tests PASS                      → manual+checklist (CI)
//
//	fix-core-apply-defects-blank-filter:
//	  AC-C1  All-blank defects → 0 todos                       → C9_019 (behavioral)
//	  AC-C2  Mixed blank+real → exactly 2 todos                → C9_020 (behavioral)
//	  AC-C3  Idempotency/deterministic-ID tests still pass     → manual+checklist (CI)
//	  AC-C4  go test ./internal/core/... → PASS               → manual+checklist (CI)
//
//	Removed ACs:
//	  Full suite 0 FAIL: CI pipeline (manual+checklist)
//	  ACS gate PASS: self-referential (unverifiable-remove)
//
// Prior-session fanout predicates (C9_001–C9_011): corrected for current state.
// C9_001 corrected from exact-241 to ratchet ≤160 (further reductions landed post
// the original session). C9_002 corrected from 241→160 (current FlagCeiling).
// C9_003–C9_011 unchanged (all GREEN, fanout consolidation complete).
```

### `go/acs/cycle9/predicates_test.go:103` — above `func TestC9_004_FanoutEnvReadsGoneFromDispatchCmd(t *testing.T) {`

```text
// TestC9_004_FanoutEnvReadsGoneFromDispatchCmd verifies that the
// fanoutEnvConfig() function's envchain.Int("EVOLVE_FANOUT_CONCURRENCY",...) call
// has been removed from cmd_fanout_dispatch.go.
//
// This is the cycle-8 anti-gaming SUBSTANCE requirement: config env reads must
// be DELETED (not hidden via split-const), replaced by policy.json-loaded
// FanoutPolicy values passed as CLI flags to the fanout-dispatch subprocess.
//
// // acs-predicate: config-check — verifies structural code requirement (no
// standalone config env reads in the fanout dispatch command implementation).
//
// RED: cmd_fanout_dispatch.go:60 currently has envchain.Int("EVOLVE_FANOUT_CONCURRENCY",...).
```
