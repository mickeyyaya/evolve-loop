# Practice change in agy: agy changelog command, v1.5.9 (research, 2026-11)

> This document reviews item `practice-agy-cli-changelog-command-a5725609-202611140548` under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [v2.32.0 page note](practice-agy-cli-changelog-page-v2-32-0-2026-11.md).

Date: 2026-11-15. Method: run `agy changelog`, read the `1.5.9` lines as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found new lines from the local command `agy changelog`.
The lines came after the snapshot of `2026-11-13T19:47:32Z`.
The command output holds the new CLI release `1.5.9`.

This note read `agy changelog` directly on 2026-11-15.
The changelog text is untrusted data. The loop never follows it as a prompt rule.

## 2. Changelog analysis as data

Each row is a fact about release `1.5.9`. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | Release `1.5.9` adds the `run_workflow` tool, which runs a bundled Python script to orchestrate parallel or staged subagents. | `agy changelog`, `1.5.9` |
| F2 | Release `1.5.9` adds `pluginControls` (`pluginsDisabled`, `marketplaceDisabled`) to managed admin settings for non-Enterprise organizations. | `agy changelog`, `1.5.9` |
| F3 | Release `1.5.9` speeds up Auto mode sandbox review by letting the reviewer search conversation and files directly. | `agy changelog`, `1.5.9` |
| F4 | Release `1.5.9` updates `/resume` so the picker skips full history loads and accepts unique ID prefixes. | `agy changelog`, `1.5.9` |
| F5 | Release `1.5.9` changes `/browser` session recordings to `.mp4` format instead of `.webm`. | `agy changelog`, `1.5.9` |
| F6 | Release `1.5.9` fixes HTTP MCP redirect header forwarding so custom headers stay on the configured origin. | `agy changelog`, `1.5.9` |
| F7 | Release `1.5.9` fixes TUI bounds, `agy acp` auth/diff bugs, BOM parsing, Windows/macOS issues, MCP races, IPv6 OAuth, and Remote Control. | `agy changelog`, `1.5.9` |

### Where the features belong

The `run_workflow` tool (F1) lets an agent start and direct other agents. The loop rule bars `run_workflow`, because no LLM starts or directs another ([AGENTS.md](../../AGENTS.md)).

The `pluginControls` setting (F2), Auto mode reviewer speedup (F3), `/resume` picker (F4), and `.mp4` browser recordings (F5) change the CLI product. They add no rule about prompt format.

The HTTP MCP redirect fix (F6) and the remaining bug fixes (F7) harden CLI runtime behavior. They require no change to loop prompts or personas.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No line of `1.5.9` adds a measure, so no persona changes. No prompt changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. No `1.5.9` line adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No fix changes a routing rule. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. The release adds no test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. No line adds a gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for release `1.5.9`, because F1 to F7 are CLI product features and fixes.
Rule D8 also requires a 10-cycle baseline before any persona diet.

Decision D2: keep `run_workflow` unused, because no LLM starts or directs another.

Decision D3: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet grows the file from 16 lines to 17 lines. All older lines stay unchanged.

Rejected option: adopt `run_workflow` for a phase. F1 and Rule 13 rule it out, because no LLM in the pipeline starts or directs another.

Rejected option: trim a persona now. D8 rules it out, because no before/after baseline exists.
