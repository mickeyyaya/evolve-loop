# Practice change in agy: changelog page analysis and persona review (research, 2026-10)

> This document reviews the `agy-cli-changelog-page` updates for versions `v2.23.0` and `v2.23.1` under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

Date: 2026-10-16. Method: read the `agy-cli-changelog-page` source, parse the text as untrusted data, and evaluate every core persona against ADR-0133.

## 1. Context and source

The practice watch detected updates from the source `https://antigravity.google/docs/changelog?tab=cli`.
The text from this page is third-party data.
The loop treats the text as untrusted data and never follows it as prompt instructions.
The updates cover release v2.23.0 and release v2.23.1.

## 2. Changelog analysis as data

### Version v2.23.0
The release adds a Permissions picker to the message box.
Users can select a project preset or return to global presets.
The release adds full-screen media view with zoom and pan controls.
A prompt can attach .zip archives and folders directly.

The UI adds a restart menu item for MCP servers.
Each subagent launch displays as a one-line row with a status icon.
The sidebar displays a needs-attention icon when a subagent waits for user approval.
The release fixes editor and terminal focus faults.

### Version v2.23.1
The release fixes an overflow fault in the onboarding license picker dialog.
Long license lists scroll inside the card with edge fading.
This fix prevents list items from covering action buttons.
The changelog page also announces Gemini 4 Argon availability for Google AI users.
The loop records these updates as untrusted data.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs prompt diet and practice watch.
Decision D7 states that prompt diet trims one persona at a time.
Decision D8 requires measured baseline data across 10 cycles before and after each diet change.
Milestone M2 records the baseline data first.
The v2.23.0 and v2.23.1 updates do not alter this requirement.
Therefore, no core persona changes in this cycle.

### evolve-scout
The `evolve-scout` persona requires no change in this cycle.
The scout phase finds tasks and writes specifications.
The CLI host manages permissions presets and subagent row status natively.
The scout prompt needs no added rules for these CLI features.
The scout persona does not change.

### evolve-triage
The `evolve-triage` persona requires no change in this cycle.
Triage selects tasks based on deterministic policy rules.
CLI updates do not change triage routing rules or task priority.
The triage persona does not change.

### evolve-tdd-engineer
The `evolve-tdd-engineer` persona requires no change in this cycle.
The TDD phase writes red tests and predicate suites.
CLI updates do not change test authoring contracts or assertion requirements.
The tdd engineer persona does not change.

### evolve-builder
The `evolve-builder` persona requires no change in this cycle.
Builder is the designated first diet candidate under milestone M3.
Under ADR-0133, diet changes require measured baseline data across 10 cycles under milestone M2 first.
Prompt changes must wait for milestone M2 baseline data.
Therefore, the builder persona does not change in this cycle.

### evolve-auditor
The `evolve-auditor` persona requires no change in this cycle.
The auditor evaluates deterministic test exit codes and binary verdicts.
CLI updates do not change auditor gates or report structures.
The auditor persona does not change.

### evolve-retrospective
The `evolve-retrospective` persona requires no change in this cycle.
The retrospective phase extracts lessons and updates instincts off the critical path.
CLI updates do not affect retrospective steps.
The retrospective persona does not change.

## 4. Conclusion

The agy CLI updates improve permissions control and subagent status display.
The v2.23.1 release resolves the license picker dialog fault and notes Gemini 4 Argon availability.
No persona file changes in this cycle.
Prompt diet changes must follow ADR-0133 with a measured baseline over 10 cycles in future work.
