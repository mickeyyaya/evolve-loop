# ADR-0155: The loop heals itself inside its boundaries: an agent proposes, and the pipeline disposes

- **Status:** Proposed (2026-10-25). Amended on 2026-10-26 by the operator: the catalog gets the action "dispatch phase agent". The orchestrator also judges the dispatch request of a phase agent ([llm-isolation.md](../llm-isolation.md) §7.2).
- **Plan:** [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) §11. **Research:** [llm-isolation-2026-10](../../research/llm-isolation-2026-10.md) §5. **Design:** [llm-isolation](../llm-isolation.md#17-self-healing-within-boundaries), Part II "Self-healing within boundaries".
- **Relates to:**
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md): the orchestrator agent is an isolated worker under I-1 to I-9. This ADR adds I-10 to I-14.
  - [ADR-0064](0064-pipeline-integrity-boundary.md): its protected surfaces are the base of the constitution (I-10).
  - [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): a repair that fails twice goes to the operator (I-13).
  - [ADR-0151](0151-the-host-writes-a-retrospective-document-for-each-recovery-and-each-judgment-lesson.md): an incident closes with a recovery retrospective (I-14).
  - [ADR-0153](0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md): the quality indexes detect incidents and prove that a repair did not regress (I-11, I-14).
  - [ADR-0101](0101-signal-center.md): the incident detector reads the Signal Center.
- **Evidence:**
  - The advisor already proposes and Go clamps the proposal: `CanTransition` and the ship floor (research F3.2).
  - `failure-decision.json` already has closed enums for the level and the action. The fix type is still an open string (research F3.1).
  - Today a pipeline defect gets a console fix, because "a loop cannot repair its own foundation while it stands on it" (F3.5).
  - A builder killed the shared tmux server, and a mutant ran `kill(-1, SIGKILL)` (F3.4).
  - Agents game a proxy when they can change it: a split-const dodge passed every gate (F3.3, P3.3).

## Context

The operator set the north star on 2026-10-25: "build the pipeline that auto agile loop which is driven by LLM agents, the pipeline defines the clear and clean boundary to make each agent and do it works within the given scope without gaming the system. Even error or accident happened, the loop itself can detect the issue and fix the pipeline issues with orchestrator agent analysis and actions to trigger phase agents to fix."

ADR-0154 says that no LLM controls another LLM. An "orchestrator agent" that triggers phase agents looks like a conflict. It is not, if the agent only writes a proposal and the deterministic pipeline does the dispatch. Today's advisor works this way. This ADR extends the same pattern from phase plans to incident repair.

## Decision

1. **Three layers.** The constitution, the pipeline and the agents.
2. The constitution holds the invariants, the gates, the oracles, the ledger, the routing floors and the stage flips.
3. Only the operator changes the constitution.
4. The pipeline is deterministic Go. It owns the flow, the detection, the validation and the dispatch.
5. The orchestrator agent is an isolated worker, like each other agent (ADR-0154).
6. **I-10 Constitution.** No agent changes a kernel surface. A diff classifier flags each change, a weaker gate among them.
7. **I-11 No agent writes its own oracle.** Checks freeze at handoff. Judges are independent or held out.
8. **I-12 Orchestrator proposes, pipeline disposes.** A proposal names one action from a closed catalog. Go checks its preconditions.
9. The catalog: retry with variation, reroute, quarantine, revert the last landing, open a repair item, halt and escalate. *Amended 2026-10-26:* dispatch a phase agent, on a request of a phase agent that the pipeline validated.
10. **I-13 Bounded autonomy.** Repair budgets for each window and circuit breakers apply. Two failed repairs escalate.
11. **I-14 Evidence closes an incident.** The red test is green, the signal clears and no index regresses.
12. A closed incident has a recovery retrospective (ADR-0151).
13. Detection is deterministic. The diagnosis is an LLM task. The decision and the dispatch are deterministic.
14. A repair runs as a normal cycle: scout, TDD (reproduce), build, audit, ship.
15. **Surface tiers.** T0 kernel: the loop only proposes. T1 pipeline code: the loop repairs with stronger review. T2 item code: normal flow.
16. T1 needs an independent-family judge, a mutation gate on changed functions and a `shadow` or canary stage.
17. Self-healing starts at `shadow`. The pipeline records proposals, and the console does the repair.
18. Each tier moves to `enforce` only on measured evidence and an operator flip.
19. The sandbox and the fence stop global effects of an accident (ADR-0154 X4, X7).

## Consequences

Good:

- The loop detects and repairs a T1 or T2 defect without a console session, inside the same boundaries as any cycle.
- The orchestrator agent cannot widen its power. It has no tool that dispatches, and the catalog is closed.
- An agent that weakens a gate is caught by a diff classifier, not by a reviewer's attention.
- The operator keeps the kernel. The loop never changes the rules that judge it.

Bad:

- Five more components. The work comes after X1 to X5, and X13 also waits for X8, so self-healing starts later.
- A wrong diagnosis costs a repair cycle. The budgets limit the cost, not the error.
- The classifier for a weaker gate can miss a new form of weakening. It covers the known forms only.
- The independent-family judge for T1 needs a second family with quota.

What changes our mind: if the `shadow` record shows a false-diagnosis rate above 30% over two windows, T1 stays at the console. If a weakened gate reaches `main` past the classifier, T1 goes back to `shadow`.

## Rejected

| Option | The fact that rules it out |
|---|---|
| An orchestrator agent with tools that dispatch phase agents | It is an LLM that controls LLMs. It breaks I-1 and I-3 of ADR-0154. |
| Deterministic playbooks only, with no LLM diagnosis | A new failure class has no playbook. The diagnosis is a judgment task (AGENTS.md rule 5). |
| The console fixes every pipeline defect, forever | It does not meet the north star. Each fix waits for a person (F3.5). |
| The loop repairs T0 surfaces | "A loop cannot repair its own foundation while it stands on it" (F3.5). Agents game a proxy that they can change (P3.3). |
| An open action space with free-text commands | A free command cannot be checked before it runs. A closed catalog can (P3.1, P3.5). |
| Show the quality indexes to the agents as targets | An index that is a target stops being a good measure (P3.3, ADR-0153 rule 8). |
