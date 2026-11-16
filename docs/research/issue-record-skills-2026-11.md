# Issue records: local evidence and current practice for the record, the analysis and the alert link (research, 2026-11)

> The plan that uses this research: [issue-record-skill-2026-11.md](../plans/issue-record-skill-2026-11.md). The skills: [issue-record](../../skills/issue-record/SKILL.md), [issue-analyze](../../skills/issue-analyze/SKILL.md) and [docs-write](../../skills/docs-write/SKILL.md). The first record: [2026-11-16-tests-leak-evolve-ledgers-under-go.md](../incidents/2026-11-16-tests-leak-evolve-ledgers-under-go.md).

Date: 2026-11-16. Method: we read the skills, the agents, the incident records, the Signal Center and the self-heal store. We ran `evolve events channels` and `evolve selfheal incidents`. We did live web searches and page reads on 2026-11-16. Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Current practice outside this repository](#2-current-practice-outside-this-repository)
3. [Adopt or reject](#3-adopt-or-reject)
4. [Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

**F1.1** The incident template exists in one home: `skills/docs-write/references/templates.md` §Incident. It has five sections: Summary, Timeline, Root cause, What changed and What we learned. The home of a record is `docs/incidents/<date>-<topic>.md` (`skills/docs-write/SKILL.md`, the type table).
*Implication:* a new skill must not hold a second incident template. It hands the text to docs-write.

**F1.2** `docs/incidents/REGRESSION-COVERAGE-INDEX.md` maps each incident to its failure modes and to the test that pins each mode. Its section "Contract for new incidents" says: "A fix without a regression row is incomplete." On 2026-11-16 it has 94 rows for 57 incidents. The two records of 2026-11-05 and 2026-11-09 have no row (`grep -n '2026-11' docs/incidents/REGRESSION-COVERAGE-INDEX.md` prints nothing).
*Implication:* the running index exists, but no procedure makes an author add the row. The new skill must make the row a step.

**F1.3** No skill starts on an issue. `skills/retro` starts after a ship or an audit FAIL of a cycle. `agents/evolve-retrospective.md` starts only on an audit FAIL or WARN. The optional phase `incident-postmortem` (`.evolve/phases/incident-postmortem/phase.json`, `catalog: on-demand`) runs inside a cycle when an incident recurs.
*Implication:* a mistake of the console, an operator correction, a CI red on main and a wrong claim have no record procedure today.

**F1.4** No skill holds a root-cause method. `skills/fable/SKILL.md` §2 "Root cause or nothing" gives the principle: fix the instance and the class. The superpowers plugin on this host has the skill `systematic-debugging` with `root-cause-tracing.md`, and the rule "NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST". Its worked example is an empty `projectDir` that made `git init` write `.git` into a source directory.
*Implication:* the analysis skill links these two sources. It adds the escape question (which gate did not catch the issue) and the output shape for the record.

**F1.5** The Signal Center has three severities: INFO, WARN and INCIDENT (`go/internal/signalcenter/event.go:21-23`). A signal has `cycle`, `phase`, `module`, `kind`, `code`, `severity`, `reason` and `fields` (`event.go:225-239`).
*Implication:* a record stub can take its header and its first timeline rows from the signal, with no LLM.

**F1.6** `evolve events channels` (2026-11-16) lists the lossless channels `errors` (`severity>=WARN`), `selfheal.incident` and `quality.breach`. `evolve events watch --channel C --filter EXPR` is built. `evolve events subscribe` is not built: the binary prints `unknown verb "subscribe"`. `docs/architecture/event-channels.md` §13 says that `subscribe` waits for the subscription store of E8.
*Implication:* an alert can reach the console now only through a watch. An `exec` delivery that writes a stub is a later component.

**F1.7** `evolve selfheal incidents` (2026-11-16) lists 39 open records in `.evolve/incidents/inc-*.json`. A record has `id`, `class`, `fingerprint`, `sources`, `recurrence`, `repro_test` and `summary`. The `repro_test` field is empty in each record (`docs/architecture/packages/internal-selfheal.md`, "the record").
*Implication:* the self-heal store holds machine records with no human document and no regression test. The incident record of docs/ is the human half. A link in each direction joins them.

**F1.8** The event that started this request: on 2026-11-16 the composed-tree `make test` of cycle 2419 failed with four `LEAK` paths under `go/internal/phases/triage/.evolve`. The signal is `ORCHESTRATOR_COMPOSED_GATE_DECLINED` (WARN) at 00:50:27Z in `.evolve/runs/cycle-2419/signals.ndjson`. CI on main stayed green, because `make test-integration` has no treewalk guard (`go/Makefile:107-127`).
*Implication:* a WARN signal can start an issue of high cost. The trigger must include WARN, not only INCIDENT.

## 2. Current practice outside this repository

**P2.1** Agent Skills format. A skill is a folder with a `SKILL.md` that starts with a `name` and a `description`. The agent loads only the name and the description at start, and the full text when a task matches (progressive disclosure). The format is an open standard at agentskills.io.

- Sources: [Anthropic engineering: Equipping agents for the real world with Agent Skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills), [agentskills README](https://cdn.jsdelivr.net/gh/agentskills/agentskills@main/README.md), [anthropics/skills loading system (mirror)](https://www.mintlify.com/anthropics/skills/spec/loading-system).
*Implication:* the description is the trigger. It must name each trigger word that the agent sees in an issue: "test failure", "CI red", "WARN", "INCIDENT", "operator correction".

**P2.2** Open-source postmortem skills for Claude Code exist in several collections:

- `postmortem-writing` (popey/claude-code-skills and wshobson);
- `post-mortem` (borghei/Claude-Skills): 5 Whys for a clear chain, and a causal tree for an incident with several causes;
- `incident-postmortem` (mohitagw15856/pm-claude-skills and majiayu000/claude-skill-registry).

- Sources: [Tessl: postmortem-writing](https://tessl.io/registry/skills/github/popey/claude-code-skills/postmortem-writing), [skills.sh: borghei post-mortem](https://www.skills.sh/borghei/claude-skills/post-mortem), [openskillindex: pm-claude-skills incident-postmortem](https://openskillindex.com/skills/mohitagw15856-pm-claude-skills-incident-postmortem-5e681a).
- We read only the index pages, not the repositories.
*Implication:* each of these skills writes the document and does the analysis in one skill. None of them starts from a machine alert or ends in a regression test. We take the method split (5 Whys or causal tree) and not the one-skill shape.

**P2.3** Google SRE, "Postmortem Culture". The triggers are user-visible impact, data loss, an on-call intervention such as a rollback, a long resolution, and a monitoring failure. Any stakeholder can ask for a postmortem. A postmortem names the contributing causes "without indicting any individual or team". "An unreviewed postmortem might as well never have existed." A postmortem never holds information that identifies an end user.

- Source: [sre.google: Postmortem Culture](https://sre.google/sre-book/postmortem-culture/).
*Implication:* the trigger list is objective and includes "a monitoring failure". The record is blameless, reviewed, and has no personal data.

**P2.4** Trigger criteria route an event to a level, not to a yes or a no. A quota of postmortems makes empty documents. Source: [OneUptime: Which incidents need a postmortem?](https://oneuptime.com/blog/post/2026-07-30-postmortem-trigger-criteria/markdown).
*Implication:* a small mistake gets a short level (a row in a session log), not a full record and not silence.

**P2.5** Etsy Morgue held each postmortem with its timeline, its chat logs, its graphs and its tickets. It pulled the logs from the chat server. The repository is archived since 2019-12-18.

- Source: [github.com/etsy/morgue](https://github.com/etsy/morgue).
*Implication:* a tool, not the author, attaches the logs. Our stub verb takes the signals from the run directory.

**P2.6** Five Whys assumes that one cause is enough for each symptom. Different analysts reach different roots for one incident. A causal tree keeps several paths and the failed barriers.

- Sources: [Kitchen Soap (Allspaw, 2012)](https://www.kitchensoap.com/2012/02/10/each-necessary-but-only-jointly-sufficient/), [OneUptime: Five Whys or causal tree?](https://oneuptime.com/blog/post/2026-07-31-five-whys-vs-causal-tree/markdown).
*Implication:* the analysis skill uses Five Whys only as a first probe. It writes a causal chain with each contributing cause and each control that failed.

**P2.7** CAST (causal analysis based on systems theory, Leveson) looks for the weak control in the whole system. It explains unsafe actions by the design of the system, not by fault of a person. Source: [UL: Introduction to STAMP, STPA and CAST](https://ul.com/sis/blog/introduction-to-stamp-stpa-and-cast).
*Implication:* the analysis asks "which control (gate, test, guard, review) let the issue through, and why". That question finds the class fix.

**P2.8** PagerDuty moved its postmortem feature to a Post-Incident Review and keeps the guide at its ops guides. The page that we read gives no template details. Source: [PagerDuty: Postmortem best practices](https://docs.pagerduty.com/ops-guides/postmortem-best-practices). The Atlassian handbook page returned no body text to our fetch. We cite neither for a rule.

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Name and description trigger (P2.1) | adopt | The harness selects a skill by its description. |
| One skill writes and analyzes (P2.2) | reject | Analysis and text are separate jobs; docs-write already owns the text (F1.1). |
| Objective triggers, any person can ask (P2.3) | adopt | Each trigger maps to a signal, a gate or an operator message. |
| Blameless text, no personal data (P2.3) | adopt | Records are public in the repository. |
| Levels, not yes or no (P2.4) | adopt | The session log is the low level. |
| A tool attaches the logs (P2.5) | adopt, later | The stub verb is code; it is component K4 of the plan. |
| Five Whys only (P2.6) | reject | It misses causes that act only together. |
| Causal chain with failed controls (P2.6, P2.7) | adopt | The failed control gives the class fix and the regression test. |
| Full CAST (P2.7) | reject | It costs too much for a software defect of one session. We keep its control question. |

## 4. Refinements to adopt

- **R1** Three skills that compose: `issue-record` (trigger, evidence, index), `issue-analyze` (cause), `docs-write` (text). From F1.1, F1.4, P2.2.
- **R2** The trigger list names WARN and INCIDENT signals, gate refusals, CI red, operator corrections, wrong claims and reverts. From F1.3, F1.8, P2.3.
- **R3** Two levels: a full record, or one row in a dated session log. A second occurrence promotes the row to a record. From P2.4.
- **R4** The analysis ends in the escape question: which control did not catch the issue. From F1.8, P2.7.
- **R5** Each record with a fix adds a row to the regression index. From F1.2.
- **R6** A record links its self-heal incident id, and a later verb writes the stub from the signals. From F1.5, F1.6, F1.7, P2.5.
- **R7** No secret, token, account name or address of a person goes into a record. From P2.3.
