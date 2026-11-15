# Build Explanation — Cycle 2391

## Build Binding
- Cycle: 2391
- Base SHA: cc1f0c2b72f5fbcdc6e473e0a3a17d6a23117f9a

## Summary
`evolve ship --class manual` pre-fills a `Closes-Inbox: <id>` trailer when the message declares no inbox trailer. It matches the last path segment of the current branch against open inbox items.

## Rationale
Calling `fillManualBranchInboxTrailer` from `verifyClass` in `go/internal/phases/ship/verify.go` keeps protected files (`gitops.go`, `ship.go`, `native.go`) untouched. It updates `opts.CommitMessage` before `atomicShip` builds the commit message. Reusing `closesinbox.Declares` and `inboxbatch.ScanDir` avoids duplicating trailer parsing or open-item scanning.

## Changed Areas
- `go/internal/phases/ship/manual_inbox_trailer.go` — adds `fillManualBranchInboxTrailer` to check `closesinbox.Declares`, resolve `currentBranch`, scan `.evolve/inbox` with `inboxbatch.ScanDir`, and append `Closes-Inbox: <id>` on an exact `Item.ID` match.
- `go/internal/phases/ship/verify.go` — invokes `fillManualBranchInboxTrailer(ctx, opts)` in the `ClassManual` arm of `verifyClass`.
- `go/internal/phases/ship/manual_inbox_trailer_test.go` — adds `TestShipManual_BranchTrailerSkipsErrorAndEmptyBranchCases` to cover branch lookup error, detached HEAD, and unreadable inbox directory branches.
- `docs/architecture/inbox-curator.md` — documents the `--class manual` `Closes-Inbox` branch pre-fill behavior in STE.
- `docs/architecture/packages/internal-phases-ship.md` — documents `fillManualBranchInboxTrailer` in the package design notes in STE.

## Design Decisions
Placing the helper in `manual_inbox_trailer.go` and wiring it into `verifyClass` avoids editing protected `gitops.go`. It also keeps `fillManualBranchInboxTrailer` small enough for 100% statement coverage under `go test -run '^TestShipManual_'`. Matching only the exact `path.Base(branch)` against open root items prevents prefix matches or consumed items from being closed.

## Verification
`go test -count=1 -v -run '^TestShipManual_' ./internal/phases/ship` passes all tests and subtests, `go test -tags acs -count=1 -v ./acs/cycle2391` passes all 5 predicates (including 100% statement coverage and zero added comments), and `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added` reports 0 findings.

## Compatibility
Existing `--class manual` commits that already declare `Closes-Inbox` or `Refs-Inbox` keep their exact commit message. Commits on branches without a matching open inbox item also keep their exact commit message.

## Limitations
Only open items at the root of `.evolve/inbox/` whose `id` equals the last path segment of the current branch match. Detached HEADs or branch lookup errors leave the commit message unchanged.
