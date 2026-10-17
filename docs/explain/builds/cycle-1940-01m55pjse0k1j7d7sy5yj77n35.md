# Build Explanation — Cycle 1940

## Build Binding
- Cycle: 1940
- Base SHA: 3448a01fddb052cf12c0b4150676d1e8b3dab4a1

## Summary
The compiled default of the cover-changed floor moves from `shadow` to `enforce`, so an uncovered changed function goes back to Build. The floor now reports a package whose tests fail once, not twice. The build and triage prompts carry an upstream digest scoped to their own upstream phases. The tdd prompt keeps the full digest through the shared helper.

## Rationale
At base the compiled default, its docs and its floor tests all said `shadow` (`go/internal/policy/budgets_recovery.go:210` at base). This change moves all three to `enforce`, so that the uncovered changed functions of cycles 1869 and 1870 are refused in the loop and not first in CI. Under `enforce` a changed package with a failing test got two refusals: the untagged unit run and the cover-changed run both reported it. The floor drops the cover-changed copy when the unit run already refused a package. It keeps the copy when only the `-tags integration` run fails, because then no other check reports the failure. The earlier digest carried every upstream section to every consumer. A per-edge table gives the build and triage prompts only their declared upstreams, which cuts prompt tokens.

## Changed Areas
- `go/internal/policy/budgets_recovery.go` — sets the compiled default stage of `CoverChangedPolicy` to `enforce`, so an absent policy block refuses an uncovered changed function.
- `go/internal/core/cover_changed_floor.go` — names the tests-failed headline as a constant and adds `unitRunRefuses`. When the tagged cover run fails, the cover-changed check reruns the untagged unit tests of the same packages and returns nothing if they also fail, because the changed-package floor already refuses that failure.
- `go/internal/phaseio/digest.go` — adds the `edgeUpstreams` table, `EdgeUpstreams` and `Handoffs.DigestFor`. The digest builder is split into `sectionLines`, `genericLines` and `capDigest` so the full and the per-edge digests share one path.
- `go/internal/phases/runner/handoffdigest.go` — adds `UpstreamDigestBlock`, the one renderer of the untrusted-data digest block.
- `go/internal/phases/build/build.go` — appends the per-edge digest for the build phase when the input is active.
- `go/internal/phases/tdd/tdd.go` — replaces the inline digest block with `runner.UpstreamDigestBlock`, still fed by the full `UpstreamDigest(0)`.
- `go/internal/phases/triage/triage.go` — appends the per-edge digest for the triage phase when the input is active.

## Design Decisions
The dedupe lives inside the cover-changed check, because `go/internal/core/build_floor_reviewer.go` is a protected control-plane path (ADR-0064) that a cycle cannot edit. The check reruns the same untagged unit run that the changed-package floor uses (`buildTagVisiblePackages` and `runBuildSelfCheck` through `buildSelfCheckRunner`), so both checks agree on the verdict. The rerun costs one extra untagged test run, and only on the failure path. If one package fails untagged and a second fails only under `-tags integration`, the cover line is dropped; the build is still refused, and the next round shows the second package. The edge table lives beside the digest code, so one file names every producer-to-consumer edge. A phase with no declared edge gets an empty digest, never the full one. The block text is unchanged, so prompt consumers read the same format.

## Verification
`go test -tags acs -count=1 ./acs/cycle1940` reports 10/10 PASS: predicates 007 and 009 pin the single refusal, 008 pins the integration-only refusal, and 010 pins the floor argument tests inside a fleet lane env. `TestCoverChangedFailures_LeavesAFailureTheUnitRunAlsoRefusesToTheChangedPackageFloor` covers the suppressed branch, and `TestCoverChangedFailures_AFailingTestRefusesAtEnforceAndOnlyWarnsAtShadow` covers the kept branch with a green unit run. The four floor argument tests clear `EVOLVE_FLEET_WIDTH`, so they pin exact arguments in any lane. `TestDefaultBuildFloorChecks_SeesCommittedBuilderWork` now expects the unnamed export to be refused by both the cover-changed and the naming floors.

## Compatibility
Operators who set `cover_changed.stage` in policy keep their value. Only an absent block changes, from shadow to enforce. The tdd prompt text is unchanged.

## Limitations
Only the build and triage prompts use the per-edge digest. The tdd prompt and the remaining consumers stay on the full digest; `edgeUpstreams["tdd"]` is declared but unused, and the rest is queued in the inbox.
