# Comment history: `acs/cycle680`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle680/predicates_test.go:3` — above `package cycle680`

```text
// Package cycle680 materializes the cycle-680 acceptance criteria for the sole
// committed top_n task selfcheck-breaker-fail-loud (triage-report.md ## top_n;
// the scout's chronicle-s2-digest-writer pick was DEFERRED by triage under the
// fleet_scope constraint — its red tests are preserved in the cycle workspace
// under deferred-chronicle-red-tests/ for the future cycle that commits it).
//
// AC map (1:1):
//
//	AC1 selfcheck MkdirAll/write failure WARNs to stderr  → C680_001/002
//	AC2 selfcheck healthy write stays silent + roundtrips → C680_003/004
//	AC3 breaker persist failure WARNs, prior state kept   → C680_005/006
//	AC4 breaker healthy write stays silent                → C680_007
//
// Each predicate shells `go test -run '^<name>$' -v` over the unit-test
// contract, which EXERCISES the SUT (writeBuildSelfCheckArtifact over real
// temp worktrees; the contract-gate breaker writer over real temp paths) —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
```
