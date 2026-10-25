# ADR-0156: The CLI is the service interface, and the pipeline reads no ambient control flags

- **Status:** Proposed (2026-10-25).
- **Plan:** [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) §12. **Research:** [llm-isolation-2026-10](../../research/llm-isolation-2026-10.md) §6. **Design:** [cli-contract](../cli-contract.md) and [llm-isolation](../llm-isolation.md) Part III.
- **Relates to:**
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md): the agent cell. I-16 closes the `EVOLVE_*` and `BRIDGE_*` hole in the I-2 allowlist.
  - [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): a bypass flag is a way around the constitution. I-16 makes each bypass a visible parameter.
  - [ADR-0127](0127-push-only-event-channels.md): the event channels. The `events` verbs of plan E9 are verbs of this contract.
  - [ADR-0152](0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md): `evolve agent run` already has `--json`, `--project-root` and an exit-code list. Only its exit codes move to the shared table.
- **Evidence:**
  - 112 verbs, with no contract document and no shared exit-code table (research F4.1, F4.3).
  - `evolve loop status foo` starts a real loop (F4.4).
  - Five inbox verbs refuse during a wave, and `gc` takes no lease (F4.5, F4.7).
  - Each operator knob and each bypass in the environment reaches the LLM children (F5.5).
  - `policy.Parse` ignores an unknown key (F5.4).

## Context

The operator asked on 2026-10-25: "Make sure evo loop cli can be invoked for most of the core functions all the time. It's the interfaces for other services to use it's functions anytime." On the same day the operator added: "this refactory is also the good timing to remove the global flag that should be removed and replaced with dynamic configuration long time ago".

The two requests share one idea: the control surface of the pipeline is explicit. A caller reaches a function through a verb with a typed output. The pipeline takes its settings from the policy file and from parameters, never from the ambient environment.

### Why a new ADR, and not an amendment of ADR-0154

ADR-0154 is about the agent cell: what an LLM worker gets. This decision is about the callers and the settings of the pipeline itself. A service that calls `evolve` is not an agent, and a policy key is not a cell rule. One ADR has one decision (docs-write law 1), and ADR-0154 already has 19 rules. The two decisions here share one ADR because X18 changes the same surface that X17 documents: a bypass flag becomes a CLI parameter.

## Decision

1. **I-15 The CLI is the service contract.** Each core verb has a documented input, output, exit code and availability class.
2. Each core verb takes `--json` and `--project-root`, and needs no terminal input.
3. Each `--json` output is one envelope with a `schema` field, for example `evolve.loop.status/1`.
4. One exit-code table holds for each core verb. A verb can add a code only above 20.
5. Each core verb has one class: `always`, `queued`, `leased` or `boundary`.
6. An `always` verb never writes, and it works while a wave runs.
7. A `queued` or `leased` verb writes safely while a wave runs. It never refuses only because a lease is live.
8. A `boundary` verb refuses while a wave runs, and it says when to retry.
9. `cli-contract.md` lists each core verb. A guard test compares it with the registry.
10. **I-16 No ambient control.** The pipeline reads no control value from the environment.
11. The pipeline markers are one named Go list. Only those names reach a cell.
12. An operator knob moves into `.evolve/policy.json`. The pipeline decodes it strictly and reads it at each cycle or boundary.
13. A bypass becomes a CLI parameter that writes an audit record, or the bypass goes away.
14. A dead flag is deleted. A test seam moves behind test injection.
15. A guard test refuses a new environment read of a name outside the marker list.
16. `loop status <word>` is fixed first, because it starts a loop.

## Consequences

Good:

- A service, a script or another CLI can call each core function at any time, and parse the result.
- An operator change is a reviewed file edit, not a hidden shell variable.
- No knob and no bypass reaches an LLM child, so I-2 holds at `enforce` with no exception.
- A typo in the policy file fails loudly.

Bad:

- Each core verb gets a JSON envelope and new tests. The work spans several PRs.
- Scripts that set a knob in the environment break. Each moved knob keeps a one-wave shadow read that warns.
- A queued inbox change lands at the next boundary, not at once. The caller must read the queue state.
- The strict decode refuses a policy file that has stale keys today. X18a lists them before it turns strict.

What changes our mind: a service can need a push channel, or a call faster than a process start. Then a local daemon becomes a separate decision. If a knob must change inside a cycle, it gets a policy key that the pipeline reads at each phase, not an environment variable.

## Rejected

| Option | The fact that rules it out |
|---|---|
| A daemon or an RPC server as the service interface now | The CLI already reaches every function, and the only server has no authentication (research F4.11). A server adds a lifecycle and an attack surface. |
| Keep the knobs in the environment, as twelve-factor does for deploy values | The environment reaches each LLM child (F5.5, P4.3). A knob changes the flow, so it is not a deploy value (P4.4). |
| A deny list of bad environment names | A new name leaks until someone adds it (research §3, the same reason as I-2). |
| Every verb available to mutate at all times | `pr merge`, `sync-main` and `cycle reset` refuse during a wave for a correct reason (F4.6). |
| Amend ADR-0154 | It mixes the agent cell with the callers and settings of the pipeline. |
