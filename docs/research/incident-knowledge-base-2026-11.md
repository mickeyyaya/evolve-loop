# Research: incident reports as a knowledge base that traces back to the evidence

- **Status:** research, 2026-11-18.
- **Plan:** [incident reports in the knowledge base](../plans/incident-knowledge-base-2026-11.md). **Earlier work:** [the issue-record research](issue-record-skills-2026-11.md) and [the issue-record plan](../plans/issue-record-skill-2026-11.md), which built the skills `issue-record` and `issue-analyze`.

## Table of contents

1. [Request](#1-request)
2. [Terms used throughout](#2-terms-used-throughout)
3. [Local evidence](#3-local-evidence)
4. [Current practice outside this repository](#4-current-practice-outside-this-repository)
5. [Refinements](#5-refinements)
6. [Sources](#6-sources)

## 1. Request

The operator wrote on 2026-11-18:

> "Have we build an incident report skill that will be invoked when any blocking or stale issues occurred? It must cover the details of history, the cause, the analysis, the approaches we took, how it takes effects, the observation how the issue and solutions perform, ...etc. Research online to learn what and how the structure of report, items, actions, and sections should be inlcuded"

> "Build the complete feature on the report structure around the signal center and I want the incident report be part of knowledge base, which can be trace back to the root, where we recorded all this logs and historical data."

## 2. Terms used throughout

- **Record:** one incident report in `docs/incidents/`.
- **Evidence:** the raw data that proves a claim of a record: signal lines, gate text, ledger entries, run files.
- **Bundle:** a committed copy of the evidence of one record, with a manifest of SHA-256 digests.
- **Anchor:** a reference that stays valid: the SHA-256 of a ledger line (with its `entry_seq` as a hint), a commit SHA, or a bundle digest.
- **Effectiveness check:** a measure, after a fix, of the recurrence of the issue. It is separate from the check that the fix was built.

## 3. Local evidence

**F3.1** The skill `issue-record` exists with eight steps: capture, trim and redact, analyze, fix, write, index, link, check (`skills/issue-record/SKILL.md`). Its triggers are a failed test, a gate refusal, a WARN or INCIDENT signal and a self-heal incident. A wrong claim, a revert and an operator correction are triggers too.
*Implication:* the procedure exists. A blocked or stale state is not a trigger.

**F3.2** The record template has five sections: summary, timeline, root cause, what changed, what we learned (`skills/docs-write/references/templates.md:127`).
*Implication:* the template has no section for the contributing factors or the approaches that the team tried. It has none for owned actions or for the effect of the fix.

**F3.3** The issue-record plan has three components that are not built (`docs/plans/issue-record-skill-2026-11.md:98`). They are K4 (`evolve issue stub`), K5 (a `next: issue-record` hint in `evolve events watch`) and K6 (a `record_doc` field on a self-heal incident). The registry has no `issue` verb.
*Implication:* a person starts each record by hand, and no code joins a record to a signal or to a self-heal incident.

**F3.4** `docs/incidents/` holds 101 records and `REGRESSION-COVERAGE-INDEX.md`. No record has frontmatter. The headings are free-form ("## Root cause" in 54 files, "## What happened" in 36, "## Timeline" in 24). No Go package parses a record. `go/internal/repocontract/datafiles.go:225` names one record path, as test-selection data only.
*Implication:* a program cannot query the records, so they are not yet a knowledge base.

**F3.5** The signal center writes each event of a cycle to `.evolve/runs/cycle-N/signals.ndjson` (`go/internal/signalcenter/event.go:222`), and each event to the channels `.evolve/events/ch/<name>/` (`go/internal/paths/paths.go:115`). An event has a `seq`, a `code`, a `severity` and up to 12 `fields` (`go/internal/signalcenter/event.go:224`). Neither store has a hash chain.
*Implication:* a signal has a stable address, `<cycle>:<seq>`, but no proof that it did not change.

**F3.6** `.gitignore` excludes `.evolve/*` (`.gitignore:35`). The live policy deletes a run directory after 14 days (`.evolve/policy.json:151`), and keeps the 10 newest in full (`go/internal/gcpolicy/gcpolicy.go:50`). On 2026-11-18 the lowest cycle directory on disk was 2087.
*Implication:* a record that links to `.evolve/runs/cycle-N/` loses its evidence after about 14 days. A record must carry a copy.

**F3.7** The ledger `.evolve/ledger.jsonl` is hash-chained. Each entry has an `entry_seq` and the `prev_hash` of the line before it. A mismatch is an error (`go/internal/adapters/ledger/ledger.go:365`). Garbage collection protects the ledger and the content-addressed store `ledger-artifacts`.
*Implication:* a ledger entry is a durable anchor. A bundle can name the ledger entries of its cycles.

**F3.8** `knowledge-base/cycles/` holds 1646 committed cycle dossiers (a `.json` and an `.md` file each), written by `internal/dossier`. `evolve kb graph neighbors` and `evolve kb graph recurrences` query a graph of cycles, fingerprints, defects, lessons and items (`go/internal/kbgraph/kbgraph.go:19`). The graph does not read `docs/incidents/`. Lesson recall searches `knowledge-base/research/`, the lessons, `docs/research/` and `docs/retrospectives/`, not `docs/incidents/` (`go/internal/research/kb.go:58`).
*Implication:* the knowledge base exists and is committed, but records are outside it.

**F3.9** A self-heal incident (`go/internal/selfheal/incident.go:62`) has a fingerprint, a class, the source cycles, a recurrence count and `repro_test`. It is stored in `.evolve/incidents/`, which is gitignored. Its close needs a green repro test, a clear signal, a held quality reading and a retrospective.
*Implication:* the self-heal incident measures recurrence already. A record can use its fingerprint as the key for the effectiveness check.

**F3.9a** The self-heal intake is the one classifier of signal codes into incidents today. `selfheal.FromSignals` applies `signalRules` (`go/internal/selfheal/signals.go:20`), and the detector dedupes the observations by fingerprint. `selfheal.WriteEvidence` already collects the evidence of an incident into `.evolve/runs/selfheal-<id>/` (`go/internal/selfheal/evidence.go:50`). Each entry is an `inputindex.Entry` with a path, a cycle, a SHA-256, a class and a `why` field (`go/internal/inputindex/index.go:28`).
*Implication:* a second trigger list or a second manifest schema duplicates these. The workspace is under `runs/`, so it shares the 14-day deletion.

**F3.9b** The ledger accepts a re-genesis and fork siblings that share a `prev_hash` (`go/internal/adapters/ledger/ledger.go:361`). Garbage collection protects the content-addressed store `ledger-artifacts` (`go/internal/gc/gc.go:204`). The one redactor in Go is `panetrust.RedactSecrets` (`go/internal/panetrust/panetrust.go:41`), in a pane-trust package.
*Implication:* an anchor must be the digest of the ledger line itself, and `ledger-artifacts` can hold the evidence before a record opens. The redactor needs a leaf home.

**F3.10** Detectors of blocked work emit codes today: `LOOP_ORCH_POOL_IDLE`, `LOOP_ORCH_SUPPLY_SHORT`, `LOOP_ORCH_LANE_STALLED`, `LOOP_WAVE_ALL_LANES_STALE`, `LIVENESS_PHASE_STALLED`, `SIGNAL_EXPECTED_ABSENT` and `SHIP_INTEGRATION_EJECTED`. Three states have no code: a stale PR, a bounced inbox item, and a stale claim (`evolve inbox release --stale` emits `inbox.released` at INFO with no code).
*Implication:* most blocked states are visible to the signal center. Three need a detector.

**F3.11** On 2026-11-15 and 2026-11-16 the console filled `docs/research/pipeline-health-readings/<date>/` by hand with raw JSON and a `readings.md`. The note in `docs/research/pipeline-health-readings/2026-11-15/readings.md:5` says: "evolve gc removes them over time. This folder keeps a copy in git."
*Implication:* the need for a committed evidence copy is already proven. No verb does it.

## 4. Current practice outside this repository

**F4.1** A postmortem template has a summary, the impact, a timeline and the root cause. It also has the contributing factors, the action items and the lessons ([PagerDuty](https://response.pagerduty.com/after/post_mortem_template/), [incident.io](https://incident.io/blog/sre-incident-postmortem-best-practices), [adhorn](https://adhorn.substack.com/p/incident-postmortem-template-7b0e0a04f7a8)). Without the root cause, the issue does not occur. A contributing factor made the impact worse or the recovery slower ([blameless template](https://oneuptime.com/blog/post/2026-07-30-blameless-postmortem-template/view), [Blameless](https://www.blameless.com/blog/how-to-analyze-contributing-factors-blamelessly)).
*Implication:* the template needs a contributing-factors section, apart from the root cause.

**F4.2** Each action item has one owner (never a team), a due date and a tracking ticket. It has a type (prevent, detect, mitigate, process) and a "done means" check ([blameless template](https://oneuptime.com/blog/post/2026-07-30-blameless-postmortem-template/view), [incident.io](https://incident.io/blog/sre-incident-postmortem-best-practices)). AWS calls the action items the main result of a Correction of Error (COE). A COE has a summary, the impact, a timeline, metrics, event questions, the five whys, the action items and related items ([AWS COE](https://aws.amazon.com/blogs/mt/creating-a-correction-of-errors-document/), [re:Invent ARC308](https://d1.awsstatic.com/events/Summits/reinvent2022/ARC308_Improving-resiliency-with-the-correction-of-error-process.pdf)).
*Implication:* an action is data with fields, not prose. In this repository the ticket is an inbox item or a PR.

**F4.3** Corrective-action practice separates two checks: implementation verification confirms that the action was done. Effectiveness verification measures if the problem recurred, against criteria set in advance. It uses set windows, often 30 to 90 days, with more than one check. A closed action does not prove that it worked ([closing the CAPA is not closing the loop](https://hlc.com/en/publications/closing-the-capa-is-not-closing-the-loop), [Qualio](https://docs.qualio.com/en/articles/8130081-verification-of-effectiveness-voe-best-practice), [CAPA effectiveness checks](https://www.pharmaceuticalonline.com/doc/capa-effectiveness-checks-how-to-verify-that-your-actions-plans-are-successful-0001), [Bizzmine](https://www.bizzmine.com/resources/blogs/why-capa-closure-does-not-prove-capa-effectiveness/)).
*Implication:* a record needs a baseline, a target, check dates and a verdict. It stays open until the verdict.

**F4.4** Learning-from-incidents practice builds the timeline and a narrative of how the situation looked as it developed. It asks about the parts that nobody explored ([Howie guide](https://howie-guide.pagerduty.com/)). NIST SP 800-61 Rev. 3 (April 2025) moves lessons into a continuous "Improve" function, not one meeting at the end ([Linford](https://linfordco.com/blog/nist-sp-800-61/)).
*Implication:* the record keeps the approaches that the team tried and dropped, and why. The lessons feed a rule or a guard.

**F4.5** Objective triggers include impact above a threshold, a data or security risk, an emergency action, a repeat pattern and a high-potential near miss. A small event gets a light review, so that people keep reporting ([trigger criteria](https://oneuptime.com/blog/post/2026-07-30-postmortem-trigger-criteria/markdown), [Rootly](https://www.rootly.com/incident-postmortems)).
*Implication:* a blocked or stale state is a near miss or an impact. The trigger list is data with levels.

**F4.6** Records are kept in one searchable store with metadata. Actions are linked to one tracker, and the store is queried for patterns across incidents ([Rootly knowledge](https://rootly.com/sre/rootly-knowledge-reinforcement-captures-postmortem-insights), [making postmortems worth reading](https://oneuptime.com/blog/post/2026-07-31-make-postmortems-worth-reading/view)). Etsy's Morgue stores each postmortem with its links, graphs and chat ([Morgue](https://github.com/etsy/morgue)).
*Implication:* metadata in a fixed schema makes the records a knowledge base.

**F4.7** Keeping raw logs is not preserving evidence. Preservation also keeps the integrity, the time stamps and the chain of custody. The usual integrity check is a SHA-256 manifest. A retention window that ends before the export breaks the chain ([evidence preservation](https://nhimg.org/glossary/incident-evidence-preservation/), [Q and A on evidence preservation](https://www.securityscientist.net/blog/12-questions-and-answers-about-evidence-preservation-after-a-security-incident/)).
*Implication:* the bundle copies the evidence before the 14-day deletion (F3.6), with a digest for each file and the source path of each copy.

### 4.1 Storage: extract or compress the history

The operator added on 2026-11-18:

> "For the historical data, due to limited store space, we need to find a way either to only extract the important data that matter to the incidents or compress the data to a manageable size that still offering enough context for reasoning, also leave the options for "recording everything" for future expansion"

**F3.12** A measurement on 2026-11-18: `.evolve/runs/` holds 330 cycle directories in 2.33 GB, about 7 MB for each cycle. The directory of cycle 2485 is 2.42 MB as a tar file, 360 KB with `gzip -9` and 138 KB with `zstd -19`. Its `signals.ndjson` has 308 lines with 12 distinct codes, and 105 lines above INFO.
*Implication:* a full copy of each cycle in git is too large. A general compressor alone gives 6.7 to 17.5 times. The signal lines repeat a few templates, so a template count removes most of them.

**F4.8** Template extraction compresses logs before an LLM reads them. Drain mines the template of each line, removes the variables such as times and ids, and merges the lines of one template into one entry. LogAn reports 2,000 Mac log lines reduced to 197 templates (90.1 percent) ([LogAn, Red Hat](https://developers.redhat.com/articles/2026/05/28/logan-large-scale-log-analysis-small-language-models), [MicroRCA-Agent](https://arxiv.org/pdf/2509.15635)). A deterministic template step gives the same template for the same line each time ([DeepParse](https://arxiv.org/html/2604.20553v1)).
*Implication:* a template count with the first and the last example of each template keeps the shape of the log. It is small and repeatable.

**F4.9** Task-aware selection keeps the lines that matter for the root cause. LogSage selects the error lines by keyword, the tail, a log diff and template dedupe, and adds context around each error ([LogSage](https://arxiv.org/pdf/2506.03691)). LogSieve removes 42 percent of the lines and 40 percent of the tokens of CI logs with little loss of meaning ([LogSieve](https://arxiv.org/html/2601.20148)). A benchmark compares a full log, the last 200 lines and a grep with context ([LogDx-CI](https://arxiv.org/html/2605.28876)).
*Implication:* the extraction is rule-based: the incident's signals, the error and gate lines with a context window, and the tail. It needs no model.

**F4.10** Lossless log compression keeps everything at a small size. CLP parses each line into a template, its variables and a time, and compresses the columns with Zstandard. Uber reports 169 times on its logs, with search of the compressed data ([Uber CLP](https://www.uber.com/blog/reducing-logging-cost-by-two-orders-of-magnitude-using-clp), [CLP, OSDI 2021](https://usenix.org/conference/osdi21/presentation/rodrigues)).
*Implication:* a "record everything" mode is possible later as a lossless archive outside git, behind one interface.

**F4.11** Model-based prompt compression drops tokens by their score from a small model: up to 20 times with a loss of 1.5 points ([LLMLingua](https://www.microsoft.com/en-us/research/blog/llmlingua-innovating-llm-efficiency-with-prompt-compression/), [LongLLMLingua](https://arxiv.org/html/2310.06839v2)). Agent practice clears old tool results, compacts history and keeps structured notes outside the context ([context engineering cookbook](https://platform.claude.com/cookbook/tool-use-context-engineering-context-engineering-tools)).
*Implication:* model compression is lossy and not repeatable, so it cannot be evidence with a digest. It fits the read side, when an agent loads a record, and the record's own analysis is the structured note.

**F4.12** Trace practice keeps each error trace and samples the routine ones. Retention has tiers by value, not one keep-or-drop rule ([tail sampling](https://oneuptime.com/blog/post/2026-02-24-how-to-control-trace-volume-without-losing-important-traces/view), [agent trace tiers](https://tianpan.co/blog/2026-04-24-trace-sampling-agents-which-spans-keep)).
*Implication:* the evidence of an incident is the high-value tier. The rest of a run keeps only its digest.

## 5. Refinements

| Id | Refinement | Because |
|---|---|---|
| R1 | Give each record YAML frontmatter with a fixed schema: id, level, status, self-heal incident, fingerprint, signals, PRs, items, actions and the effectiveness check. The cycles and the bundle path are derived. | F3.4, F4.2, F4.6 |
| R2 | Grow the template to ten sections: summary, trigger and level, timeline, key logs, root cause and contributing factors, analysis, approaches taken, actions, effectiveness, lessons. | F3.2, F4.1 to F4.4 |
| R3 | Copy the evidence of each record into a committed bundle under `knowledge-base/incidents/<id>/`, with a SHA-256 manifest. Name the source path, the source digest, the capture time and the ledger-line anchors of each file. | F3.5 to F3.7, F3.11, F4.7 |
| R4 | Open a record from a signal through one verb, which writes the frontmatter, the timeline and the bundle. | F3.3, F3.5 |
| R5 | Keep the trigger list as policy data with levels, and add detectors for the three blocked states that have no code. | F3.1, F3.10, F4.5 |
| R6 | Put the records into the knowledge-base graph and into lesson recall. | F3.8, F4.6 |
| R7 | Measure the effectiveness from the signal channels and the self-heal fingerprint at set boundaries. Keep the record open until the verdict. | F3.9, F4.3 |
| R8 | Keep three evidence modes as policy data: `pointer` (digests and addresses only), `extract` (the default) and `full` (a lossless archive, for later). | F3.12, F4.10, F4.12 |
| R9 | Make `extract` a fixed pipeline with no model: select the incident's signals, the error and gate lines with a context window, and the tail. Dedupe by template with a count and the first and last example. Then cut to a byte budget in priority order. | F4.8, F4.9 |
| R10 | Keep the digest of the whole source and the kept line ranges, so a partial copy still traces to its root. Use model compression only on the read side. | F4.7, F4.11 |

## 6. Sources

Outside sources are linked where they are cited, in section 4. Each local claim cites a `path:line` or a file in this repository. The repository map was made on 2026-11-18 from the tree at `origin/main`.
