# Build Explanation — Cycle 1856

## Build Binding
- Cycle: 1856
- Base SHA: ddd10f569d57b1b6b3c52cc66811ed3010eea0fc

## Summary
The reachabilityprobe cycle diagnostic now closes the loop at the referenced package (`storage -> core -> storage`). Before this change it repeated the pinning package (`storage -> core -> core`). The private `extractPins` loses an error result that it never set, so `ExtractFrozenPins` and `CheckFrozenPins` no longer carry unreachable error branches. The regexp identifier `goSourceLiteal` is renamed `goSourceLiteral`.

## Rationale
`Violation.Cycle` runs from ReferencedPackage to PinningPackage, both inclusive. To show the cycle closing, the message must append the head of the chain, not its tail. `extractPins` deliberately fails open on an unreadable frozen file, so its error result was always nil. Removing it makes the fail-open contract visible in the signature. The exported signatures stay the same, so the callers in `cmd/evolve`, `internal/repocontract` and `internal/cli/phasecmd` do not change.

## Changed Areas
- `go/internal/reachabilityprobe/reachabilityprobe.go` — `Violation.Error()` ends the chain with `Site.ReferencedPackage` instead of `Site.PinningPackage`, so the printed cycle closes correctly.
- `go/internal/reachabilityprobe/frozenpins.go` — `extractPins` returns only `[]pinnedRef`. Both callers drop their dead `err` branches. `CheckFrozenPins` scopes `err` to the graph build. `goSourceLiteal` is renamed `goSourceLiteral`.
- `go/internal/reachabilityprobe/reachabilityprobe_test.go` — a table test checks the exact `Error()` text for the two-hop, three-hop and self-reference cycles.
- `go/internal/reachabilityprobe/frozenpins_test.go` — a test checks that an unreadable frozen file stays fail-open in both `ExtractFrozenPins` and `CheckFrozenPins`.
- `go/acs/cycle1856/predicates_test.go` — the TDD phase's ACS predicates for this cycle. The builder did not edit them.
- `.evolve/evals/reachabilityprobe-error-path-duplicate.md` — the task eval written by an earlier phase, with the score caps and graders.

## Design Decisions
The exported `([]CallSite, error)` and `([]Violation, error)` signatures stay as they are. Narrowing them would touch three external callers for no behavioral gain. Only the private helper changes. The existing `pathString` helper also stays: replacing it with `strings.Join` would be a drive-by refactor outside this task.

## Verification
`go test -count=1 ./internal/reachabilityprobe` passes 14/14 top-level tests. `go test -tags acs ./acs/cycle1856` passes 6/6. `evolve acs suite --cycle 1856` reports green=175 red=0. `go vet` is clean on the package and on its three external callers.

## Compatibility
The only behavior change is the text of `Violation.Error()`. No caller parses that text. It serves as a human-readable failure reason.

## Limitations
The exported functions still return an `error` that is always nil. That error is kept for signature stability.
