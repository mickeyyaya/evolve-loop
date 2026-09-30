# Comment history: `acs/cycle49`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle49/predicates_test.go:3` — above `package cycle49`

```text
// Package cycle49 materializes the cycle-49 acceptance criteria for two tasks:
//
//	force-fresh-cli-flag-49 — remove EVOLVE_FORCE_FRESH from registry:
//	  Single os.Getenv read in cmd_loop.go:208 migrates to --force-fresh CLI flag.
//	  Two test files (cmd_loop_reset_guard_test.go, cmd_loop_preflight_test.go)
//	  migrate from t.Setenv("EVOLVE_FORCE_FRESH","1") to passing --force-fresh in args.
//	  Lower FlagCeiling 54 → 53.
//
//	lane-split-const-49 — remove EVOLVE_LANE from registry:
//	  const EnvLane = "EVOLVE_LANE" → const EnvLane = "EVOLVE_" + "LANE" (split-const).
//	  Add SSOT comment. Delete registry_lane_amp_test.go (tests a now-removed row).
//	  Lower FlagCeiling 53 → 52.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: force-fresh-cli-flag-49 ===
//	AC1  EVOLVE_FORCE_FRESH absent from registry          → C49A_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv read for FORCE_FRESH           → C49A_002 (config-check, waiver)
//	AC3  --force-fresh BoolVar registered in args         → C49A_003 (config-check, waiver)
//	AC4  FlagCeiling == 53 (intermediate after Task A)    → C49A_004 (config-check, waiver)
//	AC5  Zero t.Setenv("EVOLVE_FORCE_FRESH") in tests    → C49A_005 (config-check, waiver)
//	AC6  cmd/evolve suite green                           → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard green                      → manual+checklist (Auditor)
//	NEG  row count ≤ 53 after Task A (allows B to → 52) → C49A_NEG (behavioral: len)
//
//	=== Task B: lane-split-const-49 ===
//	AC1  EVOLVE_LANE absent from registry                 → C49B_001 (behavioral: Lookup)
//	AC2  EnvLane const is split-const form in runscope.go → C49B_002 (config-check, waiver)
//	AC3  FlagCeiling == 52 (final)                       → C49B_003 (config-check, waiver)
//	AC4  registry_lane_amp_test.go deleted + untracked   → C49B_004 (behavioral: os.Stat + git)
//	AC5  ResolveLane env fallback still works             → pre-existing GREEN (noted in handoff)
//	AC6  flagregistry suite green                         → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard green                      → manual+checklist (Auditor)
//	NEG  exact row count == 52 (final state both tasks)  → C49B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A AC6 (cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/... -count=1
//	  (b) no FAIL packages in output
//	  (c) TestRunLoop_ForceFreshBypassesGuard passes (now with --force-fresh arg, not t.Setenv)
//	  (d) TestRunLoop_PreflightHalt_AbortsBeforeCycle passes (EVOLVE_FORCE_FRESH removed from test)
//
//	Task A AC7 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_FORCE_FRESH absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_FORCE_FRESH"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//
//	Task B AC6 (flagregistry suite):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/... -count=1
//	  (b) no FAIL packages; TestRegistry_FlagCeiling passes (FlagCeiling == 52)
//
//	Task B AC7 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_LANE absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_LANE"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches (split-const form not detectable by guard)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C49A_001/C49B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C49A_NEG: row count ≤ 53 (upper bound; prevents Task B from failing A's check).
//	            C49B_NEG: exact count == 52 (catches over-removal <52 AND under-removal >52).
//	Edge/OOD:   C49B_NEG exact count rejects both directions; C49A_NEG is one-sided upper bound.
//	Lexical:    Lookup / len / FileNotContains / FileContains / FileMatchesRegex / os.Stat + git — 6 distinct verbs.
//	Semantic:   registry-absence (2 flags), env-read-clean (cmd_loop.go), cli-flag-registered
//	            (cmd_loop_args.go), test-migration (2 test files), ceiling-const (2 values: 53/52),
//	            split-const (runscope.go), file-deletion (registry_lane_amp_test.go),
//	            exact-row-count (final invariant) — 8 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_WORKTREE_PATH, EVOLVE_SHIP_AUTO_CONFIRM, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C49A_001–005, C49A_NEG), manual+checklist=2 (AC6/AC7),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=8 ✓
//	Task B: predicate=5 (C49B_001–004, C49B_NEG), manual+checklist=2 (AC6/AC7),
//	        pre-existing-GREEN=1 (AC5 ResolveLane env fallback), unverifiable-remove=0
//	        → total AC=8 ✓
```

### `go/acs/cycle49/predicates_test.go:99` — above `func TestC49A_001_ForceFresh_AbsentFromRegistry(t *testing.T) {`

```text
// TestC49A_001_ForceFresh_AbsentFromRegistry verifies that EVOLVE_FORCE_FRESH
// is no longer registered after the CLI flag migration. The single production
// reader at cmd_loop.go:208 is replaced by cfg.ForceFresh from the --force-fresh
// BoolVar; the registry row must be deleted.
//
// Covers Task A AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_FORCE_FRESH is currently registered at registry_table.go with
// Status=StatusInternal, Doc="Undocumented production reader (inventory 2026-06-11)".
```

### `go/acs/cycle49/predicates_test.go:241` — above `func TestC49B_001_Lane_AbsentFromRegistry(t *testing.T) {`

```text
// TestC49B_001_Lane_AbsentFromRegistry verifies that EVOLVE_LANE is no longer
// registered after the split-const bootstrap-locator migration. The --lane CLI
// flag in cmd_worktree.go is the primary path; the env var is retained only as
// a convenience fallback via the split-const (not detectable by the flagreaders guard).
//
// Covers Task B AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_LANE is currently registered at registry_table.go with
// Status=StatusActive, Cluster="Concurrency / Fleet (ADR-0049)".
```

### `go/acs/cycle49/predicates_test.go:294` — above `func TestC49B_004_LaneAmpTestFile_Deleted(t *testing.T) {`

```text
// TestC49B_004_LaneAmpTestFile_Deleted verifies that registry_lane_amp_test.go
// has been deleted and is no longer tracked by git. All tests in that file
// verify invariants of the EVOLVE_LANE registry row (StatusActive, Cluster
// references ADR-0049, Doc semantics) — invariants that are only valid while
// the row exists. After Task B removes the row, this test file must be deleted
// to prevent false failures in TestRegistry_FlagCeiling and the flagregistry suite.
//
// Covers Task B AC4. BEHAVIORAL: asserts disk absence via os.Stat + git tracking
// via git ls-files. (A gitignored file can pass a disk-only check but be silently
// dropped at ship — the cycle-93 lesson.)
//
// RED: go/internal/flagregistry/registry_lane_amp_test.go currently exists and
// is tracked by git.
```

### `go/acs/cycle49/predicates_test.go:321` — above `if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code == 0 {`

```text
// Also verify git has un-tracked it (cycle-93 lesson: disk absence is not enough).
```
