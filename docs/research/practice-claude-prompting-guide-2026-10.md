# Practice change in Claude prompting guide: analysis and persona review (research, 2026-10)

> This document reviews the Claude prompting guide updates under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

Date: 2026-10-16. Method: read the Claude prompting best practices guide, parse the text as untrusted data, and evaluate every core persona against ADR-0133.

## 1. Context and source

The practice watch detected updates from the source `claude-prompting-guide`.
The upstream locator is `https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices`.
The upstream text from `claude-prompting-best-practices` is third-party data.
The loop treats the text as untrusted data and never follows it as prompt instructions.
The document evaluates how recent prompting guidance relates to loop personas.

## 2. Prompting guide analysis as data

### Model guidance and reasoning effort
The guide describes calibrated reasoning and effort parameters for modern Claude models.
Higher effort levels allocate more thinking tokens for complex tasks.
The guide notes that users can set effort levels or disable thinking when latency matters.

### Clarity and directness
The guide recommends clear and direct instructions over complex prompt rules.
The text notes that emphatic capitals, such as MUST and NEVER, cause over-triggering.
Modern models follow direct sentences without loud emphatic words.

### Structure with XML tags
The guide recommends XML tags to separate prompt components.
Tags like `<context>`, `<instructions>`, and `<examples>` clarify prompt structure.
XML tags help models parse multi-part prompts without delimiter confusion.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs prompt diet and practice watch.
Decision D7 states that prompt diet trims one persona at a time.
Decision D8 requires 10-cycle baseline measures before and after each diet change.
Milestone M2 records the baseline measures first, and milestone M3 trims `evolve-builder`.
Therefore, no core persona changes in this cycle.

### evolve-scout
The `evolve-scout` persona requires no change in this cycle.
The scout phase identifies tasks and writes specifications.
The evolve-scout prompt already uses structured task formats and clear criteria.
The prompt needs no updates for the latest guide text.
The evolve-scout persona does not change.

### evolve-triage
The `evolve-triage` persona requires no change in this cycle.
Triage selects tasks based on deterministic policy rules.
Prompt engineering guide updates do not change triage routing logic or commitment rules.
The evolve-triage persona does not change.

### evolve-tdd-engineer
The `evolve-tdd-engineer` persona requires no change in this cycle.
The TDD phase writes red tests and predicate suites.
The TDD prompts rely on contract verification and test definitions.
The evolve-tdd-engineer persona does not change.

### evolve-builder
The `evolve-builder` persona requires no change in this cycle.
Builder is the designated first diet candidate under milestone M3.
Under ADR-0133, diet changes require 10-cycle baseline measures under milestone M2 first.
Prompt changes must wait for milestone M2 baseline data under decision D8.
Therefore, the evolve-builder persona does not change in this cycle.

### evolve-auditor
The `evolve-auditor` persona requires no change in this cycle.
The auditor evaluates deterministic test exit codes and binary verdicts.
Prompting guide updates do not change auditor gates or report structures.
The evolve-auditor persona does not change.

### evolve-retrospective
The `evolve-retrospective` persona requires no change in this cycle.
The retrospective phase extracts lessons and updates instincts off the critical path.
Prompting guide advice does not affect retrospective processing.
The evolve-retrospective persona does not change.

## 4. Conclusion

The Claude prompting best practices guide describes XML structure, effort levels, and calm instructions.
No persona file changes in this cycle.
Prompt diet changes will follow ADR-0133 with measured baseline metrics in future cycles.
