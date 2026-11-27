# Build Explanation — Cycle 2662

## Build Binding
- Cycle: 2662
- Base SHA: 83e996a69ddbd334da16cb1c4278ba09d8a6556b

## Summary
`Run` and `RunMover` now restore the previous package host `Deps` pointer on return, and `runInbox` passes `testDeps()`. A `Deps{}` call no longer leaks a nil `LaneForbidden` into later parallel `runInboxLint` tests.

## Rationale
`Run` and `RunMover` previously overwrote the package-global `current` pointer without restoring it. Because `runInbox` passed `Deps{}`, a parallel `runInboxLint` test can read a nil `LaneForbidden` from `host()` and panic. Extracting `pushHost` with `defer` restores the prior pointer on every exit path in a single shared helper.

## Changed Areas
- `go/internal/cli/inboxcmd/deps.go` — adds `pushHost(deps Deps) func()` to swap `current` atomically and return a closure that restores the previous pointer.
- `go/internal/cli/inboxcmd/cmd_inbox.go` — updates `Run` to call `defer pushHost(deps)()` instead of `current.Store(&deps)`.
- `go/internal/cli/inboxcmd/cmd_inbox_mover.go` — updates `RunMover` to call `defer pushHost(deps)()` instead of `current.Store(&deps)`.
- `go/internal/cli/inboxcmd/inboxcmd_run_test.go` — updates `runInbox` to pass `testDeps()` instead of `Deps{}`.
- `docs/architecture/packages/internal-cli-inboxcmd.md` — updates the `Deps` design note to reflect that `Run` and `RunMover` restore the prior host on return.

## Design Decisions
`pushHost` uses `current.Swap(&deps)` and returns `func() { current.Store(prev) }` so `Run` and `RunMover` share one restore mechanism without duplicated code. Removing the package-global `current` pointer by threading `Deps` through all verb handlers was deferred as a larger refactor outside this bugfix scope.

## Verification
`gofmt` and `go vet` report no issues on `go/internal/cli/inboxcmd`, and the package passes `-race -count=10` and `go test -count=20` for the isolation tests. `TestC2662_005` checks 100.0% statement coverage of `Run`, `RunMover`, and `runInboxLint`, and `go tool cover` reports 100.0% for `pushHost`.

## Compatibility
Production callers (`go/cmd/evolve/cmd_inbox.go`) and existing `inboxcmd` tests keep identical `Run` and `RunMover` signatures and behavior.

## Limitations
`Run` and `RunMover` still update a package-level `atomic.Pointer[Deps]`, so concurrent calls to `Run` or `RunMover` with different `Deps` values inside the same process remain unsupported.
