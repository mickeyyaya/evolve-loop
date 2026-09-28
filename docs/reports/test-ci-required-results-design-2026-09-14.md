# Required CI results design — 2026-09-14

The branch should receive one stable `CI required` result on every pull request and protected-branch push. That result must reject missing, failed, cancelled, or unexpectedly skipped work; documentation-only changes may skip expensive suites only through an explicit successful routing decision. This slice preserves the Go matrix, coverage gates, durable ACS checks, landing test behavior, and tagged-release dependency graph.

Baseline: `3a972e20ed55ce7ed9f422843a5ca486354e685f`. Read-only GitHub inspection found `main` branch protection absent (HTTP 404 “Branch not protected”) and repository/parent ruleset listing `[]`. Current main's five checks passed; their source was GitHub Actions, application ID `15368`. The [current Go run](https://github.com/mickeyyaya/evolve-loop/actions/runs/34811419489) and [plugin/ACS run](https://github.com/mickeyyaya/evolve-loop/actions/runs/34811419511) establish current names/source, not proof of the new aggregator.

Port, 2026-09-28: this design was written in a worktree that was parked without a commit. It was ported onto `main` at `9db78031`. No file under `.github/workflows/` or `go/internal/ciparity/` changed between `3a972e20` and `9db78031`, so the graph below applies unchanged. A read-only re-inspection on 2026-09-28 again found no `main` branch protection (HTTP 404) and an empty ruleset list. The PR status list then showed the five checks `build + test (Go) (ubuntu-latest, 1.23)`, `build + test (Go) (macos-latest, 1.23)`, `build + test (Go) (ubuntu-latest, 1.27)`, `validate` and `acs-durable`. The source line references in "Required native publication prerequisite" are updated to `9db78031`.

GitHub documents that workflow path filtering can leave required checks pending, while dependent jobs may be skipped after failure. An unconditional final job using `always()` plus explicit dependency outcomes closes that scheduling gap. A skipped job alone is not evidence of successful testing. [GitHub required-check troubleshooting](https://docs.github.com/en/pull-requests/how-tos/merge-and-close-pull-requests/troubleshooting-required-status-checks).

## Workflow graph

`required.yml` becomes the single unfiltered pull-request/push orchestrator. It runs a small changes job, reusable `ci.yml` (plugin validation plus durable ACS), conditional reusable `go.yml`, conditional reusable landing validation, and `CI required` with all four as direct dependencies and `if: always()`.

`go.yml` and `ci.yml` retain manual/reusable invocation and their existing job bodies; their automatic triggers move to the orchestrator to avoid duplicate full Go runs. Release continues to call these two workflows directly, on its tagged revision, with its existing `test`/`validate` publication dependencies. The new orchestrator has read-only contents permission and invokes no deployment workflow.

Landing tests, vet, and rendering move to a reusable read-only workflow. The existing Pages workflow calls that validator, downloads the rendered artifact, and retains its configure/upload/deploy steps and permissions. PR validation moves to the unfiltered orchestrator; Pages remains push/manual. Independent main-push CI and Pages runs each validate their own artifact; no cross-run artifact trust or deployment sequencing redesign is introduced.

The aggregator requires route and reusable plugin/ACS success. For each optional suite, only `required=true/result=success` and `required=false/result=skipped` are acceptable. Absent output, invalid booleans, missing results, failure, cancellation, and unexpected success/skip combinations fail. Reusable workflows also expose their required jobs’ status through native job outputs (`job.status`) and `workflow_call` outputs; the aggregator checks plugin, ACS, Go matrix and landing job outcomes, so a successful reusable call cannot hide an inner skipped job. These outputs use GitHub’s documented [job/jobs contexts](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts#jobs-context), without adding another aggregation runner. Reusable workflow internals retain normal successful-dependency semantics and no `continue-on-error` exceptions. A routing failure cannot masquerade as a docs-only decision.

## Routing contract

Git computes changes locally after full-history checkout, avoiding GitHub API pagination/path-filter limits. PRs compare merge-base to the PR head; pushes compare the event's before/head commits. Both sides of renames are included by disabling rename detection. NUL-delimited names preserve spaces, tabs, and newlines. Missing/invalid objects or Git command failures fail routing. Empty diffs, initial pushes, and manual runs execute both modules.

| Changed paths | Go matrix | Landing validation |
|---|---|---|
| `go/**`, `skills/**`, `agents/**` | Run | Skip |
| `landing/**`, `docs/explain/**` | Skip | Run |
| `docs/reports/**/*.md`, `docs/research/**/*.md`, `docs/private/**/*.md` (including direct children) | Explicit skip | Explicit skip |
| Any other path, including `.github/**`, `.goreleaser.yml`, `.evolve/**`, plugin/config metadata, `install.sh`, README, architecture registries, non-Markdown research fixtures and unknown new surfaces | Run | Run |
| Mixed changes | Union of required suites | Union of required suites |

Plugin validation and durable ACS still run for every event, including documentation-only changes. The narrow docs allowlist avoids pretending all Markdown is non-executable input: skills, prompts and many configuration/architecture documents are consumed by existing tests. Expansion requires evidence and a routing regression. These are suite-selection decisions, never substitutes for runtime acceptance predicates.

No merge queue is configured, so this slice does not enable it or add a merge-group event. Enabling a queue later must add its required-check trigger first. No workflow-level branch/path filters apply to pull requests; pushes retain `main` and the existing `go-rewrite-phase-1` coverage branch. A commit-message workflow-skip directive can still suppress GitHub scheduling; the required check then remains absent/pending and must block merging rather than claiming a successful aggregator.

## TDD and preservation

Before workflow edits, add tests in existing `internal/ciparity`: default-tier unfiltered event/graph assertions; integration-tier execution of the exact route shell from YAML against temporary Git histories; integration-tier execution of the exact final result shell over a manually specified success/failure table. Cases include docs-only, every consumed path family, mixed paths, rename/delete, unusual filenames, invalid/missing revisions, empty diff, initial push and manual invocation. The result matrix covers both optional suites, failed/skipped/cancelled/missing required dependencies, malformed/missing route outputs and route failure. Preserve the existing matrix/Make commands, release dependencies, full-history checkout, per-module race settings and Pages guards through workflow contract tests. Run pinned actionlint, full default and integration tests, and vet; parent owns combined final validation and sanctioned shipping.

A planned fault control must invert a route or result decision and show the tests reject it. No test may pass solely because a subprocess deadline expired. Tests inspect actual selected behavior, not a second implementation of the routing algorithm.

## Branch-rule rollout (parent applies after merge)

GitHub rulesets can restrict a required check to an expected GitHub App. Add an isolated `main` ruleset requiring the exact observed `CI required` context from app `15368`, with strict up-to-date checks and no bypass actors. This supplements existing rules rather than replacing settings. Re-read protections and all inherited/repository rules immediately before applying; if concurrent settings appeared, preserve them. [GitHub available rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets), [rules REST API](https://docs.github.com/en/rest/repos/rules#create-a-repository-ruleset).

The provisional payload is:

```json
{
  "name": "Required CI result",
  "target": "branch",
  "enforcement": "active",
  "bypass_actors": [],
  "conditions": {"ref_name": {"include": ["refs/heads/main"], "exclude": []}},
  "rules": [{"type": "required_status_checks", "parameters": {
    "required_status_checks": [{"context": "CI required", "integration_id": 15368}],
    "strict_required_status_checks_policy": true,
    "do_not_enforce_on_create": false
  }}]
}
```

Apply only after reviewed CI implementation is merged, an actual green new check confirms the exact context, app, workflow path and tested revision, and native publication is compatible with enforcement as specified below. Inspect the active branch rules and a subsequent PR's required checks after application. Existing approval/merge-method/push/deletion settings stay untouched; no reviewer-count or bypass policy is introduced by inference.

The app restriction authenticates the check publisher, not arbitrary workflow contents: another workflow run by the same GitHub Actions app is not cryptographically distinguished by app ID alone. Source review and normal protection of workflow changes remain required. This slice does not claim an immutable workflow attestation or independent CI identity provider. No external configuration is changed by the implementing agent.

## Required native publication prerequisite

Activating the proposed rule against the current autonomous publisher would block normal cycles. This is a source-backed compatibility finding, not a reason to grant a bypass. At `9db78031`, [`atomicShip`](../../go/internal/phases/ship/gitops.go) (lines 36–55) lands from the tree that `landingTree` selects, which is the worktree path for `ClassCycle` (lines 61–82, the class test at line 66). [`worktreeShip.run`](../../go/internal/phases/ship/worktree_ship.go) holds the shared ship lock through commit/integration (lines 45–75, the lock at 51–57), and `integrate` first advances the local integration branch through [`Landing.Integrate`](../../go/internal/phases/ship/landing/integrate.go) (`git merge --ff-only`, line 39), then pushes it through [`pushWithRepair`](../../go/internal/phases/ship/gitops_landing.go) (line 42; `integrate` at worktree_ship.go lines 180–188). [`Landing.Push`](../../go/internal/phases/ship/landing/push.go) executes `git push origin <branch>` (line 72); its one fast-forward retry (lines 128–136) cannot produce a missing required check. Direct/manual publication in `shipDirect` (line 211) also pushes the current branch after committing ([`gitops.go`](../../go/internal/phases/ship/gitops.go), commit at line 297, push at line 307), and [`runPushOnly`](../../go/internal/phases/ship/pushonly.go) (line 66) reaches the same push helper (line 98). (At `3a972e20` these were gitops.go 65–69 and 357–370, and worktree_ship.go 166–183.) No PR publication/check/merge stage exists on these paths. A newly created local main commit has no remote pre-merge CI evidence, and the local integration branch can already be ahead when protection rejects its push.

The parent owns a separate native publication implementation before enforcement. Its concrete contract is:

1. Preserve host/worktree identity, audit-bound tree checks, sanctioned commit gates, and existing ship receipts. Publish the isolated cycle/source branch and create or resume its PR against the configured protected branch; do not advance shared main before GitHub accepts the merge.
2. Bind PR identity to repository, source/base branch, cycle/run and expected head. Require the exact successful `CI required` check from app `15368` on the applicable current PR head/test-merge revision. Missing, pending, skipped, cancelled, failed, wrong-app or stale-revision results do not authorize merge. PR creation and waiting are not successful ship completion.
3. Use an expected-head conditional merge operation without an administrator override. A changed base that invalidates strict checks requires the existing reconciliation/re-audit path and fresh CI; do not silently mutate an audited tree, force-push, or treat old green results as current proof.
4. Do not hold the shared integration lock while waiting for remote CI. After confirmed remote merge, reacquire the short integration lock, refresh local main, verify the actual landed commit/tree against the audited composition, and record binding, ledger and post-ship work exactly once. Retry/resume must find the same PR and handle an already merged PR without duplicating publication.
5. Cover stranded `--push-only`/recovery behavior and classify a required-check rejection as a publication precondition, not a retry loop that can never create the required evidence. Keep existing local/bare-repository behavior when the protected GitHub publication mode is not selected; use existing process/network seams for boundary fakes.

Strict TDD must first demonstrate the current forbidden direct-main push through the real `ClassCycle` orchestration, then cover pending/absent/wrong-source/stale checks, head/base races, cancellation, re-entry after merge, lock release while waiting, and refusal to finalize ship before the landed tree is verified. Boundary mocks may supply Git/GitHub responses; they must not replace the ship decision under test.

Rollout order is CI graph merge and actual green check, native publication implementation/review and compatibility verification, a freshly built compatible runtime installed at a safe batch boundary, then the parent's ruleset write and readback. Account for active and stranded cycle branches before activation. The next real PR/cycle must demonstrate protected publication and its recorded landed revision. No additional human approval count or bypass is part of this plan.
