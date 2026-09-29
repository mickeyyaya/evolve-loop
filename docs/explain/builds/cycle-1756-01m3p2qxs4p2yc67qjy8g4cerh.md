# Build Explanation — Cycle 1756

## Build Binding
- Cycle: 1756
- Base SHA: df8b06138be34cdfc677e9e0ce20352caf5bc92d

## Summary
`FileLedger.Rebaseline` now decides whether its operator seal would deep-verify before it writes anything. It runs `VerifyDeep`'s verdict over the chain it has already read plus the prospective seal line and the tip the append would write. On a would-fail result it refuses with the cause wrapped and leaves `ledger.jsonl`, `ledger.tip`, `ledger-anchor.json` and the segments byte-identical. Before this change it appended first and verified afterwards, so a stale anchor sidecar or an unanchored segment grew the ledger by one seal line, moved the tip, and then returned an error.

## Rationale
The seal is a trust decision recorded in an append-only chain. A refused repair that still writes a seal leaves a record behind that the operator never accepted, and the next rebaseline then chains from it. Deciding first removes that failure path. The dry run reuses the verification code instead of copying it. `VerifyDeepScope` was split into a read (`readChain`) and a pure verdict (`verifyChain`), and both `VerifyDeep` and the dry run call them, so the refusal cannot drift from what `evolve ledger verify --deep` reports. A walk-only dry run was rejected because it misses segment binding: an unanchored segment would still get a seal and then fail.

## Changed Areas
- `go/internal/adapters/ledger/seal.go` — adds `sealedSegment`, `readChain` (segments with their relative paths and SHAs, plus the live tail, read in one pass) and `chainOrder`. `VerifyDeepScope` is now `readChain` plus `verifyChain`, and `verifyChain` holds the unchanged residue, walk, segment-binding and tip checks. `gatherAllLines` delegates to `readChain`, which keeps a single read path.
- `go/internal/adapters/ledger/ledger.go` — factors the string comparison out of `checkTip` into `compareTip`, so the dry run can check the tip it would write without touching the file.
- `go/internal/adapters/ledger/rebaseline.go` — `Rebaseline` reads the chain once, derives the seal's `entry_seq` from the tip and its `prev_hash` from the physical last live line, and calls `dryRunSeal` before `appendChainedFromTail`. It refuses an empty live tail and a malformed tip before anything is written. The post-write `VerifyDeep` stays as the backstop for a writer that races in between the dry run and the append.
- `go/internal/adapters/ledger/rebaseline_refusal_test.go` — adds the durable regression `TestRebaseline_RefusesWriteWhenForwardWalkWouldFail` (stale sidecar anchor and unanchored segment: refused, cause kept, every file byte-identical) and an edge test for the empty-live-tail and marshal-failure refusals.
- `go/acs/cycle1756/predicates_test.go` — the TDD phase's acceptance predicates 001-005 for this task, committed with the build.
- `.evolve/evals/rebaseline-dry-run-before-seal.md` — the TDD phase's eval graders for this task, committed with the build.
- `docs/architecture/packages/internal-adapters-ledger.md` — the Rebaseline design note and its gated/append-only invariant now describe the decide-before-write order and name the new pinning test.

## Design Decisions
The seal entry is built once, and the same struct is marshaled for the dry run and passed to the append. When no writer races, the appended bytes are exactly the bytes that were verified. When a writer does race in, `appendChainedFromTail` re-derives the seq and prev_hash under the lock, and the post-write `VerifyDeep` still reports it. All new helpers are unexported, and the production caller is unchanged: `runLedgerRebaseline` in `cmd/evolve/cmd_ledger.go` calls `l.Rebaseline`.

## Verification
- `go test -count=1 ./internal/adapters/ledger/` passes, including the new regression test. That test was RED before the fix: `ledger.jsonl` grew from 562 to 832 bytes and from 368 to 638 bytes on the two refused calls.
- `go test -tags acs -count=1 ./acs/cycle1756` passes 5/5, and predicate 003 confirms that ledgers one seal repairs still get exactly one seal, verify, and move the tip.

## Compatibility
Public API, error sentinels and the CLI are unchanged. A refusal's message now reads "refused, nothing written — the seal would not verify forward: <cause>", and the cause is still wrapped, so `errors.Is` checks against `core.ErrLedgerChainBroken` and `ErrSealResidue` keep working. `VerifyDeep` now checks segment bindings in segment order rather than in map order, so the ledger reports the same failure every run.

## Limitations
The dry run and the append are not one critical section. A concurrent writer between the two can still leave a seal that the post-write check then rejects. This path is the same one as before the change and is still reported loudly.
