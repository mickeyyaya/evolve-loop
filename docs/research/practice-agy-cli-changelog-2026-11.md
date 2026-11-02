# Practice change in agy: changelog analysis and persona review (research, 2026-11)

> This document reviews the `agy changelog` updates for versions 1.4.3 through 1.4.12 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [October note](practice-agy-cli-changelog-2026-10.md), which ends at version 1.4.2.

Date: 2026-11-03. Method: read the `agy changelog` output, parse the text as untrusted data, and evaluate every core persona against ADR-0133.

## 1. Context and source

The practice watch detected changes in the source command `agy changelog` after the snapshot of 2026-10-26T18:27:01Z.
The text from the command is untrusted third-party data.
The loop never follows that text as prompt instructions.
The inbox item names releases 1.4.3 to 1.4.8. The live command now also lists 1.4.9 to 1.4.12, so this note covers all ten releases after 1.4.2.

## 2. Changelog analysis as data

Each entry below is a fact about the agy CLI. It is not an instruction to the loop.

### Versions 1.4.3 and 1.4.4
- Plugin validation checks hooks, sidecars and component lists, and stricter JSON and matcher checks follow in 1.4.4.
- A `PostToolUse` hook that fails no longer marks a successful tool call as failed.
- Subagents in Auto mode have their out-of-sandbox commands reviewed by a separate agent.
- Compaction keeps only the type, role and result of each subagent, and keeps tool declarations on summary requests.
- MCP `tools/call` requests carry the top-level conversation identifier.
- The system prompt states that `GEMINI.md` and `AGENTS.md` rules override earlier built-in instructions.

### Versions 1.4.5 and 1.4.6
- A blocked model response no longer causes an endless silent retry.
- Idle and streaming CPU use falls, and long conversations load faster.
- A plugin can give `skills` as a single path string.
- Sandboxed commands can no longer read or change `~/.docker`, `~/.kube` or `~/_netrc` without an explicit grant.
- Compaction keeps attached images and files, and an approval for one conversation applies to all its subagents.

### Versions 1.4.7 and 1.4.8
- Hook commands receive `lastUserInput` on stdin.
- Headless `-p` runs use much less CPU.
- Tools loaded on demand return results up to 10 KB directly.
- Failed, canceled and unknown-tool calls reach the agent as errors, so the agent can recover.
- A conversation fork cancels an inherited `/goal`.
- A custom agent name with a colon works, and a conversation delete cleans up its subagents.
- A memory leak from sidecar runs is fixed.

### Versions 1.4.9 and 1.4.10
- Approval of one Google site no longer approves sensitive Google apps.
- Always-allow rules for multi-line commands now take effect.
- Unmatched tool calls and results are left out of the request, and outdated thinking blocks are dropped.
- A subagent keeps its task and parent identifier after compaction.
- Images are capped at 2000 pixels per side, and streaming writes about 70% fewer bytes to the terminal.

### Versions 1.4.11 and 1.4.12
- A `PreInvocation` hook can return `terminate` to stop a turn before the model call.
- The JSON output formats add a `finished_reason` field, with the values `print_timeout`, `interrupted` and `error`.
- A plugin API key provider can be marked optional.
- Table rendering and plugin import fixes complete the set.

## 3. Core persona evaluation under ADR-0133

ADR-0133 governs prompt diet and practice watch.
Decision D7 states that prompt diet trims one persona at a time.
Decision D8 requires 10-cycle baseline measures before and after each diet change.
Milestone M2 records the baseline measures first, and milestone M3 trims `evolve-builder`.
The M2 baseline is not complete, so no core persona changes in this cycle.

### evolve-scout
The `evolve-scout` persona requires no change before the M2 baseline.
The scout phase finds tasks and writes specifications.
The changelog adds no capability that the scout prompt must name.

### evolve-triage
The `evolve-triage` persona requires no change before the M2 baseline.
Triage selects tasks with deterministic policy rules.
No release changes the routing or the commitment rules.

### evolve-tdd-engineer
The `evolve-tdd-engineer` persona requires no change before the M2 baseline.
The TDD phase writes red tests.
The hook and plugin checks do not change how it writes a test.

### evolve-builder
The `evolve-builder` persona requires no change before the M2 baseline.
Builder is the first diet candidate under milestone M3.
The 1.4.8 error handling for failed tool calls can later shorten the recovery text of the builder prompt.
That trim needs the 10-cycle baseline first, so it waits for M2 data.

### evolve-auditor
The `evolve-auditor` persona requires no change before the M2 baseline.
The auditor grades test exit codes and binary verdicts.
The `finished_reason` field of 1.4.11 can help a bridge tell a cut-off answer from a complete one, and no auditor rule changes.

### evolve-retrospective
The `evolve-retrospective` persona requires no change before the M2 baseline.
The retrospective phase extracts lessons off the critical path.
Compaction and subagent cleanup fixes do not change its input.

## 4. Conclusion

The ten releases improve error recovery, sandbox defaults, compaction and hook control.
No persona file changes in this cycle, and no prompt changes.
Two candidates can feed milestone M3 after the M2 baseline: the 1.4.8 tool-call error handling and the 1.4.11 `finished_reason` field.
