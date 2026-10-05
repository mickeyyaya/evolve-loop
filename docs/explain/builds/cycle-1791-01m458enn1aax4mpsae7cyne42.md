# Build Explanation — Cycle 1791

## Build Binding
- Cycle: 1791
- Base SHA: 9761f517835683b2568022238370efd987d65cef

## Summary
`evolve loop` gains three operator surfaces:
- `--preflight-only` runs the real pre-batch readiness gate on its own. Exit 0 means READY. Exit 1 names each blocking check. It dispatches nothing.
- `--detach --log F` re-execs the same loop arguments in a new session, with output appended to F. It then waits, up to the policy bound `boot.detach_wait_s`, for a live run lease, and its exit code says whether the loop booted.
- `evolve loop status` prints the loop line of `evolve status` plus the lease heartbeat and starts nothing.

A positional goal that is exactly a reserved word (`status`, `stop`, `help`, `plan`, `watch`) now exits 10 naming the intended verb instead of launching a batch.

## Rationale
Every one of these replaces a manual or error-prone operator step:
- The readiness gate used to run only inside a real launch.
- Background launches used nohup and a hand-checked pid, so a launch that died at boot went unnoticed.
- `evolve loop status` and `evolve loop stop` silently launched batches whose goal was the verb.

The new modes reuse the existing seams so there is one source of truth for each answer:
- `runLoopPreflightFn` and `persistLoopPreflight` for the gate.
- `runlease.LiveRuns` for "is a loop running".
- `dashboard.Collect(...).Loop` and the shared loop-line writer for status.

## Changed Areas
- `go/cmd/evolve/cmd_loop.go` — `runLoop` dispatches `loop status`, then `--preflight-only`, then `--detach`, all before `takeReexecHandoff`, so no new mode consumes the boundary re-exec handoff. `loopConfig` gains `PreflightOnly`, `Detach`, `LogPath` (all omitempty) and `DetachArgv` (not serialized), so existing `--dry-run` output is unchanged.
- `go/cmd/evolve/cmd_loop_args.go` — registers `--preflight-only`, `--detach` and `--log`. `validateLoopModes` rejects combinations with exit 10 before anything runs: detach with preflight-only, preflight-only with skip-preflight or dry-run, detach with dry-run, detach without log, and log without detach. The positional goal goes through `reservedGoalVerb` before goal resolution. A goal is optional under `--preflight-only`. `detachChildArgs` strips only the detach and log tokens from the flag region of the parsed argv.
- `go/cmd/evolve/cmd_loop_preflight.go` — `runLoopPreflightOnly` runs the gate once, persists `loop-preflight.json`, prints the Summary to stdout, and returns 0 with READY or 1 with one stderr line per halting check. The real gate's sandbox host probe `mkdir`s `.evolve/worktrees` as a writability probe, so `removeEmptyProbeDir` removes that directory only when it did not exist before and is still empty. This keeps the mode trace-free without changing the probe the launch path shares.
- `go/cmd/evolve/cmd_loop_detach.go` — new `runLoopDetached`:
  - It refuses beside a live run lease.
  - It opens the log append-only and records the offset.
  - It starts the child through the `loopDetachCommandFn` seam with stdin `/dev/null` and `Setsid`, then prints the pid.
  - On each poll it checks, in order: child exit (exit 1 with this launch's log tail), a live run lease (exit 0), then the policy deadline (exit 1, child not killed).
- `go/cmd/evolve/cmd_loop_detach_unix.go` — `detachSysProcAttr` returns `Setsid: true` on unix.
- `go/cmd/evolve/cmd_loop_detach_other.go` — `detachSysProcAttr` returns an unsupported error on non-unix, so `Setsid` never appears in a non-unix build.
- `go/cmd/evolve/cmd_loop_status.go` — new `isLoopStatusInvocation` (only `status` alone or followed by a flag, so `status of the fleet` stays a goal) and `runLoopStatus`. It has the same flags, root resolution and snapshot check as `evolve status`, with exit 2 for an unreadable snapshot and 10 for a stray argument. Text output is the loop line plus `lease:   heartbeat=`. `--json` prints `{"loop": …}`.
- `go/cmd/evolve/cmd_status.go` — extracts `writeStatusLoopLine` from `writeStatusText` so both commands print a byte-identical loop line.
- `go/internal/policy/fleet.go` — `BootPolicy.DetachWaitS` (`boot.detach_wait_s`), `DefaultBootDetachWait` (10m), and `Policy.BootDetachWait()`, which falls back to the default for an absent block or a value of zero or less.
- `go/internal/policy/boot_policy_test.go` — table test of `BootDetachWait` defaults and override, plus a load-from-JSON test.
- `go/cmd/evolve/cmd_loop_cli_modes_test.go` — unit tests for `detachChildArgs` (including `--log --detach` and input immutability), `reservedGoalVerb`, `isLoopStatusInvocation`, `runLoopStatus` on an idle plane and its refusals, `tailLogSince` edges, `detachExitCode`, and `defaultLoopDetachCommand`.
- `go/cmd/evolve/cmd_loop_preflight_probe_test.go` — proves the probe-directory cleanup removes only an empty directory that the gate created.
- `go/cmd/evolve/cmd_loop_preflight_only_test.go` — TDD-authored unit contract for `--preflight-only` (stubbed gate pass, halt, handoff and conflict cases).
- `go/cmd/evolve/cmd_loop_detach_test.go` — TDD-authored unit contract for `--detach`, using fake re-exec'd children: lease, boot exit, live-run refusal, policy timeout, argv and session leader.
- `go/cmd/evolve/cmd_loop_detach_sid_darwin_test.go` — TDD-authored session-id helper for darwin.
- `go/cmd/evolve/cmd_loop_detach_sid_linux_test.go` — TDD-authored session-id helper for linux.
- `go/acs/cycle1791/predicates_test.go` — the cycle's 14 binary-level acceptance predicates.
- `.evolve/evals/cli-loop-detach.md` — eval score caps for the detach item.
- `.evolve/evals/cli-loop-status.md` — eval score caps for the status item.
- `.evolve/evals/cli-loop-preflight-only.md` — eval score caps for the preflight-only item.
- `docs/architecture/packages/cmd-evolve.md` — records that the non-dispatch modes run before the re-exec handoff, the lease-based detach verdict, and the reserved-word guard.
- `docs/operations/runtime-reference.md` — documents `--preflight-only` and `--detach --log` in the wave-boundary Launch step, `evolve loop status` and the reserved-word guard in Operator commands, and `boot.detach_wait_s` in the boot policy row.

## Design Decisions
- **Lease-based boot verdict.** Exit 0 from `--detach` requires a live run lease that did not exist before launch; any live lease refuses the launch up front. It is never keyed on the child's pid owning the lease, because in fleet mode a lane subprocess owns it. Child exit is checked before the lease, so an exited child is never reported as running.
- **The parent never kills the child.** On a timeout the child keeps running and the operator gets its pid and log.
- **Reserved-word guard on positional goals only.** `--goal-text status` remains an explicit request for that text.
- **`detachChildArgs` follows the contract's signature.** It treats a bare `--log` as consuming the next token, as Go's `flag` package does. A value-taking flag whose value happens to be `--detach` is an accepted edge.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1791`: all 14 predicates pass against the built binary, including the real readiness gate.
- The 10 TDD unit-contract tests pass.
- 9 builder unit tests and 2 policy tests pass.
- gofmt, go vet and the full module test suite were run from `go/`.

## Compatibility
- Launches without the new flags parse and dispatch exactly as before.
- New `loopConfig` JSON fields are omitempty.
- A real launch's gate halt keeps exit 2.
- No `EVOLVE_*` variable is added.
- A one-word positional goal that equals a reserved word, which previously launched a batch, now exits 10. That behaviour change is intended.

## Limitations
- `--detach` is unix-only and errors on other platforms.
- `evolve loop status` reports only the loop section, with no PRs or CI.
- The preflight probe-directory cleanup covers only the in-project `.evolve/worktrees`. Probe directories under TMPDIR are left as the gate leaves them.
