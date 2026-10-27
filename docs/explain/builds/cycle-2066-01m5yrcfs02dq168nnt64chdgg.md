# Build Explanation — Cycle 2066

## Build Binding
- Cycle: 2066
- Base SHA: 7635ab3bb717069091a31f1a47cbd9be32cc60b5

## Summary
`evolve ci telemetry` prints the critical path of a CI run from L5 (job) to L1 (test). It joins the jobs, steps, test JSON, rusage and sampler records, and names the limiting resource of each package.

## Rationale
The C0 and C0b artifacts existed, but no command joined them. A new package holds the join, so the command stays a thin wrapper and the join has 100% coverage. The limit rules follow the plan: the first rule that holds wins, and a package without a rusage record is `unknown`.

## Changed Areas
- `go/internal/citelemetry/citelemetry.go` — builds the report: the L5 and L4 path, the G1 times and the input checks.
- `go/internal/citelemetry/parse.go` — reads the jobs document, the NDJSON files and the artifact directories, and runs the whole read.
- `go/internal/citelemetry/packages.go` — groups the test stream by package, joins rusage and sampler records, and chooses the limit.
- `go/internal/citelemetry/markdown.go` — renders the report as Markdown.
- `go/internal/citelemetry/citelemetry_test.go` — tests the path, the join, each limit rule and each failure.
- `go/internal/citelemetry/apicover_named_test.go` — names every exported symbol in an assertion.
- `go/cmd/evolve/cmd_ci_telemetry.go` — adds the `ci telemetry` verb with `--jobs`, `--job-artifacts` and `--json`.
- `go/cmd/evolve/cmd_ci_telemetry_test.go` — tests the verb output, the exit codes and a write failure.
- `go/cmd/evolve/cmd_ci.go` — dispatches the verb and adds it to the usage text.
- `go/cmd/evolve/registry.go` — adds the verb to the summary of `ci`.
- `go/.cover-strict` — enrolls the package at 100%.
- `go/.apicover-enforce` — enrolls the package in the exported-symbol gate.
- `docs/architecture/packages/internal-citelemetry.md` — documents the design of the package.

## Design Decisions
Only `Run`, `Report` and its row types are exported, because the command needs nothing else. Optional artifacts (rusage and sampler) may be absent, because ubuntu jobs may not upload them. The test stream is required, so a wrong directory fails loudly.

## Verification
`go test` of `internal/citelemetry` and `cmd/evolve` pass, the package has 100% function coverage, and the 13 predicates of cycle 2066 pass.

## Compatibility
The existing `ci classify`, `ci watch` and `ci darwin-set` verbs are unchanged. The report adds no field to any existing output.

## Limitations
The `contention` limit of decision D21 and the CPU count per runner type are not built. They belong to component C12.
