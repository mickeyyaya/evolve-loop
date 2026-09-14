# Build Explanation — Cycle 1673

## Build Binding
- Cycle: 1673
- Base SHA: c6bb682cb2181c048b8afbb04804ea0803f32ff1

## Summary
Lane shipping now runs the same repo-wide Go selection as CI against the tree the lane actually merges from, and each dispatch wave checks the fetched `origin/main` commit for completed failing check-runs before any lane launcher is initialized.

## Rationale
Using `go test -count=1 ./...` is the acceptance contract's simplest CI-equivalent option and avoids a fallible reverse-dependency resolver. Breadth alone is not sufficient: a fleet lane ships from its own worktree, so a scanner rooted at the project root would grade pre-merge main and charge a green lane for an unrelated RED, which the contract forbids. Rooting the scan at the lane worktree — with a fallback for sequential ships that carry no worktree — is the smallest change that makes the widened selection mean what the contract says. GitHub's check-runs response exposes separate `status` and `conclusion` fields, so the boundary halts only on completed failing conclusions while API or transport failures remain explicit warnings ([GitHub REST API documentation](https://docs.github.com/en/rest/checks/runs#list-check-runs-for-a-git-reference)).

## Changed Areas
- `go/internal/phases/ship/ship.go` — resolves the repo-contract gate's scan root at the sole production caller, preferring the request's lane worktree and falling back to the project root when it is empty, so fleet lanes are graded on their own tree while sequential ships keep their existing behavior.
- `go/internal/phases/ship/repocontract.go` — widens the scanner selection to `./...`, preserves new-test attribution by running that backstop first, and keeps the existing real-RED versus infrastructure classification; both the module run and the staged-added-test discovery follow the root the caller supplies.
- `go/internal/phases/ship/repocontract_test.go` — adds production-gate regression coverage for the CI-equivalent selection, an untouched tracked package failure, and the worktree-versus-project-root routing including the empty-worktree fallback, while strengthening the prior modified-test contract.
- `go/internal/phases/ship/repocontract_addedtest_defects_test.go` — updates attribution expectations from the retired fixed package pack to the repo-wide scanner.
- `go/cmd/evolve/cmd_loop_wavesync.go` — queries the fetched `origin/main` SHA's latest check-runs and distinguishes red, green, and unavailable outcomes.
- `go/cmd/evolve/cmd_loop_wavesync_test.go` — injects a hermetic `gh` subprocess and covers red halt, green proceed, unavailable warning, and pre-dispatch return behavior.
- `go/cmd/evolve/cmd_loop_window.go` — maps the typed red-main error to the dedicated `main_ci_red_halt` stop reason before fleet configuration.
- `.evolve/evals/lane-ship-gate-lands-red-main-package-scoped-tests.md` — records the TDD-owned score caps for the lane contract.
- `go/acs/cycle1673/predicates_test.go` — supplies the TDD-owned adversarial acceptance predicates used by the native ACS suite.
- `go/acs/cycle1673/predicates_round2_test.go` — supplies the TDD-owned predicates that pin worktree-rooted scanning, the project-root-only false-RED direction, the empty-worktree fallback, and staged-addition discovery.

## Design Decisions
The ship gate retains one scanner implementation and only changes its selection and its root; root resolution stays four inline lines at the existing caller rather than a new helper, because there is exactly one production call path and no second consumer. The added-test backstop stays because it provides more specific attribution, and it inherits the same resolved root so scanner breadth and staged-addition discovery cannot disagree. The wave boundary uses the existing GitHub CLI authentication and the existing `LOOP_HALT` signal, with an error sentinel separating red main from history divergence. Scanning both roots was rejected: it would re-introduce exactly the false RED the contract prohibits. No new package, exported symbol, policy knob, or environment flag was added.

## Verification
Focused binding tests exercise the real ship gate and wave-boundary caller through `Phase.Run`, using distinct project-root and worktree fixtures so the routing claim is observable rather than assumed; the full touched packages pass, and the native cycle-1673 ACS suite reports zero red predicates. The repo-wide Go suite passes except for two untouched real-tree tests whose writes under `.evolve/profiles` are denied by the Builder profile; the affected packages pass in the touched-package slice and ACS.

## Compatibility
Existing ship error codes, one-retry infrastructure classification, scan-log persistence, divergence handling, and added-test attribution remain intact. A ship request without a worktree scans the project root exactly as before. GitHub API unavailability is fail-open with a visible warning, matching the acceptance contract's false-RED constraint.

## Limitations
The boundary does not wait for queued or in-progress checks; when no completed check-run is visible it warns and proceeds, because absence of completed evidence is not proof that main is red. The gate grades the lane tree as staged, so an unstaged working-tree edit is invisible to it.
