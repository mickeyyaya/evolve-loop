# Required CI results implementation — 2026-09-14

The new `required.yml` always schedules a stable `CI required` result for pull requests and the existing main/coverage-branch push events. It rejects unsuccessful routing, failed or skipped required jobs, missing outputs and malformed route decisions. Only an explicit successful path decision permits an optional suite to be skipped. The [design](test-ci-required-results-design-2026-09-14.md) was written before workflow implementation; this report records the implementation, behavioral RED evidence, fault controls and preservation limits.

Base: `3a972e20ed55ce7ed9f422843a5ca486354e685f`. Implementation worktree: `dev/test-ci-required-results-2026-09-14`, branch `fix/test-ci-required-results-2026-09-14`. The implementing agent did not commit, push, merge or modify remote protection. Local validation used Go `1.27.1`, Darwin arm64. Remote Ubuntu/Go 1.23/Go 1.27 execution remains a required post-publication check. The worktree was parked uncommitted and salvaged on 2026-09-28; [Port 2026-09-28](#port-2026-09-28) records what changed on the way onto `main`. The `/tmp` receipts named below were not preserved and remain historical references only.

## Issues and solutions

| Gap | Concrete implementation | Preserved contract |
|---|---|---|
| Workflow path filters omit a stable required status or leave it pending | Unfiltered PR trigger, local Git path routing, final `always()` job depending on every caller in [`required.yml`](../../.github/workflows/required.yml) | Existing push branches, manual entry points and actual module suites |
| A caller can be successful while an inner required job was skipped | Native `job.status` → job output → reusable-workflow output; aggregator checks both caller and child results | Plugin validation, durable ACS, Go matrix and landing tests remain required when selected |
| Unknown/new configuration can silently miss a narrow filter | Unknown paths select both modules; only Markdown under reports/research/private may explicitly skip both | Skills and agents select Go; landing/explanations select landing; mixed paths union the requirements |
| Rename detection can hide an old executable path moved into a documentation path | `git diff --no-renames --name-only -z` and real Git history tests | Deletes and either side of renames remain observable; whitespace/newline filenames remain intact |
| Shell invocation context can defeat implicit `errexit` assumptions | Explicit error branches for invalid revisions, merge-base and diff, and every required result | Git failures never publish skip decisions or become green full-suite routing |
| PR landing validation and deployment can drift | Shared read-only [`landing-validation.yml`](../../.github/workflows/landing-validation.yml) tests, vets, renders and uploads the complete artifact; Pages consumes that run's artifact | Existing Pages publishing permissions, target paths and three PR guards; no PR deployment |
| A required-check ruleset would break the current direct-main cycle publisher | Source-backed native PR publication prerequisite in the design | No bypass, approval-count invention or premature ruleset activation |

Existing `go.yml` and `ci.yml` retain reusable and manual entry points; automatic push/PR runs move to the orchestrator. Their test/build command bodies are unchanged. `release.yml` is unchanged and still requires its tagged-revision `test` and `validate` reusable calls before publication. Go matrix entries remain Ubuntu 1.23, macOS 1.23 and Ubuntu 1.27, with `fail-fast: false`. `make test-integration`, no-race `make test-e2e`, API hard gates, strict per-package statement floors, advisory reporting and artifacts remain in the Go job. This slice changes no Make recipe, coverage threshold, Go module dependency or existing production test body.

The Go output is the matrix job's native output, while `needs.go.result` is also required to succeed. A successful matrix member's output cannot excuse a failed/cancelled caller. Skipping an entire optional caller is allowed only with route `false`, caller `skipped`, and an empty inner output. Plugin and ACS never receive a docs-only exception.

## TDD and fault evidence

Permanent default-tier graph assertions live in [`required_workflow_test.go`](../../go/internal/ciparity/required_workflow_test.go), [`routing_workflow_test.go`](../../go/internal/ciparity/routing_workflow_test.go) and the extended existing [`workflows_test.go`](../../go/internal/ciparity/workflows_test.go). Actual Bash/Git execution lives in integration-tagged [`required_result_integration_test.go`](../../go/internal/ciparity/required_result_integration_test.go) and [`routing_workflow_integration_test.go`](../../go/internal/ciparity/routing_workflow_integration_test.go). Tests extract the exact YAML scripts and supply independent expected outcomes; they do not duplicate the routing implementation.

The routing fixture uses isolated temporary repositories, a pinned `main` branch, fixed identity, disabled signing/hooks and explicit commits/ref reads. The existing `test/fixtures.WorkspaceBuilder.WithGitInit` was inspected: it supplies an evolve workspace plus init/identity but no branch pinning, hook/signing isolation, commit or ref helpers. The narrow routing fixture retains those specific required semantics without adding a dependency on the broader evolve workspace builder. This is a fixture-contract difference, not an asserted import cycle. The shell helper has a ten-second context deadline and fails the test directly if the deadline expires, so a timeout cannot satisfy an intentional-failure oracle. Port: `main` now owns git test fixtures in `internal/gittest`, and the raw-git ratchet (`internal/rawgitratchet`) rejects a new raw `git init` in a test. The ported fixture builds each repository with `gittest.Fixture` (branch `main`, identity, background maintenance off, retried teardown); the local `routingGit` helper keeps the sanitized environment and disabled signing and hooks for the fixture's own commits and ref reads.

Both fixture Git and the extracted workflow script remove inherited `GIT_*` routing variables before executing, then retain explicit workflow event/output bindings. A final independent review identified this isolation gap; the two helpers now reuse one local integration-only environment function. No exported shared sanitizer exists at this baseline (`treefence.gitEnv` is private and covers a narrower variable set). Eight executable controls include clean positives and redirected repository, index and object-directory cases for each helper. They stage real files and assert the selected repository remains local, a separate harmless repository's index bytes are unchanged, and no fixture blob is written into its object directory. The first RED demonstrated those external temporary writes; the fix does not merely assert that environment strings were filtered.

| Stage | Actual evidence |
|---|---|
| Initial compiled RED before the new workflows existed | `/tmp/evolve-required-ci-red.log`: nine failed top-level assertions, including missing required graph and duplicated old automatic triggers; no compilation failure |
| First executable draft rejected by the result/revision cases | `/tmp/evolve-required-ci-red-runtime.jsonl`: 16 failed test events; failed dependencies and invalid revisions produced `pass=true want false` |
| Child-result checks added before exposing native outputs | `/tmp/evolve-required-ci-red-child-results.jsonl`: 27 failed test events; skipped/failed child jobs were accepted by the earlier caller-only implementation |
| Native output indirection pinned before its YAML change | `/tmp/evolve-required-ci-red-job-outputs.log`: one failed top-level test with eight missing-binding assertions |
| Review-added Git failures with `set +e` before explicit handling | `/tmp/evolve-required-ci-red-git-errors.jsonl`: four failed cases plus their parent; missing base/head, merge-base error and missing Git all returned `err=<nil> output="go=true\nlanding=true\n"` |
| GREEN before tier relocation | `/tmp/evolve-required-ci-green-review.jsonl`: 147 passing events (29 top-level, 118 subtests), no failures or skips |
| Review-added ambient Git routing controls before helper isolation | `/tmp/evolve-required-ci-red-git-environment.jsonl`: two clean controls pass, six poisoned cases plus parent fail through wrong repository and external index/object writes; no compile failure |
| Same ambient Git controls after helper isolation | `/tmp/evolve-required-ci-green-git-environment.jsonl`: eight cases plus parent pass, no failures/skips |

The executable failures exposed a real shell issue: Bash 3.2 did not abort this compound conditional under `-e`:

```bash
/bin/bash --noprofile --norc -e -o pipefail -c \
  '[[ failure == success && success == success ]]; echo incorrect'
```

It printed `incorrect`. The implementation now uses explicit `if ...; then ...; exit 1; fi` branches. The permanent Git-failure cases also disable `errexit` before the exact routing script, proving error handling independently of how a runner invokes Bash.

The reproducible fault runner is `/tmp/evolve-required-ci-fault-controls.py`; its invocation is `python3 /tmp/evolve-required-ci-fault-controls.py`. It copies only workflow/test/module inputs into isolated temporary trees, then runs `go test -tags integration -count=1 -json -run '^<selector>' ./internal/ciparity`. The final source is never mutated. All controls were rerun after test-tier relocation; `/tmp/evolve-required-ci-fault-summary.jsonl` records each exit and actual failing test name.

The final environment RED/GREEN command is `go -C go test -race -count=1 -tags integration -json -run '^TestWorkflowGitHelpers_KeepAmbientRoutingOutsideFixtures$' ./internal/ciparity`. For RED the outer launcher removed inherited `GIT_*` values before tests deliberately introduced their harmless redirects; GREEN used the same permanent cases with the repaired helpers. The RED fixture first received a syntax correction and macOS symlink canonicalization before the recorded behavior run; neither a compilation failure nor path-spelling mismatch is credited as the defect evidence.

| Isolated fault | Required observation | Result |
|---|---|---|
| Unmodified graph | All selected contract cases pass | Exit 0 |
| Go/skill/agent route changed to skip Go | Go paths, mixed paths, deletions and renames fail independent expected outcomes | Exit 1; ten failed subtests and parent |
| Accept arbitrary outcomes for a required suite | Caller/inner failure, cancelled, skipped, missing, neutral and pending cases fail | Exit 1; 24 failed subtests and parent |
| Enable rename detection | Source moved into docs can no longer satisfy expected Go route | Exit 1; rename subtest and parent |
| Bind ACS output to plugin validation instead | Static binding to the actual ACS job fails | Exit 1; one test |
| Allow the result step to continue after failure | Unconditional final-gate contract fails | Exit 1; one test |

No injected fault is credited solely for a compiler error, unavailable command or deadline. This targeted fault set proves the listed decisions; it is not a whole-repository mutation score.

## Final validation and scheduling

The salvaged worktree was parked before its final validation ledger (`/tmp/evolve-required-ci-full-final-results.jsonl`) was recorded, and that file did not survive. The port's validation is in [Port 2026-09-28](#port-2026-09-28). The earlier full default run passed with 16,436 passing events and 14 test skips; it preceded relocation of executable tests and is retained only as historical evidence.

Pinned workflow validation: `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`, exit 0, empty `/tmp/evolve-required-ci-actionlint-final.log`. No repository dependency or suppression was added. GitHub documents `jobs.<id>.result`, but this pinned validator models only `outputs` on a reusable job context; the tested `job.status` → job output → workflow output path avoids that validator mismatch without an extra aggregation runner.

Landing validation passed: `go -C landing test -race -count=1 -json ./...` produced 103 passing test events (94 top-level, nine subtests), no failures or skips across six packages; `go -C landing vet ./...` passed and `go -C landing run ./cmd/build` rendered 20 files. Receipts: `/tmp/evolve-required-ci-landing.jsonl` and `/tmp/evolve-required-ci-render.log`. No generated site files are part of this change.

| Orchestrator route | Executed runner jobs | Composition |
|---|---:|---|
| Both modules | 8 | route + plugin + ACS + three Go matrix jobs + landing + final result |
| Go only | 7 | route + plugin + ACS + three Go matrix jobs + final result |
| Landing only | 5 | route + plugin + ACS + landing + final result |
| Explicit documentation skip | 4 | route + plugin + ACS + final result |

These are graph-derived runner-job counts, not a wall-clock benchmark or a promise about the number of GitHub UI check rows. Required jobs run in parallel where dependencies permit. Main-push Pages and tagged-release jobs are separate existing delivery paths and are not included in this table. Native reusable calls can change prefixes on displayed matrix names; the required context is the fixed final job name `CI required`, to be verified on the first real run before rule activation.

## Enforcement state and remaining proof

Read-only remote inspection found no `main` branch protection and an empty repository/inherited ruleset list. Existing main checks were successful and published by GitHub Actions app `15368` ([Go run](https://github.com/mickeyyaya/evolve-loop/actions/runs/34811419489), [plugin/ACS run](https://github.com/mickeyyaya/evolve-loop/actions/runs/34811419511)). That proves the old checks' source; it does not prove execution of this new workflow. The parent must observe an actual successful new check, finish the documented native PR publication/runtime prerequisite, then preserve all settings while applying and reading back the supplemental rule. A read-only re-inspection on 2026-09-28 found the same state: no `main` protection and no rulesets.

The current cycle path locally integrates and directly pushes main; required pre-merge checks would reject that unvalidated commit and its unchanged fast-forward retry. The design supplies exact source references, TDD acceptance cases, expected-head merge semantics, lock/receipt/recovery requirements and rollout order. No external rule is activated by this implementation, and no future scheduled-fuzz command is guessed or stubbed here.

Workflow syntax tests and local shell execution cannot reproduce the hosted scheduler, GitHub permissions, every supported matrix environment or remote check publication. The first actual PR/push must confirm reusable output propagation, selected/skipped callers, `CI required`'s exact name and source, and preservation of release/Pages graph behavior. Full enforcement is not claimed until native publication compatibility and remote ruleset readback are complete.

## Port 2026-09-28

The worktree above was parked uncommitted. On 2026-09-28 it was ported onto `main` at `9db78031` on branch `ci/always-reporting-required-result`. No file under `.github/workflows/` or `go/internal/ciparity/` had changed between `3a972e20` and `9db78031`, so every workflow change applied as written. The test files changed only where `main`'s rules had moved, or where review found a gap:

| Reason | Change in the port |
|---|---|
| Git test fixtures come from `internal/gittest`; the raw-git ratchet rejects a new raw `git init` | `routingRepo` builds its repository with `gittest.Fixture`; `routingGit` no longer runs `init` and takes its identity from the fixture's config |
| Functions stay within 50 lines | The ambient-Git test's body moved into `assertGitStaysInFixture`, `redirectAmbientGit` and `stageThroughHelper`; the routing case table moved into `routingPathCases` over a named `routingCase` type |
| New Go code carries no comments | The one doc comment (on `workflowTestEnv`) was dropped |
| Review: `TestRequiredWorkflow_TriggersForConsumedConfiguration` was vacuous, because `required.yml` has no path filter and an empty filter counted as "selected" | Removed. Its intent (release configuration and workflow edits run the Go suite) is now four more executable routing cases: `.github/workflows/ci.yml`, `release.yml`, `landing-pages.yml` and `landing-validation.yml` join `go.yml`, `required.yml` and `.goreleaser.yml` |

Consumers of the old workflow names moved with the change: the `/evo:publish` skill watched `go`, `CI` and `release` runs by commit, and `go.yml` and `ci.yml` no longer start runs of their own, so it now watches `required.yml` and `release.yml`; the `/evo:release` skill and the `evolve release` advisory name `required CI`. `go/docs/testing.md` ("CI shape") describes `required CI`, the routing table with the tests each input feeds, the aggregator and the prefixed check names.

| Stage | Result |
|---|---|
| RED, default tier, final tests with `origin/main`'s workflows | 6 of 23 top-level tests failed: the four that read the absent `required.yml` or `landing-validation.yml`, `TestRequiredWorkflow_ReusableSuitesAreNotDuplicated` (four assertions: `go.yml` and `ci.yml` still ran on push and pull request) and `TestLandingWorkflow_TestsPullRequestsBeforeBuild` (Pages did not share landing validation) |
| RED, `-tags integration`, whole package | 11 of 42 top-level tests failed. The ambient-Git helper control passed (8 subtests and parent), as expected: it tests the fixture helpers, not the YAML |
| GREEN, default tier | 23 top-level tests and 6 subtests passed |
| GREEN, `-race -tags integration`, whole package | 42 top-level tests and 145 subtests passed, no skips |
| Fault controls on `required.yml`, restored after | Removing `if: ${{ always() }}`, routing `skills/` away from Go, accepting any Go-suite result, enabling rename detection, skipping every Markdown file, and skipping `.github/` each made the selected tests fail (1, 2, 13, 2, 2 and 7 failed test events) |

Workflow validation: `actionlint` is not installed on the porting host. Every workflow parsed with Python `yaml.safe_load`, and a graph check found every `needs` target and every local `uses` callee (each with `workflow_call`), `if: ${{ always() }}` on `CI required`, and each aggregator output bound to the child job's `job.status`. `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` over all six workflows, with shellcheck on `PATH`, exited 0. The landing module's commands from `landing-validation.yml` passed locally: 103 test events, vet clean, 25 rendered files.

Review notes kept as follow-ups rather than changed here: `go.yml`'s `result` output comes from a matrix job, so it reflects whichever leg wrote last; the three legs are covered by `needs.go.result`, which the aggregator also requires. `internal/gittest` passes the ambient `GIT_*` environment to its own git calls, which is why `workflowTestEnv` exists.

### Second review round, 2026-09-29

The code review (APPROVE-WITH-MINOR) and the architecture review (FIX_THEN_MERGE) named two surviving mutants and three smaller gaps. Each fix was shown red first; the mutant counts come from `go test -count=1 -tags integration ./internal/ciparity/`.

| Finding | Fix | Evidence |
|---|---|---|
| [HIGH] M1: a job added to `required.yml` but left out of `CI required`'s `needs` kept the result green; the dependency test pinned a literal list | `TestRequiredResult_WaitsForEveryOtherJob`: `needs(required)` must contain every other job key, read from the YAML | Before: M1 survived (exit 0, 187 pass events). After: HEAD passes; M1 killed by exactly that test |
| [HIGH] M2: widening the docs skip to `docs/*.md` passed every test, although Go tests read `docs/incidents/` Markdown | Routing cases `docs/architecture/note.md` and `docs/incidents/note.md` must run both suites | Before: M2 survived (exit 0, 187 pass events). After: HEAD passes; M2 killed by the two new subtests |
| [MEDIUM] Release gates read the newest run of any workflow, so a green `landing-pages` run could hide a red or running `required CI` | `ciparity.RequiredWorkflow`; `releasepreflight.defaultCIConclusion` and `ciwatch.NewGHFetcher` pass `--workflow` with it; both skills' pre-release checks query `--workflow required.yml` | A fake `gh` answering scoped queries with the required run and others with a green `landing-pages` run: both new tests red before, green after |
| [MEDIUM] The routing script nested five deep | Early exits; at most two levels | 43 + 147 integration tests pass; old and new scripts agree on all 18 side-by-side runs |
| [MINOR] The force-push trade-off was not stated | Stated in the design's routing contract and in `go/docs/testing.md` | — |
