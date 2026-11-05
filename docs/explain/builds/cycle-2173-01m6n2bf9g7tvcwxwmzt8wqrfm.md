# Build Explanation — Cycle 2173

## Build Binding
- Cycle: 2173
- Base SHA: e69d7f45bb52d67e4150fd401a58d66dd5390717

## Summary
The code index refresh now builds the gotypes index after the trail step. A failed build adds one warning of one line that starts with WARN. The warning keeps only the first line of the build error, because the error can carry several stderr lines of go list. The exit code stays the code of the trail step.

## Rationale
The function gotypes.Build had no production caller. The refresh is the one place that moves the index worktree, so the build belongs there. Both evolve code refresh and evolve sync-main reach it.

## Changed Areas
- `go/cmd/evolve/cmd_code_refresh.go` — adds the buildGotypes field to the host and the Warnings field to the move. It also adds the step that builds from the go directory of the index worktree.
- `go/internal/adapters/codeindex/gotypes/build.go` — Build takes the context of the refresh and stops at its deadline, before the list, between packages and before the write.
- `go/cmd/evolve/cmd_syncmain.go` — prints the warnings of the move on stderr.
- `go/cmd/evolve/cmd_code_refresh_gotypes_test.go` — tests the step order, the WARN line, the output path and the paths that run no build.
- `go/acs/cycle2173/predicates_test.go` — predicates of the cycle that run those tests and check the documents.
- `docs/architecture/code-index-cli.md` — names the step in section 4 and in the refresh list.
- `docs/plans/code-index-backends-2026-10.md` — marks G2 as built.
- `.evolve/evals/code-index-backends-g2-boundary-gotypes-build.md` — the eval graders of the task.

## Design Decisions
The host takes the build as a function field, so tests inject it and no test runs a real type check. The refresh returns the move together with the trail error, so a failed trail step still builds and still reports its own exit code. A nil field skips the step for hosts that do not set it.

## Verification
The cmd/evolve package, the codeindex adapters and the ACS suite pass: green=269 red=0 for cycle 2173.

## Compatibility
Exit codes and success lines are unchanged. The JSON envelope gains entries in warnings only on a failed build.

## Limitations
A refresh that stops before the trail step builds nothing. The build adds time to each refresh. The 5-minute refresh deadline limits the build, and a build that reaches it is one WARN line.
