# Build Explanation — Cycle 2490

## Build Binding
- Cycle: 2490
- Base SHA: 5db58d6b91d602b28eea4674bfd2dcbb3d18e095

## Summary
The git-evidence detector now returns an error when git exits with a non-zero code.

## Rationale
The bridge runner reports a non-zero exit as a code with a nil error. The detector kept only the error, so a failed rev-list with partial output was read as a list of commits. The fix checks the code in the one place that runs git.

## Changed Areas
- `go/internal/bridge/completion.go` — the git command closure turns a non-zero exit code into an error, so rev-parse, rev-list and log all fail closed.

## Design Decisions
The check lives in the single git closure, not in each caller. This keeps one home for the rule and covers every subcommand.

## Verification
The new bridge tests script a fake git with exit codes 1, 128 and -1 for each subcommand. Each case must give an error, and exit 0 must keep the trimmed output.

## Compatibility
Zero-exit behavior and the poll flow are unchanged. A failed git command keeps the baseline and the detector keeps watching.

## Limitations
Other bridge tests fail on the base tree: the agy and codex manifest tests and TestSeatJourney_ANoTokenHostGivesTheStoredShapeAndTheRecordSaysStored. This change does not touch them.
