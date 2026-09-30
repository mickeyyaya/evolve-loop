# Comment history: `acs/cycle10`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle10/predicates_test.go:3` — above `package cycle10`

```text
// Package cycle10 materializes the cycle-10 acceptance criteria for two
// committed top_n tasks (flag-reduction wave-1):
//
//	w1-dead-flags — delete 5 StatusActive/StatusInternal flags with 0 readers
//	  (PROMPT_MAX_TOKENS, REAP_ORPHANS, TESTING, SANDBOX_FALLBACK_ON_EPERM,
//	   WORKTREE_PATH) and clean all surface refs.
//
//	w1-tombstones-and-compose — delete 8 StatusDeprecated tombstones
//	  (ANTHROPIC_BASE_URL, HANG_CLASSIFIER, MODELCATALOG_AUTOREFRESH,
//	   MARKETPLACE_DIR, ADVISOR_DEPTH, DISABLE_WORKSPACE_GUARD, POLICY_BYPASS,
//	   PLATFORM) + convert EVOLVE_COMPOSE_PHASES from env signal to DI/policy.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	w1-dead-flags:
//	  AC1  5 dead flag names absent from flagregistry         → C10_001 (behavioral)
//	  AC2  go test ./internal/flagregistry/... PASS           → manual+checklist (CI)
//	  AC3  No new ACS failures after WORKTREE_PATH guards     → manual+checklist (CI)
//	  AC4  FlagCeiling=48 unchanged                           → C10_002 (config-check, pre-existing GREEN)
//	  AC5  LiveFeatureFlagCeiling=21 unchanged                → C10_003 (config-check, pre-existing GREEN)
//
//	w1-tombstones-and-compose:
//	  AC1  9 tombstone names absent from flagregistry         → C10_004 (behavioral)
//	  AC2  No EVOLVE_COMPOSE_PHASES in cmd_compose.go         → C10_005 (behavioral, absence)
//	  AC3  No env bridge reads in cmd_cycle.go / cmd_loop.go  → C10_006 (behavioral, absence)
//	  AC4  go test ./cmd/evolve/... ./internal/core/... PASS  → manual+checklist (CI)
//	  AC5  FlagCeiling/LiveFeatureFlagCeiling UNCHANGED       → covered by C10_002/C10_003
//
// Floor binding (R9.3): predicates authored only for committed top_n tasks.
// Deferred items (none this cycle) get zero predicates.
```

### `go/acs/cycle10/predicates_test.go:72` — above `func TestC10_004_TombstoneFlagsAbsentFromRegistry(t *testing.T) {`

```text
// C10_002/C10_003 (FlagCeiling/LiveFeatureFlagCeiling "unchanged at 48/21")
// retired at integration: the operator ratchets both consts when banking a cycle
// to main (the ceiling-decoupling rule), so a per-cycle exact-value predicate
// here directly conflicts with that ratchet and re-reddens as the campaign
// reduces. The durable SSOT ratchets — TestRegistry_FlagCeiling and
// TestRegistry_LiveFeatureFlagCeiling in internal/flagregistry — are the real
// guards. (Same retired anti-pattern as the cycle-N FlagCeilingConstIsN sweep,
// PR #162.)
```

### `go/acs/cycle10/predicates_test.go:100` — above `"EVOLVE_POLICY_BYPASS",`

```text
// EVOLVE_POLICY_BYPASS bridge converted to --bypass-policy CLI flag in cycle-15;
// row deleted in that cycle (bypass-policy-flag task).
```

### `go/acs/cycle10/predicates_test.go:138` — above `func TestC10_006_EnvBridgesRemovedFromCmdFiles(t *testing.T) {`

```text
// TestC10_006_EnvBridgesRemovedFromCmdFiles verifies that cmd_cycle.go and
// cmd_loop.go no longer contain env bridge reads for EVOLVE_DISABLE_WORKSPACE_GUARD
// or EVOLVE_POLICY_BYPASS after Builder removes the deprecated bridges in
// w1-tombstones-and-compose (DISABLE_WORKSPACE_GUARD) and cycle-15
// bypass-policy-flag (POLICY_BYPASS).
//
// BEHAVIORAL (absence): acsassert.FileNotContains fails only when the flag name
// IS present in the file. The DI replacement fields already exist on CycleRequest
// (DisableWorkspaceGuard bool) and PhaseRequest (BypassPolicy bool); this predicate
// verifies the bridge env read is gone, not just that the DI field exists.
//
// RED: cmd_cycle.go:190 and cmd_loop.go:186,303 still contain
// cycleEnv["EVOLVE_POLICY_BYPASS"] (DISABLE_WORKSPACE_GUARD was already removed).
```

### `go/acs/cycle10/predicates_test.go:153` — above `checks := []struct {`

```text
// EVOLVE_POLICY_BYPASS bridge converted to --bypass-policy CLI flag in cycle-15;
// now included alongside DISABLE_WORKSPACE_GUARD.
```
