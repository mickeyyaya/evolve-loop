# Plan: one phase agent runs from the command line with a standard output and no cycle

- **Status:** built, 2026-10-21.
- **Design:** [agent-run](../architecture/agent-run.md). **Decision:** [ADR-0152](../architecture/adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md). **Research:** [agent-run-standalone-2026-10](../research/agent-run-standalone-2026-10.md).
- **Inbox item:** `agent-run-standalone-profile-cli` (consumed by the pull request that builds this plan).

## 1. Request

On 2026-10-21 the operator approved the build with "Yes, design then build". The operator wants to "leverage different phase agent (without triggering the full pipeline, just the phase agent you need for particular tasks)". The operator also wants to allow a "user or program to invoke particular agent through cli with the right input information and give the standard output result."

## 2. Facts

| Fact | Research id |
|---|---|
| `evolve phase` prints only the envelope, not the report. | F1.1 |
| `--cycle N` needs a real cycle, and an invented cycle number is forbidden. | F1.2 |
| `ResolveRunner` gives the persona, the routing chain, the bridge and the verdict in one call, but the bridge is fixed inside it. | F1.3 |
| No context key reaches every spec prompt. | F1.4, F1.5 |
| The write grant of a profile follows the workspace of the request. | F1.6 |
| A worktree on the plane tree gives the agent Git writes there. | F1.7 |
| The ledger takes manual entries with cycle 0 and a `cycle_label`. | F1.8 |
| The dispatch depth tells a call from a phase agent. | F1.9 |
| `core.PhaseWritesSource` separates the source-write profiles. | F1.10 |
| Headless agent CLIs print the answer on stdout and a JSON envelope on `--json`. | P2.1 to P2.3 |

## 3. Goals and non-goals

Goals:

1. `evolve agent run <profile>` reads the input on stdin or from files and prints the report on stdout.
2. `--json` prints one envelope with the report, the verdict, the model, the cost and the tokens.
3. The run uses no cycle number. It writes no cycle state, no lease and no run directory.
4. The run goes through the phase runner and the bridge factory, with the `cli_routing` chain of the profile.
5. `evolve agent list` names each profile that can run.

Non-goals for v1:

- A source-write profile (build, tdd, ship, `bug-reproduction`, `test-amplification`). It is refused.
- A built-in phase (scout, triage, audit and the others). Each one reads and writes cycle state.
- A JSON lines event stream, and a `--diff REF` flag. A caller pipes `git diff` instead.
- A run from inside a phase agent. It is refused.

## 4. Candidate designs

| Id | Design |
|---|---|
| A | A new `evolve agent` command builds a `core.PhaseRequest` with cycle 0 and a temporary workspace, and runs the spec runner from `ResolveRunner`. |
| B | A `--standalone` mode of `evolve subagent run`. The caller composes the prompt, and the subagent path launches it and writes the ledger. |
| C | A new command loads the persona and calls `bridge.NewDefault().Launch` directly. |
| D | The command allocates a real cycle directory and calls `evolve phase --cycle N`. |

Each criterion scores 0 to 3. A higher score is better.

| Criterion | Weight | A | B | C | D |
|---|---|---|---|---|---|
| One dispatch path: runner, routing table, bridge (F1.3) | 3 | 3 | 2 | 0 | 3 |
| Isolation from a live loop: no cycle state, lease or run dir (F1.2) | 3 | 3 | 1 | 3 | 0 |
| Output contract: report, verdict, cost, tokens (F1.1) | 2 | 3 | 1 | 2 | 2 |
| Size of the change | 1 | 2 | 2 | 2 | 1 |
| Test with a fake bridge (F1.3) | 1 | 3 | 2 | 3 | 1 |
| **Weighted total (max 30)** | | **29** | **16** | **18** | **17** |

Why A wins:

- A reuses the one dispatch path. The profile gets its routing chain, its fallback walk, its sandbox and its verdict rules with no new code.
- A writes only inside its own workspace and one ledger line.
- B needs a cycle number and composes the prompt outside the runner, so the persona and the routing rules move to the caller.
- C is a second dispatch path: it skips the routing table and the verdict, which the operator forbids.
- D creates a cycle directory, which a live loop and `evolve gc` can see, and which uses a cycle number with no cycle behind it.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | The commands are `evolve agent run <profile>` and `evolve agent list`. | P2.5 |
| D2 | The input is text: stdin, or `--input FILE` (`-` is stdin). Each `--file PATH` adds one fenced section. Empty input is exit 11. | P2.2, F1.4 |
| D3 | The spec runner renders the input under the context key `agent_input`, with a note that it replaces the cycle artifacts. | F1.4, F1.5 |
| D4 | stdout carries the report text. `--json` carries one envelope. stderr carries the logs. | P2.1, P2.6 |
| D5 | Exit 0 is PASS, WARN or SKIPPED; 1 is FAIL; 2 is refused; 3 is a dispatch error; 10 is usage; 11 is input. | P2.3, P2.6, F1.1 |
| D6 | The run has cycle 0 and a run label. Its workspace is `.evolve/ephemeral/agent-runs/<label>`. It is removed at the end unless `--keep`. | F1.2, F1.6 |
| D7 | The request sets no worktree, so the agent cannot get Git writes on the plane tree. | F1.7 |
| D8 | One ledger entry records the run: cycle 0, `cycle_label` = the run label, kind `agent_run`. | F1.8 |
| D9 | v1 refuses a source-write profile, a built-in phase and a call from inside a phase. | F1.9, F1.10 |
| D10 | `ResolveRunner` builds a spec phase through `phasecmd.NewSpecRunner(spec, root, bridge)`, so a test injects a fake bridge. | F1.3 |
| D11 | `evolve agent list` names each spec profile that `phasecmd.SpecRunnable` accepts and that writes no source. | F1.10 |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| A1 | `phasecmd.NewSpecRunner(spec, root, bridge)`, `DefaultBridge` and `SpecRunnable`; `ResolveRunner` calls them. | A fake bridge is the bridge of the returned runner. | console builder W |
| A2 | `specrunner.AgentInputKey` and the input block in `ComposePrompt`; `specrunner.ArtifactName`. | The prompt holds the input block for a spec with no `prompt_context`. | console builder W |
| A3 | `internal/cli/agentcmd`: `run` and `list`, the workspace, the ledger entry and the exit codes. | `evolve agent run adversarial-review` with a diff on stdin prints the report and leaves no cycle state, lease or run dir. | console builder W |
| A4 | The `agent` entry in `go/cmd/evolve/registry.go`, with the root router installer. | The registry test finds the command. | console builder W |
| A5 | Docs: this design set, the package notes and the runtime reference row. | `evolve docs ste-lint --strict` = 0. | console builder W |

## 7. Order

1. A1 and A2 first, because A3 calls both.
2. A3, with the acceptance test from the inbox item as its first red test.
3. A4, after A3 has its entry function.
4. A5 in the same pull request, because each code change maps to its spec.

## 8. Verification

- Before: `evolve agent` does not exist, and `evolve phase` needs a request JSON with a workspace, or a real cycle.
- After: the acceptance test runs `agent run adversarial-review` with a fake bridge on a temporary project. It asserts the report on stdout and an empty change set outside the ledger files.
- Coverage: each new and changed function at 100%, measured with `go tool cover -func`, and listed in the pull request.

## 9. Risks

| Risk | Control |
|---|---|
| A persona looks for the cycle artifact that its spec names, and fails. | The input block says that it replaces the cycle artifacts (D3). |
| A leftover workspace after a crash. | The workspace is outside `.evolve/runs`, so no cycle reader sees it. The `--keep` flag is the only deliberate leftover. |
| A run from a phase agent multiplies the cost. | The dispatch-depth refusal (D9). |
| The ledger entry confuses a cycle reader. | Cycle 0 and kind `agent_run` match no cycle and no dossier key (F1.8). |

## 10. Open questions

- A source-write profile in an explicit dev worktree: a later version can allow it with `--worktree DIR` that is not the plane tree. The condition is a fence that the live loop cannot see.
- `evolve gc` does not yet sweep `.evolve/ephemeral/agent-runs`. A sweep is needed only when `--keep` runs pile up.
