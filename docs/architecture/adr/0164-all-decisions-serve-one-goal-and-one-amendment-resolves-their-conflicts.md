# ADR-0164: All decisions serve one goal, and one amendment resolves their conflicts

- **Status:** Proposed (2026-11-06). The operator asked for this amendment. Decisions 2 to 15 are the console's reading of that request, and the operator can overrule each one.
- **Research:** [doc-contradictions-2026-11](../../research/doc-contradictions-2026-11.md).
- **Spec of the goal:** [loop-goal.md](../loop-goal.md).
- **History and evidence:** [decision-reconciliation-2026-11](../../history/decision-reconciliation-2026-11.md).
- **Amends:** ADR-0001, ADR-0058, ADR-0072, ADR-0080, ADR-0096, ADR-0126, ADR-0139, ADR-0151, ADR-0154 and ADR-0155. Each of those records holds a note that links here. Their earlier text stays unchanged.

## Context

The operator, on 2026-11-06: "I want to make sure all adrs and plans are actually pointing to the same goal : a self healing ai driven development loop which continue to deliver the the best quality features with top design and top quality deliverables. It relentless building features and is capable of fixing any issues it faced with logic and research to build the best practical options".

The operator then asked: "Make the amendment to resolve the confliction and record all the history with reasoning and actions and activities, the goal is to show the progress of reasoning and secure the chain of evidence".

The research found 10 contradictions, 2 more items that it withdrew or reclassified, and 7 gaps to the code (ids C and G below). It also found that no machine reads the status or the relations of an ADR. So a reader cannot tell which decision is current.

## Decision

| # | Decision | Fact | Why |
|---|---|---|---|
| 1 | The goal above is the one goal of each ADR and plan. [loop-goal.md](../loop-goal.md) maps each record to a part of the goal. | The operator's words | A record that serves no part of the goal needs a reason to stay. |
| 2 | An ADR number means the main series. Files in `docs/adr/` are the persona series, and a citation writes "persona ADR-NNNN". They do not move. | C1: ADR-0005 is two decisions | A move breaks links. A name removes the doubt. |
| 3 | A new ADR uses one status: Proposed, Accepted or Superseded. A record can add free text after the status. | C2: at least 15 status strings | A check can read a closed list later. |
| 4 | A status does not say if a record runs. A note on the record states the stage. Only the operator moves a record to Accepted. | G1: three Proposed records run at `enforce` | This states the fact and takes no flip from the operator. |
| 5 | A superseded part gets a note on the older record. The older text stays. | C3: ADR-0058 and ADR-0096 | History stays, and the current answer is one link away. |
| 6 | A record that names an absent script gets a note. Its decision stays as its status says. | C4: 16 records | The note warns the reader without a judgment on the decision. |
| 7 | The comment rule is: no comments, except what a tool reads and the one package doc that `go/acs/regression/docgo` requires. | C5: the digest refuses what a gate requires | The gate and the convention agree, and the digest was the one outlier. |
| 8 | The live values win over an older record. `fleet.min_lanes` is 4, and the hub is `.repo.git`. | C6, C7 | The record now names the value that runs. |
| 9 | Proposed: an LLM agent proposes, and deterministic Go disposes. Where ADR-0072 says that the orchestrator decides, it writes a proposal that Go checks. | C8. ADR-0072 line 38 records an operator-chosen model: "orchestrator decides, Go enforces floor". | This changes an operator-chosen model, so the operator must confirm it. Until then ADR-0072 stays in force. |
| 10 | This ADR adds an exception: the rule on LLM control holds inside the pipeline. The console acts for the operator, outside the loop, and runs a seat only through `evolve agent run`. | C9. ADR-0154 line 12 names that bridge path, and it states no exemption. | The console wrote this exception about itself, so the operator must confirm it. |
| 11 | Today a lane lands its own work, and the console lands through `evolve integration batch`. ADR-0147 is the target, and its driver runs at shadow. ADR-0147 amends ADR-0078, ADR-0128 and ADR-0134 as they state. ADR-0057 is not yet reconciled with ADR-0147; the operator decides (see the docs review of 2026-11-06). | C10 | A reader gets one answer to "what lands today". |
| 12 | A redesign rung starts with live research, and its card scores at least three options. This amends ADR-0126 decision 15. No code for it exists yet. | Goal: "logic and research to build the best practical options". The rung has no research step. | The goal names research, and the rung is the place where the loop changes its mind. |
| 13 | A halt is a step of healing and not an end. The two zero-ship rule stays. Each halt names its cause and the condition to resume, and ADR-0155 owns the path back. | Goal: "relentless". ADR-0072 stops the loop. | The two rules agree when a halt has a way back. |
| 14 | The goal heads the constitution of ADR-0155. Two L0 rules carry it to each agent, and [constitution.md](../constitution.md) holds the duties and the limits. | The operator: "all LLM agent need to know the ultimate goal we try to achieve to avoid only on the local optimization" | L0 is the one path that each agent loads when it starts, so the goal needs no new mechanism. |
| 15 | The goal is the top law of the project. It ranks above each other rule, and [constitution.md](../constitution.md) states the ranks. A lower text that opposes it goes to the operator. | The operator: "This goal should be the top law that all agents need to follow as it's "constitution" for this project" | The ranks are the console's reading. An agent gets no power to cross a gate, because a self-graded exception is a local win. |

The top law gives an agent no power to cross a limit. The operator asked for a stop to local optimization, and a skipped gate is one more local win.

## Not resolved by this ADR

| Id | Item | Why it stays open |
|---|---|---|
| Priority | The policy ranks `feature` fifth of eight classes and `security` last. Of 58 ready items, 29 have kind `feature` and 4 have class `feature`. | This changes what the loop builds. The operator decides. |
| P4 | Holdout scenarios | It needs a design of its own. |
| G2, G3, G4 | Verdict grammar, the oversight constructor, the eject signal | These are code faults. They follow the blocker path after this PR. |

C11 needs no change: ADR-0126 decision 14 states that `J_0` is block 1 and `J_3` is block 4, so the counts are one count.

## Closed after this ADR

C12 and G5 are closed. The operator, on 2026-11-06: "From this moment forward, you are authorized to utilize the evolve ship with all necessary permissions." The console now commits through `evolve ship --class manual`, and the ledger records that reason. The rule stays as written.

## Rejected options

| Option | The fact that rules it out |
|---|---|
| Edit each old record in place | The operator asked for a chain of evidence, and an edit erases the earlier reasoning. |
| One manual sweep and no record | Nothing fails when a record goes stale again (research section 6). |
| Change the commit rule to bless the console's practice | The rule is on a protected path, and the practice is the deviation. |
| Move each Proposed record that runs to Accepted | Earlier work reserves that move for the operator. |

## Consequences

- A reader starts at `loop-goal.md` and reaches the current decision of a topic.
- The notes are prose until the check of research option B exists. So a new record can go stale until that verb lands.
- Decisions 7, 10, 14 and 15 change hard-rule text in `guidance/rules/`. The generated digests change with them.
- The hard-rule edits apply when this PR lands, while this ADR reads Proposed. To overrule one, edit its file in `guidance/rules/`, then run `evolve guidance build` and `evolve skills generate`.
- Two rules join L0, so each agent carries about 50 more words. L0 stays inside the budget that `evolve guidance check` measures.
- The two goal rules have no enforcer in code. A check proves that the text is present, not that an agent acts on it.
- Inside a cell, the persona files keep their own words. A review of each persona against the goal is open.
