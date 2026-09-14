# Test gates and CI implementation — 2026-09-14

Scope: S1 and the ready CI work in S3 from the [test-refactoring design](../architecture/test-refactoring-design-2026-09-14.md). Base revision: `80b348e6943b08b3bfc4e2d30f06f42db69c1db9`. Implementation branch: `fix/test-gates-ci-2026-09-14`. This report records local implementation evidence; the integrating operator owns full-tree checks, sanctioned commit/ship, GitHub validation and merge.

## Issue

`make cover-strict` and `make apicover-enforce` could report success after a real Go assertion failed. Go still emits coverage on failure. The strict recipe retained the downstream `sed` status, and the API recipe continued through a failed child command to a successful API inspection. Missing manifests and an invalid coverage evaluator could also pass. A final manifest row without a newline was skipped; independent review additionally reproduced a directory accepted as the strict manifest.

Release's inline test job selected fewer packages and gates than ordinary CI. `test-all` omitted the compound `e2e && evolve_test_phases` cases. Landing had existing tests but no workflow test job. Go workflow path filters omitted consumed release/workflow configuration.

## Gap

The existing gate tests exercised coverage helpers, not the real Make process exit and complete tool chain. Positive coverage was accepted as execution success. Coverage/API thresholds alone did not prove that the intended tests executed.

CI and release maintained separate execution recipes. The repository's real release-configuration tests could be broken without triggering the Go workflow. The initial workflow test's substring check also could not distinguish a correct pull-request exclusion from its inverted condition; an independent review caught this and the oracle was strengthened before integration.

## Solution

- Strict coverage checks each Go process exit before parsing coverage, retains failure output, validates the numeric measurement/floor, rejects evaluator errors, requires a readable regular manifest and processes its final unterminated row. The existing strict package thresholds remain unchanged.
- `apicover-enforce` stops on failed measurement. The shared `apicover-check` target validates an existing profile, stops on converter/list failures, rejects missing package directories and passes quoted directories to the real API checker. Both ordinary CI and standalone enforcement consume this target.
- Test-only ACS directories remain in API inspection but are excluded from ordinary measurement, matching CI's artifact-independent runtime scope. An ACS-only enrollment uses an empty measurement profile and must still pass real API inspection; production exports cannot obtain executed coverage from that profile. An explicitly empty enrollment retains its existing successful no-op behavior. No artifact-dependent predicate runs to manufacture ordinary coverage.
- `test-integration` selects every buildable non-ACS package using the integration tag, retains race detection and `-count=1`, and creates `coverage.txt`. The Go workflow calls this shared recipe. `test-all` composes integration, existing no-race E2E with both required tags and its 45-minute timeout, then durable ACS.
- Release invokes the local reusable Go and general CI workflows on its tagged revision. Publication requires both calls to succeed, so it inherits Linux/macOS tests, strict/API gates, plugin validation and durable ACS. Full checkout history is preserved. Existing post-publication verification and failure demotion remain.
- Go workflow filters now include `.goreleaser.yml` and all workflow definitions. Existing Linux/macOS Go 1.23 lanes remain; a Linux Go 1.27 lane is added. Release binaries and landing use Go 1.27. The module's Go 1.23 language baseline is unchanged. Toolchain suitability still requires the new remote matrix to pass before integration is accepted.
- Landing now tests and vets its separate module before building, including pull requests. Pull-request runs build the site but cannot configure/upload/deploy Pages. Independent mutation checks inverted all three exclusion conditions and verified that the workflow oracle rejects them.

The existing 19 strict floors and the API enforcement manifest are unchanged. The general 85% sweep stays advisory; its label now identifies function-level warnings honestly. This change does not turn an aggregate percentage into a package floor. Actionlint identified two pre-existing warning-block issues: unused bookkeeping was removed and API-report directory arguments were quoted without changing advisory behavior.

The maintained `go/docs/testing.md` guide now matches the shared recipes, compound tags, release/landing lanes and actual hard-versus-advisory coverage behavior. It removes stale timing/CI-neutrality claims, identifies the isolated bridge diagnostic correctly, and makes existing acceptance-selector preservation explicit.

`apicover-check` deliberately relies on its caller's preceding successful, fresh measurement. `apicover-enforce` and the ordered CI steps establish that sequencing. This target does not claim a persisted SHA-bound provenance receipt; no new attestation framework was added.

## Permanent regression contracts

The new tests live in the existing `go/internal/ciparity` package. Workflow topology tests are untagged; real Make/Go subprocess tests use the integration tag. Each subprocess fixture owns an isolated temporary module and builds the actual API checker into a temporary binary directory. It never recursively runs this repository's suite. The harness rejects context cancellation as a harness failure, preventing a timeout from satisfying an expected-failure assertion.

| Contract | Proof |
|---|---|
| Test failure cannot become coverage success | Both real Make targets reject a failing assertion with 100% coverage; adjacent passing controls still pass |
| Tool failures cannot be hidden by plausible output | Real test, converter and package-list commands run, then a process-boundary wrapper returns nonzero; enforcement rejects each |
| Coverage floor remains enforced | Partially covered passing test fails its unchanged 100% fixture floor; exact-floor controls pass |
| Manifest handling is fail-closed | Missing manifests, a directory manifest, invalid floor/evaluator, and final row without newline |
| Profile prerequisites are real | Missing and malformed profiles fail at the real Go coverage converter |
| ACS remains artifact-independent | Mixed runtime/ACS, ACS-only and empty enrollments preserve their distinct API inspection contracts |
| Selection includes the required runtime | Deliberately failing sentinels execute in internal, cmd, pkg, component, fixtures, trustkernel, integration and examples; ACS sentinel stays excluded |
| Compound tags execute | A real `e2e && evolve_test_phases` sentinel fails through `test-all` |
| Release uses the same gates | Structural graph checks pin reusable calls, success dependencies, full history and the shared integration/API/strict/E2E recipes |
| Workflow input routing is explicit | Required push and pull-request events exist and select consumed configuration |
| Pages cannot publish PRs | Required landing module tests precede build; exact deploy/configure/upload exclusions reject an inverted-condition mutation |

## TDD evidence

All implementation behavior changes followed observed assertion failures. Initial YAML test scaffolding was corrected before the recorded semantic RED run; parser failures were not counted as defect proof. Passing controls and existing package tests were retained.

From `go/`, the initial RED command was:

```sh
go test -count=1 -tags integration ./internal/ciparity \
  -run 'TestMake|TestRelease_Requires|TestGoWorkflow|TestLandingWorkflow' -v
```

Selected actual RED output:

```text
cover-strict: error = <nil>, want failure true
cover-strict: ./internal/probe: 100.0% >= 100% ok
apicover-enforce: error = <nil>, want failure true
summary: 1 exported, 1 covered, 0 uncovered, 0 false-green, 0 ignored
gate accepted failing test tool
gate accepted failing tool tool
gate accepted failing list tool
release test uses ""; want shared go.yml validation
publication does not require validate: [test]
```

Additional RED commands used the same package and flags with these selectors. The corresponding receipts retain the complete command output:

| Selector / control | Receipt |
|---|---|
| Initial gate, selection, release and landing regressions above | `/tmp/evolve-test-gates-red.log` |
| `TestMakeCoverageGates_RejectMissing\|TestMakeCoverStrict_Rejects\|TestRelease_Requires\|TestGoWorkflow\|TestLandingWorkflow` | `/tmp/evolve-test-gates-red-additional.log` |
| `TestGoWorkflow_UsesShared` | `/tmp/evolve-test-gates-red-wiring.log` |
| `TestMakeAPIEnforce\|TestMakeCoverStrict_ChecksFinal` | `/tmp/evolve-test-gates-red-preservation.log` |
| `TestReusableGoValidation` | `/tmp/evolve-test-gates-red-history.log` |
| `TestMakeAPIEnforce_Accepts\|TestMakeAPICheck_Rejects` | `/tmp/evolve-test-gates-red-testonly.log` |
| `TestMakeCoverStrict_RejectsManifestDirectory\|TestMakeFixture_CancellationFailsHarness` | `/tmp/evolve-test-gates-red-review.log` |
| Temporarily invert all Pages guards in the isolated worktree, run `TestLandingWorkflow`, restore in `finally` | `/tmp/evolve-test-gates-red-prguard-mutation.log` |

The directory and cancellation controls initially produced:

```text
gate accepted a directory as its manifest
read error: 0: Is a directory
canceled fixture was accepted as a test result: <nil>
PASS
```

The inverted PR mutation exited 1 and named all three incorrect guards. That was an isolated mutation of an already-correct workflow condition to test oracle sensitivity; the actual production guards were restored and were never claimed as a discovered inverted-guard bug.

## GREEN and validation

```sh
cd go
go test -race -count=1 -tags integration ./internal/ciparity -v
go vet ./internal/ciparity
```

Final result: **32/32 top-level tests and 21/21 subtests PASS, no skips, no regression; 21.943 seconds**. This includes existing package cases and the new contracts. Receipt: `/tmp/evolve-test-gates-green.log`. The three independent-review controls also passed in `/tmp/evolve-test-gates-green-review.log`.

```sh
cd landing
go test -race -count=1 ./...
```

Result: **6/6 packages PASS**, recorded in `/tmp/evolve-test-gates-landing.log`. No paid provider calls were made.

From the repository root:

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
git diff --check
```

Both pass. Actionlint validates all four workflows, including reusable-workflow permissions/dependencies and shellcheck. Receipt: `/tmp/evolve-test-gates-actionlint.log` (empty successful output). The temporary tool invocation added no repository dependency. Modified Go tests were formatted with `gofmt`.

## Full local integration checks

The integrating operator ran full formatting and `go vet ./...`, then `go test -race -count=1 -p 4 -parallel 4 -json -coverprofile=... ./...`: **232 passing packages, 16,229 passing test/subtest events, zero failures**, with the same fourteen test skips as baseline. `make build` passed. Logs: `/tmp/evolve-ci-full-{format,vet,test,build}.log`.

`make cover-strict` passed all nineteen unchanged 100% floors. The first full `make apicover-enforce` correctly exited nonzero after four native tmux startup cases failed; it did not proceed to API inspection. The retained log is `/tmp/evolve-ci-apicover-enforce.log` (SHA-256 `d2c01c4112061851c142a4c9748e4dc976ac3754e70abe2391f161c7e9c05f73`). Bridge and sandbox sources are unchanged. Independent diagnostics ran all four cases on baseline and candidate, each with fresh ambient-shell and controlled-shell tmux servers: **16 passes, zero skips**. The failure was not reproduced, so its exact cause remains unresolved; neither a shell-specific cause nor a product fix is claimed. No live server settings, assertions or timeouts were changed.

After those diagnostics, one full run with explicitly bounded process concurrency succeeded:

```sh
GOMAXPROCS=4 GOFLAGS=-p=4 make apicover-enforce
```

Result: **218 measurement packages PASS; all 231 API inspections PASS; 4,077 exports covered, zero uncovered or false-green exports**. The thirteen test-only ACS inspections remain distinct from measurement packages. This run generated its own fresh profile; the failed run's profile was not accepted as successful execution. Receipt: `/tmp/evolve-ci-apicover-enforce-bounded.log`. The successful rerun does not erase the initial startup failure or establish its root cause. The exact isolated diagnostic commands and logs are in `/tmp/evolve-tmux-env-diagnostic-2026-09-14/report.md`.

## Remaining verification and limits

The remote Linux/macOS/Go-version matrix must pass on the final revision before merge. Local checks used Go 1.27.1 on macOS; this report does not claim a local Go 1.23 or Linux result, new release publication, or an executed remote reusable release run. No production Go implementation or existing test body was changed in this slice. No repository ruleset, branch protection, release artifact, or GitHub account setting was changed.

The wider design's unfiltered aggregate required check, expanded structured-result/provenance artifacts, global package-coverage ratchet, broad mutation campaigns and remaining harness work are separate follow-ups. This implementation does not imply those have been completed. In particular, a standalone existing coverage profile is not a fresh execution receipt.
