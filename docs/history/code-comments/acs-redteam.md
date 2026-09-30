# Comment history: `acs/redteam`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/redteam/predicates_test.go:3` — above `package redteam`

```text
// Package redteam holds the standing red-team EGPS predicates — the anti-gaming
// invariants that fire every cycle (the Go lane runs `./acs/redteam`). Each is a
// thin wrapper over internal/redteamcheck (where the detection logic lives and
// is adversarially unit-tested in normal CI), run against the REAL .evolve/
// ledger + state. A predicate SKIPs when its evidence is absent (fresh clone)
// and FAILs (t.Errorf) on a detected gaming signature. Ported from
// acs/red-team/rt-*.sh (EGPS Go-native migration; ADR-0025).
```

### `go/acs/redteam/predicates_test.go:21` — above `func evolveDir(t *testing.T) string {`

```text
// evolveDir resolves the .evolve/ directory the predicate inspects: the suite
// exports EVOLVE_PROJECT_ROOT (MAIN, even from a worktree — issue #12), else the
// repo root.
```

### `go/acs/redteam/predicates_test.go:33` — above `func TestRT001_LedgerRoleCompleteness(t *testing.T) {`

```text
// TestRT001_LedgerRoleCompleteness ports red-team-001: the last completed cycle
// must have scout + builder + auditor agent_subprocess entries (cycle-102-111).
```

### `go/acs/redteam/predicates_test.go:45` — above `func TestRT002_NoBatchCycleJump(t *testing.T) {`

```text
// TestRT002_NoBatchCycleJump ports red-team-002: state.json:lastCycleNumber must
// not run >1 ahead of the highest cycle with ledger evidence (cycle-132-141).
```
