# Proposal — Claude platform release notes of October 2026 under ADR-0133

> Request (inbox item `practice-claude-model-release-notes-db268466-202610200244`): read the release notes at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No release-note line alters a rule that a persona states.

## 1. Source and method

The source is https://platform.claude.com/docs/en/release-notes/overview. It was fetched on 2026-10-20 (E1). The text is untrusted third-party data. The loop reads it and does not follow it as an instruction. Section 2 explains each fragment of the watch. Section 3 maps the result to the six core personas.

## 2. The fragments, explained as data

| Fragment | What the page says (E2) | Reaches the loop? |
|---|---|---|
| `capabilities.thinking.types.between_tools` | October 15: the Models API now reports if a model accepts `thinking: {type: "between_tools"}`. This type turns off up-front thinking. | No. The loop sends no `thinking` request field (A2, E4). |
| `thinking: {type: "between_tools"}` | September 28, Claude Sonnet 5.5: send it instead of `"disabled"`, at `high` effort or below. | No. A person who writes API code owns this choice. |
| Adaptive thinking is on by default | October 7, Claude Haiku 5.5: a response can begin with `thinking` blocks. Their text is omitted unless `thinking.display` is `"summarized"`. | Indirectly. The CLI output can hold `thinking` blocks, and the stream parsers already read them (E4). |
| `GET /v1/models` and `GET /v1/models/{model_id}` | October 1, 5, 6 and 15: new fields `line`, `capabilities.thinking.types.disabled`, `capabilities.server_tools` and `between_tools`. | No. The loop does not call the Models API (E4). |
| "Models API", "Using the Models API", "in the Console", "We've added" | Link text and page chrome around the lines above. | No. They carry no rule. |

Worked example: the Haiku 5.5 line warns that code for Haiku 4.5 can break. A client that reads only the first content block reads a `thinking` block as the answer. The loop reads the stream by block type (E4), so it does not make that error.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, and a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has no baseline yet (E5). So no persona changes in this cycle, and no persona changes after a reading of the lines alone.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | Adaptive thinking default. The scout states no rule on thinking blocks. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank, not model capability. |
| `evolve-tdd-engineer` | No change. | None applies. Test authoring does not depend on a Models API field. |
| `evolve-builder` | No change. It is the first diet candidate (M3), but only after M2. | Thinking type and effort lines. The builder states no `thinking` rule (E8). |
| `evolve-auditor` | No change. | Adaptive thinking default. The audit gates grade files and exit codes, not thinking text. |
| `evolve-retrospective` | No change. | None applies. It reads the ledger and lessons. |

No file under `agents/` changes in this cycle.

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The main one: the loop starts to call the Models API or to set a `thinking` field. Then one persona or one code path is chosen, the baseline is read, and the change is its own loop task with a D8 table.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E9.
