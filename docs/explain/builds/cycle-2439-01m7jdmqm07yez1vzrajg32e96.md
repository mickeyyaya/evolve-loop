# Build Explanation — Cycle 2439

## Build Binding
- Cycle: 2439
- Base SHA: cecc133bace0e76cc2edf17bfc070ec09b618cb4

## Summary
Pull request CI and `evolve pr merge` now validate `Closes-Inbox` and `Refs-Inbox` trailers under `inbox.curator.trailer.stage`.

## Rationale
Console pull requests previously carried no inbox trailers. Trailer checks in CI and in `prMerger.mergeOne` keep the inbox aligned with `main`.

## Changed Areas
- `.github/workflows/required.yml` — adds the `trailer-check` pull request job and wires it into `required` so every pull request validates trailers.
- `go/cmd/evolve/cmd_inbox_trailercheck.go` — extracts `checkInboxTrailerBytes` from `checkInboxTrailerRange` so both the CLI verb and `evolve pr merge` share one validator.
- `go/cmd/evolve/cmd_pr.go` — loads `Policy.InboxTrailerStage` in `newPRMerger`, requests `body,commits`, and validates trailers in `prMerger.mergeOne`.

## Design Decisions
`prMerger.mergeOne` calls `checkTrailers` directly after `m.view`. This avoids unnecessary branch updates and CI waits when an enforced pull request lacks a valid trailer.

A shared `checkInboxTrailerBytes` helper prevents duplicate `closesinbox` and `inboxmover.ResolveDispatchState` logic across two files.

## Verification
The cycle ACS predicates, `cmd/evolve` suite, `internal/mergewindow`, `internal/ciparity`, `go vet`, and `ste-lint` cover the code, workflow, and documentation changes.

## Compatibility
`Policy.InboxTrailerStage` defaults to `shadow`, and an empty `prMerger.trailerStage` in test fixtures behaves as `shadow`. Non-pull-request CI events skip `trailer-check` cleanly.

## Limitations
The repository policy keeps `inbox.curator.trailer.stage` at `shadow` until the console switches it to `enforce` after one wave.
