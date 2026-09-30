# Comment history: `acs/cycle358`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle358/predicates_test.go:3` — above `package cycle358`

```text
// Package cycle358 materializes the cycle-358 acceptance criteria for the
// committed top_n task:
//
//   - channel-bridge-retirement — retire the EVOLVE_CHANNEL deprecated bridge
//     flag from flagregistry, simplify channel.Enabled() from 2-param to
//     1-param, remove all production readers in tmux_inject.go and
//     core_adapter.go, update 5 test files, and regenerate control-flags.md
//     (286 → 285 flags).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	channel-bridge-retirement:
//	  AC-1  EVOLVE_CHANNEL absent from flagregistry.Lookup          → C358_001
//	  AC-2  explicitChannel param removed from Enabled()            → C358_002
//	  AC-3  no production Go reader of EVOLVE_CHANNEL               → C358_003
//	  AC-4  bridge files fully cleaned (subset of AC-3 surface)     → C358_003
//	  AC-5  channel tests pass (TestEnabled_UsesStageOnly)          → C358_004
//	  AC-6  bridge + observer tests pass                            → C358_005
//	  AC-7  flagregistry tests pass                                 → C358_006
//	  AC-8  shadow/off → channel off (covered by channel tests)     → C358_004
//	  AC-9  control-flags.md regenerated (EVOLVE_CHANNEL row gone)  → C358_007
//	  AC-10 ACS cycle-358 guard passes (meta — this file)           → this package
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (internal-flag-classification, test-seam-relocation, etc.) get zero predicates.
```
