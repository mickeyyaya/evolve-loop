# Build Explanation — Cycle 2448

## Build Binding
- Cycle: 2448
- Base SHA: fca979b3920e2284dfe4b2af163594427e2153e2

## Summary
Both plans, `wave next` and `boundary run`, now rebuild the plane binary right after the merge. The steps that read the policy strictly (inbox curate, selfheal, gc and the code index refresh) run that rebuilt binary. The `quality` step is the exception: it runs in-process after the merge, so it parses the merged policy with the binary that started the boundary.

## Rationale
A merged pull request can add a policy key together with the code that reads it. The old binary reads the merged policy file with a strict loader and refuses the new key. The fix adds a build step after sync-main and queue-apply in both plans. It runs each strict step through the rebuilt plane binary. A key that no reader of the merged tree knows still stops the boundary at the first strict step, and the error names the key.

## Changed Areas
- `go/cmd/evolve/cmd_syncmain.go` — sync-main reconciles the branch and prints one line. It no longer refreshes the code index.
- `go/cmd/evolve/cmd_boundary.go` — the halt steps include the build step. One shared dispatcher runs inbox, selfheal, gc and code through the rebuilt plane binary.
- `go/cmd/evolve/cmd_wave_compose.go` — the wave plan takes its build step from the halt steps and refreshes the index after gc.
- `go/cmd/evolve/cmd_wave.go` — the plane binary runner finds the project root in any argument list and keeps the exit code. An absent index maps to success.
- `docs/operations/wave-operations.md` — the code index section records the order: sync-main, the build, then code refresh.
- `go/cmd/evolve/cmd_boundary_merged_policy_test.go` — tests of the merged-policy boundary, written by the test phase.
- `go/acs/cycle2448/predicates_test.go` — predicates of the cycle, written by the test phase.
- `go/cmd/evolve/cmd_boundary_test.go` — the expected step lists include the new step.
- `go/cmd/evolve/cli_queue_test.go` — the expected step order includes the build step.
- `go/cmd/evolve/cmd_boundary_ratchet_test.go` — the expected halt step order includes the build step.
- `go/cmd/evolve/cmd_boundary_launch_test.go` — the failing step index moves by one.
- `go/cmd/evolve/cmd_wave_test.go` — the expected wave step list and count include the new step.
- `go/cmd/evolve/cmd_wave_errors_test.go` — the dry-run step count moves by one.
- `go/cmd/evolve/cli_contract_test.go` — the step count in the failure message moves by one.
- `go/cmd/evolve/cli_contract_cover_test.go` — the plan and step counts move by one.
- `go/cmd/evolve/cmd_syncmain_codeindex_test.go` — the two index move tests now call the code refresh verb. The third test asserts that sync-main leaves the index alone.
- `go/cmd/evolve/cmd_wave_plane_binary_test.go` — tests of the plane binary runner. They cover a missing project root and a binary that cannot start. They also cover an absent index and the exit codes.
- `.evolve/evals/boundary-parses-merged-policy-with-the-rebuilt-binary.md` — the eval graders of the task.

## Design Decisions
One dispatcher serves both plans, so the two paths cannot differ. The build step runs after queue-apply, which runs in-process and does not read the policy strictly. The `quality` step also runs in-process, after the merge. Both plans share the same halt steps. An absent index (exit 2) maps to 0, so a host without an index worktree still completes the boundary.

## Verification
The targeted tests of the merged-policy file pass. The full cmd/evolve package passes. The ACS suite of cycle 2448 reports PASS with red=0.

## Compatibility
The sync-main verb no longer prints the code index lines. The code refresh verb prints them. The function syncMainCodeIndex stays, because shipped predicates of cycle 2173 name its tests.

## Limitations
The plane binary must exist at go/bin/evolve after the build. A host without it stops at the first strict step with an I/O error.
