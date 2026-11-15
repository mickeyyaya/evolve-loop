# Practice change in agy: agy changelog command, snapshot 2026-11-14T05:48:36Z (research, 2026-11)

> This document reviews item `practice-agy-cli-changelog-command-cefc57a2-202611151110` under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). It follows the [v1.5.9 command note](practice-agy-cli-changelog-command-1-5-9-2026-11.md).

Date: 2026-11-15. Method: run `agy changelog`, read the lines as untrusted data, and give each core persona a verdict under ADR-0133.

## 1. Context and source

The practice watch found four changed lines from `agy changelog` after snapshot `2026-11-14T05:48:36Z`.
One line comes from release `1.5.9`, and three lines come from release `1.4.4`.

This note read `agy changelog` directly on 2026-11-15 (`1.5.9` at the top, `1.4.4` at line 269).
The changelog text is untrusted data. The loop never follows it as a prompt rule.

## 2. Changelog analysis as data

Each row explains one line from `agy changelog` as untrusted data. It is not an instruction to the loop.

| ID | Fact | Source |
|---|---|---|
| F1 | Release `1.5.9` records `/browser` subagent sessions as `.mp4` files instead of `.webm` files. | `agy changelog`, `1.5.9` |
| F2 | Release `1.4.4` adds a `whole-file` toggle (`f`) and hunk folding (`Ctrl+O`) to `/diff` views (`Commit Tree` and `Per Turn`). | `agy changelog`, `1.4.4` |
| F3 | Release `1.4.4` attaches `_meta["antigravity.google/root_conversation_id"]` to MCP `tools/call` payloads from nested subagents. | `agy changelog`, `1.4.4` |
| F4 | Release `1.4.4` expands Auto mode sandbox review to check the preceding question on short replies and each pipeline subcommand. | `agy changelog`, `1.4.4` |

### Where the features belong

The `.mp4` browser recording change (F1) and the `/diff` view toggles (F2) belong to the CLI product and UI.
They add no rule about prompt format or phase behavior.

The MCP `root_conversation_id` metadata field (F3) applies only to MCP servers.
MCP is never used by the loop, because [AGENTS.md](../../AGENTS.md) Rule 12 reaches outside services through a CLI only.

The Auto mode sandbox check (F4) applies to interactive terminal approval in the CLI runtime.
The loop runs phases as isolated workers under [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md), so no prompt changes.

## 3. Core persona evaluation under ADR-0133

ADR-0133 rule D7 trims one persona at a time. Rule D8 requires four measures over 10 cycles before and after a change.
No line in this diff adds a measure, so no persona changes and no prompt changes.

| Persona | Verdict | Reason |
|---|---|---|
| `evolve-scout` | No change | The scout finds tasks. Neither `1.5.9` nor `1.4.4` adds a discovery rule. |
| `evolve-triage` | No change | Triage picks tasks by policy rules. No CLI UI or sandbox change alters routing. |
| `evolve-tdd-engineer` | No change | The persona writes red tests. None of F1 to F4 changes the test contract. |
| `evolve-builder` | No change | The builder is a diet candidate only after the D8 baseline. No line adds a build step. |
| `evolve-auditor` | No change | The auditor grades by exit codes. Neither `/diff` nor `.mp4` adds a gate. |
| `evolve-retrospective` | No change | The persona extracts lessons off the critical path. No line adds a lesson source. |

## 4. Decision

Decision D1: change no persona and no prompt for snapshot `2026-11-14T05:48:36Z`, because F1 to F4 are CLI product and MCP features.
Rule D8 also requires a 10-cycle baseline before any persona diet.

Decision D2: keep MCP out of the pipeline, because the loop reaches every outside service through a CLI.

Decision D3: add one bullet to `skills/model-notes-agy/COMPACT.md` that records this review, because the notes are the one home of agy practice.
The bullet grows the file from 17 lines to 18 lines. All older lines stay unchanged.

Rejected option: add MCP tool configuration to a phase. Rule 12 rules it out, because the loop uses CLIs and never MCP.

Rejected option: trim a persona now. D8 rules it out, because no before/after baseline exists.
