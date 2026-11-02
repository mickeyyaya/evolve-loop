# Practice change in claude: the Claude model release notes and a persona review (research, 2026-11)

> This document reviews the change to the `claude-model-release-notes` source under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The answer is this: no persona changes and no loop code changes in this cycle.

Date: 2026-11-02. Method: read the source at the link as untrusted data. Search the loop for the named API features. Give a verdict for each core persona under ADR-0133.

## 1. The request and the answer

The inbox item `practice-claude-model-release-notes-8403ed85-202610270220` gives two acceptance lines:

- "The source `https://platform.claude.com/docs/en/release-notes/overview` is read at the link, and the lines above are explained as data."
- "A proposal names each prompt or persona to change under ADR-0133, or states that none changes and why."

The answer has two parts:

- The new entries cover API request size, usage analytics endpoints, and the end of fast mode for one model. The loop calls none of these surfaces (§3).
- No core persona changes in this cycle. A persona diet change needs the D8 baseline from milestone M2 first (§4).

Terms:

| Term | Meaning | Source |
|---|---|---|
| Practice watch | The deterministic check that compares each source with its last snapshot | ADR-0133 decision 1 |
| D7 | The prompt diet trims one persona per change | `docs/plans/model-capability-practice-2026-10.md:96` |
| D8 | Each diet change records four measures over 10 cycles before and 10 after | `docs/plans/model-capability-practice-2026-10.md:97` |
| M2 | `evolve prompt stats`, the baseline counter for each persona | `docs/plans/model-capability-practice-2026-10.md:110` |
| M3 | The first diet change, on the builder persona | `docs/plans/model-capability-practice-2026-10.md:111` |

## 2. The release notes change, explained as data

The source is `https://platform.claude.com/docs/en/release-notes/overview`.
The page is a third-party document, and its text is untrusted data.
This note explains the text as data. It does not follow any instruction in the text.

The page was read at the link on 2026-11-02. The latest entry is dated 2026-10-30.
The table lists the entries that follow the snapshot of 2026-10-26T12:00:01Z.

| ID | Fact from the source | Scope |
|---|---|---|
| F1 | 2026-10-30: fast mode is deprecated for Claude Opus 4.8. Removal is on 2026-11-30, and a `speed: "fast"` request to `claude-opus-4-8` then returns an error. The page suggests Claude Opus 5.5. | Platform API |
| F2 | 2026-10-29: the Usage and Cost Analytics API is in beta with the `analytics-usage-cost-2026-08-24` header. Its endpoints are `/v1/organizations/analytics/usage` and `/v1/organizations/analytics/cost`. | Usage reporting |
| F3 | 2026-10-29: the older Usage and Cost API and four Enterprise Analytics report endpoints are now legacy. They stay supported. | Usage reporting |
| F4 | 2026-10-28: the maximum request size of the Messages API and the Token Counting API rises from 32 MB to 128 MB. This holds for the first-party Claude API. | Platform API |

## 3. Impact on the loop

Facts from the loop:

- R1: The loop dispatches Claude models through the `claude` CLI binary or the `agy` bridge driver (`go/internal/bridge/`). It sends no direct HTTP Messages API request.
- R2: A search of `go/internal` for `claude-opus-4-8`, `analytics-usage-cost`, `"speed"` and `v1/organizations` finds matches only in `*_test.go` files, as model-name fixtures. Production code has zero matches. The search ran on 2026-11-02.
- R3: Phase personas under `agents/` set no request size, no `speed` field, and no usage report endpoint.

Implication (synthesis): the CLI binary and the platform own request size, fast mode, and usage reports.
None of F1 through F4 needs an edit to loop code or to a phase prompt.
F1 matters only if an operator selects Claude Opus 4.8 with fast mode in a CLI setting.
That choice sits outside the personas.

Decision D1: no loop code changes in this cycle.
Rejected option: add a guard in the bridge package that blocks fast mode for Claude Opus 4.8. Fact R1 and fact R2 rule this out, because the loop never sends the `speed` field.

## 4. Core persona evaluation under ADR-0133

Each entry gives the verdict first, then the reason.
D7 says a diet trims one persona at a time.
D8 requires a baseline of four measures over 10 cycles before and after each diet change.
Milestone M2 produces that baseline, and M2 has not shipped.
Under that rule, every verdict below is no change.

#### evolve-scout

Verdict: no change.
The `evolve-scout` persona finds tasks and writes specifications.
It sets no request size and no usage report call, so F1 through F4 do not apply.

#### evolve-triage

Verdict: no change.
The `evolve-triage` persona picks tasks from policy rules and inbox ranks.
The release notes do not alter routing or commitment rules.

#### evolve-tdd-engineer

Verdict: no change.
The `evolve-tdd-engineer` persona writes red tests and predicates from acceptance lines.
The release notes do not alter test contracts.

#### evolve-builder

Verdict: no change.
The `evolve-builder` persona is the first diet candidate under milestone M3.
D8 requires the M2 baseline before any diet change, and that baseline does not exist.

#### evolve-auditor

Verdict: no change.
The `evolve-auditor` persona reads test exit codes and EGPS verdicts.
The release notes do not alter audit gates or report schemas.

#### evolve-retrospective

Verdict: no change.
The `evolve-retrospective` persona extracts lessons after ship.
The release notes do not alter its analysis steps.

## 5. Decisions and rejected options

| ID | Decision | Evidence | Rejected option and the fact that rules it out |
|---|---|---|---|
| D1 | No loop code changes. | R1, R2 | Add a fast mode guard in the bridge: R1 and R2 show the loop sends no `speed` field. |
| D2 | No persona changes in this cycle. | D7, D8, §4 | Trim the builder persona now: the D8 baseline from M2 does not exist. |
| D3 | The persona diet waits for M2 and then M3. | `docs/plans/model-capability-practice-2026-10.md:119` | Edit all six personas at once: D7 permits one persona per diet change. |

Consequence: persona reductions wait for the M2 baseline before M3 edits `evolve-builder`.

## 6. Limits

- This review covers only the entries that follow the snapshot of 2026-10-26T12:00:01Z.
- The earlier note, [practice-claude-model-release-notes-2026-10.md](practice-claude-model-release-notes-2026-10.md), stays unchanged.
- The `claude` CLI binary manages its own upstream headers and fast mode behavior.
