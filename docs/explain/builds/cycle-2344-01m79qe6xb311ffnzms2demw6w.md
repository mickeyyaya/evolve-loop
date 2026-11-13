# Build Explanation — Cycle 2344

## Build Binding
- Cycle: 2344
- Base SHA: be88284173b43a7cd3cbc46ba6f60716d94e4d9d

## Summary
The process-group cancel test now reports the exit status, the signal and the elapsed time when it fails. The package note records the symptom, the diagnostic and a macOS race run. It does not prove the cause.

## Rationale
The flake did not reproduce, so a fix is not possible without a cause. A message that names the evidence is the smallest change that makes the next red diagnosable.

## Changed Areas
- `go/internal/phases/ship/repocontract_cmd_unix.go` — adds `describeWaitResult` and the `waitState` interface. The group kill is unchanged.
- `go/internal/phases/ship/repocontract_cmd_unix_test.go` — the failure message uses `describeWaitResult`.
- `go/internal/phases/ship/repocontract_cmd_describe_unix_test.go` — pins the message with a fake process state.
- `docs/architecture/packages/internal-phases-ship.md` — adds the Process-group cancel test section with the cause and the run.

## Design Decisions
`waitState` is a small consumer-side interface that `*os.ProcessState` satisfies, so a fake state can drive the test. A typed nil state is handled.

## Verification
The fake-state tests, the integration cancel tests and the four ACS predicates pass. A `-race` run of 200 passed under load.

## Compatibility
No exported name changes. The cancel behavior is unchanged.

## Limitations
The cause is not proven. The run did not reproduce the flake.
