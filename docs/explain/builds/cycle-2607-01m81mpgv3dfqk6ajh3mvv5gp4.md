# Build Explanation — Cycle 2607

## Build Binding
- Cycle: 2607
- Base SHA: 9ba6c06ca9f97daae06e7cd79c469ec5331f7335

## Summary
The `gc` check registry now includes a `temp` check that reports each pipeline temp entry with its shape while `ReapPipelineTemp` keeps its exact reap rule.

## Rationale
`ReapPipelineTemp` removed entries and returned counts only, so `evolve gc check` had no per-entry view of pipeline temp artifacts. A shared classifier lets both `tempCheck.Scan` and `ReapPipelineTemp` apply one name pattern, age cutoff, and live-holder rule.

## Changed Areas
- `go/internal/gc/tempdir.go` — extracts `classifyTempDir` and `classifyTempEntry` from `ReapPipelineTemp`, and routes `ReapPipelineTemp` through that classifier so one rule governs scan and reap.
- `go/internal/gc/check_temp.go` — adds `tempCheck` with `Name`, `Scan`, and `Apply`, mapping each matched entry to `remove`, `keep_fresh`, `spare_held`, or `spare_unknown` with a `temp.*` rule and holder evidence.
- `go/internal/gc/check.go` — registers `tempCheck{}` in `DefaultChecks` alongside `worktreeCheck{}`.

## Design Decisions
`tempCheck` leaves `CheckContext` unchanged and reads `Policy.TempTTLHours`, `os.TempDir()`, and `LiveHolderProbe` with `sysexec.DefaultRunner`. When `gc.temp_ttl_hours` is unset (`<= 0`), `Scan` returns an empty item list and `Apply` is a no-op, matching `gcRun.pipelineTemp`. An unreadable temp directory makes `Scan` return an error so `evolve gc check` fails loudly without a report. `tempCheck.Apply` delegates to `ReapPipelineTemp`.

## Verification
All 10 cycle 2607 ACS predicates (`go test -tags acs -count=1 ./acs/cycle2607`) and the `go/internal/gc` test suite pass. The three prior tempdir test files remain unchanged against the base commit.

## Compatibility
`ReapPipelineTemp` keeps its exported signature, report counts, and error strings unchanged. `evolve gc check --json` keeps its top-level `checks` array and adds a `temp` entry.

## Limitations
Unit tests in `go/internal/gc/check_temp_test.go` verify `tempCheck.Apply`. `evolve gc --only cache` still invokes `ReapPipelineTemp` from `go/cmd/evolve/cmd_gc.go` until a later item routes all sweeps through `Check.Apply`.
