# Build Explanation — Cycle 1816

## Build Binding
- Cycle: 1816
- Base SHA: 1058e13fb2e09b1716fa90f7d78dc618b301f88d

## Summary
The effort placement law now reserves both premium rungs, max and ultra, for deep/top profiles. The law is a table-tested Specification, `violatesPremiumPlacement`, and the real-tree guard takes its verdict from it. Two prose homes that restated the codex deep-tier model and rung history now point at the tier table and the guard instead.

## Rationale
The codex manifest maps `ultra` above `max`, but the old guard only looked at profiles whose effort was exactly `max`. A balanced profile at `ultra` passed every guard while raising spend on the cheapest phases. Because no tracked profile uses a premium rung today, the guard's assertions never ran; a table test over the extracted Specification makes the law execute on every run regardless of the live corpus.

## Changed Areas
- `go/internal/profiles/effort_defaults_test.go` — replaces the literal `maxEffortRung` filter with `premiumEffortRungs` {max, ultra} and the Specification `violatesPremiumPlacement`, adds the table test `TestViolatesPremiumPlacement`, and renames the guard to `TestPremiumEffortOnlyOnDeepOrTopProfiles`, which now reports every violator through the Specification.
- `go/internal/profiles/loop_unblock_contract_test.go` — the retrospective routing messages no longer restate the deep-tier model history (the old message contradicted itself); they point at `bridge/manifests/codex-tmux.json`.
- `docs/operations/runtime-reference.md` — the Deep-tier family arrangement row states current state only, drops the dated rung and model history that CHANGELOG.md already records, and names the placement guard and its Specification.
- `docs/explain/builds/cycle-1816-01m4930vd6g2shvy3nzevcqdrk.md` — this explanation document.

## Design Decisions
The premium set is a literal two-entry map rather than a top-N derived from the manifest: deriving it would couple a cost law to manifest ordering for no current benefit, and the set changes only when the codex ladder does. An unset tier counts as non-deep/top, matching dispatch, where it resolves to balanced. The Specification lives in the test file because its only caller is the guard; production code never reads it. The guard drops its zero-match log, since the table test now proves the law executes.

## Verification
`go test -count=1 ./internal/profiles/...` passes. The cycle predicates in `go/acs/cycle1816` pass: they probe the Specification with a 16-case overlay table, run the guard over shadow profile trees (violators fail and are named, compliant trees pass, a forced-false Specification makes the guard pass), and confirm the table test kills both constant mutants.

## Compatibility
No profile, manifest or runtime behavior changes; every tracked profile already complies. The guard's old name, `TestMaxEffortOnlyOnDeepOrTopProfiles`, survives only in historical documents.

## Limitations
A new rung added above ultra must be added to `premiumEffortRungs` by hand; nothing derives it from the manifest.
