# Plan: each issue, alert or mistake gets an incident record through one entry skill

- **Status:** built, 2026-11-17 (K1 to K3 on main; K4 to K6 superseded by the incident knowledge-base plan). Earlier status: proposed, 2026-11-16. Components K1 to K3 are built. Amended 2026-11-18: [the incident knowledge-base plan](incident-knowledge-base-2026-11.md) replaces K4, K5 and K6 with its components I4, I5 and I7. Do not build K4 to K6.
- **Design:** the skills [issue-record](../../skills/issue-record/SKILL.md) (the entry point), [issue-analyze](../../skills/issue-analyze/SKILL.md) (the cause) and [docs-write](../../skills/docs-write/SKILL.md) (the text, unchanged). **Decision:** no ADR; the decisions D1 to D8 are in §5. They add skills and one rule, and change no runtime contract. **Research:** [issue-record-skills-2026-11.md](../research/issue-record-skills-2026-11.md).
- **Rule:** [guidance/rules/issue-gets-a-record.md](../../guidance/rules/issue-gets-a-record.md). **First record:** [2026-11-16-tests-leak-evolve-ledgers-under-go.md](../incidents/2026-11-16-tests-leak-evolve-ledgers-under-go.md).

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
10. [Open questions](#10-open-questions)

## 1. Request

The operator wrote on 2026-11-16:

> "I intend to create a detailed document to record all the mistakes and issues encountered during development, including the incident reports, key logs, results, and the corresponding fixes, as everything needs to be documented for future learning. Write this in the system prompt and build the skills to be invoked when there is mistake or issue happened."

> "By creating doc, I mean it should be a longterm skill that be triggered when there is an issue or alert or mistakes detected"

> "The issue documenting skill should be coupled with signal center notification and also another analysis skill + document writer skill, research online how other open sources skill we can refer to"

## 2. Facts

| Fact | Research id |
|---|---|
| One incident template exists, in docs-write. | F1.1 |
| The regression index exists, but the records of 2026-11-05 and 2026-11-09 have no row. | F1.2 |
| No skill starts on an issue outside a cycle FAIL. | F1.3 |
| No skill holds a root-cause method; fable and superpowers give the principle. | F1.4 |
| A signal holds the cycle, the code, the severity and the fields that a stub needs. | F1.5 |
| `events watch` is built; `events subscribe` (exec delivery) is not. | F1.6 |
| The self-heal store has 39 open machine records with no document and no test. | F1.7 |
| A WARN signal started the issue of 2026-11-16. | F1.8 |
| Open-source postmortem skills put the analysis and the text in one skill. | P2.2 |
| Triggers are objective; a record is blameless and holds no personal data. | P2.3 |
| Triggers route to a level; quotas make empty documents. | P2.4 |
| A tool, not the author, attaches the logs. | P2.5 |
| Five Whys misses causes that act only together; a causal chain keeps them. | P2.6, P2.7 |

## 3. Goals and non-goals

Goals:

- **G1.** Each issue, alert or mistake gets a record: a full incident record, or one row in a dated session log.
- **G2.** A record holds the timeline, the key logs verbatim, the results, the root cause, the fix, the regression test and the lesson.
- **G3.** The console and each loop agent can start the procedure. The rule in the L0 digest tells each agent to start it.
- **G4.** A WARN or INCIDENT signal of the Signal Center leads to the procedure.
- **G5.** Each fix with a test adds a row to the regression index.

Non-goals:

- No second incident template and no second document writer. docs-write stays the one writer.
- No change to `retro`, to `evolve-retrospective` or to the phase `incident-postmortem`. They stay the procedures inside a cycle.
- No Go code in this change. The stub verb and the alert hint are later components (K4 to K6).

## 4. Candidate designs

| Criterion (weight) | A: one skill does all | B: three skills that compose | C: a verb writes the whole record |
|---|---|---|---|
| One home for each job (3) | 1: a second template beside docs-write | 3: record, cause and text each have one home | 2: the text lives in Go templates |
| Quality of the cause (3) | 2: the method is a short section | 3: one skill owns the method | 1: a verb cannot judge a cause |
| Cost to start (2) | 3 | 2: three skills to read | 1: code, tests and review first |
| Alert link (2) | 2 | 3: the entry skill names the watch now and the stub verb later | 3 |
| **Weighted total** | 18 | 27 | 17 |

B wins. A copies the template of docs-write (F1.1) and puts the method in a section that no other caller can reuse. C gives deterministic work to code, which is right for the logs (P2.5). But a verb cannot judge a cause (rule 5 of the core rules). B takes the good part of C as the later stub verb K4.

A fourth option, D, makes the phase `incident-postmortem` the entry point. It runs only inside a cycle (F1.3), so a console mistake or an operator correction never reaches it. D is rejected as the entry point. The phase can load `issue-analyze` later (open question Q2).

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Three skills: `issue-record` is the entry, `issue-analyze` finds the cause, `docs-write` writes the text. | F1.1, F1.4, P2.2 | One skill (A): a second template. |
| D2 | Seven triggers start the skill. They are a failed test or CI red, a gate refusal, a WARN or INCIDENT signal and a self-heal incident. The others are a wrong claim, a revert and an operator correction. | F1.3, F1.8, P2.3 | INCIDENT only: the issue of F1.8 was a WARN. |
| D3 | Two levels. A small mistake is one row in `docs/incidents/<yyyy-mm-dd>-session-log.md`. Each other issue gets its own record. | P2.4 | One file for each mistake: a quota of empty records. No record for small mistakes: the recurrence is not visible. |
| D4 | A mistake is small only when five tests are true. (1) One action fixed it. (2) No code, test or doc changed. (3) It cost under 15 minutes. (4) It did not reach main, a lane or the operator. (5) It is the first row of its cause in the log; a second row promotes it to a record. | P2.4, synthesis | A time limit only: a fast mistake that recurs stays hidden. |
| D5 | The running index is `docs/incidents/REGRESSION-COVERAGE-INDEX.md`. Each failure mode with a fix gets a row, and the summary counts change in the same edit. A session log gets no row. | F1.2 | A new `docs/incidents/index.md`: a second index of the same records. |
| D6 | The analysis writes a causal chain, each contributing cause, and the escape: the control that did not catch the issue. Five Whys is only a first probe. | P2.6, P2.7 | Five Whys only: one root, and analysts disagree. Full CAST: too much cost for one defect. |
| D7 | The alert link now: the console watches the lossless channels `errors` and `selfheal.incident` and starts `issue-record` on a WARN or INCIDENT that is new. Later: the stub verb K4 and the hint K5. | F1.5, F1.6 | An `exec` subscription now: `events subscribe` is not built. |
| D8 | No secret, token, account name, email address or person name goes into a record. Log lines are trimmed and redacted. | P2.3 | Raw logs: a record is public in the repository. |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| K1 | `skills/issue-record/SKILL.md` and `skills/issue-analyze/SKILL.md`, listed in `.claude-plugin/plugin.json`, with stubs from `evolve skills generate` | `evolve skills check` fails until the manifest lists each skill (`go/internal/skillcheck/manifest.go`) | console, this change |
| K2 | `guidance/rules/issue-gets-a-record.md`, rendered by `evolve guidance build` into the L0 digest | `evolve guidance check` reports a stale `guidance/index.md` until the build runs | console, this change |
| K3 | The first record and its regression index rows | none: a document | console, this change |
| K4 | Replaced by I4 of the incident knowledge-base plan. `evolve issue stub --signal <cycle>:<seq> \| --incident <id>`: writes `docs/incidents/<date>-<slug>.md` with the five sections of the template. It fills the header, the timeline rows and the verbatim lines of the signal; the cause sections stay as `TODO(issue-analyze)`. | `TestIssueStub_ASignalGivesARecordWithItsCodeCycleAndTimeline`; `TestIssueStub_RedactsATokenInAField`; `TestIssueStub_RefusesAnExistingRecord` | lane, later |
| K5 | Replaced by I5 of the incident knowledge-base plan. The Signal Center hint: an `errors` or `selfheal.incident` event of WARN or more prints the line `next: issue-record` in `events watch` text output. The skill name comes from policy data, not code. | `TestFormatLine_AWarnEventNamesTheRecordSkill`; `TestFormatLine_AnInfoEventHasNoHint` | lane, later |
| K6 | Replaced by I7 of the incident knowledge-base plan. A self-heal incident record gets the field `record_doc`. A close with `repro_test` and no `record_doc` is refused. | `TestIncidentClose_RefusesWithoutARecordDoc` | lane, later |

## 7. Order

1. K1 and K2 first, because the rule must name a skill that exists.
2. K3 next. It is the worked example of the skill, and it proves the procedure on a real issue.
3. K4 after the review of this plan. The stub is deterministic, so it goes to code (rule 5).
4. K5 after K4, so that the hint can name the stub verb too.
5. K6 last. It needs the store schema change of `internal/selfheal`.

## 8. Verification

| Goal | Measure | Before | After K1 to K3 |
|---|---|---|---|
| G1 | records in `docs/incidents/` for the issues of 2026-11-16 | 0 | 1 |
| G2 | sections of the first record that the template requires | not applicable | 5 of 5 |
| G3 | the L0 digest of `AGENTS.md` and `CLAUDE.md` names `issue-record` | no | yes |
| G4 | a WARN signal leads to a record | by hand | by the watch now; by K4 and K5 later |
| G5 | 2026-11 records with an index row | 0 of 2 | 1 of 3 |

Checks for this change: `evolve docs ste-lint --strict --changed refs/remotes/origin/main --added`, `evolve skills check`, `evolve guidance check`, and `go test -count=1` on `internal/guidance`, `internal/skillcheck` and the skills tests of `cmd/evolve`.

## 9. Risks

| Risk | Control |
|---|---|
| Agents skip the skill on a small mistake. | The L0 rule names the skill; the session log makes the cost of a row small. |
| Records copy secrets from logs. | D8; the skill has a redaction step; K4 redacts in code. |
| The index counts drift from the rows. | D5: the summary changes in the same edit; a later check can count the rows. |
| Records pile up with no fix. | A record without a fix keeps its row at ❌ in the index, so the gap is visible. |
| The analysis fixes a symptom. | D6: the escape question finds the class fix. |

## 10. Open questions

- **Q1.** Does the docs-review gate need a light path for a session-log row? Today each `.md` diff needs a docs review.
- **Q2.** Does the phase `incident-postmortem` load `issue-analyze` as its method? That change touches a phase persona, so it needs its own review.
- **Q3.** The memory index of the console holds many lessons as feedback files. Does each old lesson get a record? This plan does not move them.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| K1 skills `issue-record` and `issue-analyze` | done, 2026-11-17 | [#1534](https://github.com/mickeyyaya/evolve-loop/pull/1534) [`24c4f40`](https://github.com/mickeyyaya/evolve-loop/commit/24c4f40ab) (`skills/issue-record/SKILL.md`, `skills/issue-analyze/SKILL.md`) | none |
| K2 rule `issue-gets-a-record` | done, 2026-11-17 | [#1534](https://github.com/mickeyyaya/evolve-loop/pull/1534) [`24c4f40`](https://github.com/mickeyyaya/evolve-loop/commit/24c4f40ab) (`guidance/rules/issue-gets-a-record.md`) | none |
| K3 first record and index row | done, 2026-11-17 | [#1534](https://github.com/mickeyyaya/evolve-loop/pull/1534) [`24c4f40`](https://github.com/mickeyyaya/evolve-loop/commit/24c4f40ab); index row at `REGRESSION-COVERAGE-INDEX.md:52` | none |
| K4 `evolve issue stub` | superseded by I4 of the incident knowledge-base plan; no `issue stub` code on main | none | incident-kb-i4-capture-sweep |
| K5 Signal Center hint | superseded by I5 of the same plan; no hint code on main | none | incident-kb-i5-levels-and-due |
| K6 `record_doc` field | superseded by I7 of the same plan; built, PR open | [#1624](https://github.com/mickeyyaya/evolve-loop/pull/1624) | incident-kb-i7-record-doc |

K1 to K3 are on main. K4 and K5 move to the incident knowledge-base plan and stay open as inbox items. K6 is built in PR #1624 and not yet merged. The plan has no component that is unfinished and has no open item. The session-log file named in D3 does not exist yet: `docs/incidents/` has no `session-log` file.
