# internal/citelemetry

> Plan: [ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md), component C1, decisions D3 and D12. Caller: `evolve ci telemetry` (`go/cmd/evolve/cmd_ci_telemetry.go`).

## Purpose

`internal/citelemetry` joins the records of one CI run. It reads the GitHub jobs document, the `go test -json` stream, the rusage records and the sampler records. It gives the critical path from L5 (the job) down to L1 (the test), and the limiting resource of each package.

## Design

- **Entry point.** `Run(jobsFile, bindings)` reads the jobs document. Each binding is `<job name>=<directory>`. The directory holds `test-integration.json`, and optionally `test-rusage.ndjson` and `runner-samples.ndjson`. A missing optional file is an empty list.
- **Path.** L5 is the longest job, from start to completion. L4 is the longest step of that job.
- **Path, packages.** L3 is the package with the longest wall time in the artifacts bound to that job. L2 is the larger part of the package floor: the serial sum or the longest parallel test. L1 is the slowest test of that part. The path stops at the level where the evidence stops.
- **Parallel tests.** A `pause` event marks a test as parallel. Subtests are skipped, because the parent already counts them.
- **Join.** A rusage record joins a package only on the exact path. The module prefix is the prefix that most records agree on. A package that ends the same way but has another parent does not join.
- **Sampler window.** The sampler records of a package are those from its first event to its result event, inclusive.
- **Limit.** The first rule that holds names the limit of a package. The rules follow in order.
  - No rusage record: `unknown`.
  - Swap above 1024 MB: `memory`.
  - Every sample above 90% busy: `cpu-saturation`.
  - System CPU above user CPU, with 0.5 s of CPU or more: `subprocesses-and-syscalls`.
  - CPU below half of the wall time: `wait`.
  - Otherwise: `cpu`.
- **Contention.** The `contention` limit of decision D21 is the work of C12.
- **Incomplete packages.** A package that started but has no result is listed in `incomplete`, not in `packages`.
- **G1.** The macOS job time is the longest job with `macOS` in its name. The ubuntu test step time is the longest step that starts with `test` in a job with `ubuntu` in its name.
- **Failure.** Any unreadable or malformed input returns an error. The command prints no report then.

## Invariants

- `Run` never changes a file.
- Every function has 100% coverage (`go/.cover-strict`). The package is in `go/.apicover-enforce`.
