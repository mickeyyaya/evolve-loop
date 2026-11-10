# Build Explanation — Cycle 2255

## Build Binding
- Cycle: 2255
- Base SHA: 6184cfa4a88ed9140ed28f3d9bb6cce5526375b2

## Summary
The failure dossier now lists each postmortem file of the cycle workspace in a `postmortems` field. The field is sorted and absent when the cycle has no postmortem.

## Rationale
The dossier is the one artifact that the failure decision reads. A reader of the dossier had to list the workspace to find the postmortem files. Filling the field inside the shared builder `buildFailureDossier` covers every decision caller with one change.

## Changed Areas
- `go/internal/core/failure_dossier.go` — adds the `Postmortems` field and the `postmortemFiles` helper. The helper reads the workspace directory, keeps regular files that `attemptpostmortem.IsFileName` accepts, and returns them in directory order, which is sorted.
- `go/internal/attemptpostmortem/record.go` — adds `IsFileName`, the one matcher for the name that `Path` builds. This closes the audit finding that the dossier copied the name format with a narrower phase charset.
- `go/internal/attemptpostmortem/record_test.go` — tests that `IsFileName` accepts each name `Path` builds and refuses malformed names.
- `go/internal/core/failure_dossier_postmortems_test.go` — tests the sorted list, the omitted field, the ignored malformed names, directories, links and temp files, and the two production callers.
- `docs/private/research/archived-2026-11-09/superseded-predicate-packages/cycle2249/predicates_test.go` — archive move of the superseded cycle 2249 predicate package. The content is unchanged.
- `docs/private/research/archived-2026-11-09/unshipped-build-explanations/cycle-2249-01m706c2xed10r5adqkhm44mm9.md` — archive move of the explanation of the unshipped cycle 2249 build. The content is unchanged.

## Design Decisions
The field uses `omitempty`, so a cycle without a postmortem writes the same artifact as before. The name filter is the exported matcher of the owner package, and `IsRegular` rejects directories and symbolic links.

## Verification
`go test -count=1 ./internal/core/ ./internal/repocontract/` passes. The ACS predicates `TestC2255_001` to `TestC2255_003` pass. The ACS suite reports green=266 red=0.

## Compatibility
Existing readers of `failure-dossier.json` ignore the new optional key. No other field changes.

## Limitations
The dossier lists file names only. It does not read the content of a postmortem.
