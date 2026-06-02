# Phase Verdict Classifiers Drifted from Agent Report Templates → False FAIL → No Ship

**Date:** 2026-06-02
**Found in:** cycle-192 (harden rerun), after the EGPS cwd fix unblocked `red_count=0`.
**Severity:** HIGH — blocked every cycle from shipping even with audit+EGPS green.

## Symptom

Cycle 192: `acs-verdict.json` = `red_count:0, verdict:PASS` (EGPS green), build-report `Status: PASS`, work sound — yet the **auditor returned FAIL** and the cycle didn't ship. Auditor's finding:
> `build-report.md` claims a clean PASS, but `build-usage.json` records `verdict: FAIL`.

`*-usage.json` verdicts: intent=PASS, triage=PASS, **scout=FAIL, tdd=FAIL, build=FAIL, audit=FAIL**.

## Root cause — classifier ↔ template contract drift

Each phase runner derives its verdict by grepping the agent's report for a mandated section heading (`runner.go` → `hooks.Classify`). Those headings drifted in the agent templates but the Go classifiers were never updated:

| Phase | Classifier required (stale) | Template / actual report emits |
|---|---|---|
| build (`build.go:77`) | `## Files Modified` | `## Changes` (template: "Files Changed table") |
| scout (`scout.go:30`) | `## Proposed Tasks` + bullet list | `## Selected Tasks` + `### Task N:` subheadings (template `evolve-scout.md:149`) |
| tdd (`tdd.go:74-77`) | `## Acceptance` + `## RED Tests` | `## RED Run Output` / `## Test Files Written` / `## Coverage Map` |

A valid, complete report whose heading didn't match → `VerdictFAIL`. scout/tdd FAILs are non-blocking, but **build's spurious FAIL contradicts the build report's `Status: PASS`**, and the adversarial auditor correctly FAILs on the report-vs-telemetry contradiction → audit FAIL → no ship. intent/triage classifiers happened to still match their templates → PASS (which is why only those two were green).

Not a tmux-scrollback issue: the clean report FILE itself lacks the stale headings.

## Fix (committed)

Made each classifier tolerant of the legacy AND current headings, grounded in cycle-192's real reports + the template "Required sections":
- build: accept `## Changes` | `## Files Changed` | `## Files Modified`.
- scout: regex accepts `## (Proposed|Selected) Tasks` followed by a list item OR a `### ` task subheading.
- tdd: require an acceptance signal (`## Acceptance` | `## AC-Materialization` | `## Coverage Map`) AND a RED signal (`## RED Tests` | `## RED Run Output` | `## Test Files Written`).

TDD: `TestClassifyArtifact_HeadingVariants` (build), `TestClassify_SelectedTasksHeading` (scout), `TestClassify_HeadingVariants` (tdd) — each asserts current-format PASS, legacy PASS, and incomplete-report FAIL.

## Architectural follow-up (recommended, not done)

These classifiers are duplicated, hand-rolled, single-heading checks that silently drift from the templates — a recurring failure class (cf. cycle-148 audit verdict-format miss). Single-source-of-truth options: (a) derive the required-section set from the template's machine-readable "Required sections" front-matter; (b) move all phases onto the existing declarative `phasespec.ClassifyRules` (`specrunner.evaluateClassify`) instead of bespoke `classify` funcs; (c) a contract test that asserts each phase's classifier accepts a golden current-format report fixture, failing CI when template and classifier diverge. Recommend (c) as the cheapest drift alarm.
