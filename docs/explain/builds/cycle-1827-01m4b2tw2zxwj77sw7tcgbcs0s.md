# Build Explanation — Cycle 1827

## Build Binding
- Cycle: 1827
- Base SHA: fe379ced561ae48285e098e24e4a4c24602bfd4f

## Summary
Sealing the ledger no longer breaks plain `Verify` or the byte-identical carry that depends on it: `Verify` now seeds its strict walk with the newest sealed segment's last line, and `Seal` appends its anchor before releasing the chain lock, so a queued `VerifyDeep` never sees a truncated-but-unanchored segment. Separately, `atomicwrite.Durable` became the one durable write-then-rename implementation, and the three writers that carried their own copy (state.json, ledger segments, the evidence store) now delegate to it.

## Rationale
After a seal, the first live line chains from a line that lives in a gzip segment, so a live-only walk had no predecessor and failed at line 0. Reading the newest segment's last line is the smallest input that lets the walk stay strict; walking the whole sealed history would make every `Verify` a `VerifyDeep`. The residue false-positive came from `Seal` dropping `mu` and `ledger.lock` between truncation and its anchor `Append`; appending through the lock-free half of `appendChained` while the locks are still held closes the window without a new lock. For the writers, three durable copies had already drifted (two never synced their directory), so one implementation with tested fault branches replaces them.

This cycle re-binds cycle 1825's build. That build passed audit, but ship refused it with `INTEGRITY_TREE_DRIFT` because another cycle's files had leaked into its staged tree. The production diff is cycle 1825's snapshot (`839a819ca..bf4c2b1d9`), re-applied unchanged onto this base. It composes with main's later `carry.go` change (`carryExplains`, PR #796), which reaches the fixed `Verify` through `reProveCarry`.

## Changed Areas
- `go/internal/adapters/ledger/ledger.go` — `VerifyScope` reads the newest segment's sealed predecessor inside the read snapshot and walks it plus the live tail (`verifyLive`, `liveScope`, `carriesLine`); `appendChained` splits into the lock and `appendChainedLocked`; `Append` reports through `observe` and `chainInto`, which `Seal` reuses.
- `go/internal/adapters/ledger/seal.go` — `Seal` runs write, truncate and anchor append under one hold of the chain lock (`sealAndAnchor`) and reports the anchor to the observer; `sealedPredecessor` picks the seed, skipping a segment whose truncation never ran; `writeSegment` gzips in memory and writes through `atomicwrite.Durable` (0600), adding the directory fsync it lacked.
- `go/internal/adapters/ledger/anchor.go` — `VerifiedScope` gains `FromSealedSegment`, so a scope that resumed from the sealed boundary is not reported as an operator epoch anchor.
- `go/cmd/evolve/cmd_ledger.go` — `evolve ledger verify` prints the sealed-boundary scope as such and points at `--deep` for the segments.
- `go/internal/atomicwrite/atomicwrite.go` — adds `Durable(path, data, mode)` (file fsync before close, directory fsync after rename) and `TempWriter`; `Bytes`, `JSON` and `Durable` share one `write` core; `Durable`'s temp names carry the writer's pid, while `Bytes` keeps its old pattern because `defectledger`'s error goldens normalize it.
- `go/internal/adapters/storage/statejson.go` — `writeJSONAtomic` marshals and delegates to `atomicwrite.Durable` with its existing 0600 mode; the storage hooks shrink to the marshal seam.
- `go/internal/ledgerartifacts/store.go` — `writeAtomically` delegates to `atomicwrite.Durable` with mode 0444; the dead-writer reaper moves into `Put` and recognises `atomicwrite` temp names beside the legacy `.put-<pid>-*`.
- `go/internal/adapters/ledger/signals_test.go` — inventories `Seal` as a writer that appends under the lock and reports to the observer itself.
- `go/internal/adapters/storage/storage_test.go` — drops the four write/sync/close/rename fault tests whose steps moved into `atomicwrite`; their cases now run in `go/internal/atomicwrite/durable_fault_test.go`.
- `go/internal/atomicwrite/atomicwrite_test.go` — the fake temp file gains `Sync`, and the seam helper restores `openDir`.
- `go/internal/atomicwrite/durable_fault_test.go` — file-sync, directory open/sync/close, Bytes-never-syncs and temp-name tests that bring the package to 100% coverage.
- `go/internal/ledgerartifacts/store_test.go` — replaces the test of the removed `tempPattern` with one proving `Put` reaps a dead writer's `Durable` temp and keeps a live one's.
- `go/internal/adapters/ledger/verify_sealed_test.go` — sealed-ledger scope, a sidecar anchor sealed away, a sidecar anchor still live, and a seal that crashed before truncation.
- `go/cmd/evolve/cmd_ledger_sealed_test.go` — `evolve ledger verify` passes on a sealed ledger and names the sealed boundary.
- `go/acs/cycle1827/helpers_test.go` — TDD phase's predicate fixtures (seal race harness, delegation AST walk), unchanged by Build.
- `go/acs/cycle1827/predicates_test.go` — TDD phase's twelve acceptance predicates, unchanged by Build.
- `go/internal/atomicwrite/durable_test.go` — TDD phase's frozen `Durable` contract tests, unchanged by Build.
- `go/internal/phases/ship/carry_sealed_test.go` — TDD phase's frozen carry-on-sealed-ledger tests, unchanged by Build.
- `.evolve/evals/durable-atomic-write-single-implementation.md` — TDD phase's eval for the durable-writer task.
- `.evolve/evals/seal-breaks-live-only-verify-and-carry.md` — TDD phase's eval for the sealed-verify task.
- `docs/architecture/packages/internal-atomicwrite.md` — new design notes for the package, including the directory-sync divergence it fixed.
- `docs/architecture/packages/README.md` — indexes the new atomicwrite page.
- `docs/architecture/packages/internal-adapters-ledger.md` — documents sealed-ledger `Verify`, the lock-held anchor append, the segment writer, and closes the open finding.
- `docs/architecture/packages/internal-ledgerartifacts.md` — documents the move onto `Durable` and where the reaper now runs.
- `docs/plans/ledger-restructure-2026-10.md` — landing note that the C12/C13 prerequisite is built; C12 and C13 stay open.

## Design Decisions
`Verify` stays a live-tail check rather than becoming a full walk: sealed history remains `VerifyDeep`'s job, and the seed is the one line the live tail needs. When the sidecar epoch anchor was sealed away, live operator seals still move the anchor; otherwise the walk resumes from the seed and the scope says so (`FromSealedSegment`) instead of claiming an operator-adjudicated anchor. `Seal` reports its anchor to the observer after the locks drop, so an observer can never re-enter the ledger while it is locked. `Durable` names temps after the writing pid and exports `TempWriter`, rejected alternative being a per-caller temp-pattern variant, which would have been a second entry point for one algorithm; the evidence store's reaper moved into `Put` so the write itself is a pure delegation.

## Verification
The cycle's twelve ACS predicates (`go/acs/cycle1827`) pass, including the seal/VerifyDeep race over five rounds and the AST delegation checks; `atomicwrite` is at 100.0% statement coverage; the storage, ledgerartifacts, ledger and ship packages pass, as does the new CLI test; `gofmt`, `go vet` and the comment audit are clean.

## Compatibility
On-disk formats are unchanged: state.json keeps its indented JSON, trailing newline and 0600 mode; segments keep gzip bytes and 0600; evidence objects keep 0444. Durable temp file names change (Bytes keeps its pattern), and the evidence store still reaps legacy `.put-<pid>-*` temps. `VerifiedScope` gains a field, so existing comparisons of an unsealed ledger's scope are unaffected.

## Limitations
Plain `Verify` on a sealed ledger gunzips the newest segment under the shared lock, which costs a decompression per call until the plan's incremental verify (C12) lands. Plain `Verify` does not detect damage inside sealed segments or a sidecar anchor line altered inside them; `VerifyDeep` does.
