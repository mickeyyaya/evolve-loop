# ADR-0152: one phase agent runs from the command line with no cycle, through the phase runner

- **Status:** Accepted (2026-10-21).
- **Plan:** [agent-run-standalone-2026-10](../../plans/agent-run-standalone-2026-10.md). **Research:** [agent-run-standalone-2026-10](../../research/agent-run-standalone-2026-10.md). **Design:** [agent-run](../agent-run.md).
- **Relates to:**
  - [ADR-0119](0119-one-routing-table-one-resolver.md): the run resolves its CLI chain through the same routing table.
  - [ADR-0132](0132-documents-are-written-through-one-skill.md): this design set follows the docs-write skill.
- **Evidence:**
  - `evolve phase` prints only the envelope, and `--cycle N` needs a real cycle (research F1.1, F1.2).
  - `ResolveRunner` already gives the persona, the routing chain, the bridge and the verdict (F1.3).
  - The ledger takes manual entries with cycle 0 and a `cycle_label` (F1.8).
  - Headless agent CLIs print the answer on stdout and one JSON envelope on `--json` (P2.1 to P2.3).

## Context

The operator wants to run one phase agent for one task, without the pipeline. The input comes on the command line, and the result goes to stdout. The existing commands need a cycle number and print only an envelope. An invented cycle number is forbidden, and a live loop must not see the run.

## Decision

1. `evolve agent run <profile>` runs one spec phase through `phasecmd.NewSpecRunner` and the shared phase runner.
2. The input is stdin text, or `--input FILE`, plus one fenced section for each `--file PATH`.
3. The spec runner renders the input under the key `agent_input` in every spec prompt.
4. stdout carries the report. `--json` carries one envelope with the report, verdict, model, cost and tokens.
5. Exit codes: 0 PASS, WARN or SKIPPED; 1 FAIL; 2 refused; 3 dispatch error; 10 usage; 11 input.
6. The run has cycle 0, no worktree and a workspace under `.evolve/ephemeral/agent-runs/<label>`.
7. The workspace is removed at the end unless `--keep` is given.
8. One ledger entry records the run with cycle 0, the label as `cycle_label`, and kind `agent_run`.
9. v1 refuses a source-write profile, a built-in phase and a call from inside a phase.
10. `evolve agent list` names each spec profile that can run and writes no source.

## Consequences

Good:

- A person or a program gets one agent answer from a pipe, for example `git diff | evolve agent run adversarial-review`.
- The run uses the routing chain, the sandbox and the verdict rules of the profile, with no second dispatch path.
- A live loop sees only one more ledger line. The run reads no cycle state and holds no lease.

Bad:

- v1 cannot run build, tdd, ship or the other source-write profiles.
- v1 cannot run a built-in phase such as audit or scout.
- A `--keep` workspace stays until a person removes it. `evolve gc` does not sweep it yet.

Change the decision when a fence for a dev worktree exists that a live loop cannot see. Then a source-write profile can run with `--worktree DIR`.

## Rejected

- **A `--standalone` mode of `evolve subagent run`**: it needs a cycle number, and the caller composes the prompt outside the runner (plan §4, design B).
- **A direct bridge launch**: it is a second dispatch path that skips the routing table and the verdict (design C).
- **A real cycle directory for each run**: it uses a cycle number with no cycle behind it. A live loop and `evolve gc` see the directory (design D, F1.2).
