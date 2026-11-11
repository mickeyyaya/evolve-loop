# Build Explanation — Cycle 2249

## Build Binding
- Cycle: 2249
- Base SHA: ff30908c0429dfc822f3dfa0c70160a562a297bf

## Summary
The failure dossier now lists the name of each postmortem file in the cycle workspace.

## Rationale
Step W2 of the postmortem plan needs the dossier to point to the postmortem records. The dossier builder is the single place that both decision callers use, so one change covers both.

## Changed Areas
- `go/internal/core/failure_dossier.go` — adds the `postmortems` field and a scan of the workspace for files that match `<phase>-attempt-<n>-postmortem.json`.
- `go/internal/core/failure_dossier_postmortems_test.go` — tests the sorted list, the omitted field, the ignored bad entries, and both production callers.
- `go/acs/cycle2249/predicates_test.go` — predicates that run those tests.
- `.evolve/evals/attempt-postmortem-w2-dossier-paths.md` — eval graders for the task.

## Design Decisions
The scan keeps regular files only, so a directory, a symlink or a temp file never counts. The field uses `omitempty`, so a cycle without a postmortem writes the same artifact as before. `os.ReadDir` returns names in sorted order.

## Verification
The three ACS predicates and the five core tests pass. The full `./internal/core` package passes.

## Compatibility
The new JSON key is additive. No existing key changes.

## Limitations
The scan checks the file name only. It does not read the postmortem content. The name pattern is narrower than the phase names that `ValidPhase` accepts. The scan needs a phase name that starts with a lowercase letter. The rest of the name can have only lowercase letters, digits and hyphens. A postmortem for a phase name with a capital letter or an underscore is not listed.
