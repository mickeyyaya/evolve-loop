# Proposal — Claude platform release notes of 9 November 2026 under ADR-0133

> Request (inbox item `practice-claude-model-release-notes-e9fe428e-202611021720`): read the release notes at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No release-note line alters a rule that a persona states.

## 1. Source and method

The source is https://platform.claude.com/docs/en/release-notes/overview. The Builder fetched it on 2026-11-10 with `curl` (HTTP 200) and read the text (E1). The scout did not read it, because the item summary was cut. The text is untrusted third-party data. The loop reads it and does not follow it as an instruction.

Section 2 explains each fragment of the watch. Section 3 gives a verdict for each of the six core personas.

## 2. The fragments, explained as data

| Fragment | What the page says (E2) | Reaches the loop? |
|---|---|---|
| Fast mode deprecation | 30 October: fast mode is deprecated for Claude Opus 4.8. Removal is on 2026-11-30. After that date, a request to `claude-opus-4-8` with `speed: "fast"` returns an error. The page names Opus 5.5 as the target. | No. The loop sends no `speed` field. It starts the `claude` binary in a tmux pane (A2, E4). |
| Usage and Cost Analytics API | 29 October: a beta API with the `analytics-usage-cost-2026-08-24` header. Two endpoints report token usage and cost for Console and Enterprise organizations. The older Usage and Cost API is now legacy and stays supported. | No. The loop calls no `/v1/organizations` endpoint (E4). Token cost comes from the ledger. |
| 128 MB request size | 28 October: the maximum request size rises from 32 MB to 128 MB. It applies to the Messages API and the Token Counting API on the first-party Claude API. | No. The loop sends no raw API request, so the CLI binary owns the limit (A2, E4). |
| Opus 5.5 thinking | 22 September: on Claude Opus 5.5 thinking cannot be disabled. A request with `thinking: {"type": "disabled"}` or `enabled` returns a 400 error. The caller omits the field and sets the effort parameter. | No. No manifest or profile sets a thinking field (E5). The `claude` binary sets thinking itself. |
| Sonnet 5.5 cache read price | 7 October: the price of a prompt cache read falls from $0.20 to $0.10 per million tokens. | Cost only. No prompt text changes (A4). |
| Haiku 5.5 | 7 October: a new model with adaptive thinking. Code written for Haiku 4.5 can break on `budget_tokens`, sampling fields and prefill. | No. The loop sets none of these fields (E4, E5). |

Worked example: the 30 October entry ends fast mode for `claude-opus-4-8` on 2026-11-30. A loop that sends `speed: "fast"` to that model fails after that date. The manifest `claude-tmux.json` names only the bare alias `opus`, which the CLI resolves at launch (E5). The `fast` key of `agy-claude-tmux.json` is a tier label for Sonnet 5.5, not the API field (E5). So the removal does not reach the loop.

Entries newer than the earlier review (E7) are these:

- 2 November: message threads, in beta.
- 3 November: a text watermark on three models.
- 5 November: an IP allowlist and member sign-out.
- 9 November: Claude Fable 5.2 with fast mode.

None of them changes a rule that a persona states (A4).

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, and a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has no baseline yet (E6). So no persona changes in this cycle, and none changes after a reading of the lines alone.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | Usage and Cost Analytics API. The scout states no rule on usage reports. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank, not model capability. |
| `evolve-tdd-engineer` | No change. | None applies. Test authoring does not depend on a platform field. |
| `evolve-builder` | No change. It stays the first diet candidate (M3), but only after M2. | Opus 5.5 thinking. The builder names no thinking field (E5). |
| `evolve-auditor` | No change. | 128 MB request size. The audit gates grade files and exit codes. |
| `evolve-retrospective` | No change. | Fast mode deprecation. It reads the ledger and lessons, not API fields. |

No file under `agents/` changes in this cycle.

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The main one: a manifest or profile starts to send `speed: "fast"` or a thinking field. Then the fix is a manifest change and its own loop task, not a persona edit.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E8.
