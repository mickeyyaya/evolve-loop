# Plan: the console train binds a multi-commit PR to the review record of each commit

- **Status:** built, 2026-11-19. Earlier status: proposed, 2026-11-16.
- **Inbox item:** `train-record-pr-has-no-reachable-attestation`. **Decision:** no ADR, because the guarantee does not change: each landed change has a review record of its own.

## 1. Request

The inbox item (2026-11-05):

> "evolve integration record-pr accepts an attestation only when its tree equals the digest of the whole diff of the PR. The commit gate writes an attestation for the staged tree of one commit. [...] So the train that lands console PRs cannot yet land a normally reviewed console PR."

The operator, 2026-11-16: "Do not stop building the Todo inbox tasks, continue working on them".

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | `record-pr --reviewed` reads one review record, named by the digest of the whole change of the PR. | `go/cmd/evolve/cmd_integration_batch.go:230-254` |
| F2 | The train computes that digest as the SHA-256 of `git diff base head` over all the commits of the PR. | `go/internal/integration/train.go:407-421` |
| F3 | The commit gate names its record by `treestate.SHA`: the digest of `git diff HEAD`, the worktree change against HEAD, for one commit. | `go/internal/commitgate/commitgate.go:285`, `reviewrecord.go:19`, `go/internal/treestate/treestate.go:19` |
| F4 | For a PR of one commit, the two digests are equal. On 2026-11-16 `record-pr` accepted 17 such PRs (#1483 to #1503) with no manual step. | console record of 2026-11-16 |
| F5 | For a PR of two or more commits, no record matches. PRs #1473, #1474 and #1477 got a second commit for an STE fix. Each needed the manual method: a restore at the base, then the review seats and the gate again. | session record of 2026-11-15 |
| F6 | `record-pr` already refuses a merge head, so the commits of a PR form one line. | `go/internal/integration/train.go:388`, `:438-440` |

*Implication:* each commit of a gated PR already has its own record. The train needs to bind the chain of those records, not a new record for the whole change.

## 3. Goals and non-goals

Goals:

1. A PR whose every commit passed the commit gate reaches `record-pr` and `integration batch` with no manual step.
2. A PR with one commit that has no record is refused. The refusal names that commit and the next step.

Non-goals:

- No change to what a review record proves, or to how the gate writes it.
- No binding by a review class. `--class` keeps its rules.

## 4. Candidate designs

| Criterion (weight) | 1. Chain of per-commit records (chosen) | 2. The gate also writes a whole-change record | 3. Keep the manual method |
|---|---|---|---|
| No manual step (3) | 3 | 2: the gate must know the base of the PR at each commit | 0 |
| Same guarantee (3) | 3: each change has its own review | 3 | 3 |
| Cost to build (2) | 2: one binder and one evidence field | 1: the gate learns about PRs | 3 |
| **Weighted total** | **22** | **17** | **15** |

Design 1 wins. The gate stays a gate for one change, and the train composes the chain it already lists.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The binder first looks for the record of the whole change, as today. With no such record and two or more commits, it walks the commits in order. | F1, F4 | Drop the whole-change lookup: a PR reviewed as one change after a restore keeps working. |
| D2 | For each commit, the train computes the change digest of `parent..commit` and its paths. A record of that digest must exist and pass `commitgate.CheckReviewedChange` for those paths. The parent is `CommitIdentity.BaseSHA`, so no second `rev-parse` runs. | F3, F5 | Trust the CI of the PR: CI is not a review. |
| D3 | The binding is `integration.ChainedBinding(ev, check)`. It owns only the order, the first refusal and the binding digest. `TreeSHA` is the whole-change digest, and `ArtifactSHA` is the digest of the ordered record digests. A new field `AuditBinding.Kind` (`json:"kind,omitempty"`) holds `commit-chain`. `bindingErr` and `checkComposable` accept a binding with that kind and a non-empty `TreeSHA` and `ArtifactSHA`. The composed message adds the trailer `Evolve-Audit-Kind: commit-chain`. | F2, round-2 finding 2 | Put the kind in `Class`: a chain then reads as a review class. Give no field: the trailer then names a whole-change digest with no record, and nothing tells the two shapes apart. |
| D4 | The first commit with no record, or with a record that fails its check, refuses the PR. The text names the commit and its digest. It gives the next step in the shape of today's refusal: at `<parent>`, run `git restore --source=<commit> --staged --worktree -- .`, then the review seats and `evolve commit-gate run`. | goal 2 | Refuse with no commit named: the operator then searches by hand. |
| D5 | The train takes each commit digest from a new `treestate.Between(from, to)`. It shares the one argument builder of `treestate.SHA` (the worktree change against HEAD) and adds no flag. So no existing digest changes. | review findings 1 of rounds 1 and 2 | Pin new flags in `treestate` now: the ledger binding in `core/phase_bindings.go` and three more homes hash a plain `git diff HEAD`. Every cycle ship then refuses with a tree mismatch. That unification is its own item (`change-digest-has-one-pinned-home`). |
| D6 | The read, parse and check of one review record is one function `reviewedChange(store, digest, paths)` in `cmd/evolve`. The whole-change lookup and each commit use it. `internal/integration` does not import `commitgate`. | review finding 3 | Move the check into `integration`: the train then learns the record format of the gate. |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| C0 | `treestate.Between(from, to)` beside `treestate.SHA`, on one argument builder with no new flag (D5). | `TestChangeDigest_WorktreeChangeEqualsCommitted` (real git: make a worktree change, take `treestate.SHA`, commit, compare with `treestate.Between(parent, commit)`), `TestChangeDigest_AnUnstagedEditAtGateTimeDoesNotMatch` | console |
| C1 | `Evidence` gains one `{SHA, BaseSHA, DiffSHA, Paths}` entry for each commit, from `treestate.Between`. `Evidence.Commits` stays the ordered SHAs for its two readers: `ClassBinding` (`consolepr.go:55`) and the head lookup in `reviewedBinder` (`cmd_integration_batch.go:236`). | `TestEvidence_CarriesTheChangeOfEachCommit`; `TestClassBindingBindsTheClaimToTheChangeAndItsRule` stays green unchanged | console (the train is a protected surface) |
| C2 | `integration.ChainedBinding` and `AuditBinding.Kind` (D3, D4). | `TestChainedBinding_BindsEveryCommitRecordInOrder`, `TestChainedBinding_ACommitWithNoRecordIsRefusedByName`, `TestChainedBinding_TheKindNamesTheChain` (asserts the record JSON and the composed trailer) | console |
| C3 | `reviewedChange` (D6), and `reviewedBinder` falls back to the chain (D1, D2). | `TestRecordPR_ATwoCommitPRReviewedPerCommitIsRecorded`, `TestRecordPR_AnUnreviewedSecondCommitIsRefusedWithItsNextStep`. Both are journey tests through `recordPRVerb` and a real git fixture. Each fixture record takes its name from `treestate.SHA` at its staged state. | console |

Each component keeps 100% coverage of the functions it changes.

## 7. Order

1. C0 first. It changes no existing digest.
2. C1, because C2 and C3 read its fields.
3. C2 and C3 together.

## 8. Verification

| Goal | Measure | Before | After |
|---|---|---|---|
| G1 | Manual whole-change reviews at a boundary | 3 PRs on 2026-11-15 | 0 for a PR whose every commit passed the gate |
| G2 | A refusal names the commit and the next step | not measured | pinned by `TestRecordPR_AnUnreviewedSecondCommitIsRefusedWithItsNextStep` |

## 9. Risks

| Risk | Control |
|---|---|
| A commit diff that the gate never saw matches by chance. | The digest is SHA-256 of the diff text. A match means the same change. |
| A record for an old version of a commit matches a rebased commit. | The digest covers the diff, not the commit id. The same diff means the same reviewed change, so the match is correct. |
| A PR with a revert commit has a record for a change that the next commit undoes. | Each commit still had its own review, and the whole change is the composition of reviewed changes. |

## 10. Open questions

1. Must the chain also hold the `Closes-Inbox` trailers of each commit? The composer reads them from all commits today (`consolepr.go` `closingIDs`), so no change seems needed.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| C0 `treestate.Between` | done, 2026-11-17 | [#1535](https://github.com/mickeyyaya/evolve-loop/pull/1535) [`138552f`](https://github.com/mickeyyaya/evolve-loop/commit/138552f94) (go/internal/treestate/between_test.go:22) | none |
| C1 per-commit change in `Evidence` | done, 2026-11-17 | [#1535](https://github.com/mickeyyaya/evolve-loop/pull/1535) [`138552f`](https://github.com/mickeyyaya/evolve-loop/commit/138552f94) (`TestEvidence_CarriesTheChangeOfEachCommit`) | none |
| C2 `ChainedBinding` and `AuditBinding.Kind` | done, 2026-11-17 | [#1535](https://github.com/mickeyyaya/evolve-loop/pull/1535) [`138552f`](https://github.com/mickeyyaya/evolve-loop/commit/138552f94) (`go/internal/integration/consolepr.go:52`) | none |
| C3 `reviewedChange` and the chain fallback | done, 2026-11-17 | [#1535](https://github.com/mickeyyaya/evolve-loop/pull/1535) [`138552f`](https://github.com/mickeyyaya/evolve-loop/commit/138552f94) (`go/cmd/evolve/cmd_integration_batch.go:357`) | none |

All four components are on main in PR 1535. The inbox item `train-record-pr-has-no-reachable-attestation` closed through [#1567](https://github.com/mickeyyaya/evolve-loop/pull/1567) [`86e8d77`](https://github.com/mickeyyaya/evolve-loop/commit/86e8d7730) on 2026-11-19. That change adds a journey test through the real gate. Decision D5 defers the unification of the change digest to the open item `change-digest-has-one-pinned-home`. Nothing else is left.
