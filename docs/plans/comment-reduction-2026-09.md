# Comment reduction workstream (2026-09)

**Goal:** every Go file meets [the code-comment convention](../conventions/code-comments.md). Knowledge the comments carry moves to `docs/` first. No behavior changes along the way.

**Owner:** console. This is a refactor of protected surfaces, so it runs outside loop lanes and lands at wave boundaries.

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
3. **Prove comment-only.** `go run ./cmd/commentaudit verify -base HEAD <batch dirs>` must pass. That means an identical position-free AST, every directive and marker kept, no exported doc deleted, and no file added or deleted.
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

The editor prompt forbids git mutation and is scoped to its batch.

## Order

1. **Calibration.** Two small leaf packages (`internal/recovery`, `internal/profiles`) to tune the editor and reviewer prompts against the convention.
2. **Mid-size packages**, heaviest narrative first, from the `rank` list.
3. **The three large packages**, split into file groups of about 40 files: `internal/core` (≈13 batches), `cmd/evolve` (≈8), `internal/bridge` (≈7).
4. **Test-only directories** (`acs/`, `test/`) last. Their comments are mostly fixture narration.

## Guard against regrowth

- **Now (batch 0):** `AGENTS.md` and the builder and TDD-engineer personas point at the convention, so loop lanes write to it. `commentaudit check -base <ref>` names every comment line with history markers that a diff *adds*; a moved line is not added. Reviewers run it on console changes.
- **Next (batch 0b, its own design):** make `check` a gate in ship's composed gates, not only in CI. A CI-only check would let a lane that passed its local gates turn main red. Roll it out through the existing `shadow → enforce` stage config: a WARN signal until lane compliance is measured, then a correction back to build. Design inputs from the batch-0 review:
  - Check at build exit, where a correction returns to build before audit, and keep ship as the backstop. Correcting at ship costs a re-audit.
  - Count added narrative across the whole diff, not per file, so moving code into a new file during decomposition does not flag the history it carries.
  - Detect trailing comments. `forEachLine` counts `x := 1 // cycle 42` as code today.
  - Stop counting raw-string lines as comments. `forEachLine` is line-based, so a line inside a backquoted string that starts with `//` counts as a comment. Reuse the `go/scanner` walk that directive anchoring already does.
  - Measure false positives during shadow before enforcing. "incident" is also a domain term (the observer's incidents), and "cycle 1" can describe runtime behavior. Keep exemptions as config data, never flags.
  - Register the gate's WARN code, module and kind with the Signal Center.
  - Extend the loop's auditor and adversarial-review personas to flag new narrative comments, once the gate exists.


## Progress

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
