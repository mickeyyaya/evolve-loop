# Build Explanation — Cycle 1812

## Build Binding
- Cycle: 1812
- Base SHA: cc10e36d89de7fd4a3de1db66155eeed93a43045

## Summary
A malformed `.evolve/policy.json` no longer drops operator-declared phase roots without notice. `phasespec.RootsWithWarnings` returns the roots plus one coded `PHASE_ROOTS_POLICY_UNREADABLE` warning when `policy.Load` fails. `MergedCatalog`, `phases validate` and `phase lint` print that warning. Two phasespec tests that passed by accident or duplicated a helper now pin what they name, and the plugin doc §3.2 describes the real root source.

## Rationale
`Roots` discarded the `policy.Load` error, so a JSON typo silently fell back to `.evolve/phases`. The error is now reported as a warning, not a failure. `looppreflight` fails open on a `MergedCatalog` error, and `phases list` must keep listing the catalog, so a returned error would hide the problem again. `Roots` keeps its signature because five callers use only the roots.

## Changed Areas
- `go/internal/phasespec/mergedcatalog.go` — adds `RootsWithWarnings`, which carries the policy load error as a coded warning. `Roots` now delegates to it, and `MergedCatalog` puts the warning ahead of the discovery warnings.
- `go/internal/cli/phasecmd/phases.go` — `phases validate` discovers through `RootsWithWarnings`, so the policy warning reaches its stderr WARN stream.
- `go/internal/cli/phasecmd/phase_lint.go` — `phase lint` discovers through `RootsWithWarnings`, so the policy warning reaches its stdout WARN stream.
- `go/internal/phasespec/mergedcatalog_test.go` — covers missing, valid and malformed policy.json for `RootsWithWarnings`, and the malformed case through `MergedCatalog`.
- `go/internal/cli/phasecmd/phases_policywarn_test.go` — proves the warning reaches the `phases validate` and `phase lint` entry points, and that it never changes their exit code.
- `go/internal/phasespec/discover_test.go` — the vacuous "native reserved" case becomes "command reserved" with a valid multi-word name, plus a "native executable" clean case. Removing the reserved-kind rule now fails the test.
- `go/internal/phasespec/repo_phaseconfigs_test.go` — `TestRepoPhaseCatalog_NoInertFailIfSignal` now runs on the shared `eachTrackedPhaseClassify` walk instead of a copy of it.
- `docs/architecture/phase-plugin-system.md` — §3.2 says the roots come from `paths.phase_roots` in `.evolve/policy.json` through `phasespec.Roots`, not `EVOLVE_PHASE_ROOTS`, and names the warning.

## Design Decisions
The new function is a sibling, not a signature change to `Roots`. Only the three seams that already print a WARN stream use it. `phases create` reports through a JSON envelope, and `phase-inventory build` and the cmd/evolve composition root take roots as plain input, so all three still use the silent `Roots`. A missing policy.json stays silent because `policy.Load` returns an empty policy for it.

## Verification
The cycle-1812 ACS predicates (6) pass. The new unit and entry-point tests fail when run against the base `mergedcatalog.go` through a `go test -overlay` and pass on the change. The phasespec, phasecmd and dependent package suites pass.

## Compatibility
`Roots` and `MergedCatalog` keep their signatures. Exit codes are unchanged. The only new output is one WARN line, and only when policy.json is malformed.

## Limitations
`phases create`, `phase-inventory build` and the cmd/evolve composition root (`phaseRoots`) still fall back silently on a malformed policy.json.
