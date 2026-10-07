# internal/codereview

> Decision: [ADR-0124](../adr/0124-code-review-phase.md). Design: [review-loop-and-quality-index.md](../review-loop-and-quality-index.md). Plan: [code-review-phase-2026-10.md](../../plans/code-review-phase-2026-10.md). Phase config: `.evolve/phases/code-review/phase.json`, persona `agents/evolve-code-reviewer.md`, profile `.evolve/profiles/code-reviewer.json`. The index it scores on: [internal-qualityindex.md](internal-qualityindex.md).

## Purpose

`internal/codereview` is the kernel half of the `code-review` phase. The phase's agent writes `code-review-report.md` (Review Plan, Findings, Scores, Verdict). This package:
- checks the report's grammar for the deliverable gate (`ValidateReport`);
- turns the findings into rows of the cycle's defect ledger (`Record`);
- projects the outcome to the `REVIEW_FINDINGS` signal (`Outcome.Event`).

It decides no routing and gates nothing.

## Design

- **The finding grammar is `reportdoc`'s.**
  - `Parse` takes the visible `## Findings` section (`reportdoc.Section`), so a fenced example can never raise a finding.
  - It splits the section at `### ` headings, and keeps a block only when `reportdoc.Findings` reads its heading as a finding with an explicit severity. A `### Notes` heading is not a finding; the grammar check below refuses it.
  - A finding's `Dimension` is lower-cased, the spelling of the index's keys, so `Dimension: Performance` cites and records as `performance`.
  - It reads the five fields (`Dimension`, `Location`, `Scenario`, `Evidence`, `Fix`) with `reportdoc.Fields`.

  No severity regex is spelled here, so the dashboard, the audit gate, the repair brief and this parser read one grammar.
- **The report grammar is checked before the phase completes.** `ValidateReport(report, thresholds)` parses the Review Plan and the Scores (`qualityindex`), checks their agreement, and checks that every score below its threshold cites a finding (`CR<n>`) on that dimension. It also refuses a format drift that `Parse` would otherwise drop silently (`findingsProblems`). These are:
  - a `###` heading under `## Findings` that `reportdoc.Findings` does not read (a title-case severity, a missing severity, a non-finding heading);
  - any other line that opens with a `CR<n>` id, bold or not (a `####` heading, a bullet, a numbered item);
  - a body that is neither findings nor `None.`.

  The four drift shapes the architecture review found are table rows of `TestValidateReport_EveryGrammarBreachIsAProblem`. The deliverable gate runs it for `classify.grammars: ["code-review-report"]`, so a malformed report gets the contract-correction rung (`bad_grammar`), never a block.
- **A defect is never dropped for its format.** A block whose field list is malformed (`reportdoc.Fields` refuses a repeated single-valued key) is kept with its raw body as the scenario. A missing field renders as `(missing)` in the row text, so a row without a fix shows that it has none. An unknown dimension is recorded as written.
- **The row text is position-free.** `[SEVERITY] dimension location — title | scenario | evidence | fix`. The report's `CR<n>` id is left out because it is positional: a renumbered re-report would mint a second row, and the defect ledger's content-hash id would re-bind. `Finding.ID` is kept only for the gap-citation check.
- **One merge rule.** `Record` reads the workspace ledger and merges the rows through `defectledger.Append`, the rule the audit's Emit also uses. Dedupe, the 64-row cap and the stand-in for a cut are each kept per source, so the review's rows never dedupe, cut or swallow an audit row. It writes only when something was added, so a clean review mints no ledger.
- **Shadow rows are DEFERRED** with `ShadowReason` (ADR-0124, design §4.2). An OPEN row would be inherited by a continuation and owed to its audit, which the shadow stage promises not to touch. The stand-in row for a cut takes the cut rows' status and their highest severity, so a shadow overflow never mints an OPEN row.
- **The verdict is derived, never read.** `Verdict` is PASS with no findings, FAIL when any finding is at or above the strict threshold (`Settings.Repair.Threshold`, MEDIUM by default), and WARN otherwise. The report's own `## Verdict` line is the reviewer's statement, and nothing branches on it.
- **`would_repair` is the round-1 projection of the loop's decision.** In shadow every round is round 1, and round 1 is resolved only with no finding and qualifying Scores. So `would_repair` is true when any finding was raised or `qualityindex.Qualifies` reports a gap. The full decision (`REVIEW_ROUND`) replaces it with Q3.
- **The signal fits the Center.** `Outcome.Event` emits at most 12 fields (`signalcenter.MaxFields`), optional ones included: `round`, `findings` (the counts by severity in one field), `verdict`, `would_repair`, `stage`, `threshold`, `recorded`, `scores` (the vector), `gaps`, and `overflow`, `config_warning` or `error` when present. The Center drops fields over the cap and only counts them, so the field set is pinned by `TestEvent_TheWorstCaseFieldSetPassesTheCenterUntruncated`.
- **Faults are data.** `Record` never panics or returns an error to its caller. It returns an `Outcome` whose `Err` is set when the report or the ledger could not be read or written. The event is INFO, or WARN with `fields.error` (`recorded=false`) or `fields.config_warning` (either config block's warnings).
- **`Skipped`** builds `REVIEW_SKIPPED`, a WARN with `reason` and `round`. Core emits it when a code-review dispatch degrades to SKIPPED, whether its correction ladder exhausted or its verdict stayed non-canonical (`SkipMalformed`). Nothing is recorded in the ledger. Pinned by `TestSkipped_IsAWarnNamingTheReasonAndTheRoundUnderTheReviewModule`.
- **`RoundFrom`** counts the completed `code-review` dispatches in `CycleState.CompletedPhases`. Core calls it after the completion boundary appends the phase, so the first review is round 1 and the delta re-review round 2. The resume path reads the same persisted list.
- **Callers.**
  - Core's `Orchestrator.recordReviewFindings(cs, phase, verdict)` (`core/review_findings.go`) calls `Record` at both completion surfaces: `phaseCompletionRecord.persist` (the fresh loop and the resume path) and the parallel evaluate batch. It takes the cycle from `CycleState.CycleID`. A FAILed review, whose report failed its contract, records nothing.
  - The deliverable gate calls `ValidateReport` through its grammar registry.

## Invariants

- `PhaseName` equals `.evolve/phases/code-review/phase.json`'s name. Its artifact is `phasecontract.ArtifactFilename(PhaseName)`, its required sections include the Review Plan, Findings and Scores the parsers read, it declares the `code-review-report` grammar, and it asks for the Task Contract (`TestPhaseName_IsThePhaseJSONsNameArtifactSectionsAndGrammar`).
- The persona's own finding example parses to one finding with all five fields and a known dimension (`TestThePersonasFindingExampleParsesWithEveryField`).
- The signal's severity counts are reportdoc's vocabulary in rank order (`TestSeverityFields_AreTheFindingVocabularyInRankOrder`).
- A clean review writes no ledger (`TestRecord_NoFindingsWritesNoLedger`). A repeated record adds nothing (`TestRecord_AppendsTheFindingsToTheWorkspaceLedgerBesideTheAuditsRows`).

## Findings

- **The Center's 12-field cap.** The first staged version emitted four severity fields plus the scores fields, and a test caught the Center silently dropping `verdict` and `would_repair`. The counts now travel as one field.
- The shadow waves measure the would-repair rate, the findings per dimension, and the phase's added wall time and tokens ([plan §8](../../plans/code-review-phase-2026-10.md)).
