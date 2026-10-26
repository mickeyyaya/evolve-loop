# Plan: the host writes a retrospective document for each recovery and each judgment lesson

- **Status:** accepted. C1 to C3 are on `main` at stage `shadow`. On 2026-10-27 the operator moved the stage to `enforce`. The policy key of C4 is built, and the checked-in policy sets `enforce`. The rest of C4 (the pending dir and the publish), C5 and C6 are open, and each has an inbox item (section 6). Thus the closeout still writes into the workspace only.
- **Design:** [recovery-retrospective.md](../architecture/recovery-retrospective.md). **Decision:** [ADR-0151](../architecture/adr/0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md). **Research:** [recovery-retrospective-2026-10.md](../research/recovery-retrospective-2026-10.md).
- **Package note:** [internal-recoveryretro.md](../architecture/packages/internal-recoveryretro.md).

**Terms used throughout:**

- *recovery*: a ship of an item after two or more FAIL dossiers of the item since its last ship. A ship is a PASS dossier with a `commit_sha`.
- *try row*: one line of the history table, made from one committed cycle dossier.
- *judgment lesson*: the P1 carryover todo `cycle-<N>-judgment-<phase>` that `recordJudgmentLesson` writes.
- *retrospective document*: a markdown file under `docs/retrospectives/`, with a lesson YAML of the same base name.
- *stage*: `shadow` writes the documents into the cycle workspace only. `enforce` publishes them to `main` and retires the judgment todos.

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Limits](#10-limits)
11. [Open questions](#11-open-questions)

## 1. Request

The operator wrote on 2026-10-21: "that's the right thing to do to record the failed history, and also write the retrospective doc to improve continuous failed cycle once it turns green."

The operator then corrected the first half on the same day: "I think judgement lesson is not necessary to stay forever, once we turn them into written retrospective doc and store under docs, then we can remove them as everything is covered under docs."

So this plan has two outputs. A recovery gets a retrospective document. A judgment lesson becomes a retrospective document, and its todo then leaves the carryover log.

## 2. Facts

| Fact | Research id |
|---|---|
| An item keeps only its last failure, and a quarantine release resets the count. | F2.1, F2.2 |
| The ledger shows 39 items with two or more failures, and 13 of them shipped later. The detector rule finds 12 recoveries in the dossiers: the number of record. | F2.3, F2.4 |
| A PASS with no commit did not ship, and a WARN never has a commit. | F2.12 |
| 51 FAIL dossiers have the class `infra-error`. | F2.13 |
| The dossier names the items that a cycle tried. The ledger also names the items that a cycle only released. | F2.11 |
| The committed dossier of each cycle has the item ids, the verdict, the failure class, the fingerprint, the reasons, the phases and the models. | F2.5 |
| A worked recovery has three failures with three fingerprints, and one failure is infrastructure. | F2.6 |
| The retrospective phase runs only on FAIL or WARN. No record of a recovery exists. | F2.7 |
| A judgment-lesson todo never expires today. | F2.8 |
| The lesson recall reads only YAML files in three roots. | F2.9 |
| The dossier reaches `main` through the boundary publish, never from a lane mid-wave. | F2.10 |
| Current practice: learn from what went right, build the timeline from records, keep lessons in reviewed Markdown. | P3.1, P3.3, P3.7 |

## 3. Goals and non-goals

Goals, in order:

1. **G1** Each recovery has a document that shows each failed try, the passed try and the difference between them.
2. **G2** Each judgment lesson has a document. Its todo leaves the carryover log only when the document is on `main`.
3. **G3** Scout, tdd, build and audit still find each lesson through the existing recall.
4. **G4** The build adds no LLM call, and the first PR changes nothing on `main` (stage `shadow`).

Non-goals:

- The design does not change the failure retrospective phase or its lessons.
- The design does not add a per-try history field to the inbox item. The dossiers already hold the history (F2.5).
- The design does not judge why the passed try passed. It states the facts and the differences. A person or a later stage adds the judgment.

## 4. Candidate designs

**A. The host writes the documents from records.** At the closeout of a PASS cycle, the host reads the committed dossiers of each item that the cycle shipped. If two or more FAIL dossiers name the item, it renders a recovery document. At each closeout it also renders a document for each judgment-lesson todo.

Stage `shadow` writes into the workspace. Stage `enforce` publishes through the dossier boundary path and then retires the judgment todos.

**B. An LLM retrospective phase on recovery.** `retroApplies` also fires on a PASS after two or more failures. The retrospective agent reads the dossiers and writes a narrative and a lesson YAML. Judgment lessons go to the same agent at its next run.

**C. A lesson only, no document.** On a recovery, the host writes one lesson YAML into `.evolve/instincts/lessons/`. A judgment-lesson todo gets the 30-day carryover TTL, like the other failure-learning todos of PR #1081.

**D. Records first, an LLM paragraph later.** Design A, plus one LLM call at enforce that adds a "why it passed" paragraph to each recovery document.

Each criterion has a weight. The score is 1 (poor) to 5 (good).

| Criterion | Weight | A | B | C | D |
|---|---|---|---|---|---|
| Fidelity: the record of each try is exact (F2.6) | 3 | 5 | 3 | 1 | 5 |
| Cost: no extra LLM tokens (G4, repository rule) | 3 | 5 | 1 | 5 | 3 |
| The operator ask: documents under `docs/`, the todo removed (§1) | 3 | 5 | 4 | 1 | 5 |
| The recall signal for scout and triage stays (G3) | 2 | 4 | 4 | 4 | 4 |
| Safe removal of the todo (G2) | 2 | 5 | 3 | 2 | 5 |
| Build size and risk | 1 | 4 | 2 | 5 | 3 |
| The depth of the lesson: the "why" | 1 | 2 | 5 | 1 | 4 |
| **Total** | | **69** | **45** | **39** | **64** |

**A wins.** It is exact, it costs no tokens, and it matches the operator ask. B costs one agent run for each recovery, and an agent can misread a fingerprint. C does not write a document, so it fails the operator ask, and a TTL drops a lesson that nobody wrote down. D is A plus a cost. Its paragraph is the one gain, and A keeps a place for it.

**The grafts from the other designs:**

- From B and D: each document has a "Judgment" section with a fixed placeholder line. The console or a later LLM stage fills it. A does not block on it.
- From C: each document has a sibling lesson YAML. The recall reads it, so G3 holds with no prompt change.
- From PR #1081: a judgment todo that has no document keeps its retention until the document lands. A lesson is never dropped without its document.

## 5. Decisions

| Id | Decision | Because (findings) | Rejected options and why |
|---|---|---|---|
| D1 | The trigger is a ship of the item after two or more FAIL dossiers that name it. The count starts at its last ship, in a window of 400 cycles. A PASS with no commit and a WARN are neither a failure nor a reset. | F2.1, F2.2, F2.4, F2.12 | `failure_count`: it resets and is 0 or 1 today. Any PASS as a ship: cycle 1641 then gets a false document, and the real ship 1647 gets none. |
| D2 | The history comes from `knowledge-base/cycles/` and `.evolve/dossiers-pending/`. | F2.5, F2.10 | Run dirs: the GC deletes them. The ledger: it has no class or fingerprint, and it counts a release as a try (F2.11). |
| D3 | A recovery document has one try row for each FAIL dossier and one for the PASS dossier. | F2.6, P3.3 | A summary only: it hides the fingerprints. |
| D4 | A row with the class `infra-error` or `unknown` (the constants of `internal/dossier`, which the classifier also uses) gets the mark "infrastructure?". | F2.6, F2.13, P3.4 | No mark: the reader takes an infrastructure failure for a defect of the item. |
| D5 | The difference section compares the PASS dossier with the last FAIL dossier. It shows the phases, the models and the ladder rungs. | P3.5, P3.6 | A full diff of the code: the PASS commit already holds it, and the document cites the commit. |
| D6 | A judgment-lesson document has the base name of the todo id: `cycle-<N>-judgment-<phase>`. A recovery document has `cycle-<N>-recovery-<slug>`. The slug is the item id in lower case, with each other run of characters as one `-`. It has at most 80 bytes, and it is `item` when empty. Two items with one name write one pair and report the collision. | F2.8 | A date in the name: the same lesson can then get two names. |
| D7 | Each document has a sibling lesson YAML with the same base name. | F2.9 | A prompt change for scout: it adds a second recall path. |
| D8 | The compiled stage is `shadow`. It writes the documents into `<workspace>/retrospectives/` and removes nothing. | G4 | A live default: the operator must see the output first. |
| D9 | At `enforce`, the documents go to `.evolve/retrospectives-pending/`, and the boundary publish commits them with the dossiers. | F2.10 | A lane commit: it moves `main` mid-wave. |
| D10 | At `enforce`, a judgment todo leaves the log only when `docs/retrospectives/<id>.md` exists in the published tree. The check repeats at each closeout. The publish is create-only (D13). | G2 | Removal at write time: a failed publish then drops the lesson. |
| D11 | The host text in each document follows the STE house rules. Agent text appears only as a quoted code span. | ste100 convention | Free agent text: the strict lint fails the publish. |
| D12 | A write failure prints a WARN line and never fails the closeout. | dossier precedent | A fatal error: a report must not stop a cycle. |
| D13 | Each document write is create-only: a file that exists stays as it is. | R10, G2 | An overwrite: the next closeout erases the "Judgment" text of a person. |
| D14 | The action of the lesson YAML names the document in one sentence with no period, and its pattern holds the slug of the item. | R11, F2.9 | The raw path and id: `firstSentence` cuts the recall digest at `.md`, and an agent-written id reaches the scout prompt. |
| D15 | A corrupt or unreadable dossier of the current cycle is an error, so the closeout prints a WARN. A corrupt history dossier is still skipped with no report (limit). | D12 | A silent skip: a lost recovery leaves no trace. |

**What changes our mind:** look at the first 20 recovery documents, over both stages. If most of them show no difference between the two tries, a fact table alone does not teach. Then D adds its LLM paragraph.

## 6. Components

| Id | Component | Red test first | Stage |
|---|---|---|---|
| C1 | `internal/recoveryretro`: `Recoveries` and `RenderRecovery`. It reads the dossiers and renders the try rows, the infrastructure marks and the difference section. | A history with three FAILs and one PASS renders three FAIL rows, one PASS row and one mark. One FAIL gives no recovery. | shadow (this PR) |
| C2 | `internal/recoveryretro`: `JudgmentLessons` and `RenderJudgmentLesson`, plus `LessonYAML` for both kinds. | Only `cycle-<N>-judgment-<phase>` todos are selected. The YAML parses in `internal/research` and its keywords match. | shadow (this PR) |
| C3 | `WriteShadow`, called at the closeout of each cycle. It writes into `<workspace>/retrospectives/` only. | A PASS closeout after two FAILs writes one recovery pair. A FAIL closeout writes only the judgment pairs. Nothing is written outside the workspace. | shadow, on `main` |
| C4 | The stage key `retrospectives.stage` and the pending dir. The boundary publish commits the pairs into `docs/retrospectives/`. | At `enforce`, a pair goes to the pending dir, and the publish commits it with the dossiers. | the key is built (2026-10-27). The pending dir and the publish are open: inbox item `recovery-retrospective-c4-stage-key-publish` (2026-10-27). |
| C5 | The judgment-todo retirement after the publish, through `carryover.Retire`. | A todo without its document on `main` stays. A todo with its document leaves. The second run changes nothing. | enforce. Inbox item `recovery-retrospective-c5-judgment-todo-retire` (2026-10-27). |
| C6 | `docs/retrospectives/` joins the default recall roots. | A lesson YAML there is recalled for a matching goal. | enforce. Inbox item `recovery-retrospective-c6-recall-root` (2026-10-27). |

## 7. Order

1. C1 and C2 first. They are pure functions, so their tests are exact.
2. C3 next. It is the only wiring in this PR, and it writes only into the workspace.
3. The operator reads the shadow documents of the next recoveries, then moves the stage.
4. C4, C5 and C6 come together in one PR. The retirement (C5) needs the publish (C4), and the publish needs the recall root (C6) to keep G3.

## 8. Verification

| Goal | Measure | Before | Target |
|---|---|---|---|
| G1 | Recovery documents for the recoveries of a wave (shadow files) | 0 | one for each recovery whose tries have dossiers with `tasks` |
| G1 | The worked example: `inboxbatch-utf8-and-resolution` (F2.6) renders three FAIL rows and marks cycle 1806 | — | yes, in a test with the real dossier values |
| G2 | Judgment todos in `state.json` with no document on `main` (enforce) | all | 0 after one boundary |
| G3 | A recall query with the item keywords returns the lesson YAML | — | yes, in a test |
| G4 | LLM calls added by the feature | — | 0 |
| G4 | Files that the shadow stage writes outside the workspace | — | 0, in a test |

Each new and changed function has 100% measured coverage. The PR body has the table.

## 9. Risks

| Risk | Control |
|---|---|
| A recovery that was luck, not a fix (P3.4) | The difference section says "no difference in phases or models" when it finds none. |
| The 400-cycle window misses an old failure | The document gives the window and the count that it found. |
| A pending FAIL dossier of a sibling lane is not yet committed | D2 reads the pending dir too. |
| Agent text breaks the markdown or the STE lint | D11: the renderer puts it in a code span and removes backticks and line breaks. |
| The judgment todo is removed and the document is lost | D10: removal only after the document is in the published tree. |
| Two closeouts write the same document | At `shadow`, each closeout writes a fresh copy into its own workspace. At `enforce`, the base name is fixed (D6), so the second write replaces the first with the same content. |

## 10. Limits

- The document states facts. It does not prove why the passed try passed.
- A try before cycle 1624 has no row: those dossiers have no `tasks` (F2.11). A try outside the window has no row.
- A FAIL dossier names each item of the cycle. A multi-item FAIL thus counts for each item, also an item that it did not touch.
- `dossier.ReadDir` skips a corrupt history dossier with no report, so a corrupt FAIL can hide one failure.
- The ladder rungs are `R0` until the ladder leaves `shadow` (retry-ladder spec §6). The rung line then shows no climb.
- A test cannot prove that a person reads the documents. The operator review at `shadow` is that check.

## 11. Open questions

- **Q1 (operator, decided 2026-10-27):** when does the stage move to `enforce`? The operator approved the move to `enforce` on 2026-10-27. The first recommendation was to wait for five recovery documents at `shadow`.
- **Q2 (operator, pending):** does a judgment todo keep its long retention until its document lands (C5)? Recommendation: yes, because a TTL alone drops a lesson that has no document.
