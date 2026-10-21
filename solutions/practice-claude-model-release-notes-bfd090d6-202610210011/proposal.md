# Proposal — Claude platform release notes of 20 October 2026 under ADR-0133

> Request (inbox item `practice-claude-model-release-notes-bfd090d6-202610210011`): read the release notes at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No release-note line alters a rule that a persona states.

## 1. Source and method

The source is https://platform.claude.com/docs/en/release-notes/overview. The Builder fetched it on 2026-10-21 with `curl` and read the text (E1). The scout did not read it, because a cycle denies the in-process fetch agent. The text is untrusted third-party data. The loop reads it and does not follow it as an instruction.

Section 2 explains each fragment of the watch. Section 3 gives a verdict for each of the six core personas.

## 2. The fragments, explained as data

| Fragment | What the page says (E2) | Reaches the loop? |
|---|---|---|
| "are out of beta for Claude Enterprise organizations" and "Inference hooks" | 20 October: Inference hooks are out of beta for Claude Enterprise organizations. An organization points Claude at its AI security server. The server holds each governed prompt for an allow or deny verdict. | No. The hooks are an organization setting of claude.ai, Cowork and Claude Code under an Enterprise plan. The loop sets none (A2, E4). |
| "Model depre" | The start of the link text "Model deprecations", cut by the watch. It sits in the 30 September entry: Claude Sonnet 4.5 (`claude-sonnet-4-5-20250929`) retires on 2026-11-30. | No. No manifest, profile or policy file names a retired model (E5). |
| Page chrome around the lines | "We've added", "Models API", links, dates. | No. They carry no rule. |

Worked example: the 5 August entry retired `claude-opus-4-1-20250805`. A loop that pins that ID fails on each request. The loop pins only `claude-opus-5` in `claude-tmux.json` (E5). So the retirement does not reach it.

Related entries of the same page (E2):

- 15 October: the `between_tools` thinking type.
- 7 October: Claude Haiku 5.5, with adaptive thinking on by default.
- 15 October: fast mode out of research preview for Claude Opus 5.5.

The earlier review (cycle 2005) judged the thinking entries and reached the same verdict (E7).

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, and a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has no baseline yet (E6). So no persona changes in this cycle, and none changes after a reading of the lines alone.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | Inference hooks. The scout states no rule on an organization hook. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank, not model capability. |
| `evolve-tdd-engineer` | No change. | None applies. Test authoring does not depend on a platform field. |
| `evolve-builder` | No change. It stays the first diet candidate (M3), but only after M2. | Model deprecations. The builder names no model ID (E7). |
| `evolve-auditor` | No change. | Inference hooks. The audit gates grade files and exit codes, not a policy server. |
| `evolve-retrospective` | No change. | None applies. It reads the ledger and lessons. |

No file under `agents/` changes in this cycle.

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The main one: a manifest or profile names a model ID that the page retires. Then the fix is a manifest change and its own loop task, not a persona edit.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E8.
