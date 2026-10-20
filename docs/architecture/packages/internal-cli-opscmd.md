# internal/cli/opscmd

## Purpose

`internal/cli/opscmd` holds the `evolve` handlers for the release and operations subcommands: `doctor` (probe, boot and live), `changelog-gen`, `version-bump`, `marketplace-poll`, `release-preflight`, `release-consistency`, `release` and `rollback`. It also holds the operator lease writer and the plane report. Each handler parses its own flags and delegates the work to an `internal/*` package. The package holds no business logic.

## Design

- **Handler signature.** Each exported `Run*` function takes the subcommand arguments, stdin, stdout and stderr, and returns an exit code. The composition root in `cmd/evolve` wires them into the registry. The extraction keeps the root small. Nothing in `cmd` imports this package, so no cycle is possible.
- **Exit codes mirror the legacy bash scripts.** Exit 0 means success, 1 a failed check or write, 2 a runtime error, 3 a post-publish failure, and 10 invalid arguments. The codes are part of the operator contract. The named constants are in `exit.go`.
- **Live probe.** `doctor live` submits one trivial prompt. A boot smoke test passes against a rate-limited CLI, and the quota wall appears only after work is submitted. So only the live probe can see a wall, and the exit code 1 names the escalation pattern.
  - The probe reports the model id that served the reply: `[doctor] served model: <id>` on stderr and `served_model` in `--json`. `runDoctorLive` gives the bridge the default token resolver over the Claude config root, which reads the id from the transcript. A driver with no transcript prints `unreported`, and its exit code does not change. Pinned by `TestDoctorLive_ReportsTheModelThatServedTheReplyFromAFakeTranscript` and `TestDoctorLive_ADriverThatReportsNoModelPrintsUnreportedAndKeepsItsExitCode`.
- **Boot probe.** `doctor boot` boots the driver's REPL without a prompt or an artifact. It rejects an unknown driver or a non-tmux driver with exit 10, before any tmux call.
- **Console lease.** The lease lives in the git common directory, which is outside every worktree, so no lane phase can write or shadow it. Its wire struct is a copy of the core schema, which keeps this package free of a core import. The path is normalized to the exact repo-relative form that the guard compares. A `./` prefix or an absolute path will otherwise waive nothing while the command reports success.
- **Argument order.** The lease command collects its positional paths itself and re-parses the rest. The shared reorder helper is for boolean flags only. Applied to `--ttl 10m`, it will scatter the value into the path list.

## Invariants

- The exit codes of each handler match the legacy script it mirrors. Pinned by `cli_error_paths_test.go`, `release_preflight_test.go` and `marketplace_poll_test.go` (for example `TestCmd_MarketplacePoll_BadSemver`, exit 2).
- An unknown or non-tmux driver is a usage error, and it touches no tmux. Pinned by `TestDoctorBoot_UnknownDriver` and `TestDoctorBoot_Usage`.
- The JSON output of the boot probe keeps its documented fields. Pinned by `TestDoctorBoot_JSONShape`.
- The lease lands in the common directory, its paths are normalized, value flags survive the paths-first form, and `--clear` removes only the lease. Pinned by `console_lease_test.go`.
- The plane report warns with exit 2 when loop state lives in the console checkout. In a hub it reports the broken worktree links of the store records only (exit 1). Under `--repair` it repairs them (exit 0, or exit 2 on git error). Pinned by `doctor_plane_test.go`.
- Every exported handler answers its usage text without side effects. Pinned by `apicover_smoke_test.go`.
