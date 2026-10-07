# Build Explanation — Cycle 1817

## Build Binding
- Cycle: 1817
- Base SHA: 2c3c41442d4c065c6b4748f8b0b7185c4a005e62

## Summary
The effort placement law now reserves both premium rungs, max and ultra, for deep/top profiles. The law is a table-tested Specification, `violatesPremiumPlacement`, and the real-tree guard `TestPremiumEffortOnlyOnDeepOrTopProfiles` gets its verdict from it. Two prose homes that restated the codex deep-tier model and rung history now point at the tier table and the guard instead. This cycle resumes cycle 1816's preserved work. That cycle's audit never ran because the audit launch crashed (claude exited rc=1), not because the work had a defect. This cycle re-verifies the work against its own predicates.

## Rationale
The codex manifest maps `ultra` above `max`, but the old guard only checked profiles whose effort was exactly `max`. A balanced profile at `ultra` passed every guard while raising spend on the cheapest phases. No tracked profile uses a premium rung today, so the old guard's assertions never ran. A table test over the extracted Specification runs the law on every test run, whatever the live corpus holds. Resuming the preserved diff costs less than rebuilding it, and the 1816 failure carried no code defect to fix.

## Changed Areas
- `go/internal/profiles/effort_defaults_test.go` — replaces the literal `maxEffortRung` filter with `premiumEffortRungs` {max, ultra} and the Specification `violatesPremiumPlacement`. Adds the table test `TestViolatesPremiumPlacement`. Renames the guard to `TestPremiumEffortOnlyOnDeepOrTopProfiles`, which now reports every violator through the Specification.
- `go/internal/profiles/loop_unblock_contract_test.go` — the retrospective routing messages no longer restate the deep-tier model history. The old message contradicted itself (sol per 2026-09-10, astra since 2026-09-09). The messages now point at `bridge/manifests/codex-tmux.json`.
- `docs/operations/runtime-reference.md` — the Deep-tier family arrangement row now states current state only. It drops the dated rung and model history that CHANGELOG.md already records, and names the placement guard and its Specification.
- `.evolve/evals/premium-rung-placement-law-and-narrative-dedup.md` — the task's eval. It caps the score unless the Specification and its table test exist, the guard and the package pass, the satellite no longer restates the model history, and the doc row names the guard.
- `go/acs/cycle1817/predicates_test.go` — this cycle's TDD-authored predicates. They probe the Specification with an overlay table and run the guard over shadow profile trees. They also check that the guard's verdict and the table test both depend on the Specification, by swapping in constant mutants.
- `docs/private/research/archived-2026-10-07/superseded-predicate-packages/cycle1816/predicates_test.go` — cycle 1816's predicate package, which the cycle-1817 package supersedes. It is archived outside the module so the stale cycle never runs as a live predicate.
- `docs/private/research/archived-2026-10-07/unshipped-build-explanations/cycle-1816-01m4930vd6g2shvy3nzevcqdrk.md` — cycle 1816's explanation, which never shipped. It is archived because explanation documents are cycle-owned and immutable, so this cycle publishes its own.
- `docs/explain/builds/cycle-1817-01m4a132jgtazhx2yqka3acw8p.md` — this explanation document.

## Design Decisions
The premium set is a literal two-entry map, not a top-N derived from the manifest. Deriving it would tie a cost law to manifest ordering for no current benefit, and the set changes only when the codex ladder does. An unset tier counts as non-deep/top, which matches dispatch, where an unset tier resolves to balanced. The Specification lives in the test file because the guard is its only caller; production code never reads it. The guard drops its zero-match log, because the table test now proves the law runs.

## Verification
`go test -count=1 ./internal/profiles/...` passes, and `gofmt` and `go vet` are clean. All three predicates in `go/acs/cycle1817` pass:
- The Specification gives the right verdict for all 16 overlay cases.
- The shadow-tree guard fails and names every violator, and passes a compliant tree.
- With the Specification forced to false, the guard passes. With it forced to false or to true, the table test fails.

## Compatibility
No profile, manifest or runtime behavior changes; every tracked profile already complies. The guard's old name, `TestMaxEffortOnlyOnDeepOrTopProfiles`, now appears only in historical documents.

## Limitations
A new rung added above ultra must be added to `premiumEffortRungs` by hand; nothing derives it from the manifest.
