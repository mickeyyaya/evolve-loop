# ADR-0170: The loop keeps the LLM at the leaves and hardens its host, resume, fleet and self-edit edges

- **Status:** Proposed (2026-11-27). The operator asked: "Run another deep dive research to collect the popular open source harness design to learn more about their logics and architecture, record and summarize all the findings using the research skill (build one if it doesn't exist) with research report, adr, design, execution and plan following the policy defined under docs."
- **Plan:** [oss-harness-architectures-2026-11](../../plans/oss-harness-architectures-2026-11.md). **Research:** [oss-harness-architectures-2026-11](../../research/oss-harness-architectures-2026-11.md). **Design:** [harness-edge-contracts](../harness-edge-contracts.md) (the spec of record).
- **Relates to:**
  - [ADR-0154](0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) makes the pipeline the only controller. This record keeps that rule and adds no second controller.
  - [ADR-0137](0137-one-bridge-factory-resolves-every-llm-cli-and-the-manifest-holds-its-facts.md) puts the transport in manifest data. This record adds the typed event vocabulary to that data.
  - [ADR-0147](0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md) owns the merge train. This record adds the claim, the generation and the head-SHA check around it.
  - [ADR-0159](0159-the-loop-keeps-its-integration-train-and-takes-six-practices-from-the-2026-harness-projects.md) took six practices from the 2026-11-04 survey. This record takes the practices of the second survey and does not repeat the first.
  - [ADR-0167](0167-shadow-stages-prove-themselves-on-chained-evidence.md) governs each new stage key. This record adds no second bar for a move to `enforce`.
- **Evidence:**
  - F1.1, F1.2: the production chain starts on a tmux driver, and a stream-json driver for Claude exists.
  - F1.7: the four guards fall back to the tracked `go/evolve`. A guard that exits with a code other than 0 and 2 lets the action run.
  - F1.9: no graph, contract or policy hash exists in `go/internal`.
  - P3.5: the OpenHands gain of 61% came from the removal of a process boundary, which evolve-loop does not have.
  - P5.4: a DGM node removed the markers of a detector that was visible to it.

## Context

The loop already has one controller, typed phases, a hash-chained ledger and a merge train in shadow. The research of 38 open source harnesses shows that these are the parts that the field converges on. The weak parts are at three edges: the host transport, the resume identity and the self-edit path. The fleet tools add proven primitives for claims, liveness and breakers. The surveyed projects that failed carry LLM supervisors, force-accept rules or environment switches, which the hard rules of this repository forbid.

## Decision

The rules map to the plan decisions as follows:
- rules 1 and 2 to D18;
- 3 to D1, 4 to D4, 5 to D3, 6 to D2, 7 to D5, 8 to D6;
- 9 and 10 to D7, 11 and 12 to D8, 13 and 14 to D9;
- 15 to D11, 16 to D10, 17 to D13, 18 and 19 to D12;
- 20 to D15, 21 to D16.

D14 (the smoke test of a new build) and D17 (keep the ledger) are in the plan only.

1. Keep the pipeline as the only controller. Add no second loop driver.
2. Keep the LLM at the leaves. No LLM supervises, starts or ends another LLM.
3. Define one typed event and failure vocabulary. Every transport feeds the failure adapter with it.
4. Declare the approval mode and the sandbox mode of each CLI in its manifest.
5. Record the CLI session id in the ledger entry of each phase.
6. Run `claude-p` beside the tmux route as a shadow. Promote it only on the bar of rule 17.
7. Keep tmux as the fallback link, with typed failures.
8. Treat each phase as one durable unit. A resume never re-runs a finished CLI phase.
9. Write the graph hash, the contract hash and the policy snapshot hash in the first record of a cycle.
10. Refuse a resume on a mismatch of those hashes, unless the cycle declares a migration.
11. Mark a phase start with no end as `outcome_unknown`. Re-dispatch it only if its contract is idempotent.
12. Finish the restart reconcile before the driver admits a new lane.
13. Claim an inbox item with a lease and a generation number. Refuse a write with a stale generation.
14. Refuse a merge when the head moved since the claim.
15. Declare a stall only when two signals agree. Park an operator wait as `blocked` and do not retry it.
16. Quarantine an item after three dispatch failures. Route it to the console. Do not charge a stale base to the breaker.
17. Move each stage to enforce on one bar: one changed axis, at least two matched pairs and a reported interval.
18. Run a cycle that edits pipeline code on a pinned verified-good build.
19. Mount gate code, baselines and evaluator data read-only in each builder sandbox.
20. Describe each host as profile data. Render the knowledge layer from it, with a byte budget and an allowlist.
21. Add `onFailure: block` to the four guards. The operator owns this change, because it is gate config.

## Consequences

**Good.**

- Phase failures come from typed events, not from regexes on a pane.
- A resume cannot run on a changed graph or a changed policy.
- Waves share one driver. The width can rise from 0.20, and a second wave does not need an idle loop.
- A self-edit cannot weaken the evaluator that judges it.
- A guard that fails closes the action.

**Bad.**

- The ledger gains new record kinds, so every ledger reader must read them.
- A pinned build adds disk use and a drain step. State formats must stay readable by two builds.
- A shadow of `claude-p` beside tmux doubles the cost of one phase until the bar passes.
- `onFailure: block` can stop all guarded actions if a guard has a defect.
- The read-only mount can block a legitimate builder write. Each blocked write needs a fix of the mount data.

**What changes this decision.** If the shadow of `claude-p` loses the quota-pause or login signal, rule 6 stops at rule 7. If agy offers no structured mode, the typed vocabulary serves the tmux drivers only.

## Rejected

| Option | The fact that rules it out |
|---|---|
| Codex app-server JSON-RPC (A7) | The operator has no Codex subscription. The manifests stay as data. |
| An event-sourced runtime or an outside durable engine (A14) | The OpenHands gain came from the removal of a process boundary (P3.5). Evolve-loop has none. A rewrite discards the hash chain that no fleet tool has (P4.3). |
| LLM supervisors (A23) | A hard rule: no LLM starts or directs another. Gas Town's author reports that its design fell apart (P4.7). |
| Open-ended evolutionary search, force-accept and never-stop loops (A32) | They weaken a gate, break the stop rule for two empty cycles and cost about USD 22,000 per run (P5.5). |
| Per-hook switches and environment profiles (A41) | A switch is a feature flag, and an environment profile is a control value outside `policy.json`. |
| A second loop driver | The concurrency plan rejects it. Waves become tenants of one driver (WC6). |
| Pure optimistic admission | Ship recoveries already reach 46 in 29 of 53 cycles (PR #1847 dossier F4.9). Admission without a footprint check adds conflicts. WC5 makes admission queue, not refuse. |
