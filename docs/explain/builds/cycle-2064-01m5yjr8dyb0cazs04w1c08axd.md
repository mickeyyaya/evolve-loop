# Build Explanation — Cycle 2064

## Build Binding
- Cycle: 2064
- Base SHA: 73be29bcb595b6c2807e05c56bad1a179f89172e

## Summary
`testlatency` gates each test package against a committed ratchet file. It warns about regressions in the job summary. A `tighten` subcommand lowers recorded values.

## Rationale
The plan (latency-gates T4) needs a gate that runner noise cannot trip. A limit of min(2 x class ceiling, TEST_TIMEOUT) leaves wide headroom, and a p90 warning shows slow drift before it fails CI. The existing flags stay unchanged, so the advisory budget step keeps its contract.

## Changed Areas
- `go/cmd/testlatency/ratchet.go` — loads and validates the ratchet file, computes the limit, the failures and the warnings, and tightens values.
- `go/cmd/testlatency/main.go` — adds `-ratchet`, `-test-timeout` and the `tighten` subcommand.
- `go/cmd/testlatency/ratchet_test.go` — tests the limit, the exit code 2 cases, the warning margins and the tighten rules.
- `go/.test-latency.json` — records the class and the measured wall time of each of the 349 tested packages.
- `.github/workflows/go.yml` — adds a ratchet step to build-test and to the darwin job, without a swallowed exit status.
- `go/internal/ciparity/workflows_test.go` — the budget step lookup takes the first testlatency step, so the new later step does not replace it.
- `docs/architecture/packages/cmd-testlatency.md` — documents the ratchet, the warning and tighten.

## Design Decisions
The class ceilings are constants in code and the file holds only the class name and the p90. A package without a record is limited by the timeout alone. Tighten writes through a temporary file and a rename, and writes nothing when no value is lower.

## Verification
`go test ./cmd/testlatency ./internal/ciparity` and the 13 predicates of cycle 2064 pass. The ACS suite is green with red=0.

## Compatibility
Without `-ratchet`, the output and exit codes are the same as before. `-budget`, `-package-budget` and `-fail-over-budget` are unchanged.

## Limitations
The first p90 values are single local measurements, not CI p90 values. Run `testlatency tighten` and review the diff to refine them.
