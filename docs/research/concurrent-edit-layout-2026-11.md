# Concurrent edits: where lane changes collide in the code, and how other harnesses lay out code so that parallel changes do not conflict (research, 2026-11)

> The plan that uses this research: [Concurrent edits: one entry, one file](../plans/concurrent-edit-layout-2026-11.md). Earlier research on the landing side: [concurrent cycle landing](concurrent-cycle-landing-2026-10.md) (merge queues and re-verification), [merge concurrency](merge-concurrency-2026/README.md) and [wave orchestrator](wave-orchestrator-2026-10.md) (admission by cell).

Date: 2026-11-17. Method: a census of the files that main commits changed in 30 days (`git log origin/main --since=2026-10-18 --name-only`). The other sources are the conflicts of 2026-11-17, code reads with `path:line`, and a live web search. Each local claim names its command or file. "Synthesis" marks our own inference.

The operator asked on 2026-11-17:

> "Could we breakdown the code in to small independent pieces and restructuring them to be easier to be concurrently modifiable without resolving the merge conflict? Research online to learn how other harness reduce the time for merge conflicts, I remembered we did the similar research before, check it out and do the research against the latest online results"

Terms used throughout:

- **Lane.** One cycle of the loop in its own worktree. A wave runs up to 8 lanes at once.
- **Touch.** One commit that changes one file. A file with many touches is a hot file.
- **Central list.** One file that holds one entry for each member of a set, such as each verb or each serial test. Each new member edits the file.
- **Generated aggregate.** A tracked file that a generator writes from many sources, such as `docs/research-index.md`.
- **Cell.** A part of the code that one lane holds while it edits it. Admission refuses a second lane in the same cell.
- **Textual conflict.** Git cannot merge two changes line by line. **Semantic conflict.** The merge is clean, but the result is wrong.

## Table of contents

1. [What the earlier research covered](#1-what-the-earlier-research-covered)
2. [Local evidence](#2-local-evidence)
3. [Current practice](#3-current-practice)
4. [Adopt or reject](#4-adopt-or-reject)
5. [Refinements to adopt](#5-refinements-to-adopt)

## 1. What the earlier research covered

**E1.1** The research of 2026-07 and 2026-10 studied the landing side. Its topics were merge queues, re-verification after a base move, prefix speculation and flake policy ([merge-concurrency-2026](merge-concurrency-2026/README.md), [concurrent-cycle-landing-2026-10.md](concurrent-cycle-landing-2026-10.md)).
*Implication:* that work made a conflict cheaper to verify. It did not change the code layout that makes the conflict.

**E1.2** The earlier research found that the size of a change is the main lever. About 9.9% of agent PRs conflict at a median churn of 2 lines, and about 30% at 25 lines. The source is AgenticFlict, cited in [merge-concurrency-2026](merge-concurrency-2026/README.md). It put conflict avoidance at dispatch.
*Implication:* a hot file raises the conflict rate of each lane that touches it, also for a small change.

**E1.3** The wave orchestrator design admits lanes by cell, and counts 139 disjoint Go cells ([wave-orchestrator-2026-10.md](wave-orchestrator-2026-10.md)). ADR-0143 holds a lane only on a shared edit.
*Implication:* the cell is a package. A big package is one cell, whatever the file.

## 2. Local evidence

**F2.1** 986 main commits from 2026-10-18 to 2026-11-17 made 12,747 touches on 6,520 files. 383 of the commits are lane or train landings (`evolve-cycle`, `evolve-pr`).

**F2.2** Seven central lists take 330 touches in 30 days. The counts are touches in 30 days:

| File | Touches | What each change adds |
|---|---|---|
| `go/cmd/evolve/parallel_serial_tests_test.go` | 91 | one line for each test that cannot run in parallel (1,014 lines) |
| `go/.apicover-enforce` | 63 | one package pattern (883 lines) |
| `go/.cover-strict` | 45 | one package and its coverage floor |
| `go/cmd/evolve/registry.go` | 39 | one row in the `commands` table of each verb |
| `go/internal/repocontract/repocontract.go` | 35 | one test name in a pack list |
| `go/internal/guards/integrity_surface.go` | 31 | one protected-surface entry |
| `.evolve/policy.json` | 26 | one policy key |

The serial-test maps of all packages hold 155 touches in 10 files.
*Implication:* each new verb, test, package or gate file edits one of these files. Two lanes that add two unrelated things meet in the same lines.

**F2.3** Two generated aggregates take 164 touches: `docs/architecture/signal-codes.md` (90) and `docs/research-index.md` (74). Both have one source for each entry. Signal codes come from `signalcenter.RegisterCode`, one call in each owning module. Index entries come from one shard file each under `docs/research-index/`.
*Implication:* the sources do not conflict. Only the tracked aggregate conflicts, and a generator rewrites it with no judgment.

**F2.4** Two packages are very large, and lanes touch them most:

| Package | Go files | Lines | Touches in 30 days |
|---|---|---|---|
| `go/cmd/evolve` | 997 (282 non-test, at `cfceadb83`) | 159,896 | 1,618 |
| `go/internal/core` | 910 (230 non-test) | 132,991 | 1,036 |

`go/cmd/evolve` holds 250 non-test `cmd_*.go` verb files at `cfceadb83` (`git ls-tree --name-only cfceadb83 go/cmd/evolve/`). Six verb groups already live in their own packages under `go/internal/cli/` (`agentcmd`, `cloudcmd`, `contextcmd`, `guardcmd`, `opscmd`, `phasecmd`).
*Implication:* the pattern of a verb package exists, and most verbs do not use it.

**F2.5** Admission counts any shared edited package as a conflict (`go/internal/admission/admission.go:153-159`, `conflict` and `sharedEdit`). In wave 177, 19 to 22 waiting items named `cmd/evolve` among their cells ([console-build-or-route research F1.8](console-build-or-route-2026-11.md)). In wave 178, 20 items wait on a cell (`evolve wave supply`, 2026-11-17).
*Implication:* the size of a package sets how many lanes wait, not the size of the change.

**F2.6** The package notes of the big packages are hot too: `internal-core.md` (110 touches), `cmd-evolve.md` (65) and `internal-bridge.md` (65). The 210 package notes take 1,069 touches in all.
*Implication:* a big package has one big note, and each lane adds a line to it.

**F2.7** The console resolved 9 conflicts in 8 PRs on 2026-11-17 when it rebuilt old PRs on the current main. #1533 and #1534 share one row:

| PR | File | Kind |
|---|---|---|
| #1533, #1534 | `docs/research-index.md` | generated aggregate |
| #1536 | `repocontract.go` test-name list | central list |
| #1538 | `phases/ship/parallel_default_test.go` serial map | central list |
| #1538 | `cmd_selfcheck_protected_integration_test.go` stub | shared test, signature change |
| #1535 | `integration/train.go` import block | import block |
| #1537 | `inboxcurator.Inputs` struct and its one literal | shared struct |
| #1544 | `cmd_inbox_curator_test.go` | shared test file, two appended test groups |
| #1542 | `regressiontia.Pure` | semantic: two designs of one function |

*Implication:* 7 of 9 were textual collisions of independent additions. One was a stub that had to follow a changed signature. One was a real design conflict. Only the last needed judgment.

**F2.8** One textual merge was clean but wrong. In #1537, main added reroute fields to `Inputs` and #1487 added a history consume. The merge was clean, but the reroute came before the consume, and the apply step dropped the consume (incident record of 2026-11-17).
*Implication:* a clean merge is not proof. The gates on the composed tree stay, whatever the layout.

**F2.9** (corrected on 2026-11-17) The signals name each conflicted file. A fleet rebase conflict emits `ORCHESTRATOR_FLEET_REBASE_CONFLICT` with `fields.paths` (`go/internal/core/fleet_rebase_conflict_signal.go`). A train ejection emits `SHIP_INTEGRATION_EJECTED` with `fields.paths` (`go/internal/integration/brief.go:71-77`). The first version of this fact said that the signals had no file list.

The first version was wrong: the census counted the code `GIT_FLEET_REBASE_NEEDED`, which marks a rebase and not a conflict.
*Implication:* the measure of conflict files exists. Component C1 of the plan is already built.

**F2.10** The count ran on 2026-11-17 over the 149 run directories of cycles 2300 to 2462. They hold 65 signals with the code `GIT_FLEET_REBASE_NEEDED`, 0 `ORCHESTRATOR_FLEET_REBASE_CONFLICT` signals and 3 `SHIP_INTEGRATION_EJECTED` signals. The 3 ejections are on inbox files, not on code.

The fleet rebase already regenerates the outputs of the derived catalog when only they conflict (`derivedConflictIn` in `go/internal/core/ship_recovery.go`). The catalog holds `docs/architecture/signal-codes.md` and `docs/research-index.md` (`go/internal/derived/derived.go`).
*Implication:* lanes do not lose time to merge conflicts. The 9 conflicts of F2.7 were in console PRs that stayed open for days.

**F2.11** On 2026-11-17, all 21 items that wait on a cell wait on `cmd/evolve`. An admission cell is a package (`footprintOf` in `go/internal/admission/admission.go`). A file that is not Go code and is outside a package, such as `go/.apicover-enforce`, maps to no cell (`packageOf` in `go/internal/admission/modulegraph.go`). 16 of the 21 items name files of `cmd/evolve` that no other waiting item names.
*Implication:* a new fragment file in `cmd/evolve` stays in the `cmd/evolve` cell, so one entry for each file does not let more lanes run. A package split (C6) or a file cell (the admission plan) does.

## 3. Current practice

The search ran on 2026-11-17. [P] marks a primary source that we read. [S] marks a secondary source.

**P3.1 Agent PRs conflict often, and more across agents.** A study of 33,596 agent PRs in 2,807 repositories found co-active agent PR pairs in about 40% of the repositories. Textual conflicts were 19.8% for pairs of one agent and 41.7% for pairs of two agents. About 42% of the conflicts were structural: a change on one side and a delete on the other, or two adds. [P] [arXiv 2607.04697](https://arxiv.org/abs/2607.04697).

**P3.1b** AgenticFlict holds more than 142K agent PRs. A merge simulation of 107K of them found 27.67% in conflict. [P] [arXiv 2604.03551](https://arxiv.org/html/2604.03551v2).

**P3.2 Shared hot files are where parallel agents collide.** Guides name routes, configs and registries as the hot files, and say that worktrees move a conflict to merge time but do not stop it. [S] [Augment, multi-agent workspace](https://www.augmentcode.com/guides/how-to-run-a-multi-agent-coding-workspace), [agentpatterns.ai](https://agentpatterns.ai/workflows/concurrent-agent-pr-merge-conflicts/).

**P3.3 Disjoint ownership is the cheapest structural fix.** agentpatterns.ai gives each agent a disjoint directory or module, and keeps shared contract files read-only with one owner. [S] [agentpatterns.ai](https://agentpatterns.ai/workflows/concurrent-agent-pr-merge-conflicts/), [Vantaige](https://vantaige.io/blog/run-5-ai-agents-parallel-no-collisions-2026).

**P3.4 One fragment file for each entry.** towncrier reads one news fragment for each pull request and builds the changelog at release, because one shared changelog is where conflicts come from. [P] [towncrier](https://towncrier.readthedocs.io/en/trunk/).

**P3.5 Self-registration removes the central switch.** In Go, each plugin file calls a register function in `init()`, so a new plugin adds a file and edits no core file. Trivy discovers its analyzers this way. [S] [Trivy analyzers](https://sneaky-potato.github.io/oss/trivy-analyzers), [Eli Bendersky, plugins in Go](https://eli.thegreenplace.net/2021/plugins-in-go/). The caveat: the package must be imported, and a global registry makes tests harder to isolate.

**P3.6 A syntax-aware merge driver resolves independent additions.** mergiraf merges on syntax trees, supports Go, `go.mod` and `go.sum`, and merges Go import statements commutatively. [P] [mergiraf, supported languages](https://mergiraf.org/languages.html), [commutative merging](https://mergiraf.org/adding-a-language/enabling-commutative-merging.html). The earlier research kept it as a help to resolve conflicts whose output still passes every gate ([merge-concurrency-2026](merge-concurrency-2026/README.md)).

**P3.7 The union merge driver is unsafe for lists that also delete.** `merge=union` keeps the lines of both sides with no conflict marker, so a deleted line can come back. GitHub does not run it for pull-request merges. [S] [git mailing list](https://lore.kernel.org/git/A7C11212-1ECB-4D8D-B3B1-C7DB6673F3E6@me.com/t/), [GitHub community](https://www.Github.com/orgs/community/discussions/9288).

**P3.8 Clean merges still break.** "Passes Alone, Fails Together" builds a benchmark of patches that pass alone and fail when merged, mostly when one agent changes an interface that another agent uses. [P] [arXiv 2609.25396](https://arxiv.org/pdf/2609.25396).

**P3.9 Change coupling finds the split.** CodeScene measures change coupling between files and components from the version history. It says that the history, not the code alone, shows how well a design supports parallel work. [P] [CodeScene, change coupling](https://codescene.com/blog/change-coupling-visualize-the-cost-of-change).

**P3.10 Conflicts as data.** Jujutsu records a conflict in a commit and lets a rebase finish, so a conflict does not stop the work behind it. [P] [jj conflicts](https://docs.rs/crate/jj-cli/0.41.0/source/docs/conflicts.md), [S] [Chris Krycho](https://v5.chriskrycho.com/journal/deferred-conflict-resolution-in-jujutsu/).

**P3.11 The harness products isolate and serialize.** Cursor and Codex run each agent in its own worktree or sandbox and leave an overlapping edit to the person who merges. No primary source shows an automatic resolver. [S] [Cursor parallel agents](https://dev.to/thegdsks/cursor-3-ships-parallel-ai-agents-here-is-the-multi-agent-workflow-that-actually-works-2bk8), [Codex vs Cursor](https://morphllm.com/comparisons/codex-vs-cursor).

Weak sources: P3.2, P3.3, P3.5 and P3.11 rest on vendor or personal blogs. The 41.7% and 19.8% figures are from the arXiv abstract. We did not read the full PDFs of P3.1 and P3.8.

## 4. Adopt or reject

| Practice | Recommendation | Reason |
|---|---|---|
| One file or one registration call for each entry (P3.4, P3.5) | adopt | 330 touches go to 7 central lists, and 7 of the 9 conflicts on 2026-11-17 were independent additions (F2.2, F2.7). |
| Regenerate a generated aggregate at merge, not by hand (P3.4) | adopt | 164 touches go to two aggregates whose sources never conflict (F2.3). |
| Split big packages along change coupling (P3.3, P3.9) | adopt, step by step | 2 packages hold 2,654 touches and serialize admission (F2.4, F2.5). A big move at once conflicts with every lane. |
| Syntax-aware merge as a help to resolve conflicts (P3.6) | adopt in shadow | It resolves import blocks and struct fields. Its output still runs every gate (F2.8, P3.8). |
| `merge=union` for lists (P3.7) | reject | Our lists also delete entries, for example when the size ratchet tightens. A silent return of a deleted entry weakens a gate. |
| File-level ownership at dispatch (P3.3) | reject as the main fix | Lanes do not know their files before the build. The cell layout does the same job with no prediction. |
| First-class conflicts in the VCS (P3.10) | reject for now | The train and the ship gate use git. A VCS change is out of scope. |

## 5. Refinements to adopt

- **R1** A set that each change adds to has one file or one registration call for each member. A generator writes the list that a person or a gate reads. (F2.2, F2.7, P3.4, P3.5)
- **R2** A generated aggregate is rewritten by its generator when a merge or a rebase conflicts in it alone. No person or model resolves it. (F2.3, P3.4)
- **R3** A big package splits into smaller packages along its change coupling, one group at a time, at a boundary, with no lane in it. (F2.4, F2.5, P3.9)
- **R4** A syntax-aware merge runs in shadow on each fleet rebase and train composition, and its result runs every gate. (F2.7, F2.8, P3.6, P3.8)
- **R5** The rebase and train signals name each conflicted file, so the loop measures its hot files. (F2.9: already true.)
- **R6** To let more lanes run, make the admission cell smaller. A layout change inside one package does not change its cell. (F2.10, F2.11)
