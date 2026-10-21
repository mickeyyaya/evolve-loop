# Recovery retrospectives: what to learn when a failed item finally ships (research, 2026-10)

> The plan that uses this research: [recovery-retrospective-2026-10.md](../plans/recovery-retrospective-2026-10.md). The design: [recovery-retrospective.md](../architecture/recovery-retrospective.md). The decision record: [ADR-0151](../architecture/adr/0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md).

Date: 2026-10-21. Method: I read the failure records in the code and on disk, counted the recoveries in `.evolve/ledger.jsonl`, and ran live web searches on 2026-10-21. Each local claim names its command or its file. "Synthesis" marks my own inference.

## Table of contents

1. [The request](#1-the-request)
2. [Local evidence](#2-local-evidence)
3. [Current practice](#3-current-practice)
4. [Adopt or reject](#4-adopt-or-reject)
5. [Refinements to adopt](#5-refinements-to-adopt)

## 1. The request

The operator wrote on 2026-10-21: "that's the right thing to do to record the failed history, and also write the retrospective doc to improve continuous failed cycle once it turns green."

A correction came on the same day: "I think judgement lesson is not necessary to stay forever, once we turn them into written retrospective doc and store under docs, then we can remove them as everything is covered under docs."

So the research has two questions:

- What can the loop learn when an item that failed again and again ships green?
- Where does a judgment lesson live when it leaves the carryover log?

## 2. Local evidence

**F2.1** An inbox item keeps only its last failure. `bumpWith` deletes and writes again `last_failure_class` and `last_failure_fingerprint` on each failure (`go/internal/inboxmover/lifecycle/item.go:140-158`). A quarantine release deletes the count and the fingerprint (`go/internal/inboxmover/lifecycle/quarantine.go:41-44`).
*Implication:* the item cannot give the history of its tries. Another record must give it.

**F2.2** On 2026-10-21, no inbox file has `failure_count` 2 or more. Only two files have the field at all, each with the value 1. Command: `find .evolve/inbox -name '*.json' -exec jq -r 'select((.failure_count // 0) >= 2) | .id' {} + | wc -l` in the plane tree, result 0.
*Implication:* the counter is new and resets. A trigger on `failure_count` alone fires on almost nothing today.

**F2.3** The ledger has the full history. `.evolve/ledger.jsonl` has 184,695 lines on 2026-10-21. Its `inbox-lifecycle` records give each item event:

- A failure release has `action` `recover`, the `task_id` and the message `cycle-failure-release`.
- A ship has `action` `promote` and `processed` in the message.

39 task ids have two or more failure releases, and 13 of them shipped later. Command: the `jq -rs` join of `recover` and `promote` records by `task_id`, over the records with `kind` `inbox-lifecycle`. The older records with `class` `inbox-lifecycle` (298 lines) add 5 items and 3 ships (44 and 16).
*Implication:* about one item in three that failed twice ships later. The recoveries are real, and each one has a lesson that nobody writes today.

**F2.4** The ledger join of F2.3 lists 13 items that shipped after two or more releases. The ledger is not the number of record, because it counts a release as a try (F2.11). The number of record is the output of the detector rule (R1, R2, R9) over the dossiers. It finds 12 recoveries on 2026-10-21:

| Ship cycle | Item | FAIL dossiers since the last ship |
|---|---|---|
| 1632 | `tokenopt-handoff-digests` | 1628, 1631 |
| 1647 | `overlay-family-name-transport-ambiguity` | 1629, 1637, 1638 |
| 1659 | `triage-empty-commitment-still-dispatches-spine` | 1640, 1652, 1657 |
| 1712 | `lineage-datestamp-normalization` | 1707, 1708 |
| 1720 | `triage-unified-solution-synthesis` | 1629, 1637, 1638 |
| 1765 | `sizeratchet-shrink-eval-validators` | 1761, 1764 |
| 1766 | `fence-kept-empty-macos-ci-flake` | 1762, 1763 |
| 1801 | `gc-reaps-before-its-refusal` | 1798, 1800 |
| 1803 | `cli-failures` | 1798, 1800 |
| 1809 | `inboxbatch-utf8-and-resolution` | 1804, 1806, 1808 |
| 1842 | `generated-skill-command-shadows-its-skill` | 1828, 1840 |
| 1944 | `tokenopt-handoff-digests-per-edge-remainder` | 1642, 1644, 1646, 1651, 1654, 1656, 1938 |

Command, in the plane tree: `jq -rs -f recoveries.jq knowledge-base/cycles/cycle-*.json .evolve/dossiers-pending/cycle-*.json`. The program `recoveries.jq` applies the same rule as `recoveryretro.Recoveries`:

```jq
[.[] | select(.cycle != null)] | unique_by(.cycle) | sort_by(.cycle) as $all
| [ $all[] | select(.final_verdict == "PASS" and (.commit_sha // "") != "" and .tasks != null) as $p
    | $p.tasks[] as $t
    | ([ $all[] | select(.cycle < $p.cycle and .cycle >= ($p.cycle - 400) and ((.tasks // []) | index($t)) != null) ]
       | reduce .[] as $d ([]; if ($d.final_verdict == "PASS" and ($d.commit_sha // "") != "") then [] elif $d.final_verdict == "FAIL" then . + [$d.cycle] else . end)) as $f
    | select(($f | length) >= 2)
    | "\($p.cycle) \($t) \($f | map(tostring) | join(","))" ]
| .[]
```
 An independent replay of `Recoveries` in Go gave the same 12 events.

The two sets differ:

- `dossier-producer-params-struct` is only in the ledger set. It has one FAIL dossier (F2.11).
- `generated-skill-command-shadows-its-skill` is only in the dossier set.
- `retro-prompt-delivery-stall` is in neither: it shipped in cycle 1574 after one failure, and failed again in cycles 1577, 1578 and 1586.

*Implication:* a recovery can take up to seven failures, and a ship is not always the end of the story.

**F2.5** The committed cycle dossier holds what a retrospective needs. `knowledge-base/cycles/cycle-N.json` has `tasks`, `final_verdict`, `commit_sha`, `phases` (with `resolved_model`) and, on a FAIL, `failure` with `fingerprint`, `pre_class` and `reasons` (`go/internal/dossier/dossier.go:22-45`). The corpus is tracked in git: 2,554 files on 2026-10-21 (`git ls-files knowledge-base/cycles | wc -l`).
*Implication:* the host can build the try history from committed records, with no LLM call.

**F2.6** A worked example, `inboxbatch-utf8-and-resolution`, from its dossiers:

| Cycle | Verdict | Class | Fingerprint | First reason (cut) |
|---|---|---|---|---|
| 1804 | FAIL | `gate-block` | `build\|gate-block\|9af1ff95a11a` | build handoff floor: "Explanation Documentation: Changed Ar…" |
| 1806 | FAIL | `unknown` | `worktree\|unknown\|38ddaf05594b` | worktree provisioning failed: `cannot lock ref 'refs/remotes/origin/train/wave64-boundary'` |
| 1808 | FAIL | `gate-block` | `build\|gate-block\|0b91bd88eb78` | build handoff floor: `./acs/cycle1808 (-tags acs): unit tes…` |
| 1809 | PASS | — | — | commit `31ed49bc` |

*Implication:* the three failures have three fingerprints and two causes. 1806 was infrastructure, not the item. A table of this shape shows that at once. A human reader can then ask the right question: what did 1809 do that 1804 and 1808 did not?

**F2.7** The retrospective phase runs only after FAIL or WARN. `retroApplies` returns true only for `VerdictFAIL` or `VerdictWARN` (`go/internal/phases/retro/retro.go:130-133`). It writes `retrospective-report.md` into the cycle workspace and lesson YAML files into `.evolve/instincts/lessons/` (`retro.go:143`, `retro.go:238`).
*Implication:* no record of a recovery exists today. The workspace reports are host runtime state, not docs.

**F2.8** A judgment lesson is a P1 carryover todo with the id `cycle-<N>-judgment-<phase>`. `recordJudgmentLesson` stamps it with the `intent-rejected` retention, so it never expires (`go/internal/core/judgment_lesson.go:21-43`). PR #1081 bounds every other failure-learning todo to the 30-day carryover TTL and leaves this one open as a policy choice.
*Implication:* the operator correction answers that choice. The lesson moves to a document, and the todo then leaves the log.

**F2.9** The lesson recall for scout, tdd, build and audit reads YAML lesson files from three roots: `knowledge-base/research/`, `.evolve/instincts/lessons/` and `docs/research/` (`go/internal/research/kb.go:58`). It reads only `.yaml` and `.yml` files (`go/internal/research/lesson.go:89-104`). `seedTaskRecall` puts the selected lessons into the prompt as `recalled_lessons_untrusted_json` (`go/internal/core/task_recall.go:15-47`).
*Implication:* a markdown document alone is not recalled. A lesson YAML next to the document, in a search root, keeps the signal for scout and triage.

**F2.10** The dossier already has a safe path from a lane to `main`. A fleet lane writes its dossier into `.evolve/dossiers-pending/`, and the loop publishes the pending pairs at each wave boundary with one scoped commit (`docs/architecture/packages/internal-dossier.md`, "A fleet lane does not commit its dossier").
*Implication:* a host-written document can take the same path. A lane does not move `main` mid-wave.

**F2.11** The dossier names the items that the cycle tried. The ledger names each item that the cycle released. They differ:

- Each dossier since cycle 1624 has `tasks`. Older dossiers have `tasks: null`. Command: a `jq` scan of `knowledge-base/cycles/cycle-*.json`.
- 9 recoveries have each failure after cycle 1624. In 8 of them, each failed cycle of the ledger has a dossier that names the item.
- The exception is `dossier-producer-params-struct`. Cycles 1640 and 1657 released it, but their dossiers name other items: triage did not commit it.
- A FAIL dossier names each item of the cycle. Cycles 1798 and 1800 name both `cli-failures` and `gc-reaps-before-its-refusal`.

*Implication:* the dossier is the exact record of a try. The ledger counts a release as a try, so it counts too many. A cycle before 1624 has no try record, and a multi-item FAIL counts for each item.

**F2.12** A PASS dossier is not always a ship. Cycles 1624, 1630 and 1641 are PASS with `tasks` and no `commit_sha`. Cycle 1641 ran only scout and triage, and the loop shipped its item in cycle 1647. The 26 WARN dossiers with `tasks` have no `commit_sha` either.

Command: `jq -rs` over `knowledge-base/cycles/cycle-*.json`, on 2026-10-21.

*Implication:* a ship is a PASS with a commit. A PASS with no commit and a WARN are neither a try that failed nor a reset of the count.

**F2.13** 51 FAIL dossiers have the class `infra-error`, the first bucket of the failure classifier (`preClassRules`, `go/internal/core/failure_digest.go`). Four cycles of the recoveries in F2.4 have it: 1652, 1657, 1708 and 1800. Cycle 1800 counts for both `cli-failures` and `gc-reaps-before-its-refusal`, so five tries have the class.
*Implication:* the infrastructure mark must use the classes of the classifier, from one table, not a pattern fitted to one example.

## 3. Current practice

The searches ran on 2026-10-21.

**P3.1 Safety-II: learn from what goes right.** The EUROCONTROL white paper "From Safety-I to Safety-II" (2013) defines Safety-II as the aim "to ensure that 'as many things as possible go right'", and the ability "to succeed under varying conditions". Source: [EUROCONTROL white paper (ERA copy)](https://www.era.europa.eu/system/files/2022-10/From%20Safety-I%20to%20Safety-II%20White%20Paper.pdf); Hollnagel, Wears and Braithwaite, [From Safety-I to Safety-II: A White Paper (2015)](https://www.england.nhs.uk/signuptosafety/wp-content/uploads/sites/16/2015/10/safety-1-safety-2-whte-papr.pdf).
*Implication:* a recovery is the "went right" case. Today the loop studies only the failures.

**P3.2 Software teams rarely write the "what went well" part.** Lorin Hochstein (2026-04-26) writes that "Safety-II hasn't caught on our field". Source: [The normal work of creating reliability](https://surfingcomplexity.blog/2026/04/26/the-normal-work-of-creating-reliability/). A reader comment on that post (niketasharma, 2026-04-27) says that postmortems are "almost entirely structured around failure". It adds: "There's usually a 'what went well' section but nobody fills it out seriously." This is one practitioner, not a study.

A 2025 systematic review finds that "what should be learned, and who should be involved" in learning from success "remains unclear". Source: [BMC Health Services Research, 2025](https://link.springer.com/article/10.1186/s12913-025-12680-2).
*Implication:* a free-form "what went right" field stays empty. A record that the host fills from facts does not depend on effort.

**P3.3 Blameless postmortems use a fixed template and an automatic timeline.** 2026 guides list an evidence-backed timeline, the causes, what went well, and owned actions. incident.io says that most postmortems fail through "manual reconstruction work", and recommends an automatic timeline. Sources: [incident.io, SRE postmortem best practices (2026-03-13)](https://incident.io/blog/sre-incident-postmortem-best-practices), [OneUptime, SRE postmortem templates (2026-01-30)](https://oneuptime.com/blog/post/2026-01-30-sre-postmortem-templates/view).
*Implication:* the host builds the timeline from records. A person or a later phase adds the judgment.

**P3.4 A fail-then-pass is a signal to diagnose, not a success to accept.** Flaky-test guides of 2025 and 2026 say that a retry to green hides the cause. They also say that a flake to green can hide a real bug. Source: [ContextQA, flaky tests without retries (2026)](https://contextqa.com/blog/flaky-tests-in-ci-cd-without-retries/), [TestDino, flaky tests guide](https://testdino.com/blog/flaky-tests).
*Implication:* the retrospective must separate an infrastructure failure (F2.6, cycle 1806) from a failure of the item. It must also show when the pass came with no change of approach, so a reader can suspect luck.

**P3.5 Agents learn from a pair of a failed and a successful try on the same task.** ExpeL collects "success/failure pairs" on the same task instance through Reflexion retries. It edits an insight list with ADD, UPVOTE, DOWNVOTE and EDIT. Source: Zhao and others, [ExpeL: LLM Agents Are Experiential Learners (AAAI 2024)](https://arxiv.org/html/2308.10144v3).

ERL (ICLR 2026 MemAgents workshop) finds that a selective recall of heuristics beats the injection of all insights. It also finds that heuristics transfer better than full trajectories. Source: [Experiential Reflective Learning for Self-Improving LLM Agents, arXiv:2603.24639](https://arxiv.org/abs/2603.24639).
*Implication:* a recovery is a natural contrastive pair: the same item, failed tries and one passed try. The output must be a short lesson that the recall selects by keyword, not a full transcript.

**P3.6 Trajectory memory must find the strategy that worked.** IBM Research (March 2026) says that agents "fail to recover from similar errors, and miss opportunities to apply successful strategies", and extracts learnings from execution trajectories. Source: [Trajectory-Informed Memory Generation for Self-Improving Agent Systems, arXiv:2603.10600](https://arxiv.org/html/2603.10600v1).
*Implication:* the record of the passed try is the part that the loop does not keep today.

**P3.7 Markdown in the repository is the default memory for project lessons.** A Laravel team removed a semantic search layer over its convention files after five days, because the index drifted from the Markdown.

Other 2026 posts say that a pull request shows a bad line in a reviewed file. A bad entry in a vector store stays hidden. Sources: [Laravel, Semantic memory or just Markdown?](https://laravel.com/blog/semantic-memory-or-just-markdown), [Coding Agents Do Not Need Personal Memory](https://softwareguru.substack.com/p/coding-agents-do-not-need-personal).
*Implication:* the operator's choice (lessons go to `docs/`) matches current practice. A derived index (here, the lesson YAML) must come from the document, not replace it.

## 4. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Learn from what went right (P3.1) | adopt | A recovery is the "went right" case, and 12 exist (F2.3, F2.4). |
| A free-form "what went well" field (P3.2) | reject | It stays empty. The host fills the record from facts (F2.5). |
| An automatic timeline from records (P3.3) | adopt | The committed dossiers hold each try (F2.5, F2.6). |
| Separate the infrastructure failures (P3.4) | adopt | Cycle 1806 failed in the worktree, not in the item (F2.6). |
| A contrastive pair of failed and passed tries (P3.5) | adopt | Each recovery is one, at no extra cost. |
| An LLM reflection on each recovery (P3.5, ExpeL) | reject for now | The repository rule keeps deterministic work in code. An LLM pass is an option for a later stage. |
| Inject all lessons into every prompt (ExpeL) | reject | ERL shows that a selective recall wins. The existing keyword recall selects (F2.9). |
| Markdown in `docs/` as the record, an index derived from it (P3.7) | adopt | The operator asked for it, and the recall needs YAML (F2.9). |

## 5. Refinements to adopt

- **R1** The host writes a recovery retrospective when a PASS cycle ships an item that failed in two or more earlier cycles (F2.3, P3.1).
- **R2** The trigger counts FAIL dossiers that name the item, not `failure_count` or ledger releases. The counter resets, and a release is not always a try (F2.1, F2.2, F2.11).
- **R3** The document has one row for each try: cycle, verdict, class, fingerprint and the first reason (F2.6, P3.3).
- **R4** The document marks a try with the class `infra-error` or `unknown` as a possible infrastructure failure (F2.6, F2.13, P3.4).
- **R5** The document compares the passed try with the last failed try: the phases, the model of each phase and the ladder rungs (P3.5, P3.6).
- **R6** The document has a fixed "lesson" line that the host builds, and the lesson YAML repeats it for the recall (F2.9, P3.5).
- **R7** A judgment-lesson todo becomes a document with the same id. The todo leaves the log only when the document is on `main` (F2.8, P3.7).
- **R8** The documents reach `main` through the boundary publish path of the dossier, never from a lane mid-wave (F2.10).
- **R9** A ship is a PASS dossier with a `commit_sha`. Only a ship triggers a document or resets the count. A WARN is neither a failure nor a reset (F2.12).
- **R10** A document write is create-only, so a person can fill the "Judgment" section and no closeout erases it (P3.3).
- **R11** The lesson YAML keeps the recall digest whole. Its action names the document in a first sentence with no period. Its pattern holds the slug of the item, not the raw id (F2.9).
