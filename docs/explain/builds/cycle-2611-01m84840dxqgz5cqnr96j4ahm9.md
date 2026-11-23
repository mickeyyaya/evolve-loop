# Build Explanation — Cycle 2611

## Build Binding
- Cycle: 2611
- Base SHA: a099157d14664c4483d0db2009b6551b6ded79d6

## Summary
`inboxstamps` now accepts tracked inbox deletions whose matching copy sits under `.evolve/inbox/processed/` and commits only the deletion for those gitignored destinations.

## Rationale
`evolve integration batch` closes tracked inbox items through `inboxmover.ReconcileSuperseded` into `.evolve/inbox/processed/cycle-0/<id>.json`. Because `Classify` checked only `quarantine/` and `consumed/`, `evolve sync-main` rejected those deletions with exit code 3. Treating verified `processed/` moves like other loop moves unblocks `sync-main` and `evolve wave next` without force-adding gitignored files.

## Changed Areas
- `go/internal/inboxstamps/inboxstamps.go` — extends `quarantineMove` with `decodedCandidates` and `processedCopies` to match `.evolve/inbox/processed/` copies matching `HEAD`, and commits only the deletion for `processed/` moves.
- `docs/architecture/packages/internal-inboxstamps.md` — documents that `Classify` recognizes moves into `processed/` and `CommitMoves` omits `git add` for that gitignored directory.

## Design Decisions
Candidate discovery walks `.evolve/inbox/processed/` and matches `<base>` or `*-<base>` before checking `stampBetween` with `inboxmover.IsMoverWritten`. That matches `CommitSHA`-prefixed filenames and cycle directories while edited or unrelated copies stay `Other` dirt. Force-adding into `.evolve/inbox/processed/` was rejected because `.gitignore` keeps `processed/` untracked runtime state. Modifying `integration.Train` to push closures into batch PRs was rejected because `record-pr` attests each PR diff SHA and `sync-main` already owns plane-side inbox moves.

## Verification
The `internal/inboxstamps` unit suite passes at 100.0% statement coverage, the `cmd/evolve` `TestJourneyInboxClosure*` and `TestSyncMain*` tests pass, and `go test -tags acs -count=1 ./acs/cycle2611` passes all 3 predicates.

## Compatibility
`Classify`, `CommitMoves`, `Move`, and `Partition` keep their exported signatures and behavior for `quarantine/` and `consumed/` moves. Other tracked deletions still fail with exit code 3.

## Limitations
If `origin/main` edits a tracked inbox item that a local batch closed to `processed/`, `git merge` reports a conflict like `quarantine/` moves do.
