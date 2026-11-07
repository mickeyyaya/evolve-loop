# Build Explanation — Cycle 2215

## Build Binding
- Cycle: 2215
- Base SHA: 1128e9d82c2a92d9c81ab18461be35fe2298ce42

## Summary
The macOS CI job now reports how close the slowest package ran to the go test timeout. A margin under 20% of the timeout is flagged LOW.

## Rationale
The job timed out once in cmd/evolve and no output showed the margin. The report reuses the testlatency stream parser, so no new tool is needed. The timeout has one home in the Makefile, and the job reads it from there.

## Changed Areas
- `go/cmd/testlatency/main.go` — adds the `-timeout-margin` flag. A value that is not a positive duration gives exit 2.
- `go/cmd/testlatency/report.go` — adds the section `## Margin against the go test timeout` with the slowest completed package and each package with no terminal summary.
- `go/Makefile` — adds `DARWIN_TEST_TIMEOUT` and the target `print-test-timeout-darwin`, so the printed number is the applied number.
- `.github/workflows/go.yml` — adds a darwin job step that writes the margin report to the job summary from the Makefile value.
- `docs/architecture/packages/cmd-testlatency.md` — documents the flag and the section.

## Design Decisions
The LOW test is `margin * 100 < timeout * 20`. It avoids the float rounding of `0.2 * timeout`, so a margin of exactly 20% is not LOW. The step is advisory and never changes an exit code.

## Verification
The nine predicates of `go/acs/cycle2215` pass. The `cmd/testlatency` tests pass.

## Compatibility
Without `-timeout-margin` the report is unchanged. The darwin timeout stays 30m.

## Limitations
The report covers only the slowest completed package. The three-run pass condition needs three CI runs and is not provable in this cycle.
