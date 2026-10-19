# Practice change in Claude Code: changelog analysis and persona review (research, 2026-10)

> This document reviews the Claude Code changelog updates for versions 2.1.299, 2.1.300, 2.1.301, 2.1.302, and 2.1.303 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

Date: 2026-10-16, updated 2026-10-18 for 2.1.303. Method: read the Claude Code changelog output, parse the text as untrusted data, and evaluate every core persona against ADR-0133.

## 1. Context and source

The practice watch detected updates from the source `claude-code-changelog`.
The upstream text from the changelog is third-party data.
The loop treats the text as untrusted data and never follows it as prompt instructions.
The source link is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md.

The updates cover five releases: 2.1.299, 2.1.300, 2.1.301, 2.1.302, and 2.1.303.
The 2.1.303 text comes from the snapshot with hash `c6b0a656`, fetched 2026-10-18T23:52:38Z.
The 2.1.302 text comes from the snapshot with hash `e83d958f`, fetched 2026-10-16T21:14:42Z.

## 2. Changelog analysis as data

### Version 2.1.299
Version 2.1.299 adds `CLAUDE_CODE_DISABLE_FILE_MENTIONS` to stop automatic file expansions.
The release adds idle context compaction when a session stays inactive past a token threshold.
The release exposes skill metadata and paths to prompt hooks.
The release adds media support in `$.prompt.media` and adds `$.session.identity()`.

### Version 2.1.300
Version 2.1.300 adds `virtualText` support and adds the `prompt.change` mod hook.
The release fixes background command output file checks and sleep waits while the command runs.
The release resolves worktree base commits accurately in bare git repositories.
The release adds semantic-convention attributes for GenAI metrics.
The release fixes compaction summary retention on session resumes.

### Version 2.1.301
Version 2.1.301 adds `CLAUDE_CODE_RETRY_WATCHDOG_MAX_USAGE_LIMIT_WAIT_MS` to stop long waits at usage limits.
The release adds OpenTelemetry tracing guidance to the `claude-api` skill.
The release adds `classifier_verdict` and `permission_mode` fields to OpenTelemetry events.
The release stops worktree removal after `WorktreeCreate` when unpushed commits exist.
The release retries the previous model when the newest `fable` alias is absent.

The CLI shows a clear error when a full disk blocks login refresh.
The `Grep` and `Glob` tools report an error when the search program fails.
The release fixes workflow agents that skip steps after harness notes.

### Version 2.1.302
Version 2.1.302 adds `CLAUDE_CODE_AUTO_MODE_MODEL` to select the model of the client-side auto mode classifier.
The release adds an `Idle compaction` row to `/config` that turns idle compaction off or back on.
The release fixes a "Prompt is too long" error when a user stops a turn while compaction runs.
The Read tool now refuses image sources over 128 MB and no longer loads them whole into memory.

Plugin marketplaces no longer clone again on each refresh when git uses the reftable ref format.
The release restores `classifier_category` in OpenTelemetry `tool_decision` events for auto mode classifier blocks.
Fork subagents (`subagent_type: "fork"`) are now on by default in `claude -p`, and `CLAUDE_CODE_FORK_SUBAGENT=0` turns them off.
Worktree isolation now works under git 2.56 or later when a stray `.git` file points at an absent directory.

### Version 2.1.303
Version 2.1.303 lists "Bug fixes and reliability improvements".
The release introduces no flag, command, or behavior changes.
The maintenance release updates internal reliability without prompt modifications.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs prompt diet and practice watch.
Decision D7 states that prompt diet trims one persona at a time.
Decision D8 requires 10-cycle baseline measures before and after each diet change.
Milestone M2 records the baseline measures first, and milestone M3 trims `evolve-builder`.
Source: the plan [model-capability-practice-2026-10.md](../plans/model-capability-practice-2026-10.md) defines D7, D8, M2 and M3 in its decision and milestone tables.
Therefore, no core persona changes in this cycle.

### evolve-scout
The `evolve-scout` persona requires no change in this cycle.
The scout phase identifies tasks and writes specifications.
The Claude Code CLI handles context compaction and background execution natively.
The scout prompt needs no added rules for these CLI capabilities.

Release 2.1.302 adds the `Idle compaction` row and fixes the "Prompt is too long" error at a stop. The CLI owns both, so the scout prompt needs no rule.
Release 2.1.303 adds routine bug fixes without CLI feature shifts.
The evolve-scout persona does not change.

### evolve-triage
The `evolve-triage` persona requires no change in this cycle.
Triage selects tasks based on deterministic policy rules.
CLI updates do not change triage routing logic or commitment rules.

Release 2.1.302 changes the auto mode classifier model and the fork subagent default. The inbox rank and the `## top_n` commitment read neither.
Release 2.1.303 adds reliability improvements with no task selection changes.
The evolve-triage persona does not change.

### evolve-tdd-engineer
The `evolve-tdd-engineer` persona requires no change in this cycle.
The TDD phase writes red tests and predicate suites.
The CLI improves tool error reporting for file search natively.
This improvement assists test discovery without persona prompt changes.

Release 2.1.302 fixes empty search results in the Windows PowerShell tool. The loop runs bash on macOS, so the TDD prompt needs no rule.
Release 2.1.303 contains bug fixes that require no test authoring prompt updates.
The evolve-tdd-engineer persona does not change.

### evolve-builder
The `evolve-builder` persona requires no change in this cycle.
Builder is the designated first diet candidate under milestone M3.
Under ADR-0133, diet changes require 10-cycle baseline measures under milestone M2 first.
Prompt changes must wait for milestone M2 baseline data under decision D8.

Release 2.1.302 adds no builder practice that the persona must state; the Read tool limit of 128 MB applies inside the CLI.
Release 2.1.303 provides maintenance fixes with no code editing alterations.
Therefore, the evolve-builder persona does not change in this cycle.

### evolve-auditor
The `evolve-auditor` persona requires no change in this cycle.
The auditor evaluates deterministic test exit codes and binary verdicts.
Claude Code changelog updates do not change auditor gates or report structures.

Release 2.1.302 restores `classifier_category` in telemetry events. The auditor grades test exit codes, not telemetry, so no rule changes.
Release 2.1.303 introduces bug fixes without affecting verification gates.
The evolve-auditor persona does not change.

### evolve-retrospective
The `evolve-retrospective` persona requires no change in this cycle.
The retrospective phase extracts lessons and updates instincts off the critical path.
CLI changes do not affect retrospective processing.

Release 2.1.302 fixes the reftable clone loop of plugin marketplaces. The retrospective reads the ledger and the lessons, not plugin marketplaces.
Release 2.1.303 adds internal stability fixes that do not change reflection logic.
The evolve-retrospective persona does not change.

## 4. Conclusion

The Claude Code CLI changes up to 2.1.303 provide bug fixes and reliability improvements.
No persona file changes in this cycle.
Prompt diet changes will follow ADR-0133 with measured baseline metrics in future cycles.
