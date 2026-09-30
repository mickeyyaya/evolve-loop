# Comment history: `acs/cycle1694`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1694/predicates_test.go:3` — above `package cycle1694`

```text
// Package cycle1694 materialises the cycle-1694 acceptance criteria for the one
// fleet-scoped inbox id `atomicwrite-linked-state-sweep` (step 3 of
// `statejson-latent-unresolved-writers`).
//
// The defect (latent, cycle-999 class). core/worktree.go linkGuardDeps symlinks
// exactly three state files into every cycle worktree: .evolve/state.json and
// .evolve/ledger.jsonl (to the canonical host files) and .evolve/cycle-state.json
// (to the run's run.json mirror). The adapters/storage writers WriteState,
// WriteCycleState and UpdateState all end in the private writeJSONAtomic, which
// tmp+renames onto the UNRESOLVED path. A rename over a symlink replaces the LINK
// with a regular file — the cycle-999 sever — and every later write strands in
// the detached copy. statemap.WriteStateMap and core.SealCycle already resolve
// the target first (cycle 1690); storage does not.
//
// The accepted fix: route the storage writers' write target through
// statemap.ResolveWriteTarget, keeping storage and statemap as separate paths
// (statemap.go:1-21) — only the write target is resolved.
//
// Predicate strategy — every predicate exercises the system under test (the
// cycle-85 degenerate-predicate ban) and every fixture lives under t.TempDir()
// (the worktree's own .evolve/state.json IS a live link to the host state file,
// so no predicate may ever touch a repo-relative .evolve path):
//
//   - 001/002 drive storage.WriteState through absolute, relative and two-hop
//     links (plus a regular-file baseline) and through a dangling link.
//   - 003 drives storage.UpdateState's locked lossless RMW through every link
//     shape, including dangling.
//   - 004 drives storage.WriteCycleState through links to a canonical
//     cycle-state.json and through the exact linkGuardDeps topology (worktree
//     cycle-state.json -> the run's run.json, dangling until the first write).
//   - 005 is the separation negative: through a link, the storage writers must
//     keep their own semantics (no statemap CAS refusal, no statemapRevision
//     bump) — rerouting storage through statemap.WriteStateMap fails it.
//   - 006 proves the durable in-package regression tests ran, passed per link
//     shape, and are git-tracked (a `-run` matching nothing exits 0).
//   - 007 proves those durable tests are RED on the pre-fix code: it
//     neutralises every statemap.ResolveWriteTarget reference in the storage
//     package through a `go test -overlay` and requires every shape to fail.
//   - 008 is the no-regression floor for the one touched package.
//
// Audit repair (round 2). Round 1's audit found the code correct but FAILed the
// cycle on two things the predicates above never observed:
//
//   - 009 is AC3 / audit M1: build-report.md carried no atomicwrite-caller
//     inventory. It measures the inventory at the base commit and requires the
//     report to record it: the three linked files, the measured count, "none
//     targets a linked file", and "not migrated".
//   - 010 is AC3's "do not migrate them": while the cycle is live, no
//     atomicwrite caller changed and no production file outside storage/statemap
//     gained a ResolveWriteTarget reference.
//   - 011 is the host predicate-execution gate (audit/predicate_authority.go):
//     while the cycle is live, the full worktree tree must equal the tracked
//     ship tree. Round 1 left this predicate file and the eval untracked, so
//     the gate refused to run the suite and acs-verdict.json was never written.
```

### `go/acs/cycle1694/predicates_test.go:537` — above `func TestC1694_011_PredicateInputsAreInTheShipTree(t *testing.T) {`

```text
// TestC1694_011_PredicateInputsAreInTheShipTree encodes the gate that refused
// round 1's audit (go/internal/phases/audit/predicate_authority.go
// predicateTreeFor). The tree the predicates execute (every non-ignored file)
// must equal the tracked ship tree. An untracked predicate, eval or test file is
// an "undeclared input absent from the ship tree". The suite then never runs and
// acs-verdict.json is never written. It calls the gate's own two treefence
// snapshots.
```

### `go/acs/cycle1694/predicates_test.go:829` — above `func assertLinkIntact(t *testing.T, path, want string) {`

```text
// assertLinkIntact fails when path is no longer a symlink to want — the
// cycle-999 sever is exactly a rename replacing the link with a regular file.
```
