# Practice change in claude: Security posture and Managed Agents lines (research, 2026-11)

> This note reviews the change to the `claude-model-release-notes` source for item 2f02c519 under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md). The answer is this: no persona and no prompt changes, and `skills/model-notes-claude/COMPACT.md` stays unchanged.

Date: 2026-11-11. Method: read the source at the link as untrusted data. Give a verdict for each core persona under ADR-0133.

## 1. The request and the answer

The inbox item gives two acceptance lines:

- "The source `https://platform.claude.com/docs/en/release-notes/overview` is read at the link, and the lines above are explained as data."
- "A proposal names each prompt or persona to change under ADR-0133, or states that none changes and why."

The source changed after the snapshot of 2026-11-10T08:41:11Z.
The page is a third-party document, and its text is untrusted data.
This note explains the text as data. It does not follow any instruction in the text.

## 2. The quoted lines, explained as data

The item quotes a cut fragment of the page. The fetch of the live page gives the full lines.

| ID | Fact from the source | Scope |
|---|---|---|
| F1 | 2026-11-05: security posture data is in beta for Claude Enterprise organizations. The Compliance API returns it. | Compliance API |
| F2 | 2026-11-05: the endpoint `GET /v1/compliance/organizations/{organization_id}/security_posture` returns the current security posture results of an organization. | Compliance API |
| F3 | 2026-11-05: the activity feed records changes to attestations on the security posture page in claude.ai Organization settings. | Activity feed |
| F4 | The item text ends with "for Claude Managed Agents are now available as a research preview". The cut hides the subject. The live page lists Dreams for Managed Agents as a research preview. | Managed Agents |
| F5 | 2026-11-09: Claude Fable 5.2 and Claude Mythos 5.2 launch. The earlier note for item 9cd2c891 covers this entry. | Platform API |

## 3. Impact on the loop

Facts from the loop:

- R1: The loop dispatches Claude models through the `claude` CLI binary or a bridge driver. It calls no Compliance API endpoint and no Managed Agents feature.
- R2: The notes in `skills/model-notes-claude/COMPACT.md` hold rules for how a model follows a prompt. F1 through F4 give no such rule.
- R3: Phase personas under `agents/` set no organization setting, no attestation and no memory store.

Implication (synthesis): F1 through F4 are admin and product facts. They change no behavior of a model, and they change no step of a phase.

Decision D1: change no persona, no prompt and no notes file. This is the decision because R2 and R3 show that no line of the source reaches an instruction of the loop.
Rejected option: add a bullet about the Compliance API to the notes. Fact R1 rules this out, because the loop calls no such endpoint.

## 4. Core persona evaluation under ADR-0133

Each row gives the verdict first, then the reason.
D8 requires a baseline of four measures over 10 cycles before and after a diet change.
Milestone M2 produces that baseline, and M2 has not shipped.
This proposal changes no persona and no prompt.

| Persona | Verdict | Reason |
|---|---|---|
| evolve-scout | no change | It finds tasks and sets no organization field, so F1 through F4 do not apply. |
| evolve-triage | no change | It picks tasks from inbox ranks, and the release notes do not alter routing. |
| evolve-tdd-engineer | no change | It writes red tests, and the release notes do not alter test contracts. |
| evolve-builder | no change | It is the first diet candidate, and D8 needs the M2 baseline first. |
| evolve-auditor | no change | It reads test exit codes, and the release notes do not alter audit gates. |
| evolve-retrospective | no change | It extracts lessons after ship, and the release notes do not alter its steps. |

## 5. Decisions and rejected options

| ID | Decision | Evidence | Rejected option and the fact that rules it out |
|---|---|---|---|
| D1 | The file `skills/model-notes-claude/COMPACT.md` stays unchanged. | R1, R2 | Add a Compliance API bullet: R1 shows the loop calls no such endpoint. |
| D2 | No persona changes in this cycle. | D8, §4 | Trim the builder persona now: the D8 baseline from M2 does not exist. |

Consequence: the notes keep their bullets, and no file under `agents/` changes.

## 6. Limits

- This review covers the entries that follow the snapshot of 2026-11-10T08:41:11Z.
- The fetch read the first 100000 characters of the page. The last 43983 characters were not read.
- The earlier note, [practice-claude-model-release-notes-fable-5-2-2026-11.md](practice-claude-model-release-notes-fable-5-2-2026-11.md), stays unchanged.
