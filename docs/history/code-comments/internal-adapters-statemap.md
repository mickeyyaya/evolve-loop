# Comment history: `internal/adapters/statemap`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/adapters/statemap/statemap.go:1` — above `package statemap`

```text
// Package statemap is the single source of truth for the full-fidelity,
// lock-owning read-modify-write of state.json (and any sibling JSON-object
// state file).
//
// Three phases previously each carried their own copy of this exact
// read→mutate→atomic-write dance — ship/statefile.go (flocked),
// core/reset.go (readJSONMapFile/writeJSONMapFileAtomic) and
// phaseintegrity/repin.go (inline). They are consolidated here so exactly one
// lock-owning implementation exists (cycle-659 single-source pin).
//
// The state file is modelled as a raw map[string]any rather than a typed
// struct: state.json holds operator-owned keys no orchestrator struct models
// (expected_ship_sha, currentBatch, …) and those MUST survive a round-trip that
// touches an unrelated key. The typed adapters/storage.UpdateState is the
// complementary path for the orchestrator-modelled subset; this package is the
// full-fidelity path.
//
// statemap is a LEAF: it imports only internal/adapters/flock (the sidecar-lock
// convention) and the standard library, so core → statemap → flock is acyclic
// (the cycle-644 core→storage import-cycle trap is why storage was rejected as
// the home).
```

### `go/internal/adapters/statemap/statemap.go:60` — above `func ResolveWriteTarget(path string) string {`

```text
// ResolveWriteTarget follows a symlink chain (bounded, dangling-tolerant) to
// the FINAL write target. Rename-over-a-symlink replaces the LINK with a
// regular file — exactly how worktree .evolve/state.json links to canonical
// were severed and mutations stranded in detached copies (cycle-999/1000).
// Writing through to the resolved target keeps the link intact and every
// mutation visible on the canonical file. Every writer of a possibly-linked
// state file must also LOCK the resolved path, so its "<target>.lock" sidecar
// is the one canonical-path writers contend on. A regular file, a missing
// path, or a dangling tail returns as-is; a chain deeper than 8 hops returns
// the hop reached.
```

### `go/internal/adapters/statemap/statemap.go:115` — above `func WriteStateMap(path string, m map[string]any) error {`

```text
// WriteStateMap atomically replaces path with the indented JSON of m via
// tmp-file + rename. The 2-space indent matches jq's default output so diffs
// against bash-written state.json files stay minimal. This is the UNLOCKED
// primitive: callers that compose several read/writes under one lock (the
// ship withStateLock pattern, the reset seal) call it inside their own held
// flock.PathLock; standalone callers use UpdateStateMap.
//
// Two integrity floors (cycle-1001 lost-write / cycle-999 stranded-write):
//   - symlink write-through: the rename lands on ResolveWriteTarget(path), so
//     a worktree's state.json link to canonical survives and the bytes reach
//     the live file;
//   - stateRevision CAS: when both the incoming map and the on-disk target
//     carry a numeric stateRevision, an incoming revision BELOW the on-disk
//     one is a stale writer and is refused with ErrStaleRevision — the lock
//     serializes writers, this floor validates their freshness.
//
// A forensic tripwire WARNs (never blocks) when a write would shrink
// carryoverTodos by more than half from >20 entries — the cycle-1001
// signature — so a legal-but-suspicious mass drop is loud in the log.
```

### `go/internal/adapters/statemap/statemap.go:203` — above `func UpdateStateMap(path string, mutate func(map[string]any)) error {`

```text
// UpdateStateMap performs a serialized, full-fidelity read-modify-write of
// path. The path is symlink-RESOLVED first and the lock is taken on the
// RESOLVED path's sidecar — a worktree writer (through the .evolve/state.json
// link) and a canonical writer therefore share ONE lock instead of racing on
// two different lock files over the same data (the cross-tree half of the
// cycle-1001 lost-write). It holds flock.PathLock across the WHOLE
// read→mutate→write, so concurrent writers (goroutines or processes)
// serialize and no update is lost.
//
// Every successful update advances stateRevision (seeded to 1 when absent)
// and refreshes lastUpdated — the frozen-timestamp forensics of cycle-1001
// showed some writers skipped both, making stale snapshots indistinguishable
// from live state. mutate receives the parsed map and edits it in place; it
// must be fast and side-effect-free (it runs under the cross-process lock)
// and must NOT call UpdateStateMap on the same path (the blocking flock would
// deadlock). A malformed file aborts before the write, leaving it untouched.
```

### `go/internal/adapters/statemap/statemap_integrity_test.go:3` — above `import (`

```text
// statemap_integrity_test.go — the cycle-999/1000/1001 shared-state disease
// pins (inbox: statejson-stalerevision-cas-lost-write 0.96 +
// statejson-worktree-canonical-propagation 0.95).
//
// Two defects, one chokepoint:
//  1. PROPAGATION: WriteStateMap's tmp+rename REPLACED a symlink at path with
//     a regular file — the first statemap write inside a worktree severed the
//     .evolve/state.json link to canonical and stranded every later mutation
//     in a detached copy (cycle-999: "135->14 stranded"; ledger.jsonl's link
//     survived only because nothing rename-wrote it).
//  2. LOST WRITE: a writer holding a stale in-memory map won the flock and
//     legally overwrote newer state (cycle-1001: a Jul-14 snapshot, rev 1356,
//     clobbered the live file destroying 136 carryoverTodos). The lock
//     serialized but never VALIDATED freshness.
```

### `go/internal/adapters/statemap/statemap_integrity_test.go:219` — above `func TestResolveWriteTarget(t *testing.T) {`

```text
// TestResolveWriteTarget pins the exported chain walk every linked-state
// writer locks and renames on (cycle 1690): it returns the FINAL target of
// absolute, relative and multi-hop chains, returns a dangling tail and a
// missing path as-is (never errors), and stays bounded on a link loop.
```

### `go/internal/adapters/statemap/statemap_test.go:3` — above `import (`

```text
// statemap_test.go — RED contract for the cycle-659 task
// `statefile-rmw-flock-single-source`.
//
// The single source of truth for state.json read-modify-write is promoted here
// from ship/statefile.go's readStateMap/writeStateMap: a NEW LEAF PACKAGE
// importing ONLY internal/adapters/flock + stdlib (the verified-acyclic home
// from cycle-644's retrospective; storage was rejected because storage imports
// core, and stateutil was rejected as orphaned scaffolding). core/reset.go
// (SealCycle), phaseintegrity/repin.go (RepinShipSHA) and ship become callers;
// their three duplicate RMW implementations are deleted.
//
// This file is `package statemap` in a dir that (until Builder writes
// statemap.go) has NO non-test Go file, so `go test ./internal/adapters/statemap/`
// FAILS TO COMPILE — the RED state. Builder makes it GREEN by implementing:
//
//	func ReadStateMap(path string) (map[string]any, error)
//	func UpdateStateMap(path string, mutate func(map[string]any)) error
//
// UpdateStateMap MUST hold flock.PathLock(path) across the whole read→mutate→
// write and write atomically (tmp + rename), mirroring ship/statefile.go's
// documented contract and storage.UpdateState's lock discipline (CA.3).
```
