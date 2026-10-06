# Build Explanation — Cycle 1800

## Build Binding
- Cycle: 1800
- Base SHA: 838158dbbb616d5f7c868efaf194b44cb78b16fb

## Summary
A bare non-dry `evolve gc` now refuses before any tmux reaper runs, instead of reaping sessions and sockets first and refusing only at the workspace sweep. The gc reapers become an injectable seam so cmd/evolve tests never reach the host tmux. A new `evolve failures` command group (`list`, `reset`, `prune`) inspects and clears `state.json:failedApproaches`, and its `reset` shares one prune-plus-acknowledge function with `loop --reset`.

## Rationale
At the base, `runGC` ran the session and socket reapers before `project()` resolved `--project-root`, so a bare `evolve gc` killed tmux sessions and then refused with "mutating run refused". Moving the root resolution to the top of `runGC` makes the refusal happen before any side effect. The same ordering meant cmd/evolve tests that run a non-dry gc reached the real host reapers; injecting them through the `gcRun` struct lets tests substitute no-ops. The operator had no command to see or clear failed approaches other than `loop --reset`; the `failures` verb adds that, and one `resetFailures` function keeps the two reset paths from drifting.

## Changed Areas
- `go/cmd/evolve/cmd_gc.go` — `runGC` now calls `resolveGCProjectRoot` right after flag parsing and returns its refusal (exit 1) before `newGCRun` or any reaper runs; the `-project-root` help text now says the flag is required for a non-dry run and only `--dry-run` falls back to the current directory; adds the `gcReapers` struct on `gcRun`, which only `newGCRun` defaults to the real `swarm` exec reapers, with a package-level override for tests.
- `go/cmd/evolve/gc_reapers_fake_test.go` — installs no-op reapers for every cmd/evolve test so none can kill host tmux sessions.
- `go/cmd/evolve/cmd_loop_maintenance.go` — extracts `resetFailures`, the single prune-by-class plus fingerprint-acknowledge implementation. It runs the prune and the acknowledgement independently, as the base did: a prune error does not skip the acknowledgement, and both errors come back in a `resetOutcome` beside the prune result. `resetBatchState` calls it and prints the base messages on every path: `[loop] --reset: <err>` or the pruned line, then `[loop] --reset --fingerprint: <err>` or the acknowledged line.
- `go/cmd/evolve/registry.go` — registers the new `failures` verb in the command table so `evolve failures list|reset|prune` dispatches to `runFailures` and appears in the command listing.
- `go/cmd/evolve/cmd_failures.go` — new command group: `list` prints or emits as JSON (`--json`) the failed approaches, optionally filtered by `--class`, reads state without writing it, and treats an absent state file as empty; `reset` drops the infrastructure and ship-gate-config classes through `resetFailures` and optionally acknowledges `--fingerprint`; `prune` removes expired entries via `failurelog.PruneExpired`. `reset` and `prune` refuse (exit 1) without an explicit `--project-root`; a missing verb, unknown verb, bad flag or unknown class exits 10.
- `go/cmd/evolve/cmd_failures_test.go` — table test for list filtering, unknown class, missing verb and the mutating-root refusal, plus a table test pinning `resetBatchState`'s base messages on the unparseable-state, ack-error, clean and no-fingerprint paths.
- `go/acs/cycle1800/predicates_test.go` — the TDD-authored predicates for both tasks.

## Design Decisions
Root resolution is hoisted into `runGC` rather than reordering the reapers after `project()`, so every side effect stays behind the one refusal check. A package-level override pointer was chosen over threading reapers through `runGC`'s signature, which keeps the registry's verb signature unchanged. `failures reset` reuses `resetFailures` instead of re-implementing it; the function returns the prune and acknowledgement errors separately rather than aborting on the first, so neither caller loses a step or a message when the other fails, and `failures reset` exits 1 if either failed while still printing the committed prune; and the read-only `list` keeps the current-directory default that the mutating verbs refuse.

## Verification
The cycle1800 predicates pass 22/22, including 017-022, which pin the base `loop --reset` behavior on the prune-error and ack-error paths and the parity with `failures reset`; the full cmd/evolve test package passes; gofmt and go vet are clean.

## Compatibility
This change alters CLI behavior. A bare non-dry `evolve gc` still exits 1 with "mutating run refused", but no longer prints session or socket reaper output or kills anything before refusing; `gc --dry-run` and `gc --project-root <dir>` behave as before. The `gc -project-root` help text changed. `evolve failures` is a new verb with its own output and exit codes 0, 1 and 10. `loop --reset` keeps the base behavior and messages on every path, including an unparseable state.json (the fingerprint is still acknowledged) and an acknowledgement error (the committed prune is still reported, and the error keeps the `[loop] --reset --fingerprint:` prefix).

## Limitations
The deferred cli-ci-watch and cli-boundary-run verbs are not built.
