# Comment history: `acs/cycle1176`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1176/predicates_test.go:3` — above `package cycle1176`

```text
// Package cycle1176 materialises the cycle-1176 acceptance criteria for the
// three fleet-scoped ids pinned to this lane:
//
//   - wave-lane-task-quarantine-dead          → predicates 001-004
//   - workspace-hygiene-s5-wiring-shadow-default → predicate 005
//   - wave-planner-pass-scope-prune           → predicate 006
//
// STATE NOTE (read this before treating a GREEN here as a rubber stamp). All
// three implementations are already resident in this worktree's base: the
// lifecycle seam (inboxmover.ApplyCycleOutcome + ClaimLaneScope, wired at the
// production FAIL site cmd_loop.go:728 and the PASS site postship.go:188)
// landed in 4e523438 (#372), and the gc workspace sweep + wave-plan scope prune
// landed in the salvage snapshot f2d87339. Those cycles' own predicates
// (acs/cycle1156, acs/cycle1172) are CYCLE-SCOPED — they do not run again — so
// this package re-pins the same behaviour under this cycle's scope rather than
// asserting a fresh RED. See test-report.md for the pre-existing-GREEN
// disposition and the "consumed scope was re-picked" finding it raises.
//
// Predicate strategy — every predicate exercises the system under test, never
// greps production source (the cycle-85 degenerate-predicate ban):
//
//   - 001-004 drive inboxmover.ApplyCycleOutcome over a real temp-dir inbox in
//     the exact WAVE-LANE shape the defect described (committed ids that were
//     never claimed into processing/) and assert on the resulting filesystem
//     state: durable failure_count, release destination, quarantine parking.
//   - 005/006 shell the landed unit suites for the CLI-surface and planner
//     halves. `go test -run` exits 0 when its pattern matches NOTHING, so these
//     run with -v and require an explicit `--- PASS: <name>` line per expected
//     test — a renamed or deleted test reads as FAIL here, never a silent pass.
```

### `go/acs/cycle1176/predicates_test.go:123` — above `func TestC1176_001_WaveLaneFailBumpsUnclaimedCommittedID(t *testing.T) {`

```text
// TestC1176_001_WaveLaneFailBumpsUnclaimedCommittedID is the crux predicate for
// wave-lane-task-quarantine-dead: a lane that never claimed its scope into
// processing/ must STILL accrue a task-level failure on its committed id. This
// is the exact batch-14 shape in which failure_count stayed at 0 across four
// FAILs and the ADR-0072 S5 ceiling was structurally unreachable.
```

### `go/acs/cycle1176/predicates_test.go:145` — above `func TestC1176_002_UncommittedMenuIDNeitherBumpsNorMoves(t *testing.T) {`

```text
// TestC1176_002_UncommittedMenuIDNeitherBumpsNorMoves is the NEGATIVE half —
// menu semantics (PR #366). No phase worked the uncommitted id, so it must not
// accrue a task-level failure. Without this, N failures of an unrelated task
// walk the whole menu to the quarantine ceiling and a healthy backlog is parked.
```

### `go/acs/cycle1176/predicates_test.go:188` — above `func TestC1176_004_SystemLevelFailureNeverBumps(t *testing.T) {`

```text
// TestC1176_004_SystemLevelFailureNeverBumps is the second NEGATIVE (ADR-0072
// S3 precedence, AC4). A quota/infra storm is not the task's fault: it must
// neither bump nor quarantine, or one later task-level FAIL parks a backlog
// that never failed on its own merits.
```

### `go/acs/cycle1176/predicates_test.go:246` — above `func TestC1176_006_WavePlanSeedDropsConsumedScope(t *testing.T) {`

```text
// TestC1176_006_WavePlanSeedDropsConsumedScope pins
// wave-planner-pass-scope-prune: the consumed-state filter runs at wave-PLAN
// seed time, so lane-scope.json never records an id already resolved to
// processed/ (cycle-1116), while pending/unknown scopes survive (the anti-no-op
// negative: a planner that pruned everything would pass a prune-only check) and
// an all-consumed prior decision still plans live work rather than an empty wave.
```
