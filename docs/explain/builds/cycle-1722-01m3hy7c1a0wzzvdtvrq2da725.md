# Build Explanation — Cycle 1722

## Build Binding
- Cycle: 1722
- Base SHA: dfedd77abee97d7295cf8a7f042e881373162cc8

## Summary
The dashboard server now publishes exactly one sequence number for an
unchanged project root, whichever caller builds the first snapshot.
`Server.refresh` runs behind a dedicated mutex and no longer takes a `force`
flag. `Run`'s startup poll and on-demand readers therefore see each other's
publication instead of each bumping `seq`. `TestServer_UnchangedRootDoesNotBumpSeq`
now waits on the poller's first publication instead of sleeping a fixed 60ms.
While it waits it also asserts that nothing is published over ten quiet poll
ticks.

## Rationale
`refresh(true)` skipped the "unchanged fingerprint" check, and the write-lock
section bumped `seq` without looking again. `Run` calls `refresh(true)` once
at startup, and `currentEpoch` calls it for every reader that arrives before
the first snapshot exists. Those racing callers each published a new `seq`
for the same unchanged root. Cycle 1722's reproduction measured seq 12–26
after 32 concurrent early readers, where 1 is correct. The unit test sampled
`seq` across fixed sleeps, so under whole-module load it failed whenever the
poller's startup refresh landed between its two samples.

Serialising `refresh` means the second caller computes its fingerprint after
the first has published, so the existing unchanged-fingerprint fast path
stops it. The `force` flag has no remaining purpose: when no snapshot exists,
`s.snap == nil` already makes the gate fail, so a first build still happens.

## Changed Areas
- `go/internal/dashboard/server.go` — adds `refreshMu` to serialise `refresh`, drops the `force` parameter, and updates the `Run` and `currentEpoch` call sites, so an unchanged root never publishes twice.
- `go/internal/dashboard/server_test.go` — rewrites `TestServer_UnchangedRootDoesNotBumpSeq`: it subscribes before `Run`, waits on the first publication, and fails on any publication during a quiet window, with no `time.Sleep`. Adds `TestServer_ConcurrentOnDemandReadersPublishOnce` as the durable regression test for the race.
- `go/internal/dashboard/fleet_test.go` — updates its direct `s.refresh(false)` call to the new zero-argument signature.
- `go/acs/cycle1722/predicates_test.go` — this cycle's acceptance predicates (authored by the TDD phase), covering the condition-wait test, concurrent early readers, Run after an on-demand build, the real-change negative case, and contention stability.
- `.evolve/evals/dashboard-unchanged-root-seq-test-flakes-under-load.md` — this task's eval and score caps (authored upstream), which map each acceptance criterion to its predicate.

## Design Decisions
One mutex around the whole of `refresh` was chosen over a second fingerprint
check under the write lock. The re-check would also stop the duplicate bump,
but it would still let 32 early readers each run a full `collect`. It would
also let a slower caller holding an older fingerprint overwrite a newer
snapshot. The mutex removes both problems and adds only one field. Readers
that already hold a snapshot never touch `refreshMu`. They read under the
existing `RWMutex`, so the poller does not block them.

The unit test's quiet window is a `select` on the subscription channel
against a timer. That makes it stronger than the old second sample: any
publication at all during ten poll ticks fails the test, not only one that
happens to change `seq` between the two samples.

## Verification
- `go test -count=1 -tags acs ./acs/cycle1722/` — all 5 predicates pass (001–003 were RED before the fix).
- `go test -race -count=20 -run '^TestServer_UnchangedRootDoesNotBumpSeq$' ./internal/dashboard/` passes.
- `go test -count=1 -race ./internal/dashboard/`, `go vet`, and `gofmt -l` are clean.
- `TestC1722_004` confirms that a real inbox change still reaches the snapshot with a higher `seq`.

## Compatibility
`refresh` is unexported, and the only callers are in this package. The HTTP,
SSE and `--snapshot` surfaces are unchanged. The only behaviour change is
that duplicate `seq` values are no longer published for an unchanged root.
SSE clients therefore stop re-fetching an identical snapshot.

## Limitations
The snapshot is still rebuilt only when the mtime fingerprint moves, as
before. A change whose mtime and size both stay the same is not detected.
This fix does not change that.
