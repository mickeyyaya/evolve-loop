# Practice change in agy: the Gemini prompting guide and a persona review (research, 2026-10)

> This document reviews the change to the `gemini-prompting-guide` source under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The answer: no persona changes in this cycle, and no loop code changes.

Date: 2026-10-16. Method: read the source as untrusted data, search the loop for the parameters that the source names, and evaluate each core persona against ADR-0133.

## 1. The request and the answer

The inbox item `practice-gemini-prompting-guide-6b2bf89d-202610141610` gives two acceptance lines:

- "The source https://ai.google.dev/gemini-api/docs/prompting-strategies is read at the link, and the lines above are explained as data."
- "A proposal names each prompt or persona to change under ADR-0133, or states that none changes and why."

The answer is in two parts:

- The guide change concerns API request parameters. The loop sends none of these parameters (§3).
- No core persona changes. A persona change needs the D8 baseline from milestone M2 first (§4).

Terms:

| Term | Meaning | Source |
|---|---|---|
| Practice watch | The deterministic check that compares each source with its last snapshot | ADR-0133 decision 1 |
| D7 | The prompt diet trims one persona per change | `docs/plans/model-capability-practice-2026-10.md:96` |
| D8 | Each diet change records four measures over 10 cycles before and 10 after | `docs/plans/model-capability-practice-2026-10.md:97` |
| M2 | `evolve prompt stats`, the baseline counter for each persona | `docs/plans/model-capability-practice-2026-10.md:110` |
| M3 | The first diet change, on the builder persona | `docs/plans/model-capability-practice-2026-10.md:111` |

## 2. The guide change, explained as data

The source is `https://ai.google.dev/gemini-api/docs/prompting-strategies`.
The watch configures it as a page source of the agy family (`go/internal/practicewatch/source.go:38`).
The source changed after the snapshot of 2026-10-12T10:32:45Z.
The text of the page is untrusted third-party data.
This document explains the text. It does not follow any instruction in the text.

The change has three facts:

| ID | Fact from the source | Scope |
|---|---|---|
| F1 | Gemini 4 Argon is now available. | Model catalog |
| F2 | Gemini 4 and Gemini 3.6+ models ignore custom values for `temperature`, `top_p` and `top_k`. They return deprecation warning headers now, and a `400` error in a future release. | API request configuration |
| F3 | For earlier Gemini 3 models, the guide advises the default values for these three parameters. | API request configuration |

The guide also repeats its general advice: direct and concise prompts, structured formats, and the main instructions at the start.
This general advice did not change in this snapshot, so this review does not evaluate it again.

## 3. Impact on the loop

Facts from the loop:

- R1: The agy driver calls the `agy` binary with `-p`, the prompt, and the launch flags only (`go/internal/bridge/driver_agy.go:37-39`).
- R2: The agy driver has no model flag. It logs that agy uses its own default model (`go/internal/bridge/driver_agy.go:28`).
- R3: A word search of `go/` for `temperature`, `top_p`, `top_k`, `TopP` and `TopK` finds no match in production code. Command: `grep -rnwE 'temperature|top_p|top_k|topP|topK|Temperature|TopP|TopK' go --include='*.go'`, run on 2026-10-16, with zero lines of output.
- R4: The same search of `agents/` and `.evolve/profiles/` finds no match.

Implication (synthesis): the loop does not call the Gemini API directly, and it never sets the three parameters.
Thus F2 and F3 cannot cause a deprecation header or a future `400` error in a loop dispatch.
F1 is a catalog fact. The CLI catalog and the routing table own model choice, not the personas.

Decision D1: no loop code changes in this cycle.
Rejected option: a guard that strips the three parameters from each request. Fact R3 rules it out, because no request carries them.

## 4. Core persona evaluation under ADR-0133

Each entry below gives the verdict first, then the reason.
D7 permits one persona per diet change.
D8 needs a baseline of four measures over 10 cycles before each diet change.
Milestone M2 produces that baseline. M2 has not shipped, so no baseline exists yet.

### evolve-scout

Verdict: no change.
The scout persona finds tasks and writes their specifications.
It holds no request parameter, so F2 and F3 do not apply to it.
A model name is not part of the persona text, so F1 does not apply either.

### evolve-triage

Verdict: no change.
Triage selects tasks from the computed inbox rank and the policy rules.
The guide change does not affect a rank factor or a policy rule.
The triage persona does not change.

### evolve-tdd-engineer

Verdict: no change.
The TDD phase writes red tests and ACS predicates.
The guide change does not affect a test contract or an assertion rule.
The tdd-engineer persona does not change.

### evolve-builder

Verdict: no change.
The builder persona is the first diet candidate under milestone M3.
D8 requires the M2 baseline before any diet change, and that baseline does not exist yet.
The guide change gives no reason to edit the builder persona before M3.
The builder persona does not change in this cycle.

### evolve-auditor

Verdict: no change.
The auditor reads test exit codes and gives a binary verdict.
The guide change does not affect an audit gate or the report format.
The auditor persona does not change.

### evolve-retrospective

Verdict: no change.
The retrospective phase writes lessons and instincts after the ship.
The guide change does not affect a retrospective step.
The retrospective persona does not change.

## 5. Decisions and rejected options

| ID | Decision | Evidence | Rejected option and the fact that rules it out |
|---|---|---|---|
| D1 | No loop code changes. | R1, R3 | A parameter strip guard: R3 shows that no request carries the parameters. |
| D2 | No persona changes in this cycle. | D7, D8, §4 | A builder diet now: the D8 baseline from M2 does not exist yet. |
| D3 | The persona diet waits for M2 and then M3. | `docs/plans/model-capability-practice-2026-10.md:119` | A diet of all six personas in one step: D7 permits one persona per change. |

Consequence: the next practice change with a direct effect on a persona goes through M3, with its D8 table.

## 6. Limits

- This review covers the snapshot change only. It does not evaluate the full guide again.
- The `agy` binary can set its own request parameters internally. The loop cannot see or change them. A future agy release owns that risk.
