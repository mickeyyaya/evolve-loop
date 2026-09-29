# Build Explanation — Cycle 1755

## Build Binding
- Cycle: 1755
- Base SHA: ad310db6bd35d64a676c4e820186140697d67b76

## Summary
`runACSSuite`, the `evolve acs suite` entry point, now fits the 50-line size ratchet. Its summary-line and per-RED-predicate printing moved into one helper, `printACSSuiteVerdict`. The ratchet allowances for `runACSSuite`, `internal/cyclehealth.Check` and `pkg/naminguard.Fix` are removed from `offenders.json`. `Check` and `Fix` already measured 33 and 37 lines at base, so both of those entries were slack.

## Rationale
At base, `runACSSuite` measured 51 lines. Pulling out the one self-contained block that has no control-flow exits (the stdout reporting of the verdict) is the smallest extraction that brings it under the cap without moving any exit-code decision. For `Check` and `Fix` the scout read each 51 allowance as a measured size. Those functions were shrunk in earlier cycles, so the only remaining step is to drop their allowances. An allowance cannot be lowered to 50 or below, so the entries are removed rather than lowered. The two packages themselves are left byte-identical.

## Changed Areas
- `go/cmd/evolve/cmd_acs.go` — adds `printACSSuiteVerdict(stdout, v)` holding the unchanged summary `Fprintf` and the RED-line loop, and replaces that block in `runACSSuite` with a call to it. Exit codes, flag parsing, root auto-resolution and verdict writing all stay inside `runACSSuite`.
- `go/internal/sizeratchet/offenders.json` — removes the `cmd/evolve.runACSSuite`, `internal/cyclehealth.Check` and `pkg/naminguard.Fix` entries. No key is added and no allowance is raised.
- `docs/explain/builds/cycle-1755-01m3nzmym6x453naf7z0rqtcea.md` — this explanation document.

## Design Decisions
The helper takes `acssuite.Verdict` by value and writes only to stdout. It returns nothing, so every exit-code path is still visible in `runACSSuite`. The alternative was to extract flag parsing into a struct-returning helper. That was rejected because it would split the exit-10 decisions across two functions and add a struct type just to carry four fields. Following the `cmd/evolve` craft rule, the helper carries no comment.

## Verification
The cycle-1755 ACS predicates pass 11/11. They include the CLI-contract characterization run through the production dispatch `runACS("suite", ...)` and the module-wide `sizeratchet.Check`. The cyclehealth and naminguard characterization tests pass unmodified, and `go vet ./cmd/evolve/` and gofmt are clean.

## Compatibility
The `evolve acs suite` output bytes, exit codes (0/1/2/10), flags and the verdict file are all unchanged.

## Limitations
This change does not touch the other remaining `offenders.json` entries.
