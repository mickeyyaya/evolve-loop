# Build Explanation — Cycle 1787

## Build Binding
- Cycle: 1787
- Base SHA: 60821745599e0b37848be0f533f345b4042d8cab

## Summary
Two operator verbs are added to the `evolve` CLI: `clihealth` (also spelled `cli-health`; list and clear CLI-family benches) and `ratchet check [size|rawgit]` (run the function-size and raw-git-fixture ratchets on demand, both or the named one). Each ratchet package gains a `Scan(root)` entry point, and both the command and the packages' repo-wide gate tests (`TestRatchet_ModuleFunctionsFitTheirAllowances`, `TestRatchet_NoNewRawGitFixtures`) call it, so they run one code path.

## Rationale
Operators previously had to edit `.evolve/cli-health.json` by hand to inspect or lift a bench, and could only run the ratchets through `go test`. Thin verbs over the existing `clihealth.Store` and the existing ratchet `Walk`/`Check` functions give a scriptable surface with no new state. Extracting `Scan` into each ratchet package, instead of re-implementing the pipeline in the command, keeps a single definition of what "clean" means. A separate `ratchet` subpackage or flag on an existing command was rejected as more surface for the same behavior.

## Changed Areas
- `go/cmd/evolve/cmd_clihealth.go` — new `clihealth list [--json]` over `Store.Active` and `clear <family>` over `Store.Active`/`Clear`, so clear accepts exactly the families list shows; an unbenched or expired family exits 1 and touches nothing, usage errors exit 2.
- `go/cmd/evolve/cmd_clihealth_test.go` — tests the list/clear verbs through the dispatcher, including unknown-verb cases, clear of an expired bench exiting 1, and the `cli-health` alias.
- `go/cmd/evolve/cmd_ratchet.go` — new `ratchet check [size|rawgit] [--root DIR]` that runs both `Scan` functions or the selected one, prints every violation to stderr and exits 1, exiting 0 when clean; flags and the selector may come in any order, and an unknown selector or any extra argument exits 2 without scanning, so `--root` can never be silently dropped.
- `go/cmd/evolve/cmd_ratchet_test.go` — proves the real module is clean, other verbs are rejected, and a table pins the selector, `--root` after the selector, and exit 2 on unknown selectors and trailing arguments.
- `go/cmd/evolve/registry.go` — registers the `clihealth` (alias `cli-health`) and `ratchet` commands in the dispatch table.
- `go/cmd/evolve/main.go` — lists both groups in the usage text.
- `go/internal/sizeratchet/sizeratchet.go` — adds `Scan(root)` and `OffendersRelPath`, chaining Walk, LoadOffenders and Check; a missing offender list fails loudly.
- `go/internal/sizeratchet/sizeratchet_test.go` — the repo-wide gate now calls `Scan(moduleRoot(t))`, and `TestScan_*` exercises the scan on a temporary module.
- `go/internal/rawgitratchet/rawgitratchet.go` — adds `Scan(root)` and `BaselineRelPath`, chaining BoundTestFiles, Sites, LoadBaseline and Check.
- `go/internal/rawgitratchet/rawgitratchet_test.go` — the repo-wide gate now calls `Scan(moduleRoot(t))`, and `TestScan_*` covers unlisted raw init, baselined site, and missing baseline.
- `go/acs/cycle1787/predicates_test.go` — the 21 acceptance predicates for both verbs, run against the built binary, including selector fail-closed and gate-reaches-`Scan` coverage checks.
- `docs/operations/runtime-reference.md` — documents both verbs in a new operator-verbs section.

## Design Decisions
Both commands are read-mostly wrappers: `ratchet check` never edits `offenders.json` or `baseline.json`, and `clihealth clear` removes exactly the named family. Exit codes follow the repo convention: 0 clean, 1 finding, 2 usage.

## Verification
The cycle1787 ACS predicates pass 21/21 after the audit-round-1 repair. Unit tests for `./cmd/evolve` and the two ratchet packages pass, `gofmt` and `go vet` are clean, and `evolve selfcheck build` is GREEN.

## Compatibility
Purely additive: new commands, one alias and new exported `Scan` functions. No existing flag, file format or exit code changes; the offender and baseline files are untouched.

## Limitations
`ratchet check` targets the evolve-loop module layout (fixed relative paths for the offender and baseline lists). The `cmd/evolve` test package runs close to the build floor's 120s deadline under load.
