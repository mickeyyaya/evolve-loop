# Plan: incident reports in the knowledge base, opened from the signal center and traced to their evidence

- **Status:** partly built, 2026-11-21 (5 of 13 components on main, 1 PR open, 1 in review, 6 not started). Earlier status: proposed, 2026-11-18. Revised the same day after the design review (round 1). I7 built 2026-11-20.
- **Research:** [incident reports as a knowledge base that traces back to the evidence](../research/incident-knowledge-base-2026-11.md). **Amends:** [the issue-record plan](issue-record-skill-2026-11.md). Components I4, I5 and I7 of this plan replace its K4, K5 and K6. **Decision:** no ADR. The record schema is data in one package, and the spec is section 6.
- **Skills:** [issue-record](../../skills/issue-record/SKILL.md), [issue-analyze](../../skills/issue-analyze/SKILL.md) and [docs-write](../../skills/docs-write/SKILL.md).

## Table of contents

1. [Request](#1-request)
2. [Goals and non-goals](#2-goals-and-non-goals)
3. [Candidate designs](#3-candidate-designs)
4. [Decisions](#4-decisions)
5. [Worked example](#5-worked-example)
6. [The record, the bundle and the verbs](#6-the-record-the-bundle-and-the-verbs)
7. [Components](#7-components)
8. [Order](#8-order)
9. [Verification](#9-verification)
10. [Risks](#10-risks)
11. [Open questions](#11-open-questions)
12. [Design review record](#12-design-review-record)

## 1. Request

The operator asked on 2026-11-18 for an incident report that starts on each blocking or stale issue. It covers the history, the cause, the analysis, the approaches taken, how the fix takes effect and how the issue and the fix perform after. The operator then asked to "build the complete feature on the report structure around the signal center", with each report "part of knowledge base, which can be trace back to the root, where we recorded all this logs and historical data". The full quotes are in section 1 of the research.

The operator then added a storage limit (research section 4.1). Keep only the evidence that matters, or compress it with enough context for reasoning. Leave a "record everything" option for later.

## 2. Goals and non-goals

Goals:

1. A record opens from the signal center, with no copy by hand of the header, the timeline or the logs. (F3.3, F3.5)
2. Each blocked or stale state of the trigger data starts a record or a log row at its level. (F3.10, F4.5)
3. Each record has the ten sections of R2 and the frontmatter of R1. (F3.2, F3.4)
4. Each claim of a record traces to evidence that stays after garbage collection. (F3.6, F3.7, F4.7)
5. The records are nodes of the knowledge-base graph and roots of lesson recall. (F3.8)
6. A record closes only after a measured effectiveness verdict. (F4.3)

Non-goals:

- No rewrite of the 101 old records. The legacy rule of D11 keeps them as they are.
- No outside incident service. Each outside service needs a CLI, and the records must stay in git.
- No hash chain on the signal stores. The source digests and the ledger-line anchors give the integrity (D3).

## 3. Candidate designs

| Design | What it does |
|---|---|
| A. Template only | Grow the template and the skill to ten sections. No code. |
| B. A new trigger list | Frontmatter, a bundle, `evolve issue` verbs, and a new policy list of trigger codes beside the self-heal intake. |
| C. Self-heal incidents as the store | Extend `.evolve/incidents/` with the report fields, and render the report from it. |
| D. An outside incident tool | Keep the reports in an incident service or in Morgue. |
| E. Records on the self-heal intake | B, but the self-heal intake stays the one classifier and dedupes by fingerprint. Its evidence set becomes the committed bundle. |

| Criterion | A | B | C | D | E |
|---|---|---|---|---|---|
| A record opens from a signal (goal 1) | 0 | 2 | 2 | 1 | 2 |
| The evidence stays after garbage collection (goal 4) | 0 | 2 | 0: `.evolve/` is gitignored (F3.6) | 1 | 2 |
| A program can query the records (goal 5) | 0 | 2 | 1 | 1 | 2 |
| A measured effectiveness verdict (goal 6) | 0 | 1: no fingerprint for a code with no self-heal rule | 2 | 1 | 2 |
| One classifier, one manifest schema (F3.9a) | 2 | 0: a second list and a second schema | 2 | 1 | 2 |
| No outside service, no MCP | 2 | 2 | 2 | 0 | 2 |
| Size of the change | 2 | 0 | 1 | 1 | 1 |
| **Total** | 6 | 9 | 10 | 6 | 13 |

E wins. B duplicates the classifier of the self-heal intake and its dedupe key. One stall over five cycles then gives one incident and five records. C loses the evidence and the history, because `.evolve/` is not committed. E keeps C's one classifier and adds B's committed records.

## 4. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | A record is a Markdown file `docs/incidents/<id>.md` with YAML frontmatter. The id is `<yyyy-mm-dd>-<slug>`, so the file name is the id. One package, `internal/incidentkb`, owns the schema `incident-record/1`. | F3.4, F4.6, R1 | A JSON file beside the Markdown: two homes for one fact. An id apart from the file name: two names for one record. |
| D2 | The body has the ten sections of R2, in a fixed order with fixed headings. The renderer of `incidentkb` writes the status line, section 8 (Actions) and section 9 (Effectiveness) from the frontmatter, between GENERATED markers. `issue check` refuses a body that differs from its rendering. | F3.2, F4.1 to F4.4, R2 | Keep the data in the body and in the frontmatter by hand: an edit to one home makes the other wrong. |
| D3 | The bundle `knowledge-base/incidents/<id>/` is built from the stage index of D18, not from the `selfheal.WriteEvidence` workspace under `runs/`. Its `manifest.json` uses `inputindex.Entry` and adds `captured_at`, `source_address`, `source_sha256` and `redactions`. `source_sha256` is the digest of the raw source before the trim and the redaction. The ledger anchors are the SHA-256 of each anchored ledger line, with `entry_seq` as a hint. The bundle holds a copy of each anchored line. | F3.7, F3.9a, F3.9b, F4.7, R3 | Link to `.evolve/runs/`: deleted after 14 days. A new manifest schema: a third digest schema. `entry_seq` and `prev_hash` as the anchor: not unique (F3.9b). |
| D4 | The capture is a level-triggered sweep: `evolve issue capture`. It reads the stored incidents in `.evolve/incidents/`, so it covers each producer: the detect pass, the live reaction and the main-red stop. For each incident at level `record`, it stages each new source of the journal (D18), keyed by the incident id and the source id. A repeat run stages nothing twice, and a failed copy runs again on the next sweep. The sweep runs after each detect pass, after each live reaction and at each wave boundary, well inside the 14-day window. | F3.6, F3.9b, F4.7 | Capture at `issue open`: the deletion does not wait for a person. Capture on a change: it misses a producer inside `selfheal`, and one failed copy loses the evidence. |
| D5 | The self-heal intake stays the one classifier. It opens an incident on the first observation, as today. Each class gets a level (`record` or `log`) and a threshold as the proposed policy key `selfheal.levels`. The sweep marks a record due when `level == record`, the distinct cycles in the journal reach the threshold, and no due mark exists (D18). Then it emits `INCIDENT_RECORD_DUE`, and `evolve events watch` prints `next: evolve issue open --incident <id>`. A reopened incident keeps its id, so its record moves to `reopened` (D14). | F3.9a, F4.5, R5 | A second trigger list: two lists drift. A threshold inside `Detect`: the incident opens on the first observation, so the threshold never fires. |
| D15 | The evidence mode is policy data, `incident.evidence.mode`: `pointer`, `extract` (the default) or `full`. `pointer` keeps only the manifest: the digest, the address and the size of each source, and the ledger-line anchors. `extract` adds the bundle of D16. `full` adds a lossless archive of each source through the port of D17. | F3.12, F4.10, F4.12, R8 | One fixed mode: no room to grow to "record everything". A copy of each run in git: about 7 MB for each cycle (F3.12). |
| D16 | The `extract` pipeline is fixed code with no model in the leaf package `internal/evidencex`, and it runs at staging. Step 1 selects the incident signals, the lines at WARN or above, the last 40 lines of each log, and 5 lines around each error. Step 2 merges the lines of one template into one entry, with its count and its first and last example. The manifest keeps the whole-source digest and the kept ranges. Step 3 redacts (D13), and step 4 cuts to the one budget of the whole bundle (policy data, first value 256 KB), in priority order. When a new source does not fit, the oldest repeat source becomes a `pointer` entry (digest, address, size); the sweep never refuses. | F4.8, F4.9, R9, R10 | A model compressor: lossy and not repeatable, so a digest cannot prove it (F4.11). A trim of the head and tail only: it drops the error in the middle. |
| D17 | `full` writes through one port, `EvidenceArchive`. Its first adapter is a local store of zstd files outside git, with a size cap and a retention in policy data. Each archived file is keyed by its `source_sha256`, so the manifest finds it. A later adapter can write a lossless log store such as CLP, or an object store. The default mode never needs the archive. | F4.10, R8 | Put the full copy in git: too large. Build a CLP adapter now: no need yet. |
| D18 | The sweep state has named homes that garbage collection spares. `selfheal` appends each absorbed source id to an append-only journal, `.evolve/incidents/<id>.sources.ndjson`, because `Incident.Sources` keeps only the last 20. The staged bytes go into `ledgerartifacts.Store.Put` (`go/internal/gc/gc.go:204`). The stage index `.evolve/incidents/<id>.stage.json` holds each source id with its digest, its kept ranges and its state, and the due mark. One lock, `.evolve/incidents/.sweep.lock`, covers a sweep, and the index write comes before the emit of `INCIDENT_RECORD_DUE`, so two sweeps emit it once. The garbage collector names `.evolve/incidents/` in its protect list. | F3.6, F3.9a, F3.9b | Read `Incident.Sources`: a burst of more than 20 sources is lost. Stage under `runs/`: deleted after 14 days. No lock: two sweeps emit twice. |
| D6 | Three new detectors feed the self-heal intake as new rules and classes. A PR that stays open and green past two boundaries is `stale-pr`. An inbox item that triage routes back two times is `bounced-item`. A claim that `inbox release --stale` frees is `stale-claim`. Each emits a code first. | F3.10 | No detector: these blocked states stay silent. |
| D7 | `internal/kbgraph` reads the frontmatter through `incidentkb` and adds the node kind `incident`. Its edges are `affected` (cycle), `recurs-as` (fingerprint), `raised-by` (signal code), `fixed-by` (PR) and `tracked-by` (item). Lesson recall adds `docs/incidents/` to its roots. | F3.8, R6 | A new graph: two graphs for one knowledge base. |
| D8 | The effectiveness check is a typed metric in the frontmatter. It has a signal `code` and a list of filters `{field, op, value}`, with `op` in `eq`, `contains` or `prefix`. It has a `unit` (`cycle` or `wave`) and `since`, the commit or the time when the fix landed. It also has the baseline, the target and the check points (a count of waves). When the baseline rate times the check points is under 3, the verdict is `insufficient-data`. | F3.9, F4.3, R7 | A free-form query: no verb can compute it. A target of 0 on a rare code: it proves nothing. |
| D9 | The status moves as the table of section 6.4 shows. A record is `verified` only with an `effective` verdict at its last check point. A `log` level writes one row in `docs/incidents/<yyyy-mm-dd>-session-log.md` and needs no metric. | F4.3 | Close at the fix: closure does not prove the effect. |
| D10 | Today nothing runs the detect pass at a wave end: only `evolve selfheal incidents` calls it (`go/cmd/evolve/cmd_selfheal.go:95`). The boundary step list `waveSteps` (`go/cmd/evolve/cmd_wave_compose.go:121`) gets three steps: `evolve selfheal incidents`, `evolve issue capture` (D4), then `evolve issue due --verify`. The last step calls `incidentkb` from `cmd/evolve`. It turns the evidence of a due record that nobody opened in 3 boundaries into one `log` row. A due check emits `INCIDENT_CHECK_DUE`, and the verify rows of one boundary go to one docs PR. The steps are `advisory: true` (`go/cmd/evolve/cmd_boundary.go:32`), and the verbs keep a non-zero exit on an error. | F4.3 | A verb that nobody calls: the records wait open for ever. A step that blocks the boundary: the wave waits on a report. |
| D11 | A file in `docs/incidents/` with no frontmatter and a date before 2026-11-19 is `legacy`. `REGRESSION-COVERAGE-INDEX.md` and each `<yyyy-mm-dd>-session-log.md` are not records, and the check skips them by name. `issue check` skips it, `issue list` reports it, and the graph adds it as a node with no edges. A new file with no frontmatter fails the check. | F3.4 | Rewrite 101 records: no value for the cost. A check that fails on 101 files: a red that someone skips. |
| D12 | A self-heal incident gets the field `record_doc`, a path string. The CLI refuses a close when the file at that path is absent. `selfheal` never imports `incidentkb`. | F3.9, K6 of the issue-record plan | A check inside `selfheal` that reads the record: the protected detector depends on the docs schema. |
| D13 | The redaction moves to one leaf package, `internal/redact`. `panetrust`, the bundle copy and the frontmatter and timeline fields call it. `issue check` and a commit-gate scan of `knowledge-base/incidents/` call it too. The scan also refuses a run of 32 or more base64 characters with an entropy of 4.0 bits or more for each character. It exempts pure hex of 40 or 64 characters, and the manifest digest fields by name. That rule does not use the pattern list. | F3.9b | Two pattern lists: the copy and the check drift. |
| D14 | One rule in one home maps the two lifecycles. The self-heal incident has `open` and `closed`; the record has the statuses of section 6.4. The `issue due --verify` step moves a `verified` record to `reopened` when its incident has a `LastSeenAt` after the record's `verified_at`. This holds also for evidence that the open incident took in with no transition. The self-heal close and the record verdict are independent: the close needs `record_doc` (D12), and the verdict needs the metric (D8). | F3.9 | Reopen on a self-heal transition: an open incident takes in a new match with no transition, so the recurrence stays hidden. |

Dependency direction: `incidentkb` is a leaf: the schema, the parser, the renderer and the checks. It keeps the self-heal id, the fingerprint and `LastSeenAt` as plain values. A separate package, `incidentcapture`, imports `selfheal`, `incidentkb`, `evidencex` and `redact`, and owns the sweep, the due mark, `verify` and the reopen rule of D14. `kbgraph` and lesson recall import only `incidentkb`. `EvidenceArchive` is a port that `cmd/evolve` fills. The CLI in `cmd/evolve` wires the verbs and the checks.

Consequences:

- Good: each record traces to its evidence through source digests and ledger-line anchors, after the run directories are gone.
- Good: the graph answers "which incidents touched cycle N", "which fixes did not work" and "which code raised most incidents".
- Bad: each record adds a bundle to git. D16 holds the one size policy.
- Bad: a record waits open for its last check point. D10 makes the wait visible at each boundary.

What changes our mind: if more than one record in five is reopened, the levels or the thresholds are wrong. Then the policy data changes before the code.

## 5. Worked example

The record of 2026-11-18, "the train ejected four console PRs on the research index", in this design:

1. `SHIP_INTEGRATION_EJECTED` becomes a self-heal observation of the proposed class `integration-ejection` (I11), at level `record`. The intake opens an incident. The next sweep (D4) stages the ejection lines and emits `INCIDENT_RECORD_DUE`.
2. The watch prints `next: evolve issue open --incident <id>`. The verb writes `docs/incidents/2026-11-18-the-train-ejected-four-console-prs-on-the-research-index.md` and the bundle.
3. The console writes the cause and the analysis. The approaches are a union attribute, a merge driver, the regeneration and one docs PR per batch.
4. The actions are C2 (PR #1558, type `prevent`) and the docs PR #1555 (type `mitigate`). C2 is done when its journey test is green.
5. The metric is the code `SHIP_INTEGRATION_EJECTED`, with the filter `{field: paths, op: contains, value: docs/research-index.md}`, per `wave`, since the commit of #1558. The baseline is 4 in 2 waves, and the target is 0 over 3 waves.
6. At each wave end the detect pass runs `verify`, and the renderer adds a row. After the third wave with 0 ejections, the status is `verified`.

## 6. The record, the bundle and the verbs

### 6.1 Frontmatter

```yaml
---
schema: incident-record/1
id: 2026-11-18-the-train-ejected-four-console-prs-on-the-research-index
title: the train ejected four console PRs on the research index
level: record            # record | log
status: fixed            # see 6.4
selfheal: inc-2026-11-18-ab12
fingerprint: integration-ejection:derived
signals: ["cycle:2481:57", "channel:errors:1832"]
prs: [1555, 1556, 1558]
items: [edit-layout-c2-regenerate-generated-conflicts]
actions:
  - {id: A1, type: prevent, owner: console, item: edit-layout-c2-regenerate-generated-conflicts, pr: 1558, done: "the journey test composes two shard PRs", state: done}
effectiveness:
  metric: {code: SHIP_INTEGRATION_EJECTED, filters: [{field: paths, op: contains, value: docs/research-index.md}], unit: wave, since: 051ad74bf}
  baseline: {value: 4, units: 2}
  target: 0
  checks: 3
  results: []
verified_at: ""
---
```

The cycles and the bundle path are derived from `signals` and `id`, so the frontmatter does not store them.

### 6.2 Sections

| # | Heading | Holds |
|---|---|---|
| 1 | Summary | the impact, the duration, the cause, the fix and the status line (rendered) |
| 2 | Trigger and level | the self-heal class, the rule and the reason for the level |
| 3 | Timeline | start, detection, response, fix and each effectiveness check, each row with its evidence |
| 4 | Key logs | the trimmed lines, each with its bundle file and digest |
| 5 | Root cause and contributing factors | the causal chain, and the factors apart from the cause |
| 6 | Analysis | how the situation looked at each step, and which gate did not catch it |
| 7 | Approaches taken | each approach, its result, and why it was kept or dropped |
| 8 | Actions | rendered from `actions` |
| 9 | Effectiveness | rendered from `effectiveness`: the metric, the baseline, a row for each check, and the verdict |
| 10 | Lessons | each lesson with the rule or the guard that it became |

### 6.3 Verbs

| Verb | Does |
|---|---|
| `evolve issue open --incident <id> \| --signal <cycle>:<seq>` | writes the skeleton and moves the staged evidence into the bundle; `--signal` is for a code with no self-heal rule |
| `evolve issue capture` | the sweep of D4: stages the evidence of each incident at level `record` in the mode of D15, and marks the due records of D5 |
| `evolve issue evidence add <id> <source>` | copies one more source into the bundle and updates the manifest |
| `evolve issue check [<id>]` | checks the schema, the sections, the rendered parts, the links, each digest and the redaction |
| `evolve issue verify <id>` | computes the metric, appends a result and renders it |
| `evolve issue due [--verify]` | lists the records with a check point at this wave end; `--verify` runs `verify` on each and moves a record to `reopened` (D14) |
| `evolve issue list [--status S] [--code C] [--json]` | queries the frontmatter of each record, and reports the legacy files |

The proposed signal codes are `INCIDENT_RECORD_DUE`, `INCIDENT_RECORD_OPENED`, `INCIDENT_CHECK_DUE`, `INCIDENT_RECORD_VERIFIED` and `INCIDENT_RECORD_REOPENED`, in the module of the self-heal intake. None exists yet.

### 6.4 Status

| From | Event | To |
|---|---|---|
| (none) | `issue open` | `open` |
| `open` | each action is done | `fixed` |
| `fixed` | the last check gives `effective` | `verified` |
| `fixed` | a check gives `not-effective` | `reopened` |
| `fixed` | a check gives `insufficient-data`, under the cap of extensions (policy data, first value 3) | `fixed`, with one more check point |
| `fixed` | the cap of extensions is reached, or the signal code is retired | `unverifiable`, with the reason that a person writes (terminal) |
| `verified` | the incident has a `LastSeenAt` after `verified_at` (D14) | `reopened` |
| `reopened` | a new action is done | `fixed` |

## 7. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| I1 | The template of ten sections in docs-write, and the procedure in `issue-record` (D2) | the I2 check, run over a fixture record that lacks section 7, fails | console (the docs-write skill is protected) |
| I2 | `internal/incidentkb`: the schema, the parser, the renderer, the legacy rule, and `issue check` and `list` (D1, D2, D11) | `issue check` passes on the current tree and fails on a new file with no frontmatter. A hand edit of section 8 fails with the field name | lane |
| I3 | `internal/redact`, the bundle, the manifest on `inputindex.Entry`, the size limit, the size ratchet and `issue evidence add` (D3, D13) | a changed bundle file fails `issue check`. A token in a source is redacted in the copy. When the sources exceed the budget, each error template stays and the oldest repeat source becomes a `pointer` entry. A valid bundle passes the entropy rule, and a random 40-character token in a log line fails it | lane |
| I12 | `internal/evidencex`: the fixed `extract` pipeline and the modes `pointer` and `extract` (D15, D16) | on a fixture log of 2,000 lines with 3 errors, the extract keeps each error with its 5 lines of context. It merges the repeats into templates with a count, and stays under the budget. The manifest holds the digest of the whole source and the kept ranges. The same input gives the same bytes | lane |
| I13 | The `EvidenceArchive` port and its local zstd adapter, for the mode `full` (D17) | a source archived in `full` mode is found by its `source_sha256`. The archive refuses a write above its size cap | lane, after I12 |
| I4 | The package `incidentcapture`: the journal, the stage index, the lock, the sweep `evolve issue capture` and `evolve issue open` (D4, D18) | incidents opened by the detect pass, the live reaction and the main-red stop are each staged by one sweep. A copy that fails once is staged on the next sweep, and a second sweep stages nothing twice. A burst of 30 sources in one pass is staged in full from the journal. Two concurrent sweeps emit one `INCIDENT_RECORD_DUE`. The staged evidence stays after a garbage-collection run. `issue open --incident` writes the frontmatter and a bundle with the signal line and its `source_sha256` | lane |
| I5 | `selfheal.levels`, the `INCIDENT_RECORD_DUE` code, the append on a repeat, the watch hint and the release of an unopened due record (D5) | a stall over three cycles emits one `INCIDENT_RECORD_DUE` and gives one record, keyed by the fingerprint. The staged evidence holds the source line of each of the three cycles; the test fails when the sweep ignores a new source. With a threshold of 3, observations 1 and 2 give no due mark, and observation 3 gives one. A due record that nobody opens in 3 boundaries becomes one `log` row | lane |
| I11 | Self-heal rules for the five blocked-state codes that have no rule, each with its level and threshold in `selfheal.levels`. The proposed classes are `integration-ejection` (`SHIP_INTEGRATION_EJECTED`), `pool-idle` (`LOOP_ORCH_POOL_IDLE`) and `lane-stalled` (`LOOP_ORCH_LANE_STALLED`). The others are `wave-stale` (`LOOP_WAVE_ALL_LANES_STALE`) and `expected-absent` (`SIGNAL_EXPECTED_ABSENT`) | each code opens an incident of its class at its threshold, and not below it | lane |
| I6 | The three detectors and their self-heal rules (D6) | an item that triage routes back two times opens a `bounced-item` incident | lane |
| I7 | The self-heal `record_doc` field and the refused close (D12) | `TestIncidentClose_RefusesWithoutARecordDoc` | console |
| I8 | The graph and recall (D7) | `evolve kb graph neighbors incident:<id>` lists its cycle, its PR and its item; recall finds a record by a word of its lesson | lane |
| I9 | `verify`, `due` and the two boundary steps (D8, D10, D14) | a fixture record with a baseline of 4 and 0 events after the fix gives `effective` at its last check point. A rare code gives `insufficient-data`, then `unverifiable` at the cap. `waveSteps` holds the two steps, and a boundary through `evolve wave next --dry-run` lists them. A `LastSeenAt` after `verified_at` gives `reopened` | lane |
| I10 | The wiring proofs: `issue check` in the commit gate and in CI, and the redaction scan | a commit that adds a record with a bad digest is refused by the commit gate | console (the gates are protected) |

Each component maps to an inbox item, so the loop builds it. Each item names its red test and links this plan.

## 8. Order

1. I1, I2, I3 and I12: the shape of a record, the check, the bundle and the extract pipeline.
2. I4, I5 and I11: the capture, the opening from the signal center and the rules of the blocked states. The worked example then runs end to end.
3. I6 and I10: the detectors and the gate wiring.
4. I8, then I9 and I7: the knowledge base and the closed loop of effect.
5. I13, when the operator chooses the mode `full`.

## 9. Verification

| Measure | Before (2026-11-18) | Target | Command |
|---|---|---|---|
| New records with valid frontmatter and rendered parts | 0 of 101 | each new record | `evolve issue check` |
| New records whose source digests match their manifest | 0 | each new record | `evolve issue check` |
| Records that a program can query | 0 | each new record | `evolve issue list --json` |
| Blocked states with no signal code | 3 | 0 | the detectors of I6 |
| Blocked-state codes with no self-heal rule | 5 | 0 | the rules of I11 |
| Records closed with a measured verdict | 0 | each record after I9 | `evolve issue list --status verified` |
| The share of records that reopen | not measured | under 1 in 5 | `evolve issue list --status reopened` |

## 10. Risks

| Risk | Control |
|---|---|
| A bundle copies a secret. | One redactor (D13) for the copy and the fields, a refusal in `issue check`, and a commit-gate scan (I10). |
| The bundles make the repository large. | The one size policy of D16, and a size ratchet on `knowledge-base/incidents/` with its limit in policy data (I3). |
| One fault opens many records. | One record for each fingerprint (D5), and the levels and thresholds as policy data. |
| A metric cannot be computed, or proves nothing. | `issue check` runs the metric on a fixture, and a rare code gives `insufficient-data` (D8). |
| An anchor cannot be checked on another clone. | The ledger is not committed, so an anchor is checked only where the ledger exists. The source digest and the copy of the anchored line are the portable proof (D3). |

## 11. Open questions

- **Q1.** Does the bundle of a record that a lane opens ride the PR of its fix, or its own docs PR? I3 decides, with the landing rules of the lanes. The verify rows of one boundary go to one docs PR (D10).
- **Q2.** What is the stale threshold for a PR? D6 starts at two boundaries, as policy data.

## 12. Design review record

The design review of 2026-11-18 blocked the evidence capture five times: round 1, round 2, two fix-up reviews and the review of a first redesign. Each version captured on a change: a transition, or a compare before and after an update. Each one missed a case. The cases were a repeat with no transition, a new kind that reached the event path and the brake, and a producer inside `selfheal`. A failed copy and a threshold were the last two.

Under the rule "blocked more than three times means a redesign", D4 is now a level-triggered sweep over the stored incidents. It needs no hook on any producer, a failed copy runs again, and the threshold is a predicate of the sweep. The guards are the I4 and I5 tests.

The review of the sweep and the storage modes then found six gaps of specification, each with a fix. The gaps were a 20-entry window, the homes of the stage state and two size rules. The others were a heavy import, an entropy rule with no parameters and a sweep with no lock. D16 and D18 now close them, and the dependency line moves the join into `incidentcapture`. Under the convergence rule, no fourth round runs. Each build item pins these rules with its red tests, and each build gets its own code review.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| I1 template and procedure | in review | none yet; the build is in review | `incident-kb-i1-template-and-skill` |
| I2 `internal/incidentkb`, `issue check`, `list` | done | cycle 2530, [`2343835`](https://github.com/mickeyyaya/evolve-loop/commit/234383500cc7a7a113a70a9889b950ed247c6a6b); `go/internal/incidentkb/check.go` | none |
| I3 redact, bundle, `issue evidence add` | done | cycle 2496, [`883c2ed`](https://github.com/mickeyyaya/evolve-loop/commit/883c2ed5a6a661652c43b5d7e858d6dbd5d688bd); `go/internal/redact/redact.go` | none |
| I12 `internal/evidencex` extract | done | cycle 2502, [`4468539`](https://github.com/mickeyyaya/evolve-loop/commit/446853962f50df71bef18ab0115adf0958a98798) | none |
| I13 `EvidenceArchive` port and zstd adapter | done | cycle 2537, [`a467cc8`](https://github.com/mickeyyaya/evolve-loop/commit/a467cc81ba0c09789b89e06b22ae722d623ca2e1); `go/internal/evidencex/archive.go` | none |
| I4 `incidentcapture`, capture sweep | not started | none; no `go/internal/incidentcapture` | `incident-kb-i4-capture-sweep` |
| I5 levels, `INCIDENT_RECORD_DUE` | not started | none; the code is absent from `go` | `incident-kb-i5-levels-and-due` |
| I11 blocked-state rules | not started | none | `incident-kb-i11-blocked-state-rules` |
| I6 three stale detectors | not started | none | `incident-kb-i6-stale-detectors` |
| I7 `record_doc` and refused close | built, PR open | [#1624](https://github.com/mickeyyaya/evolve-loop/pull/1624) | `incident-kb-i7-record-doc` |
| I8 graph and recall | done | [#1577](https://github.com/mickeyyaya/evolve-loop/pull/1577), [`0c2fbec`](https://github.com/mickeyyaya/evolve-loop/commit/0c2fbec2c), cycle 2533; `go/internal/kbgraph/kbgraph.go:178` | none |
| I9 `verify`, `due`, boundary steps | not started | none | `incident-kb-i9-verify-and-due` |
| I10 gate wiring proofs | not started | none | `incident-kb-i10-gate-wiring` |

The record, the bundle, the extract pipeline, the archive and the graph are on main. The capture sweep, the due levels, the detectors, the verify step and the gate wiring are not built. Each has an open inbox item. I7 waits for its pull request to merge. I1 is in review. Every unfinished component has an open item, so no follow-up draft is needed.
