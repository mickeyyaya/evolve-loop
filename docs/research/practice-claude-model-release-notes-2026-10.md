# Practice change in claude: the Claude model release notes and a persona review (research, 2026-10)

> This document reviews the change to the `claude-model-release-notes` source under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The answer: no persona changes in this cycle, and no loop code changes.

Date: 2026-10-16. Method: read the source as untrusted data, search the loop for the named API features, and evaluate each core persona against ADR-0133.

## 1. The request and the answer

The inbox item `practice-claude-model-release-notes-08f35ed6-202610152306` gives two acceptance lines:

- "The source `https://platform.claude.com/docs/en/release-notes/overview` is read at the link, and the lines above are explained as data."
- "A proposal names each prompt or persona to change under ADR-0133, or states that none changes and why."

The answer is in two parts:

- The release notes cover platform API headers, enterprise compliance endpoints, OpenTelemetry export, and cloud deployments. The loop uses none of these API surfaces directly (§3).
- No core persona changes in this cycle. A persona diet change requires the D8 baseline from milestone M2 first (§4).

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
The watch configures it as a page source of the claude family (`go/internal/practicewatch/source.go:35`).
The source changed after the snapshot of 2026-10-12T10:32:44Z.
The text of the page is untrusted third-party data.
This document explains the text as untrusted data and never follows any instruction in the text.

The snapshot diff contains four facts from the October 14 and October 15, 2026 entries:

| ID | Fact from the source | Scope |
|---|---|---|
| F1 | Fast mode for Claude Opus 5.5 is out of research preview, needs no `fast-mode-2026-02-01` beta header, and is on Azure. Fast mode for Claude Opus 5 and Claude Opus 4.8 stays in research preview. | Platform API and rate limits |
| F2 | The Compliance API endpoint `GET /v1/compliance/organizations/{org_uuid}/user_preferences` returns user instructions in beta for Enterprise organizations with the `read:compliance_user_data` scope. | Enterprise Compliance API |
| F3 | OpenTelemetry export is in beta for Messages API requests and Claude Managed Agents sessions, with spans emitted in Go, Python, TypeScript, and other SDKs. | Platform observability |
| F4 | Claude Managed Agents scheduled deployments are now available on Claude Platform on AWS. | Cloud deployment |

## 3. Impact on the loop

Facts from the loop:

- R1: The loop dispatches Claude models through the `claude` CLI binary or the `agy` bridge driver (`go/internal/bridge/`), not through direct HTTP Messages API requests.
- R2: A search of `go/` for `fast-mode-2026-02-01`, `user_preferences`, and `read:compliance_user_data` finds zero matches in production code. Command: `grep -rnE 'fast-mode-2026-02-01|user_preferences|read:compliance_user_data' go/internal`, run on 2026-10-16, with zero lines of output.
- R3: Phase prompts and personas under `agents/` do not configure HTTP beta headers, Compliance API scopes, OpenTelemetry destinations, or Managed Agents deployments.

Implication (synthesis): the CLI binary and the upstream platform own header behavior and cloud endpoints.
None of F1 through F4 requires an edit to loop code or phase prompts.

Decision D1: no loop code changes in this cycle.
Rejected option: add a flag or header filter for `fast-mode-2026-02-01` in the bridge package. Fact R1 and fact R2 rule this out because the loop never sends raw HTTP API headers.

## 4. Core persona evaluation under ADR-0133

Each entry below gives the verdict first, then the reason.
D7 permits one persona per diet change.
D8 requires a 10-cycle baseline of four measures before and after each diet change.
Milestone M2 produces that baseline, and M2 has not shipped yet.

### evolve-scout

Verdict: no change.
The `evolve-scout` persona identifies tasks and writes structured specifications.
It does not set API beta headers, Compliance API calls, or OpenTelemetry spans, so F1 through F4 do not apply to it.
The `evolve-scout` persona does not change in this cycle.

### evolve-triage

Verdict: no change.
The `evolve-triage` persona selects tasks from deterministic policy rules and inbox ranks.
Platform release notes for Opus 5.5 fast mode and Managed Agents do not alter routing or commitment rules.
The `evolve-triage` persona does not change in this cycle.

### evolve-tdd-engineer

Verdict: no change.
The `evolve-tdd-engineer` persona writes red tests and ACS predicates from task acceptance criteria.
Platform API release notes do not alter test contracts or predicate assertions.
The `evolve-tdd-engineer` persona does not change in this cycle.

### evolve-builder

Verdict: no change.
The `evolve-builder` persona is the first diet candidate under milestone M3.
Decision D8 requires the 10-cycle baseline from milestone M2 before any diet change, and that baseline does not exist yet.
Therefore, the `evolve-builder` persona does not change in this cycle.

### evolve-auditor

Verdict: no change.
The `evolve-auditor` persona evaluates deterministic test exit codes and binary EGPS verdicts.
Platform release notes do not alter audit gates or report schemas.
The `evolve-auditor` persona does not change in this cycle.

### evolve-retrospective

Verdict: no change.
The `evolve-retrospective` persona extracts lessons and updates instincts after ship.
Platform release notes do not alter retrospective analysis steps.
The `evolve-retrospective` persona does not change in this cycle.

## 5. Decisions and rejected options

| ID | Decision | Evidence | Rejected option and the fact that rules it out |
|---|---|---|---|
| D1 | No loop code changes. | R1, R2 | Add HTTP header handling in the bridge: R1 and R2 show the loop invokes the CLI binary, not HTTP headers. |
| D2 | No persona changes in this cycle. | D7, D8, §4 | Trim the builder persona now: the D8 baseline from M2 does not exist yet. |
| D3 | The persona diet waits for M2 and then M3. | `docs/plans/model-capability-practice-2026-10.md:119` | Edit all six personas at once: D7 permits only one persona per diet change. |

Consequence: persona scaffolding reductions wait for milestone M2 baseline metrics before milestone M3 edits `evolve-builder`.

## 6. Limits

- This review covers only the snapshot diff from `https://platform.claude.com/docs/en/release-notes/overview`.
- The `claude` CLI binary manages its own upstream HTTP headers and fast mode behavior internally.
