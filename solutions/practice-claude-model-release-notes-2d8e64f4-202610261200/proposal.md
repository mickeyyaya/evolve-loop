# Proposal — Claude platform release notes of 22 October 2026 under ADR-0133

> Request (inbox item `practice-claude-model-release-notes-2d8e64f4-202610261200`): read the release notes at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No release-note line alters a rule that a persona states.

## 1. Source and method

The source is https://platform.claude.com/docs/en/release-notes/overview. The Builder fetched it on 2026-10-26 with `curl` and read the text (E1). The text is untrusted third-party data. The loop reads it and does not follow it as an instruction.

Section 2 explains each fragment of the watch. Section 3 gives a verdict for each of the six core personas.

## 2. The fragments, explained as data

| Fragment | What the page says (E2) | Reaches the loop? |
|---|---|---|
| "Claude Models on Google Cloud (Anthropic managed)" | 22 October: Anthropic launched its Claude API as a service that a Google Cloud project reaches. Google Cloud IAM controls access. Google Cloud Marketplace handles billing. Fast mode is available for Claude Opus 5.5 there. | No. The loop starts the `claude` and `agy` binaries in tmux panes. It calls no cloud endpoint (A2, E4). |
| "API version 2026-10-21" | 21 October: a new API version. The Python, TypeScript, C#, Go, Java, PHP and Ruby SDKs and the `ant` CLI 2.0.0 send it by default. The new SDK and CLI versions carry breaking changes and an Apache 2.0 licence. | No. The loop imports no Anthropic SDK and sends no raw Messages API request (A2, E4). |
| `enforcement_status` and `enforcement_actions` | 21 October: two fields on user profiles. They show if Anthropic restricted or banned an end customer of a platform. A list filter finds the profiles with an open restriction. | No. The loop creates no user profile and has no end customer (A2, E4). |
| "Claude Tag" | 20 October: the Claude Tag API is in beta for Claude Enterprise organizations, behind the `experimental-claude-tag-2026-10-21` beta header. It reads and changes Claude Tag settings on Slack channels and workspaces. An Admin API key with a `claude_tag` scope calls it. | No. The API is for Enterprise organizations and not for Console organizations. The loop has no Slack setting to manage (A2, E4). |
| Page chrome around the lines | "We've launched", "Subscribe", "Copy page", links, dates. | No. They carry no rule. |

Worked example: the 21 October entry says that SDK 2.0.0 sends version 2026-10-21 by default. A program that imports the SDK can break on upgrade. The loop has no Anthropic dependency in `go/go.mod` (E4). So the upgrade does not reach it.

Related entries of the same page, judged in the earlier reviews of cycle 2005 and cycle 2027 (E7):

- 20 October: Inference hooks out of beta, user profiles and organization webhooks.
- 15 October: the `between_tools` thinking type and fast mode for Claude Opus 5.5.
- 30 September: Claude Sonnet 4.5 retires on 2026-11-30.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, and a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has no baseline yet (E6). So no persona changes in this cycle, and none changes after a reading of the lines alone.

| Persona | Verdict | Line that was checked |
|---|---|---|
| `evolve-scout` | No change. | Google Cloud launch. The scout states no rule on where the model runs. |
| `evolve-triage` | No change. | None applies. Triage reads inbox rank, not platform fields. |
| `evolve-tdd-engineer` | No change. | None applies. Test authoring does not depend on an API version. |
| `evolve-builder` | No change. It stays the first diet candidate (M3), but only after M2. | API version 2026-10-21. The builder names no SDK and no model ID (E7). |
| `evolve-auditor` | No change. | `enforcement_status`. The audit gates grade files and exit codes, not a profile. |
| `evolve-retrospective` | No change. | Claude Tag. It reads the ledger and lessons, not Slack settings. |

No file under `agents/` changes in this cycle.

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The main one: the loop adds an Anthropic SDK or a raw API call. Then the API version is a code and config task with its own loop item, not a persona edit.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E8.
