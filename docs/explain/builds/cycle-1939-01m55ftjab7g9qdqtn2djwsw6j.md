# Build Explanation — Cycle 1939

## Build Binding
- Cycle: 1939
- Base SHA: 2ab5823a7237968e8e5c193e04043f4f3c253441

## Summary
The apicover and apicover-check make targets now write and read coverage.apicover.func.txt. They no longer touch the shared coverage.func.txt.

## Rationale
The apicover targets overwrote the coverage.func.txt that test-integration and make cover produce. A scoped apicover run then replaced a full-run report. The scoped profile coverage.apicover.txt already existed, so the func report gets the matching scoped name. The name fits the ignore rule coverage.*.txt, so no ignore change is needed.

## Changed Areas
- `go/Makefile` — apicover, apicover-check and clean use coverage.apicover.func.txt; the cover target keeps coverage.func.txt.

## Design Decisions
Only the func report is renamed. The profile name and the COVER_PROFILE variable stay unchanged, so apicover-enforce works as before.

## Verification
Four ACS predicates (TestC1939_001 to 004) run make -n on the targets and check the file names. The ACS suite reports green=171 red=0.

## Compatibility
The make cover output and the CI step in go.yml are unchanged. Local readers of coverage.func.txt after apicover must read the new name.

## Limitations
The documentation of the gc allowlist does not list the new file; the pattern coverage.*.txt in the root ignore file covers it.
