# Build Explanation — Cycle 2035

## Build Binding
- Cycle: 2035
- Base SHA: 8dbfc8d92a174d203bf9b39de8b6e39eb626806e

## Summary
The two kernel minters of inbox items, faillearn and ciwatch, now file through internal/inboxfile.

## Rationale
Both minters previously wrote inbox JSON directly without full filing validation, missing fix and acceptance criteria, and risking concurrent race conditions or timestamp collisions. Moving filing logic to internal/inboxfile ensures atomic filing under the .bump.lock, validates required filing fields, and guarantees idempotent skips on duplicate ids.

## Changed Areas
- `go/.apicover-enforce` — enrolls ./internal/inboxfile so the new package passes the repo-wide apicover gate.
- `go/internal/ciwatch/basered.go` — includes fix and acceptance in baseRedItem and delegates filing to inboxfile.Mint under .bump.lock.
- `go/internal/ciwatch/ciwatch.go` — includes fix and acceptance in escalationItem, delegates filing to inboxfile.Mint, and removes direct file writes.
- `go/internal/faillearn/inbox.go` — defaults missing summary, fix, and acceptance fields on cloned items without mutating caller slices, and delegates filing to inboxfile.Mint.
- `go/internal/faillearn/writer.go` — forwards ev.Now to writer configuration so inboxfile.Mint stamps filenames deterministically.
- `go/internal/inboxfile/inboxfile.go` — implements Mint, Publish, File, FiledIDs, and validation rules for safe concurrent inbox item filing.

## Design Decisions
Creating internal/inboxfile as a leaf package respects the protected status of internal/inboxmover/lifecycle. In faillearn, missing fields default on cloned items to protect caller slices.

## Verification
cd go && go test -tags acs -count=1 ./acs/cycle2035 passes all 23 predicates. Unit test suites for internal/inboxfile, internal/ciwatch, and internal/faillearn pass with go test -count=1. Full ACS suite passes with red=0.

## Compatibility
Existing callers of ciwatch.Watch remain compatible. Direct callers of faillearn.WriteArtifacts succeed, but downstream assertions expecting bare `<id>.json` require updates for timestamped file names.

## Limitations
Filing validation in internal/inboxfile duplicates lifecycle rules pending unification in console item retro-minting-files-through-inbox-add-console. Tests in protected core/failurelearning require companion console item retro-minting-files-through-inbox-add-lane-ac-5-console.
