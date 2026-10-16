# Comment history: `internal/adapters/storage`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/adapters/storage/cyclestate_fleet_isolation_test.go:13` — above `func TestCycleState_FleetPerRunOverride_NoClobber(t *testing.T) {`

```text
// cyclestate_fleet_isolation_test.go — the fix for the fleet cycle-state
// singleton race (2026-07-03): two concurrent lanes wrote the host-global
// .evolve/cycle-state.json (last-writer-wins), so lane A's phase-gate
// (guards.Phase reads cycle state) saw lane B's Phase and stalled before audit.
// When ipcenv.CycleStateFileKey is set, each lane reads+writes its OWN per-run
// file — no shared singleton.
```

### `go/internal/adapters/storage/statejson.go:52` — above `func (s *FilesystemStorage) WriteState(_ context.Context, st core.State) error {`

```text
// WriteState atomically replaces .evolve/state.json. A cycle worktree's
// state.json is a symlink to the canonical file (core/worktree.go
// linkGuardDeps); the rename lands on the resolved target so the link survives
// (cycle-999 sever).
```

### `go/internal/adapters/storage/statejson.go:80` — above `path := statemap.ResolveWriteTarget(s.cycleStatePath())`

```text
// A worktree's cycle-state.json links to the run's run.json: lock, read and
// rename on the resolved target so the link survives (cycle-999 sever).
```

### `go/internal/adapters/storage/statejson.go:83` — above `return flock.WithPathLock(path, func() error {`

```text
// ADR-0049 G7: serialize the whole read-modify-write (and the run.json
// mirror) on the cycle-state.json sidecar lock. checkpoint.ApplyToStateFile
// (the other read-modify-writer of this file) holds the SAME lock, so a
// concurrent fleet cycle that owns a different key (the checkpoint block)
// never renames over a stale read and reverts this write — or vice versa.
// No-op cost for the live sequential loop (uncontended); shared with the
// checkpoint writer across packages via flock.WithPathLock (the single
// sidecar-lock home).
```

### `go/internal/adapters/storage/statejson_checkpoint_test.go:14` — above `func writeCheckpointedState(t *testing.T, evolveDir string, cp map[string]any) string {`

```text
// statejson_checkpoint_test.go — RED contract for cycle-295 task
// checkpoint-clobber-fix.
//
// Live incident (host reboot during cycle-294 mutation-gate): every
// pre-dispatch WriteCycleState does a whole-struct json replace, and
// core.CycleState has NO "checkpoint" field, so each write ERASES the
// "checkpoint" block that PhaseBoundaryCheckpointer/ApplyToStateFile
// wrote after the prior phase. Net: a crash mid-phase leaves no
// checkpoint and `evolve loop --resume` fails with "no live checkpoint".
//
// Fix (builder): make WriteCycleState a read-merge-write that carries the
// existing "checkpoint" key through the rewrite (mirrors
// checkpoint.ApplyToStateFile). These tests exercise the REAL
// FilesystemStorage.WriteCycleState and assert on the REAL file bytes —
// a magic string cannot satisfy them.
```

### `go/internal/adapters/storage/statejson_checkpoint_test.go:142` — above `func TestWriteCycleState_PreservesWorktreeBaseSHAWithCheckpoint(t *testing.T) {`

```text
// TestWriteCycleState_PreservesWorktreeBaseSHAWithCheckpoint: the cycle-156
// resume-parity fix persists the cycle base in CycleState.WorktreeBaseSHA.
// It must round-trip through WriteCycleState/ReadCycleState AND coexist with
// the checkpoint-block preservation splice (cycle-295 fix) — proving the two
// mechanisms don't interact.
```

### `go/internal/adapters/storage/statejson_symlink_test.go:3` — above `import (`

```text
// cycle-999 sever regression (atomicwrite-linked-state-sweep, cycle 1694):
// core/worktree.go linkGuardDeps symlinks a cycle worktree's
// .evolve/state.json and .evolve/cycle-state.json at their canonical files. A
// tmp+rename onto the UNRESOLVED path replaces the link with a regular file, so
// every later write strands in a detached copy. Each writer below must keep the
// link intact and land its bytes on the canonical target — for an absolute
// link, a relative link, and a link that dangles until the first write.
```
