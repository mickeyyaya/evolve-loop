# ADR-0154: The pipeline is the only controller, and each agent is an isolated, replaceable worker

- **Status:** Proposed (2026-10-25). Amended by [ADR-0156](0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) (2026-10-25): the "pipeline markers" of I-2 are one named list (I-16). Amended on 2026-10-25 by the operator: rules 20 to 24 add I-17 (component X19). Amended on 2026-10-26 by the operator: I-3 allows the own subagents of a CLI and adds a dispatch request ([llm-isolation.md](../llm-isolation.md) §7.2).
- **Plan:** [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md). **Research:** [llm-isolation-2026-10](../../research/llm-isolation-2026-10.md). **Design:** [llm-isolation](../llm-isolation.md).
- **Relates to:**
  - [ADR-0064](0064-pipeline-integrity-boundary.md): the protected control plane. This ADR adds the agent side of the same boundary.
  - [ADR-0101](0101-signal-center.md): each invariant reports a violation through the Signal Center.
  - [ADR-0119](0119-one-routing-table-one-resolver.md): the one routing table. The family floor and the flow defaults move into it as data.
  - [ADR-0129](0129-cycle-scoped-agent-messaging.md): the cycle broker carries messages. A message goes through the pipeline, so it obeys I-4.
  - [ADR-0152](0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md): `evolve agent run` is the bridge path for the reviewers and the console seat.
  - [ADR-0155](0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): the companion decision. The orchestrator agent of self-healing is a worker under these invariants.
  - [ADR-0153](0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md): the module, kind and code pattern for the isolation signals.
  - [ADR-0156](0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md): the companion decision on the control surface. Its marker list (I-16) is the only source of `EVOLVE_*` and `BRIDGE_*` names in the I-2 allowlist.
- **Evidence:**
  - Every LLM child inherits the full environment of the launcher, and a test pins this (research F1.1).
  - Seven launch paths start an LLM binary outside the bridge launch path, and no test forbids it (F1.2).
  - A phase agent can start another LLM, because the depth marker is absent in a phase pane (F1.3).
  - The Claude floor and about 15 flow defaults are Go constants, and `gemini` maps to `claude-tmux` with no error (F1.5).
  - Current practice keeps the orchestrator as the only router, and gives each worker the least privilege that its task needs (P2.2, P2.3).

## Context

The operator asked on 2026-10-25: "the pipeline evo loop itself should the center of invoking all LLM cli by design... There is no dependency between a LLM to control over another LLM directly." And: "the agent should be isolated and let the pipeline to handle the flow and communication. Isolation is the key."

The phase order, the state machine and the typed contracts already follow this rule. Eleven gaps break it at the edges. They are in the environment, the launch paths, the reads, the family pins, the guards, the parsers, the instruction files, the console and the proof. A worker that leaks, reads or starts another worker makes the pipeline only one of several controllers.

The operator then set the limit of read isolation: "phase agent can read the output / deliverables from previous phases and also when retry and failed recover cycle, those suggestions and improvement recommendations should be accessible when needed (not necessary to be loaded at the beginning)." Isolation therefore removes raw transcripts, live channels and unrelated cycles. It keeps the deliverables and the lessons that the pipeline records.

## Decision

1. **I-1 Single launcher.** Only the bridge launch API starts an LLM binary.
2. **I-2 Hermetic environment.** A child gets an allowlist plus manifest variables. No other launcher variable reaches it.
3. **I-3 No nested agency.** An agent cannot start a new LLM CLI process of any family, its own family too. It cannot run an `evolve` dispatch verb.
   - *Amended 2026-10-26:* a subagent of the own harness, of any type and any model of the same family, is allowed. The harness chooses it, and it runs inside the boundary of the cell.
   - The pipeline enforces the cell boundary: the sandbox scope, the input index, the exec denies, the network, the budget and the telemetry. The usage of each in-harness subagent counts against the budget of the cell.
   - The latest-model rule governs only the model that the pipeline assigns to a seat. It does not govern the subagent choices of a harness.
   - To get another phase agent, the agent writes a dispatch request. The pipeline validates it, the orchestrator judges it, and only the pipeline dispatches.
4. **I-4 Pipeline-mediated communication.** Agents communicate only through declared artifacts that the pipeline records and indexes.
5. **I-5 Read isolation.** An agent reads its own cycle, its item lineage and its own CLI files, and nothing else.
6. **I-6 Family-agnostic flow.** Flow logic names no family. Floors, defaults and fallbacks are routing data.
7. **I-7 CLI-neutral guarantees.** The OS sandbox, the fence and the gates hold each guarantee. CLI hooks are extra.
8. **I-8 Typed flow decisions.** Each output that steers the flow is a declared file that a schema checks.
9. **I-9 Proven substitution.** A test proves that any family can fill any seat through a policy change.
10. Under I-4, no agent reads a raw transcript or a pane log, and no agent has a live channel.
11. Under I-4, a phase can read the declared deliverables of the earlier phases of its cycle.
12. Under I-5, the lineage of an item is its inbox item id and its retry or recovery chain.
13. A retry or a recovery can read the retrospectives, lessons and digests of its lineage.
14. The input bundle is an artifact index of readable artifacts. The agent opens only what it needs.
15. The sandbox grants reads of the indexed paths and denies other run records.
16. Each runtime invariant lands at `shadow` and goes to `enforce` after one clean wave. A static guard (I-1, I-6) lands at `enforce`.
17. Each invariant has one guard test and one Signal Center code, `ISOLATION_I<n>_VIOLATION`.
18. Today's Claude floor stays in force. It moves from Go code to the `floors` key of `cli_routing`.
19. The phase order, the transitions, the phase contracts and the gates do not change.
20. **I-17 The assignment is CLI-neutral.** The driver layer is the only home of CLI specifics.
21. A task assignment holds only neutral fields. Spec §28 lists them.
22. Model names, CLI flags, tool syntax, prompt delivery, environment, dialogs and parsers live only in the drivers and manifests.
23. The pipeline asks for capabilities. A seat that no routed driver can serve gets a routing refusal, never a silent downgrade.
24. Routing data names a driver identity. Only the bridge interprets it.

The operator said on 2026-10-25: "The Pipeline itself must be agnostic for the assignment and configurations, LLM specification can only be appended under the driver layer." I-17 is numbered after I-16 of ADR-0156, because ADR-0156 took I-15 and I-16 first.

## Consequences

Good:

- The pipeline is the only launcher, router and carrier of messages. A reader can find every LLM start in one package.
- Any family can take any seat with a policy change, and a test proves it.
- A prompt injection in one agent cannot reach another agent, an unrelated cycle or a secret through the read path.
- A phase still reads the deliverables of the earlier phases. A retry still reaches the lessons of its earlier tries, on demand.
- A safety guarantee does not depend on the hooks of one vendor's CLI.

Bad:

- The pipeline builds an index and read grants for each phase. A wrong lineage rule can hide a lesson that a retry needs.
- An allowlisted environment can drop a variable that a CLI needs. The manifest must declare it, and a missing one fails the launch.
- The OS sandbox rules grow. A wrong deny rule can stop a phase, so each rule starts in `shadow`.
- Eleven components touch the bridge, the sandbox, the router and the personas. The work takes several waves.

What changes our mind: if `shadow` shows that a rule fires on a legal phase action in two waves, the rule goes back to design. If a vendor CLI cannot run with an allowlisted environment, its manifest names the variables, and the invariant stays.

## Rejected

| Option | The fact that rules it out |
|---|---|
| (D) A privileged controller LLM that directs the other agents | It breaks the rule of the operator. One vendor's agent then drives another vendor's agent (plan §4). |
| Preload every earlier report and lesson into the prompt | It fills the context with text that the task does not need. The operator asked for access "when needed (not necessary to be loaded at the beginning)". |
| Deny reads of every other cycle | A retry then loses the retrospectives and lessons of its earlier tries in other cycles. |
| (B) Out-of-process phase workers over `pkg/phaseproto` now | It is the strongest cell, but it moves every phase at once. It stays the next step after A (plan §4). |
| (C) A container for each agent cell | Apple's `container` runs Linux VMs only on macOS 26 with Apple Silicon. A Linux cell loses the Keychain login and the host tmux pane (P2.4, plan §4). |
| Isolation through CLI hooks only | agy and codex panes get no hooks today (F1.6). A guarantee in one vendor's hook is not neutral. |
| Remove the Claude floor | The operator kept today's floor. The decision moves it to data and does not change its value. |
