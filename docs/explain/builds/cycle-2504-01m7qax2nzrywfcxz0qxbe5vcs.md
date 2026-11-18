# Build Explanation — Cycle 2504

## Build Binding
- Cycle: 2504
- Base SHA: ebf677e65020027f6ed3b4fab1ecf1a39301a104

## Summary
The testaudit detector no longer flags two valid bridge tests. Two weak bridge tests now assert real outcomes. The four tautology findings of `internal/bridge` leave the report.

## Rationale
Two findings were detector false positives. One test builds a key from rows that production decoded. The other test hands a recorder to the code under test. A change in the callers can only hide the defect, so the detector is fixed. Two findings were real weak tests: one asserted nothing, and one asserted a struct that it built itself.

## Changed Areas
- `go/internal/testaudit/rules.go` — a composite literal is own setup only when no value in it comes from a range variable or a non-literal definition. A literal variable that is assigned into a field or element of another variable is no longer own setup.
- `go/internal/testaudit/rules_branches_test.go` — one table case covers the derived key, the handed-out recorder and the true positive.
- `go/internal/bridge/engine_concurrent_launch_test.go` — the test collects each launch outcome and asserts the bad-flags refusal, so a Launch that swallows its outcome fails.
- `go/internal/bridge/apicover_named_test.go` — the doctor test drops the self-built failing probe and keeps the typed read of the real deep probe.
- `go/acs/cycle2504/predicates_test.go` — predicates of the TDD phase for this task.
- `.evolve/evals/test-audit-t6b-tautology-bridge.md` — eval graders of the task.

## Design Decisions
The detector keeps one pass over the test body. It tracks a set of derived names next to the set of literal names. This keeps the rule local and adds no setting.

## Verification
The ACS predicates TestC2504_001 to TestC2504_010 pass. The mutant predicates compile the bridge tests against a changed production file and see each rewritten test fail. The testaudit package tests pass.

## Compatibility
No exported symbol changes. Six bridge tests fail in `go test -count=1 ./internal/bridge` on this change: TestSeatJourney_ANoTokenHostGivesTheStoredShapeAndTheRecordSaysStored, TestCodexHeadlessManifest_PointsAtFamilyTable, TestLaunch_Codex_ResolvesTierViaFamilyManifest, TestCodexTmux_LaunchCmd_ModelMapAndMarker, TestLaunchArgs_Codex_ModelMap and TestAgyManifestTierModelsAreNamesAgyAccepts. They are unrelated to this task.

## Limitations
A literal variable that is read by a helper call is still dropped as escaped, as before.
