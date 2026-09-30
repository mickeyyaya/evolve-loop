# Comment reduction workstream (2026-09)

**Goal:** every Go file meets [the code-comment convention](../conventions/code-comments.md). Knowledge the comments carry moves to `docs/` first. No behavior changes along the way.

**Target (the operator's rule, 2026-09-30): zero comments** beyond the ones something reads: toolchain directives, generated-file headers, `Deprecated:` and `Output:` lines, machine-read markers, and the package doc `docgo` enforces. Exported identifiers' docs are no longer kept, and `commentaudit verify` accepts their deletion.

**Owner:** console. This is a refactor of protected surfaces, so it runs outside loop lanes and lands at wave boundaries.

## Phase 2: to zero (2026-09-30)

| Step | Scope | How |
|---|---|---|
| 2a | Directories no batch has touched (214 directories, 1,504 files, 29,636 comment lines outside `acs/`, measured on main b67e0bb3) | Editor rounds as in the batch protocol below, several editors per round on disjoint directories, now deleting exported docs too |
| 2b | Directories an earlier batch finished under the old rule | One sweep deleting the exported docs and pointers those batches kept |
| 2c | The one-line *whys* kept because no test pins them | Each is filed as an inbox item to pin the invariant with a test; the comment goes in that item's change. An item on a protected surface is console-owned, like the rest of this workstream; the others are lane-sized work for the loop. |
| 2d | The per-cycle predicate packages `go/acs/cycle*` (494 files, 42,863 comment lines) | Editor rounds; a predicate's intent lives in its cycle's eval and reports, so there is little to capture. Four older predicates still require a doc on a named export (`acs/cycle1706` line 774, `cycle1698` line 673, `cycle1690` line 215, `cycle1685` line 306); they run again only when a lane changes their packages, so each is updated or archived in the same change that deletes the docs it reads (2a/2b) |
| 2e | Regrowth | Set `comment_floor.stage` to `enforce` in `.evolve/policy.json`, so a lane build that adds a comment is corrected before its audit; see [Guard against regrowth](#guard-against-regrowth) |
| 2f | Rounds 1–11 (batches 1–78), reduced before the self-explanation step existed | The same self-explanation pass per package, landed as reviewed refactor commits |

## Baseline

Measured with `commentaudit rank` on `origin/main` 3ce14dd0 (2026-09-26):

| Measure | Value |
|---|---|
| Directories with Go files | 708 |
| Comment lines | 152,164 |
| Code lines | 460,993 (comments are 33% of code) |
| Narrative comment lines (history markers; a lower bound) | 7,988 |

Heaviest directories, by narrative then comment lines:

| Directory | Narrative | Comment | Code |
|---|---|---|---|
| `internal/core` | 1,189 | 18,974 | 55,492 |
| `cmd/evolve` | 574 | 10,032 | 39,739 |
| `internal/bridge` | 553 | 9,383 | 30,642 |
| `internal/phases/ship` | 330 | 5,142 | 16,442 |
| `internal/phases/audit` | 220 | 3,096 | 8,761 |

## Batch protocol

Each batch covers one package, or one file group of a large package.

1. **Capture.** An editor agent reads the batch and lists every comment that carries knowledge not already in `docs/`: design rationale, a finding, an invariant's reason. It writes each one into the package's design notes, `docs/architecture/packages/<dir>.md`, which has Purpose, Design, Invariants and Findings sections. It links an existing ADR or incident instead of repeating it, and the batch adds its row to `docs/architecture/packages/README.md`. As batches land, their Findings sections are gathered into one pipeline findings report under `docs/research/`.
2. **Reduce.** The same agent rewrites the comments to the convention. It never touches code.
3. **Prove comment-only.** `go run ./cmd/commentaudit verify -base HEAD <batch dirs>` must pass. That means an identical position-free AST, every directive and marker kept, and no file added or deleted.
4. **Test.** Run `gofmt -l`, `go vet`, `golangci-lint run` and `go test -count=1` on the batch's packages, then `make -C go test-acs-durable`. The durable tier holds the gates that read comments across the repo: `docgo` for package docs and `envtaint` for its marker. A test that reads a comment fails here, and that comment is restored.
5. **No review for a proven batch.** The proof in step 3 is stronger than a reader, so a batch that passes it lands without a reviewer. The commit gate accepts the same proof. A batch that also touches code gets code-simplifier and a reviewer as usual; that includes a conflict resolution that changes a code line. The editor reports code problems it finds, and they are filed as inbox items rather than fixed in the batch.
6. **Land.** Editors share one working worktree, but batches land from a separate landing worktree. A manual ship with no workspace manifest stages every changed file (`ship.stageExplicitPaths`), so committing in the shared tree would sweep up batches still in flight.
   - **Move each batch as a patch, never as copied files.** Take `git diff` against the editor base and apply it in the landing tree with `git apply --3way`. Copying whole files would silently revert every lane change made to them since the editor base, tests included, so the tests would stay green. New capture docs are the only files copied.
   - **Re-prove in the landing tree.** Run `commentaudit verify -base $(git merge-base HEAD origin/main)` with no directory arguments. Its verified count must equal the sum of the per-batch file counts in the progress table below. Then check that `git diff --name-only --no-renames $(git merge-base HEAD origin/main) -- ':(exclude)*.go' ':(exclude)docs/'` and `git status --porcelain --no-renames -- ':(exclude)*.go' ':(exclude)docs/'` both print nothing: every non-Go change is under `docs/`, committed or not.
   - **After a rebase or `gh pr update-branch`, re-prove before merge.** Fetch and pull the branch locally first. Build `commentaudit` from the rebased tree and re-prove with it: the cumulative proof re-checks every earlier batch under the current rules, and it catches a conflict resolution that silently reverted train code.
   - **Before the PR opens,** `docs/architecture/packages/README.md` lists every page the branch adds.
   - **One batch at a time:** apply, prove, commit, then apply the next. `git apply --3way` stages its result and ship stages every changed file, so two applied batches can only land as one commit.
   - **Merge at a wave boundary only, after the full CI-parity floor.** Comment PRs merge after that boundary's feature train, then rebase and re-prove. The workstream always yields: lane PRs never rebase onto a comment PR mid-wave.
   - **Choosing a batch.** Skip files that an open PR, a carried-over or stranded lane, or an in-flight decomposition unit will touch. Comment edits next to code edits conflict textually.
7. **Make the code say it** (from round 12). After the comment-only commit lands on the round's branch, a second agent per group reads what that commit deleted (`git show` of it) and, where a deleted comment said what the code does and the code no longer shows it, makes the code say it ([What a deleted comment leaves behind](../conventions/code-comments.md#what-a-deleted-comment-leaves-behind)). Steps 3 to 5 apply to the comment-only commit only: this pass changes code, so it is its own commit in the same PR, reviewed by the simplifier, the architecture reviewer and the Go reviewer, with the full floor. The editors' comment-only state is snapshotted first (`git stash create` plus a local ref), so the refactor delta is a clean diff against it.

The editor prompt forbids git mutation and is scoped to its batch.

## Order

1. **Calibration.** Two small leaf packages (`internal/recovery`, `internal/profiles`) to tune the editor and reviewer prompts against the convention.
2. **Mid-size packages**, heaviest narrative first, from the `rank` list.
3. **The three large packages**, split into file groups of about 40 files: `internal/core` (≈13 batches), `cmd/evolve` (≈8), `internal/bridge` (≈7).
4. **Test-only directories** (`acs/`, `test/`) last. Their comments are mostly fixture narration.

## Guard against regrowth

- **Now (batch 0):** `AGENTS.md` and the builder and TDD-engineer personas point at the convention, so loop lanes write to it. `commentaudit check -base <ref>` names every comment line with history markers that a diff *adds*; a moved line is not added. Reviewers run it on console changes.
- **Batch 0b, first step shipped (2026-09-28), in shadow:** the build handoff floor counts the comment lines a build adds (`commentaudit comments`; both it and `check` now count across the whole diff through one lister, so a moved or renamed comment is not added) and WARNs; `policy.json` `comment_floor.stage: enforce` turns it into a correction back to build. Cycle 1730 (wave 22) is the evidence: a correct refactor failed its first audit on 54 added comment lines. The inputs below that remain open before enforce are tracked as the inbox item `comment-floor-enforce`: trailing comments, raw-string lines, `.go` fixtures under `testdata/`, generated files' bodies, the false-positive measurement and the Signal Center code. "Count across the whole diff" is done.
- **Original design (batch 0b):** make `check` a gate in ship's composed gates, not only in CI. A CI-only check would let a lane that passed its local gates turn main red. Roll it out through the existing `shadow → enforce` stage config: a WARN signal until lane compliance is measured, then a correction back to build. Design inputs from the batch-0 review:
  - Check at build exit, where a correction returns to build before audit, and keep ship as the backstop. Correcting at ship costs a re-audit.
  - Count added narrative across the whole diff, not per file, so moving code into a new file during decomposition does not flag the history it carries.
  - Detect trailing comments. `forEachLine` counts `x := 1 // cycle 42` as code today.
  - Stop counting raw-string lines as comments. `forEachLine` is line-based, so a line inside a backquoted string that starts with `//` counts as a comment. Reuse the `go/scanner` walk that directive anchoring already does.
  - Measure false positives during shadow before enforcing. "incident" is also a domain term (the observer's incidents), and "cycle 1" can describe runtime behavior. Keep exemptions as config data, never flags.
  - Register the gate's WARN code, module and kind with the Signal Center.
  - Extend the loop's auditor and adversarial-review personas to flag new narrative comments, once the gate exists.

## Progress

What each round found beyond the comments themselves is written up per round: [round 12 findings](../reports/comment-round-12-findings-2026-09-30.md). The decision record is [ADR-0111](../architecture/adr/0111-code-carries-no-comments.md).

Go files is each batch's count of changed Go files. The landing proof's verified count must equal the sum over the landed batches.

| Batch | Scope | Go files | Comment lines before → after | Narrative before → after | Status |
|---|---|---|---|---|---|
| 0 | tooling, convention, plan, persona rules, commit-gate waiver | — | — | — | its own PR, which lands before the comment PR |
| 1-2 | `internal/recovery`, `internal/profiles` | 28 | 687 → 95, 500 → 63 | 67 → 5, 54 → 5 | on the comment PR |
| 3 | `internal/policy` | 84 | 2,111 → 306 | 107 → 0 | on the comment PR |
| 4 | `internal/router` | 52 | 1,725 → 264 | 105 → 0 | on the comment PR |
| 5-6 | `internal/triagecap`, `internal/config` | 68 | 1,439 → 155, 930 → 147 | 100 → 0, 65 → 0 | on the comment PR as one commit |
| 7 | `internal/prompts` | 20 | 875 → 40 | 71 → 0 | on the comment PR |
| 8 | `internal/bridge/panestream` | 26 | 1,402 → 127 | 57 → 0 | on the comment PR |
| 9 | `internal/inboxbatch` | 24 | 743 → 110 | 57 → 0 | on the comment PR |
| 10 | `internal/fleet` | 30 | 1,106 → 105 | 38 → 0 | on the comment PR |
| 11 | `internal/dossier` | 33 | 947 → 162 | 41 → 0 | held: the package has lint debt from main, fixed in PR #635 |
| 12 | `internal/evalgate` | 19 | 690 → 94 | 40 → 3 | on the comment PR; the 3 are read by `go/acs/cycle1685` |
| 13 | `internal/looppreflight` | 29 | 717 → 118 | 37 → 0 | on the comment PR |
| 14 | `internal/phasecoherence` | 15 | 478 → 46 | 37 → 0 | on the comment PR |
| 15 | `internal/cli/phasecmd` | 26 | 551 → 98 | 42 → 0 | on the comment PR |
| 16 | `internal/topngate` | 9 | 370 → 52 | 33 → 0 | on the comment PR |
| 17 | `internal/tokenusage` | 17 | 760 → 66 | 31 → 0 | on the comment PR |
| 18 | `internal/adapters/observer` | 16 | 552 → 119 | 35 → 0 | on the comment PR |
| 19 | `internal/guards` | 30 | 758 → 118 | 30 → 0 | on the comment PR |
| 20 | `internal/llmroute` | 18 | 595 → 51 | 23 → 0 | on the comment PR |
| 21 | `internal/swarm` | 36 | 929 → 145 | 20 → 0 | on the comment PR |
| 22 | `internal/adapters/ledger` | 26 | 878 → 140 | 29 → 0 | on the comment PR |
| 23 | `internal/changedpkgs` | 12 | 504 → 43 | 23 → 0 | on the comment PR |
| 24 | `internal/cyclestate` | 13 | 384 → 100 | 24 → 0 | on the comment PR |
| 25-32 | eight more packages, one design page each (see the package index) | — | — | — | batches 0-32 merged: #638, then #640 as one squashed commit |
| 33 | `internal/phasecontract` | 29 | 1,033 → 113 | 71 → 3 | on the comment PR |
| 34 | `internal/phasespec` | 35 | 1,004 → 125 | 73 → 0 | on the comment PR |
| 35 | `internal/inboxmover` | 39 | 1,237 → 155 | 108 → 1 | on the comment PR |
| 36 | `internal/inboxmover/lifecycle` | 20 | 459 → 75 | 29 → 0 | on the comment PR |
| 37 | `internal/phases/runner` | 62 | 1,681 → 152 | 87 → 0 | on the comment PR |
| 38 | `internal/deliverable` | 53 | 2,123 → 268 | 138 → 0 | on the comment PR |
| 39 | `internal/core/advisor` | 24 | 659 → 110 | 36 → 0 | on the comment PR |
| 40 | `internal/phases/runner/verdict` | 13 | 439 → 106 | 28 → 0 | on the comment PR |
| 41 | `internal/core`, file group 1 of 13 | 39 | 1,509 → 169 | — | on the comment PR |
| 42 | `cmd/evolve`, file group 1 of 8 | 39 | 719 → 142 | — | on the comment PR |
| 43 | `internal/bridge`, file group 1 of 7 | 40 | 1,685 → 275 | — | on the comment PR |
| 44 | `cmd/evolve`, file group 2 of 8 | 40 | 1,381 → 341 | 137 → 1 | on the comment PR; `cmd_composition_wiring.go` deferred |
| 45 | `internal/core`, file group 2 of 13 | 39 | 1,167 → 161 | 73 → 0 | on the comment PR |
| 46 | `internal/core`, file group 3 of 13 | 40 | 2,675 → 1,943 | — | on the comment PR |
| 47 | `cmd/evolve`, file group 3 of 8 | 42 | 829 → 327 | — | on the comment PR |
| 48 | `internal/bridge`, file group 2 of 7 | 33 | 1,085 → 376 | — | on the comment PR; 7 files the P3 train edits deferred to group 2b |
| 49 | `internal/core`, file group 4 of 13 | 40 | 1,490 → 230 | — | on the comment PR |
| 50 | `cmd/evolve`, file group 4 of 8 | 42 | 2,273 → 1,116 | — | on the comment PR |
| 51 | `internal/bridge`, file group 3 of 7 | 38 | 1,176 → 367 | — | on the comment PR; 2 files the P3 train edits deferred to group 2b |
| 52 | `internal/core`, file group 5 of 13 | 40 | 1,316 → 330 | — | on the round-3 comment PR |
| 53 | `cmd/evolve`, file group 5 of 8 | 42 (40 edited) | 1,390 → 273 | — | on the round-3 comment PR |
| 54 | `internal/bridge`, file group 4 of 7 | 40 (37 edited) | 1,462 → 330 | — | on the round-3 comment PR |
| 55 | `internal/core`, file group 6 of 13 | 37 | 1,554 → 381 | — | on the round-4 comment PR |
| 56 | `cmd/evolve`, file group 6 of 8 | 36 | 1,315 → 418 | — | on the round-4 comment PR |
| 57 | `internal/bridge`, file group 5 of 7 (with the 9 files group 2 deferred) | 47 | 1,593 → 567 | — | on the round-4 comment PR |
| 58 | `internal/core`, file group 7 of 13 | 37 | 1,778 → 488 | — | on the round-5 comment PR |
| 59 | `cmd/evolve`, file group 7 of 8 | 38 | 1,002 → 120 | — | on the round-5 comment PR |
| 60 | `internal/bridge`, file group 6 of 7 | 34 | 1,327 → 645 | — | on the round-5 comment PR |
| 61 | `internal/core`, file group 8 of 13 | 37 | 1,506 → 860 | — | on the round-6 comment PR |
| 62 | `cmd/evolve`, file group 8 of 8 | 40 | 1,132 → 449 | — | on the round-6 comment PR |
| 63 | `internal/bridge`, file group 7 of 7 | 32 | 1,101 → 376 | — | on the round-6 comment PR |
| 64 | `internal/core`, file group 9 of 13 | 37 | 1,352 → 505 | — | on the round-7 comment PR |
| 65 | `internal/phases/ship`, file group 1 of 4 | 38 | 1,745 → 411 | — | on the round-7 comment PR |
| 66 | `internal/phases/audit`, file group 1 of 2 | 39 | 1,860 → 1,132 | — | on the round-7 comment PR |
| 67 | `internal/core`, file group 10 of 13 | 32 | 1,250 → 244 | — | on the round-8 comment PR |
| 68 | `internal/phases/ship`, file group 2 of 4 | 33 | 1,249 → 656 | — | on the round-8 comment PR |
| 69 | `internal/phases/audit`, file group 2 of 2 | 29 | 1,220 → 823 | — | on the round-8 comment PR |
| 70 | `internal/core`, file group 11 of 13 | 32 | 1,331 → 783 | — | on the round-9 comment PR |
| 71 | `internal/phases/ship`, file group 3 of 4 | 37 | 1,441 → 754 | — | on the round-9 comment PR |
| 72 | `internal/subagent`, all files (1 of 1) | 30 | 1,118 → 836 | — | on the round-9 comment PR |
| 73 | `internal/core`, file group 12 of 13 | 37 | 1,102 → 320 | — | on the round-10 comment PR |
| 74 | `internal/phases/ship`, file group 4 of 4 | 12 | 403 → 91 | — | on the round-10 comment PR |
| 75 | `internal/acssuite`, all files (1 of 1) | 14 | 612 → 302 | — | on the round-10 comment PR |
| 76 | `internal/core`, file group 13 of 13 | 38 | 1,047 → 241 | — | on the round-11 comment PR |
| 77 | `internal/flagregistry`, all files (1 of 1) | 9 | 240 → 39 | — | on the round-11 comment PR |
| 78 | `internal/phases/triage`, all files (1 of 1) | 16 | 385 → 56 | — | on the round-11 comment PR |
| 79 | `internal/modelquery`, `internal/phases/specrunner`, all files (5 decision-surface files deferred: a test hashes their bytes) | 33 | 1,139 → 6 | — | on the round-12 comment PR |
| 80 | `internal/releasepipeline`, `internal/skillcheck`, `cmd/evolve-fake-cli`, all files | 37 | 1,374 → 15 | — | on the round-12 comment PR |
| 81 | `internal/faillearn`, `internal/setup`, `internal/phases/retro`, all files | 39 | 1,397 → 15 | — | on the round-12 comment PR |
| 82 | `internal/phases/audit/ciparitygate`, `internal/auditchain`, `internal/rollback`, all files | 42 | 1,300 → 12 | — | on the round-12 comment PR |
| 83 | `internal/releasepreflight`, `internal/subagent/subagentrun`, `internal/commitgate`, all files | 38 | 1,345 → 9 | — | on the round-12 comment PR |
| 84 | `internal/dashboard`, `internal/scopedelta`, `internal/gitexec`, all files | 39 | 1,265 → 10 | — | on the round-12 comment PR |

The three largest packages are split into file groups of about 40 files, taken in name order. Each group is one batch, and the package's design page fills in group by group. Narrative is not measured per group.

Batches 33–45 were rebuilt as one squashed commit on origin/main after cycle 1698 rewrote `cmd_composition_wiring.go` (it deleted `auditLedgerEntry` in favor of `internal/auditledger`). That file keeps main's text for now and is redone in a later batch; batch 44's figures still count it.

The narrative figures after batches 1-2 predate the pointer fix. The 5 left in each are `See ADR` pointers, which no longer count.

Code problems the editors found are reported, not changed, and filed as inbox items:
- `profiles-test-hygiene`
- `config-dead-dials`
- `policy-resolver-hygiene`
- `triagecap-demotion-write-and-dead-seams`
- `router-silent-errors`
- `inboxbatch-utf8-and-resolution`
- `fleet-runpool-silent-success`
- `dossier-sweep-pairing`
- `evalgate-floorbinding-silent-errors`
- `looppreflight-version-cache`
- `phasecoherence-provenance-silent`
- `phasecmd-silent-policy-fallbacks`
- `topngate-tdd-scope-fail-open`
- `tokenusage-silent-reads-and-vacuous-test`
- `observer-exec-without-context`
- `llmroute-default-trigger-aliasing`
- `swarm-worker-pgid-never-recorded`
- `ledger-plain-verify-fails-after-seal`
- `auditchain-scout-report-literal`: found while landing, not by an editor
- `guards-path-traversal-and-ship-bypass` (P1)
- `lane-lint-debt-class`: the lint debt that held batch 11.
- `codex-pretrust-presence-and-escaping` (P2), from batch 43.
