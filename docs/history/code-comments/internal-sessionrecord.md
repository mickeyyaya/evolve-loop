# Comment history: `internal/sessionrecord`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d11

### `go/internal/sessionrecord/resolve_test.go:8` — above `func TestReadAllResolving_ReadsTombstoneAfterReap(t *testing.T) {`

```text
// TDD RED (cycle-806, task sweep-tombstone-attribution).
//
// ReadAllResolving and ReapedSuffix do not yet exist → this file fails to
// compile until Builder adds the tombstone-aware resolver. Contract: after
// ReapOrphans tombstones a fully-reaped registry (rename <path> →
// <path>+ReapedSuffix, sessionreaper.go:92-95), a caller must STILL be able to
// attribute the killed sessions to their owning run. Attribution is preserved
// in the `.reaped` tombstone, not lost — but the live-path ReadAll can no
// longer discover it. ReadAllResolving closes that gap by unioning the live
// registry with its tombstone, deduped by session id.
```

### `go/internal/sessionrecord/sessionrecord.go:1` — above `package sessionrecord`

```text
// Package sessionrecord is the single source of truth for the per-run tmux
// session registry (CB.5, concurrency campaign): the on-disk JSONL schema one
// writer (the bridge driver, at session creation) and one reader (run
// teardown's swarm.ReapRunSessions) share. A leaf package — stdlib only — so
// both bridge and swarm can import it without bending the import graph
// (swarm → bridge → core forbids bridge → swarm).
//
// The registry lives INSIDE the run's workspace (.evolve/runs/cycle-<N>/),
// which is what makes registry-based reaping structurally run-isolated: a
// run's teardown can only ever see the sessions its own file records — there
// is no server-wide listing to mis-match against (the 2026-06-11 killer-B
// class).
```

### `go/internal/sessionrecord/sessionrecord.go:36` — above `const ReapedSuffix = ".reaped"`

```text
// ReapedSuffix is the tombstone suffix a fully-reaped registry is renamed to
// (sessionreaper renames <registry> → <registry>+ReapedSuffix so every later
// boot sweep skips the run for free — the cycle-769 bounded-reap contract).
// It lives here, in the registry SSOT, so the reaper that writes the tombstone
// and the attribution reader that recovers it (ReadAllResolving, below) share
// one definition rather than two `.reaped` literals drifting apart.
```
