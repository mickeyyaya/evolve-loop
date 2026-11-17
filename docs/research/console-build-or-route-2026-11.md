# Console work: when the console builds an item and when it routes the item to a lane (research, 2026-11)

> The plan that uses this research: [Console work: build only what a lane cannot build](../plans/console-build-or-route-2026-11.md). Related plan: [the inbox classifier tiers](../plans/inbox-classifier-room-2026-11.md). The rule of record: [operating-policy.md §2, rule 6](../operations/operating-policy.md).

Date: 2026-11-17. Method: the live supply and inbox of wave 177 (`evolve wave supply --json`, `evolve inbox list --json`, `evolve inbox curate --json`), code reads with `path:line`, and `gh pr list`. A live web search covered six areas of practice. Each local claim names its command or file. "Synthesis" marks our own inference.

The operator asked on 2026-11-17:

> "How do we justify when to continue building the console only feature and when to stop?"

> "Research first and building the doc"

Terms used throughout:

- **Console.** The one operator-attended session. It can merge to main, run `gh`, and change the protected code.
- **Lane.** One cycle of the loop in its own worktree. A wave runs up to 8 lanes at once (the window).
- **Console route.** An item that only the console builds. `inboxbatch.ConsoleRouted` decides it.
- **Admissible item.** An item that a lane can draw now: no console route, its deps met, and no cell conflict.
- **Protected surface.** The paths that the gates and guards of the loop read. A lane must not change them.
- **Cell.** A part of the code that one lane holds while it edits it. Admission refuses a second lane in the same cell.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Current practice](#2-current-practice)
3. [Adopt or reject](#3-adopt-or-reject)
4. [Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

**F1.1** The lanes ran at 18% of the window in wave 177. `evolve wave status` at 02:26 UTC gave `width 1/8 · utilization 0.18`, and `pool: live 1`.
*Implication:* seven of eight lane slots stood idle while the console worked.

**F1.2** One item of 288 was admissible. `evolve wave supply --json` at 07:20 UTC gave these counts:

| Class | Items | Meaning |
|---|---|---|
| `admissible` | 1 | a lane can draw it now |
| `console_route` | 180 | the `route` field starts with `console` |
| `console_kind` | 49 | the kind is `pipeline-*` or `ref-state` |
| `console_protected` | 23 | a declared file is on the protected surface |
| `waiting_cell` | 19 | a running lane holds one of its cells |
| `waiting_deps` | 13 | a dependency is not consumed |

The classes hold 285 items. The other 3 of the 288 fall in no class that the verb prints.

*Implication:* the console route, not the lane count, sets the width of the wave.

**F1.3** No console route carries a stored protected path. Of the 180 console routes, `routed_by` is empty on 180, `protected_files` is empty on 180, and `routed_reason` is empty on 74 (`evolve inbox list --json`, 2026-11-17). The kinds are 71 `feature`, 24 `bug`, 23 `pipeline-repair`, 13 `fix`, 10 `pipeline-integrity` and 39 others. The protected check did not run on the 180, because the route returns first (F1.4).

The classifier plan uses a wider word. There, a fact is a protected file, a pipeline kind or a commit on main, and 69 of 221 routes had none (2026-11-16, its F1.2). By that word, the 33 `pipeline-*` items here have a fact. For the other 147, no stored field shows one.
*Implication:* most of the routes are a filing habit, not a capability that only the console has. The protected check must run on the 147 before a move.

**F1.4** The route field alone decides. `ConsoleRouted` returns `true` on a `console` route before it calls the protected check (`go/internal/inboxbatch/consoleroute.go:26-30`).
*Implication:* a route with no fact behind it has the same force as a route on a gate file.

**F1.5** The curator does not undo these routes. `evolve inbox curate --json` at stage `enforce` gave 12 proposals: 10 `consume` and 2 `merge`, and 0 reroutes to a lane. The classifier plan found the cause: the curator refuses a reroute for an item with free text in `routed_reason` ([inbox-classifier-room-2026-11.md](../plans/inbox-classifier-room-2026-11.md), F1.7).
*Implication:* the routes stay until a person or a new rule moves them.

**F1.6** The console has more work in flight than one boundary lands. `gh pr list --state open` on 2026-11-17 gave 28 open console PRs. The oldest is from 2026-11-09. One batch of the train landed 10 PRs on 2026-11-16 ([the train incident](../incidents/2026-11-17-the-integration-train-bisected-for-three-hours-on-a-red-main.md)).
*Implication:* the console starts work faster than it finishes work.

**F1.7** A console build costs more review than a lane build. PR #1531 took two design review rounds and two code review rounds (three seats each), then the commit gate. A lane cycle took a median of 12.0 LLM minutes to its verdict and 46.6 LLM minutes for each shipped cycle (`evolve wave status`, wave 177).
*Implication:* a console build of a lane item costs console hours. Only the console can spend those hours on merges, gates and reviews.

**F1.8** A coarse cell serializes the lanes. 19 to 22 waiting items named `cmd/evolve` among their cells, held by cycle 2447 (`evolve wave supply`, 02:30 and 07:20 UTC). Admission counts any shared edited package as a conflict (`go/internal/admission/admission.go:153-159`). Almost each new verb edits the one package `cmd/evolve`.
*Implication:* a lane-capable item can still wait. The fix is a lane enabler, not a console build.

**F1.9** The policy already names one class of console work. Operating policy §1.1 gives a pipeline-integrity defect a direct console fix. Such a defect is a false verdict, corrupted shared state, a broken main, or a failure without a diagnosis ([operating-policy.md](../operations/operating-policy.md)).
*Implication:* §1.1 covers integrity defects. No rule covers a console feature, and 71 console routes are features.

## 2. Current practice

The search ran on 2026-11-17. [P] marks a primary source that we read. [S] marks a secondary source.

**P2.1 Theory of Constraints.** Step 2 says: "Maximize the utilization and productivity of the constraint (NOT utilization and productivity of non-constraints)." Step 3 says the constraint "must never ever be starved for inputs". [P] [TOC Institute, five focusing steps](https://www.tocinstitute.org/five-focusing-steps.html). "Do only the work that only it can do" is a common reading, not a quote.

**P2.2 Queues.** Little's law gives lead time = work in progress / throughput. [S] [Thoughtworks, classes of service](https://www.thoughtworks.com/en-br/insights/blog/predictability-and-classes-service). Kingman's formula makes the wait grow as U/(1-U) with the load U of one server. [S] [Kingman's formula](https://en.wikipedia.org/wiki/Kingman%27s_formula).

**P2.3 Team Topologies.** "Platform teams create services that accelerate stream-aligned teams." "Enabling teams temporarily boost skills in other teams, then move on." [P] [Team Topologies, key concepts](https://teamtopologies.com/key-concepts).

**P2.4 Google SRE.** Keep toil "below 50% of each SRE's time". [P] [Eliminating toil](https://sre.google/sre-book/eliminating-toil/). When the budget is spent, "halt all changes and releases other than P0 issues or security fixes until the service is back within its SLO". [P] [Error budget policy](https://sre.google/workbook/error-budget-policy/).

**P2.5 Separation of duties.** SLSA v1.2: "Changes in protected branches MUST be agreed to by two or more trusted persons", and the uploader and the reviewer are two different persons. [P] [SLSA source requirements](https://slsa.dev/spec/v1.2/source-requirements). GitHub lets an owner protect the CODEOWNERS file itself. [P] [About code owners](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).

**P2.6 Agents change their own grader.** METR found that models raise their scores "by modifying the tests or scoring code" in 39 of 128 RE-Bench runs, and an instruction not to cheat did not stop it. [P] [METR, 2025-06](https://metr.org/blog/2025-06-05-recent-reward-hacking/). A model rewrote its own reward function 45 times in 32,768 trials. [P] [Anthropic, reward tampering](https://www.anthropic.com/research/reward-tampering).

**P2.7 Orchestrators delegate.** The lead agent "coordinates the process while delegating to specialized subagents". [P] [Anthropic, multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system). The Claude Code docs list "the lead starts implementing tasks itself instead of waiting for teammates" as a failure. They keep a "quick, targeted change" or shared-context work in the main session. [P] [Agent teams](https://code.claude.com/docs/en/agent-teams), [Subagents](https://code.claude.com/docs/en/sub-agents).

**P2.7b Counterpoint.** Cognition warns that agents without shared context make fragile systems. [P] [Don't build multi-agents](https://cognition.ai/blog/dont-build-multi-agents).

**P2.8 Cost of delay.** "If you only quantify one thing, quantify the cost of delay." [P/S] [SAFe, WSJF](https://framework.scaledagile.com/wsjf/). A full column means "stop starting, start finishing". [S] [Limit WIP](https://conceptualleader.substack.com/p/limit-wip-possibly-the-simplest-most).

**P2.9 Stop conditions.** Give an agent loop a stop condition, "such as a maximum number of iterations". [P] [Building effective agents](https://www.anthropic.com/engineering/building-effective-agents).

Weak sources: Little's law, Kingman and "stop starting" have only secondary sources here. We found no exact Goldratt text for "only the work only it can do".

## 3. Adopt or reject

| Practice | Recommendation | Reason |
|---|---|---|
| The constraint does only the work that only it can do (P2.1) | adopt | The console is the one server for merges, gate changes and reviews (F1.6, F1.7). |
| A protected-path list decides the console route (P2.5, P2.6) | adopt | A gate change by a lane is self-grading. A free-text route is not a fact (F1.3, F1.4). |
| Build the enabler, then route the item (P2.3) | adopt | 19 to 22 lane items wait on one coarse cell (F1.8). |
| Stop feature work while a measured signal is breached (P2.4) | adopt | Utilization 0.18 and 1 admissible item are measured breaches (F1.1, F1.2). |
| A work-in-progress limit and "stop starting" (P2.2, P2.8) | adopt | 28 open console PRs against about 10 for each batch (F1.6). |
| A 50% cap on console build hours (P2.4) | reject for now | No verb measures console hours. Add the cap when a verb measures it. |
| Rank console work by cost of delay per console hour (P2.8) | reject for now | The inbox rank has no cost-of-delay field. The stop rules give most of the gain. |
| Keep work in the console when the context is shared (P2.7) | adopt, narrow | Only for a small change in the files of an open console PR. |

## 4. Refinements to adopt

- **R1** A console route needs a capability that a lane must not have. That is a protected path, a console-only act (`gh`, a merge, a boundary, an operator decision), or a §1.1 integrity defect. (F1.3, F1.4, F1.9, P2.1, P2.5, P2.6)
- **R2** Lanes can lack a verb, a seam or a cell split. Then the console builds that enabler and routes the item. (F1.8, P2.3)
- **R3** While the supply is starved, the console starts no feature build. It works on the supply: routes, splits, enablers and merges. (F1.1, F1.2, P2.1, P2.4)
- **R4** While the open console PRs are more than one batch, the console starts no new build. It finishes the open ones first. (F1.6, P2.2, P2.8)
- **R5** The current stop rules stay: blocked more than 3 times means a redesign, and no review round 4. (P2.9, [the convergence policy](../architecture/convergence-policy.md))
