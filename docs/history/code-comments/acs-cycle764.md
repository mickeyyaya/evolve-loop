# Comment history: `acs/cycle764`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle764/predicates_test.go:3` — above `package cycle764`

```text
// Package cycle764 materializes the cycle-764 acceptance criteria for the sole
// task committed by triage-report.md "## top_n":
// ship-manual-deletes-running-binary (inbox
// 2026-07-13T08-45-00Z-ship-manual-deletes-running-binary.json, weight 0.94).
//
// Defect: `evolve ship --class manual` routes through shipDirect →
// discardBinaryChurn on the SUCCESS path; cmd_ship.go never sets
// Options.ShipBinaryPath, so the discard falls back to os.Executable() — the
// running, UNTRACKED (gitignored) go/bin/evolve — and os.Remove()s it (live
// incidents 2026-07-12; cycle-243 precedent: kernel hooks silently degrade).
// Rollback shells `ship --class manual` (rollback.defaultRevertAndShip), so
// the same shipDirect guard covers that path transitively.
//
// AC map (1:1), from the inbox item's acceptance list:
//
//	AC1 discard never removes the running executable (skip+WARN) → C764_001
//	AC2 manual-ship success leaves untracked go/bin/evolve       → C764_002
//	AC3 rollback shellout path covered (same shipDirect entry)   → C764_002 (see doc)
//	AC4 ship+rollback suites -race PASS (no regression)          → C764_003
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/phases/ship, which EXERCISES discardBinaryChurn /
// shipDirect against real Options — behavioral via subprocess, no source-grep
// predicates (cycle-85 rule). The `-v` + "--- PASS:" guard rejects a rename /
// no-tests-matched silent green.
```
