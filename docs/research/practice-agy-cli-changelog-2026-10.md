# Practice change in agy: changelog analysis and persona review (research, 2026-10)

> This document reviews the `agy changelog` updates for versions 1.3.4, 1.3.5, 1.3.6, and 1.4.0 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

Date: 2026-10-16. Method: read the `agy changelog` output, parse the text as untrusted data, and evaluate every core persona against ADR-0133.

## 1. Context and source

The practice watch detected updates from the source command `agy changelog`.
The text from the command is third-party data.
The loop treats the text as untrusted data and never follows it as prompt instructions.
The updates cover four releases: 1.3.4, 1.3.5, 1.3.6, and 1.4.0.

## 2. Changelog analysis as data

### Version 1.3.4
Version 1.3.4 adds a `--json` flag to `agy plugin list` for machine-readable output.
The release changes `--model` and `/model` to pick the default thinking level when `--effort` is absent.
The CLI limits automatic retries to 30 seconds instead of four minutes.
The headless `-p` mode exits with code 1 if an agent name is not found.

### Version 1.3.5
Version 1.3.5 adds Auto mode for Linux and macOS.
The release adds `/compact` to summarize conversations and free context space.
The release adds syntax highlighting and line folding to the `/diff` view.
Performance improves for large file diffs.

### Version 1.3.6
Version 1.3.6 adds Claude models for Google Cloud Application Default Credentials.
The headless `-p` mode skips subagent questions so that headless runs do not stop.
The release sets `GIT_EDITOR`, `HGEDITOR`, and `JJ_EDITOR` to `false` for agent commands.
This setting stops invisible editor hangs.

### Version 1.4.0
Version 1.4.0 adds terminal status reporting over OSC 7501.
The release adds `--strict` and `--json` flags to `agy plugin validate`.
The tool records final task output when background commands stop.
The release adds the `alt+m` shortcut for queued messages.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs prompt diet and practice watch.
Decision D7 states that prompt diet trims one persona at a time.
Decision D8 requires 10-cycle baseline measures before and after each diet change.
Milestone M2 records the baseline measures first, and milestone M3 trims `evolve-builder`.
Therefore, no core persona changes in this cycle.

### evolve-scout
The `evolve-scout` persona requires no change in this cycle.
The scout phase identifies tasks and writes specifications.
The agy CLI handles retries and status reporting natively.
The scout prompt needs no added rules for these CLI capabilities.

### evolve-triage
The `evolve-triage` persona requires no change in this cycle.
Triage selects tasks based on deterministic policy rules.
CLI updates do not change triage routing logic or commitment rules.
The triage persona does not change.

### evolve-tdd-engineer
The `evolve-tdd-engineer` persona requires no change in this cycle.
The TDD phase writes red tests and predicate suites.
The agy CLI suppresses editor prompts through `GIT_EDITOR=false`.
This CLI protection stops git commit hangs without persona changes.
The tdd engineer persona does not change.

### evolve-builder
The `evolve-builder` persona requires no change in this cycle.
Builder is the designated first diet candidate under milestone M3.
Under ADR-0133, diet changes require 10-cycle baseline measures under milestone M2 first.
Prompt changes must wait for milestone M2 baseline data.
Therefore, the builder persona does not change in this cycle.

### evolve-auditor
The `evolve-auditor` persona requires no change in this cycle.
The auditor evaluates deterministic test exit codes and binary verdicts.
Agy changelog updates do not change auditor gates or report structures.
The auditor persona does not change.

### evolve-retrospective
The `evolve-retrospective` persona requires no change in this cycle.
The retrospective phase extracts lessons and updates instincts off the critical path.
CLI changes do not affect retrospective processing.
The retrospective persona does not change.

## 4. Conclusion

The agy CLI changes improve headless execution, error recovery, and editor suppression.
No persona file changes in this cycle.
Prompt diet changes will follow ADR-0133 with measured baselines in future cycles.
