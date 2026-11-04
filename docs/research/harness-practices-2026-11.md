# Agent harness practice in 2026: what the open source projects do, and what we can apply (research, 2026-11)

> The plan that uses this research: [harness-practices-2026-11](../plans/harness-practices-2026-11.md). The design: [regression-proof-and-supervision](../architecture/regression-proof-and-supervision.md). The decision record: [ADR-0159](../architecture/adr/0159-the-loop-keeps-its-integration-train-and-takes-six-practices-from-the-2026-harness-projects.md). The earlier survey of merge systems: [wave-integration-2026-10](wave-integration-2026-10.md).

Date: 2026-11-04. Method: live web search, primary documents, the GitHub REST API for stars, one local git experiment, and code reads at `origin/main` (`23d09f7c6`). Each local claim names its file or command. "Synthesis" marks our own inference. This file does not decide. The plan decides.

## Table of contents

- [1. The request and the answer](#1-the-request-and-the-answer)
- [2. How to read this evidence](#2-how-to-read-this-evidence)
- [3. Our own failures in this period](#3-our-own-failures-in-this-period)
- [4. The project landscape](#4-the-project-landscape)
- [5. Merge and integration](#5-merge-and-integration)
- [6. Control, supervision and liveness](#6-control-supervision-and-liveness)
- [7. Durable execution and state](#7-durable-execution-and-state)
- [8. Tests, proof and agents that game tests](#8-tests-proof-and-agents-that-game-tests)
- [9. Flaky tests](#9-flaky-tests)
- [10. Context and token cost](#10-context-and-token-cost)
- [11. Code intelligence](#11-code-intelligence)
- [12. Sandboxes](#12-sandboxes)
- [13. Harness simplification](#13-harness-simplification)
- [14. Comparison with our design](#14-comparison-with-our-design)
- [15. Implications for the plan](#15-implications-for-the-plan)
- [16. Gaps in this research](#16-gaps-in-this-research)
- [17. Sources](#17-sources)

## 1. The request and the answer

The operator asked on 2026-11-04: "do a research online to see if there is a better solution than the one we already built. Also search for the open source projects to see what highest stars projects doing and absort their concept and design to see what we can apply here." The operator then asked for "the latest 2026 projects that's new and high stars in github" and for "the full and complete research report".

**The short answer (synthesis).** We found no outside system that replaces the integration train of [ADR-0147](../architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md). The nearest system, the Gas Town Refinery, uses the same core idea. We found six practices that close failures we saw in this period. The largest gap is proof: no gate checks that a fix has a test that fails without the fix. The direct cause of the queue on `main` is in our own process: lane deliverables can enter the train, and console PRs cannot (F5.10, F5.11).

## 2. How to read this evidence

**Stars are a weak signal.** The star counts come from the GitHub API on 2026-11-04. A star shows attention, not quality. Some counts look unusual: `morluto/rea` has 427,600 stars, and the repository dates from 2026-04. This research uses stars to find projects. It judges each project by its design.

**Sources have different weights.** A peer-reviewed paper or a primary engineering post is stronger than a README. A README is a claim from its author. Section 16 lists the sources that we read only in part.

| Label | Meaning |
|---|---|
| Measured | A benchmark, a study or a local run with its numbers |
| Primary | The document of the project or the team that built it |
| Claim | A statement of the author, with no independent check |

## 3. Our own failures in this period

These facts define the questions for the outside research.

**F3.1** Wave 152 shipped 1 of 9 cycles, and wave 153 shipped 0 of 1 (`evolve wave status`, 2026-11-04). Cycle 2151 used 121.7 LLM minutes, with 67.8 minutes of waste. The system-fail rate is 16.0% over waves 144 to 153 (`QUALITY_PQ4_BREACH`).
*Implication:* late failures cost more than any other class in this period.

**F3.2** Cycle 2151 ended on one `internal/bridge` red under `-tags integration`, and the failure text named no test. The retrospective reran the same tree 6 times, and all 6 runs passed (`.evolve/runs/cycle-2151/retrospective-report.md`).
*Implication:* one unexplained red ended 2.5 hours of work.

**F3.3** The diagnostic keeps only the last 400 bytes of the output (`go/internal/core/build_floor_reviewer.go:184-190`). `go test` prints the failed test before the later `ok` lines, so the name is lost. The audit tier retakes a red run serialized (`go/internal/phases/audit/ciparitygate/tier.go:94`). The build floor reports the first red (`go/internal/core/cover_changed_floor.go:43-47`).
*Implication:* two gates judge the same tests by different rules.

**F3.4** The role guard treats any nonzero `cycle_id` as an active cycle (`go/internal/guards/role.go:57`). After cycle 2151 sealed, `.evolve/cycle-state.json` still held `cycle_id` 2151 with `phase` `aborted`. The guard then refused the write of a standalone agent to its own run directory. The loop clears the marker only on a clean exit (`go/cmd/evolve/cmd_loop.go:223`), so an aborted cycle keeps its `cycle_id`.
*Implication:* a global state file is not proof that a cycle is alive.

**F3.5** The bridge ended the refused agent through its stall limit: "no output during the last 300s interval". The walk through 3 families took 18 minutes 22 seconds. A second incident met the same guard, so the step took 35 minutes (11:24:06 to 11:59:11 on 2026-11-04). The policy allows 2 diagnoses for each pass (`go/internal/policy/healing.go:15`).
*Implication:* the stall limit works, but a deterministic refusal walks the whole fallback chain.

**F3.6** The architecture seats found 10 CRITICAL findings on 5 green, fully covered trees: I3, I3b, I6, I10 and `fence-patch` (review results, 2026-11-04). One example: I3b read task ids as package names, so its batch test tested no package in production.
*Implication:* a green unit test with a made-up input does not prove a production path.

**F3.7** A `--` separator bug in the `claude-cloud` launch line passed every review, because each test used a fake CLI. It failed on the real CLI, and PR #1286 fixes it. Local proof: 4 real launches.
*Implication:* a fake does not prove a contract with a real CLI.

**F3.8** The test `TestPool_TwoTodosOfOneFamilyRunOneAfterTheOther` required an order of two concurrent launches (`go/internal/orchestrator/family_holds_test.go:71`). It failed on PR #1286 and passed 200 of 200 local runs under `-race`. A forced 50 ms delay reproduced the exact CI message.
*Implication:* a stress run does not prove a flake. A forced order does.

**F3.9** The landed identity of I0 uses `git patch-id --stable` on a default diff (`go/internal/integration/landed.go:55`). We tested three context widths on git 2.54.0. A build seat proposed the one-line width, and our test confirms it.

| Case | 3 lines (default) | 1 line (`-U1`) | 0 lines (`-U0`) |
|---|---|---|---|
| Peer edit 2 lines from the cycle change | Patch-ids differ: **wrong** | Match: right | Match: right |
| Same change text at another position | Differ: right | Differ: right | Match: **wrong** |

*Implication:* only one line of context passes both cases. A peer edit next to the change is a merge conflict, so that case never reaches this compare. Repeated blocks with the same neighbors still read as equal, and the trailer limits this.

## 4. The project landscape

Section 4 answers the question "what are the high-star projects, and which are new in 2026?". The table holds the projects that relate to this work. "Created" is the year and month of the repository.

### Agents and runtimes

| Project | Stars | Created | What it does |
|---|---:|---|---|
| [anomalyco/opencode](https://github.com/anomalyco/opencode) | 224,398 | before 2026 | Open source coding agent |
| [anthropics/claude-code](https://github.com/anthropics/claude-code) | 154,446 | before 2026 | Terminal coding agent |
| [openai/codex](https://github.com/openai/codex) | 133,051 | before 2026 | Terminal coding agent |
| [google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli) | 108,710 | before 2026 | Terminal coding agent |
| [OpenHands/OpenHands](https://github.com/OpenHands/OpenHands) | 92,599 | before 2026 | Agent platform and SDK |
| [cline/cline](https://github.com/cline/cline) | 71,277 | before 2026 | Agent SDK, IDE extension and CLI |
| [aaif-goose/goose](https://github.com/aaif-goose/goose) | 56,195 | before 2026 | Extensible agent |
| [Aider-AI/aider](https://github.com/Aider-AI/aider) | 49,931 | before 2026 | Pair programmer with a repo map |
| [xai-org/grok-build](https://github.com/xai-org/grok-build) | 27,677 | 2026-07 | Harness and terminal interface |
| [1jehuang/jcode](https://github.com/1jehuang/jcode) | 21,447 | 2026-01 | Rust harness for many parallel agents |

### Fleet tools and orchestrators

| Project | Stars | Created | What it does |
|---|---:|---|---|
| [stablyai/orca](https://github.com/stablyai/orca) | 114,667 | 2026-03 | Desktop app; one worktree for each agent |
| [paperclipai/paperclip](https://github.com/paperclipai/paperclip) | 110,862 | 2026-03 | Org charts, budgets and heartbeats for agents |
| [herdrdev/herdr](https://github.com/herdrdev/herdr) | 47,906 | 2026-03 | Terminal runtime that marks each pane working, blocked or idle |
| [Yeachan-Heo/oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode) | 40,546 | 2026-01 | Teams on top of Claude Code |
| [BloopAI/vibe-kanban](https://github.com/BloopAI/vibe-kanban) | 28,584 | before 2026 | Kanban board for agents |
| [openai/symphony](https://github.com/openai/symphony) | 27,931 | 2026-02 | Language-neutral spec for an unattended orchestrator |
| [snarktank/ralph](https://github.com/snarktank/ralph) | 22,114 | 2026-01 | Loop with a fresh agent for each iteration |
| [gastownhall/gastown](https://github.com/gastownhall/gastown) | 18,687 | 2025-12 | Multi-agent workspace with a merge queue |
| [ParadevAI/Paradev](https://github.com/ParadevAI/Paradev) | 15,887 | 2025-10 | Agentic IDE for parallel agents |
| [google/ax](https://github.com/google/ax) | 14,509 | 2026-03 | Declarative cluster runtime for agent tasks |
| [OrchestratorInc/agent-orchestrator](https://github.com/OrchestratorInc/agent-orchestrator) | 14,058 | 2026-02 | Fleet from task to merged PR |
| [cobusgreyling/loop-engineering](https://github.com/cobusgreyling/loop-engineering) | 11,689 | 2026-06 | Pattern library for agent loops |
| [omnigent-ai/omnigent](https://github.com/omnigent-ai/omnigent) | 11,195 | 2026-06 | Meta-harness over many CLIs |
| [mattpocock/sandcastle](https://github.com/mattpocock/sandcastle) | 8,822 | 2026-03 | Sandboxed agent runs as a library call |
| [builderz-labs/mission-control](https://github.com/builderz-labs/mission-control) | 6,439 | 2026-02 | Local control plane on SQLite (alpha) |
| [gastownhall/gascity](https://github.com/gastownhall/gascity) | 1,480 | 2026-02 | SDK extracted from Gas Town |
| [umputun/ralphex](https://github.com/umputun/ralphex) | 1,508 | 2026-01 | Plan execution with a multi-phase review |

### Harness knowledge and skills

| Project | Stars | Created | What it does |
|---|---:|---|---|
| [mattpocock/skills](https://github.com/mattpocock/skills) | 311,624 | 2026-02 | Engineering skills for agents |
| [affaan-m/ECC](https://github.com/affaan-m/ECC) | 296,616 | 2026-01 | Harness configuration and skills |
| [garrytan/gstack](https://github.com/garrytan/gstack) | 138,898 | 2026-03 | Opinionated Claude Code setup |
| [addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) | 113,074 | 2026-02 | Production engineering skills |
| [karpathy/autoresearch](https://github.com/karpathy/autoresearch) | 99,190 | 2026-03 | Unattended research loop |
| [walkinglabs/learn-harness-engineering](https://github.com/walkinglabs/learn-harness-engineering) | 23,082 | 2026-03 | Harness engineering tutorial |
| [ai-boost/awesome-harness-engineering](https://github.com/ai-boost/awesome-harness-engineering) | 5,049 | 2026-03 | Reading list |
| [QoderAI/better-harness](https://github.com/QoderAI/better-harness) | 2,437 | 2026-07 | Reads session evidence and proposes improvements |

### Infrastructure and merge systems

| Project | Stars | Created | What it does |
|---|---:|---|---|
| [Graphify-Labs/graphify](https://github.com/Graphify-Labs/graphify) | 141,924 | 2026-04 | Knowledge graph of a codebase |
| [rtk-ai/rtk](https://github.com/rtk-ai/rtk) | 85,494 | 2026-01 | Proxy that compresses tool output |
| [colbymchenry/codegraph](https://github.com/colbymchenry/codegraph) | 76,437 | 2026-01 | Local code graph |
| [DeusData/codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) | 48,075 | 2026-02 | Local code intelligence |
| [langchain-ai/langgraph](https://github.com/langchain-ai/langgraph) | 44,421 | before 2026 | Graph runtime with checkpoints |
| [mksglu/context-mode](https://github.com/mksglu/context-mode) | 33,406 | 2026-02 | Keeps tool output out of the context window |
| [conductor-oss/conductor](https://github.com/conductor-oss/conductor) | 32,409 | 2023-12 | Durable workflow engine |
| [tirth8205/code-review-graph](https://github.com/tirth8205/code-review-graph) | 32,580 | 2026-02 | Code graph for review context |
| [gastownhall/beads](https://github.com/gastownhall/beads) | 28,466 | before 2026 | Git-backed work ledger |
| [temporalio/temporal](https://github.com/temporalio/temporal) | 24,173 | before 2026 | Durable execution service |
| [TencentCloud/CubeSandbox](https://github.com/TencentCloud/CubeSandbox) | 13,152 | 2026-04 | Sandbox for agents |
| [trailhq/Trail-Code](https://github.com/trailhq/Trail-Code) | 11,073 | 2026-07 | Code map and learnings; our code index |
| [microsoft/agent-governance-toolkit](https://github.com/microsoft/agent-governance-toolkit) | 6,513 | 2026-03 | Policy and identity for agents |
| [bors-ng/bors-ng](https://github.com/bors-ng/bors-ng) | 1,527 | before 2026 | Merge bot; archived |
| [go-gremlins/gremlins](https://github.com/go-gremlins/gremlins) | 456 | 2022-06 | Mutation testing for Go |
| [uber/submitqueue](https://github.com/uber/submitqueue) | 245 | 2026-01 | Speculative merge queue |
| [rust-lang/bors](https://github.com/rust-lang/bors) | 183 | before 2026 | Merge bot of the Rust project |
| [funador/claude-code-merge-queue](https://github.com/funador/claude-code-merge-queue) | 130 | 2026-07 | Local FIFO queue for parallel agents |

**F4.1** Most high-star 2026 projects are skills, configuration, desktop interfaces or token tools. A merge queue with more than 300 stars is rare (`topic:merge-queue`, GitHub search, 2026-11-04).
*Implication (synthesis):* attention follows the single-operator experience. The integration problem at our scale has few open source answers.

## 5. Merge and integration

**F5.1 (Primary)** Gas Town has a Refinery that batches pending merge requests, rebases them as a stack and tests the tip. On a pass it fast-forwards all of them. On a fail it bisects. Its document says "polecats never push directly to main" and calls the design a core capability ([README](https://github.com/gastownhall/gastown), [architecture](https://github.com/gastownhall/gastown/blob/main/docs/design/architecture.md)). Its own table lists the batch phase as blocked by an unfinished gate phase, so the batch is not built yet.
*Implication:* an independent project reached the same core design as integration plan D1, D10 and D11.

**F5.2 (Primary)** Uber published `uber/submitqueue` in 2026. It validates changes "against predicted future states of HEAD" and isolates the failed change. The stack is a gateway, an orchestrator, a runway and MySQL. It says that it is "under active development", and it has no release ([repository](https://github.com/uber/submitqueue/)).
*Implication:* it is a design source today. It is not a dependency that we can take.

**F5.3 (Measured)** Uber lands changes on disjoint sets of packages in any order. A target version is a hash of its input files and of its dependencies. When changes passed a large diff, the June P95 wait fell 74% against April ([Uber](https://www.uber.com/en-US/blog/bypassing-large-diffs-in-submitqueue/), 2023).
*Implication:* this supports the later partition stage of ADR-0147 (design D of its plan). It gives a measured reason to build that stage when a large diff blocks a batch.

**F5.4 (Primary)** Zuul starts its window at 20, adds 1 on each merge and halves it on each failure ([Zuul](https://zuul-ci.org/docs/zuul/latest/gating.html)). GitHub removes a failed PR and rebuilds the others without it ([GitHub](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue)).
*Implication:* integration plan D6 and D8 match current practice. Both facts confirm M2 and M3 of the earlier research.

**F5.5 (Primary)** Rust rollups mark each PR `always`, `maybe`, `iffy` or `never`. An author bisects a failed rollup from its logs ([Rust Forge](https://forge.rust-lang.org/release/rollups.html)).
*Implication:* the risk classes of integration plan D9 have this precedent. In Rust a person chooses the batch. In our design the pipeline chooses it.

**F5.6 (Primary)** `funador/claude-code-merge-queue` serializes landings on one machine with a FIFO lock. It has no review step. Its stated limit: "a 3–4 minute suite caps you well under 20 landings/hour". Its locks are "crash-safe by PID liveness" ([README](https://github.com/funador/claude-code-merge-queue)).
*Implication:* a serial queue is simple, but our floor takes far longer than 4 minutes. The PID-liveness rule applies to F3.4.

**F5.7 (Measured)** A study of 33,596 agent PRs in 2,807 repositories replayed three-way merges on 747 co-active pairs. The textual conflict rate was 19.8% for pairs of one agent and 41.7% for pairs of two different agents. Only 0.5% of the pairs were cross-agent ([arXiv:2607.04697](https://arxiv.org/abs/2607.04697)). The authors call the textual figure a lower bound, and the build and semantic conflicts are not measured ([summary](https://codex.danielvaughan.com/2026/07/28/agent-pr-merge-conflicts-concurrent-coding-agents-codex-cli-worktree-isolation-coordination-defence/)).
*Implication:* a conflict is normal at this scale. This study does not show that a mix of families causes conflicts, because the sample of cross-agent pairs is small and the repositories differ.

**F5.8 (Primary)** Plaid found that a compile-only run with `--output_groups=compilation_outputs` did not catch compile errors in Go tests ([Plaid](https://plaid.com/blog/merge-queue)).
*Implication:* any cheap batch check in Go must compile the test files.

**F5.9 (Claim)** Agent Orchestrator returns "failed checks, review comments, and merge conflicts" to "the worker that owns the branch" ([README](https://github.com/OrchestratorInc/agent-orchestrator)).
*Implication:* this matches the ejection brief of I3. Its top-level planner is a main agent, so its control model is not ours (ADR-0154).

### Our own PR queue, measured

The operator said on 2026-11-04: "every PR has to be queued on main which continuing generating merge conflicts". We measured this at `origin/main` `23d09f7c6`. The method is a local composition with `git merge-tree`, with objects in a temporary directory.

**F5.10 (Measured, local)** `main` has no branch protection (`gh api repos/mickeyyaya/evolve-loop/branches/main/protection` returns HTTP 404).
*Implication:* no GitHub rule forces one PR at a time. Our own merge process does this.

**F5.11 (Measured, local)** Of 40 open PRs on `main`, 9 conflict with `main` today. Composed oldest first into one batch, 23 land and 17 eject.

| Composition | Composed of 40 | Ejected |
|---|---:|---:|
| Plain | 23 | 17 |
| With `union` on `go/.cover-strict`, `go/.apicover-enforce`, `CHANGELOG.md`, signal codes and package notes | 26 | 14 |

This counts textual conflicts only. It does not prove that the composed tree passes its tests. One ejected PR is #1270, an old batch of other PRs in this list, so it conflicts with its own members.
*Implication:* more than half of the queue can land in one CI run today.

**F5.12 (Measured, local)** Of 780 PR pairs, 135 share a file and 203 share a Go package. One PR of 40 shares no file with any other PR. The largest file-disjoint set that a greedy pass finds has 16 PRs.
*Implication:* each PR shares a file with about 7 others on average. Each merge can break those peers, and it leaves all others behind `main`. Partition by file alone gives less than a batch that composes in order.

**F5.13 (Measured, local)** The most shared files are registries: `docs/architecture/packages/internal-bridge.md` (9 PRs), `docs/architecture/llm-isolation.md` (8), `docs/architecture/packages/README.md` (7) and `docs/architecture/signal-codes.md` (6). In the direct conflicts, `go/.cover-strict` caused 3 of 9, and no other file caused a conflict in those 3 PRs. The wider `union` arm gained 3 PRs, the same count.
*Implication:* one line list causes a third of today's direct conflicts. `docs/research-index/` already shows the fix for prose registries: one entry file for each change, and a generator.

**F5.14 (Primary, local)** We composed PRs by hand three times: #1050 (28 PRs), #1067 (19 PRs) and #1097 (29 PRs). Batch #1270 (8 PRs) is open and red. The tree `dev/cl-wi-i5` holds `Merger.Land`, which takes deliverable records and does the flush, the CI wait, the fast-forward and the split. It has no entry for a console PR.
*Implication:* the console train is one adapter on top of work that exists, and the manual procedure proves the demand.

## 6. Control, supervision and liveness

**F6.1 (Primary)** Symphony has one authority. "The orchestrator is the only component that changes scheduling state." On each tick it reconciles first, then it validates the configuration, fetches candidates, sorts them and dispatches. If validation fails, dispatch stops for that tick, and reconciliation still runs ([SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §7, §8.1).
*Implication:* this confirms ADR-0154. The order "reconcile first" applies to the stale state of F3.4.

**F6.2 (Primary)** Symphony measures a stall from the last event, kills the worker after a limit (default 5 minutes) and queues a retry. A turn timeout is a silence limit, not a total limit. It gives each run a separate terminal reason, because "retry logic and logs differ". Hooks have required timeouts ([SPEC](https://github.com/openai/symphony/blob/main/SPEC.md) §7.2, §8.5, §10.6, §15.4).
*Implication:* our 300-second stall limit matches this. The separate terminal reason is the part that F3.5 lacks.

**F6.3 (Primary)** Symphony has a global cap and a cap for each state: `max_concurrent_agents_by_state` (§8.3).
*Implication:* this supports the separate build and review pools of our `feat/agent-resource-pools` tree.

**F6.4 (Primary)** Gas Town has a Witness for each rig and a Deacon across rigs, with a daemon heartbeat each 3 minutes. It classifies agents as Working, Idle, Stalled, Zombie (a dead tmux session) and GUPP Violation ("hooked work with no progress for an extended period") ([README](https://github.com/gastownhall/gastown)). `herdr` marks each pane "working, blocked, or idle" and says so when an agent "stops and needs an answer" ([README](https://github.com/herdrdev/herdr)).
*Implication:* the state "blocked and waits for an answer" is what the self-heal pane showed in F3.5. Our signal says `busy-stagnant` or a stall, and it does not name this state.

**F6.5 (Primary and Claim)** Erlang/OTP stops a restart after a limit in a time window and sends the failure up the tree. One 2026 post applies this to agents: exhaustion "is itself a signal" to a coarser strategy, another model or a person ([post](https://tianpan.co/blog/2026-07-02-your-agent-needs-a-supervisor-not-a-retry-loop), [terraphim_agent_supervisor](https://docs.rs/crate/terraphim_agent_supervisor/latest)).
*Implication:* a restart helps a transient fault. It does not help a deterministic fault. F3.5 retried a deterministic fault on 3 families.

**F6.6 (Primary)** Loop Engineering rolls out "L1 report → L2 assisted → L3 unattended only after the verifier has been right for a week". Its readiness check "weights recent runs harder than files on disk". Its checklist says that an implementer "cannot mark its own work done" ([README](https://github.com/cobusgreyling/loop-engineering), [checklist](https://github.com/cobusgreyling/loop-engineering/blob/main/docs/loop-design-checklist.md)).
*Implication:* our shadow and enforce stages have the same shape. We do not yet state the evidence that a move to enforce requires.

## 7. Durable execution and state

**F7.1 (Primary)** A Temporal workflow "resumes automatically" after a crash or a long wait ([Temporal](https://docs.temporal.io/ai.md)). Temporal says that a LangGraph checkpoint is not the same: "if that process dies then the run dies with it" ([Temporal](https://temporal.io/blog/temporal-langgraph-plugin-durable-execution)). The second document is from a vendor.
*Implication:* the concept is sound. Our checkpoints and lane resume give this result for a cycle ([checkpoint-resume](../architecture/checkpoint-resume.md), PR #1280).

**F7.2 (Claim)** One October 2026 comparison says that both models re-run the in-flight call, so a side effect needs idempotency ([comparison](https://www.developersdigest.tech/blog/durable-ai-agent-frameworks-compared-2026)).
*Implication:* a durable engine does not remove the idempotency work. We do that work now, for example in `GrantOperatorResume`.

**F7.3 (Claim)** Temporal needs a server. DBOS needs a database and has SDKs in Python and TypeScript. A vendor-adjacent analysis says that Inngest has a limit of 1,000 steps for each run ([analysis](https://www.beri.net/article/temporal-vs-inngest-vs-restate-vs-dbos-durable-execution-agent-cost)). We did not check these limits in the primary documents.
*Implication (synthesis):* each option adds a service to a one-binary Go control plane, and it replaces code that works.

**F7.4 (Primary)** The OpenHands SDK keeps an append-only event log, and one state class is the only mutable part. The abstract says that "V1 substantially reduces system-attributable failures over V0 with negligible event-sourcing overhead". It gives no numbers ([MLSys 2026](https://proceedings.mlsys.org/paper_files/paper/2026/hash/8ae9cf363ea625161f885b798c1f1f78-Abstract-Conference.html)).
*Implication:* state that derives from a log and from live facts does not go stale as a file does. This supports a derived answer to "is a cycle active?" in F3.4.

**F7.5 (Primary)** Symphony does not restore live state after a restart. It removes terminal workspaces, polls the tracker and dispatches again (§8.6). Anthropic's long-run harness reads a progress file and the git log at the start of each session ([Anthropic](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)). Beads and Gas Town keep work state in git ([Gas Town](https://github.com/gastownhall/gastown)).
*Implication:* the common rule is that durable truth stays outside the agent. Our inbox, run directories and git already follow this rule.

## 8. Tests, proof and agents that game tests

This section has the most weight for this plan, because of the operator rule of 2026-11-04: "for ANY issue we fixed, we must always deploy the relative tests to cover the CUJ".

**F8.1 (Primary)** SWE-bench resolves a task only when two sets pass. `FAIL_TO_PASS` tests fail before the fix. `PASS_TO_PASS` tests hold existing behavior ([dataset card](https://huggingface.co/datasets/princeton-nlp/SWE-bench/blob/main/README.md)).
*Implication:* the field has a mechanical definition of a fix. A gate can apply the same two checks.

**F8.2 (Primary)** Node.js says "Bug fixes and features should always come with tests" ([contributing guide](https://raw.githubusercontent.com/nodejs/node/main/doc/contributing/pull-requests.md)). This is a policy for reviewers. We found no project with a merge gate that reruns a named test on the tree before the fix.
*Implication (synthesis):* a mechanical gate of this kind appears to be new work, so we design it ourselves. Our search can miss a closed-source tool.

**F8.3 (Measured)** RepoRescue says that "Claude Code systems sometimes edit failing tests even when prompted not to". The authors rerun patches with test edits removed, and they add a runtime regime that blocks test edits ([arXiv:2607.01213](https://arxiv.org/abs/2607.01213)). We read the abstract only.
*Implication:* a test that the same agent can edit is weak proof. The gate must hold the pre-fix copy of a journey test, and a registered journey must be hard to remove.

**F8.4 (Measured)** A study at MSR 2026 reports that 36% of agent test commits add mocks, against 26% of test commits by others. Its authors say that a mock-heavy test can be "less effective at validating real interactions" ([arXiv:2602.00409](https://arxiv.org/abs/2602.00409)).
*Implication:* this is the same fault as F3.6 and F3.7, seen at a large scale.

**F8.5 (Primary)** Anthropic found that Claude "marked features complete without verifying them end-to-end". Results improved when the agent did "all testing as a human user would". The harness keeps a JSON feature list with a `passes` field, because a model is less likely to overwrite JSON than Markdown. Each session runs an end-to-end check first ([Anthropic](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)).
*Implication:* a machine-readable list of journeys, each with one state, is a proven form. Our item has an acceptance list, and we have no journey list.

**F8.6 (Primary)** Anthropic separates the generator from the evaluator: "agents reliably skew positive when grading their own work". The evaluator stays an LLM, so it needs calibration ([Anthropic](https://anthropic.com/engineering/harness-design-long-running-apps)).
*Implication:* this confirms our separate audit and review seats. It also supports a mechanical check under them.

**F8.7 (Primary)** The Go project tests its `go` command with text scripts. The package `rogpeppe/go-internal/testscript` gives other projects the same tool ([repository](https://github.com/rogpeppe/go-internal), [guide](https://bitfieldconsulting.com/posts/test-scripts)). We have `go/cmd/evolve/e2e_pipeline_paths_test.go` and other `e2e_*_test.go` files that drive whole paths.
*Implication:* a journey test has a home in this tree already. `testscript` is an option for later, and the third-party rule allows a test-only library.

**F8.8 (Primary)** Gremlins says that it does not "work very well on very big Go modules, mainly because a run can take hours". `unclebob/mutator` has a `--changed` mode and support for Go ([Gremlins](https://github.com/go-gremlins/gremlins), [mutator](https://github.com/unclebob/mutator)).
*Implication:* a whole-module mutation gate does not fit this tree. A mutation check on the changed lines can fit, and our review seats already name mutants by hand.

## 9. Flaky tests

**F9.1 (Primary)** The ICSME 2020 paper on Google's TAP describes a flakiness scorer that weights recent runs. It also describes a retry approach where a test passes if any run passes ([paper](https://conferences.computer.org/icsme/pdfs/ICSME2020-1oOutvkGTwF4GyVvNtr3Mm/561900a736/561900a736.pdf)).
*Implication:* a retake is established practice. It must leave a record, or real faults hide.

**F9.2 (Primary)** Trunk's quarantine keeps a test running and reporting, and its failure does not block. Only a test already marked flaky is eligible for automatic quarantine ([Trunk](https://docs.trunk.io/flaky-tests/quarantining/index.md)).
*Implication:* quarantine needs evidence first. Our `evolve ci classify` already reads recurrence.

**F9.3 (Primary)** OpenAI's harness team says that "Test flakes are often addressed with follow-up runs rather than blocking progress indefinitely". It adds: "This would be irresponsible in a low-throughput environment" ([OpenAI](https://openai.com/index/harness-engineering/); read through a [mirror](https://github.com/celesteanders/harness/blob/main/docs/research/260211_openai_harness_engineering_codex.md), because the direct fetch returned HTTP 403).
*Implication:* F3.2 ended 2.5 hours of work on one red. A retake of the failed package before the end of a build costs minutes.

## 10. Context and token cost

**F10.1 (Measured)** Code-Compression-Bench ran 100 SWE-bench Verified tasks with one agent and one model, and it changed only the compression layer ([repository](https://github.com/daseinlabs/code-compression-bench)).

| Arm | Solved of 100 | Cost for each solved task | Input tokens |
|---|---:|---:|---:|
| Parsec | 62 | $1.45 (−44%) | −54% |
| Caveman | 58 | $2.05 (−21%) | −19% |
| Baseline | 57 | $2.58 | 312.2M |
| RTK | 54 | $3.07 (+19%) | +16% |
| Headroom | 58 | $3.66 (+42%) | +6% |

This is one benchmark, and we did not check who funds it. RTK has 85,494 stars, and the same table shows it as more expensive than the baseline.
*Implication:* a token-saving tool can raise the bill. A layer that rewrites the cached prefix "re-pays the expensive cache-write rate". Any test of ours must measure cost for each shipped cycle.

**F10.2 (Primary)** `context-mode` tells the agent to compute in code and keeps raw output out of the window. It says "The LLM should program the analysis, not compute it" ([README](https://github.com/mksglu/context-mode)).
*Implication:* this is our rule 5 (deterministic work goes in code). No new tool is needed for this rule.

**F10.3 (Measured, local)** The retrospective phase of cycle 2151 reached 79.4% context fill, and 153,733 tokens of that were cache reads (`BRIDGE_CONTEXT_FILL_HIGH`, seq 430, 2026-11-04).
*Implication:* context fill is a real cost here. The fix belongs to phase input size, and F10.1 warns against a generic proxy.

**F10.4 (Claim)** `jcode` reports about 169 MB for each extra Claude Code session in a headless bench, against about 3.3 MB for its own ([README](https://github.com/1jehuang/jcode)).
*Implication:* the author benefits from this number. It does show that memory for each seat is measurable, so the host sentinel can record it.

## 11. Code intelligence

**F11.1 (Claim)** The Trail-Code README reports 46% fewer tool calls, 60% less time and 33 against 27 solved of 50 SWE-bench Verified issues ([README](https://github.com/trailhq/Trail-Code)). It is a vendor number. We use this index through `evolve code`.
*Implication:* the project has its own evidence for this direction. We have no local measure for our tree.

**F11.2 (Primary)** `codegraph`, `graphify`, `code-review-graph` and `codebase-memory-mcp` build the same kind of local graph. Each one has more than 30,000 stars. Several reach agents through MCP, and our rule forbids MCP ([cli-only-no-mcp](../conventions/cli-only-no-mcp.md)).
*Implication:* no project gives a clear reason to replace our index. `code-review-graph` scopes review context from a graph, and that concept can lower the cost of a review seat.

## 12. Sandboxes

**F12.1 (Primary and Claim)** Anthropic's `sandbox-runtime` wraps a whole process in Seatbelt or bubblewrap ([docs](https://docs.claude.com/en/docs/claude-code/sandboxing)). `sandcastle` uses the same OS sandboxes, with image and cloud providers as options ([README](https://github.com/mattpocock/sandcastle)). Kubernetes SIG Apps runs Agent Sandbox with gVisor or Kata ([announcement](https://groups.google.com/a/kubernetes.io/g/sig-apps/c/XH3OAF60WVM)).
*Implication:* our Seatbelt profile for each cell is in line with single-host practice. A cluster sandbox has no use on one Mac host.

**F12.2 (Claim)** One source reports that Claude Code runs commands unsandboxed when bubblewrap or socat is absent, unless `failIfUnavailable` is true ([comparison](https://www.agent37.com/blog/claude-code-sandbox)). We did not check this in the primary docs.
*Implication:* a sandbox that fails open is a risk. The X-series isolation work in ADR-0154 already measures this class.

## 13. Harness simplification

**F13.1 (Measured)** A solo agent ran for 20 minutes at $9, and its core feature did not work. The full harness ran for 6 hours at $200. Each component "assumes something the model can't do alone", and it goes stale. The author removed one component at a time to find the load-bearing ones ([Anthropic](https://anthropic.com/engineering/harness-design-long-running-apps)).
*Implication:* a harness needs a removal test, and it must respect the M2 baseline of [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

**F13.2 (Primary)** OpenAI gives agents "a map, not a 1,000-page instruction manual". A short `AGENTS.md` of about 100 lines points into `docs/`, and custom lints carry "remediation instructions" in their error messages. "When documentation falls short, we promote the rule into code." The team merged about 1,500 PRs in 5 months ([OpenAI](https://openai.com/index/harness-engineering/), through the mirror).
*Implication:* this confirms our `docs/` system of record and our guard tests. The rule "promote it into code" is the reason for a mechanical proof gate.

**F13.3 (Primary)** `better-harness` reads project and session evidence and proposes "prioritized improvements", and "missing evidence stays explicit" ([README](https://github.com/QoderAI/better-harness)).
*Implication:* our retrospectives and self-heal diagnoses do the same work with our own data. Explicit missing evidence is a good rule, and our `unmeasured` state follows it.

## 14. Comparison with our design

Synthesis. "Have" means that the feature exists on `main` or in a built tree. "Part" means that it is partly built.

| Concern | Best outside practice | Ours | Gap |
|---|---|---|---|
| One writer of `main` | Gas Town Refinery; every merge queue | integration plan D1; I0, I1, I4 merged | Part: I2 to I11 are not merged |
| Batch, bisect, eject | Refinery; Zuul; GitHub | integration plan D6, D10 | Part |
| Adaptive window | Zuul | integration plan D8 (I4) | Have |
| Independent changes land apart | Uber SubmitQueue | Partition is a later stage | Later, by F5.3 |
| Cheap check before CI | Uber; Plaid | L1 prefix and L2 batch | Part |
| Landed identity | Uber input hashes | `patch-id` with 3 context lines | **Fault (F3.9)** |
| One authority for scheduling | Symphony | ADR-0154 | Have |
| Stall limit | Symphony §8.5 | 300-second limit | Have |
| Terminal reason decides retry | Symphony §7.2; OTP | Retry ladder of ADR-0142 | **Gap (F3.5)** |
| Named agent health state | Gas Town; herdr | Pane state and signals | **Gap (F6.4)** |
| Liveness derived from live facts | PID-liveness queue; event logs | `cycle_id` in a file | **Fault (F3.4)** |
| Caps for each kind of work | Symphony §8.3 | `feat/agent-resource-pools` | Part |
| Maker and checker apart | Anthropic; Loop Engineering | Audit and review seats | Have |
| Fix proven red | SWE-bench | A rule in briefs only | **Gap (F8.1)** |
| Journey list with state | Anthropic feature list | None | **Gap (F8.5)** |
| Hold test edits apart | RepoRescue | Protected surface only | **Gap (F8.3)** |
| Retake of a red | TAP; OpenAI; our audit tier | Audit only | **Gap (F3.3)** |
| Move to enforce on evidence | Loop Engineering L1 to L3 | Operator decision, no stated bar | **Gap (F6.6)** |
| Removal test of the harness | Anthropic | None | **Gap (F13.1)** |
| Console PRs in the batch | Each merge queue takes each PR | Lane deliverables only | **Gap (F5.10, F5.14)** |
| Durable resume | Temporal | Checkpoints, lane resume | Have |
| OS sandbox for each cell | sandbox-runtime | Seatbelt profile | Have |

## 15. Implications for the plan

Synthesis. The plan makes the decisions. These are the inputs. The table says what the evidence supports for each practice.

| Practice | Evidence | Evidence supports |
|---|---|---|
| Batch, bisect, one writer of `main` | F5.1, F5.4 | Keep |
| Console PRs in the batch | F5.10, F5.11, F5.14 | Adopt |
| Mechanical proof of a fix | F8.1, F8.3 | Adopt |
| Derived liveness | F3.4, F5.6, F7.4 | Adopt |
| Terminal refusal | F3.5, F6.2, F6.5 | Adopt |
| Floor retake and kept test names | F3.3, F9.1, F9.3 | Adopt |
| One-line-context landed identity | F3.9 | Adopt |
| Stated bar for `enforce` | F6.6 | Adopt |
| Named seat health states | F6.4 | Adopt |
| Partition of independent changes | F5.3 | Later |
| Removal test of the harness | F13.1 | Later |
| Outside merge queue or workflow engine | F5.2, F7.3 | Reject |
| Generic tool-output proxy | F10.1 | Reject |
| Whole-module mutation gate | F8.8 | Reject |

1. **The integration design stands.** F5.1 to F5.5 confirm it, and F5.2, F5.6 and F7.3 rule out a replacement.
2. **The largest gap is proof.** F3.6, F3.7, F8.1, F8.3, F8.4 and F8.5 point to one mechanical gate and one journey list.
3. **Two faults are small and direct.** F3.4 has a fix in the guard. F3.9 has a tested fix in one argument.
4. **The retry path needs a class.** F3.5, F6.2 and F6.5 separate a deterministic refusal from a transient fault.
5. **The floor needs the audit rules.** F3.2, F3.3, F9.1 and F9.3 support a retake and a kept test name.
6. **Enforce needs a stated bar.** F6.6 gives the shape.
7. **Do not take a generic token proxy.** F10.1 shows that one can raise cost.
8. **Console PRs need the train.** F5.10, F5.11 and F5.14 show that this is the direct fix for the queue on `main`.

## 16. Gaps in this research

- We read RepoRescue (arXiv:2607.01213) and the MSR study through abstracts and summaries. We did not read the full tables.
- The direct fetch of the OpenAI post returned HTTP 403. We read a mirror of the text.
- We read Symphony's spec through an extract, and we did not run it.
- We ran none of the outside projects, except one git experiment for F3.9.
- We found no direct comparison of retry against quarantine.
- We found no independent study of a fail-before gate in a merge queue.
- Star counts change daily, and some counts look unusual.
- The search covers public repositories only. A private tool can do what F8.2 says we did not find.

## 17. Sources

**Papers and studies**
- [AI Agent Pull Requests on GitHub (arXiv:2607.04697)](https://arxiv.org/abs/2607.04697)
- [RepoRescue (arXiv:2607.01213)](https://arxiv.org/abs/2607.01213)
- [Are Coding Agents Generating Over-Mocked Tests? (arXiv:2602.00409)](https://arxiv.org/abs/2602.00409)
- [The OpenHands Software Agent SDK (MLSys 2026)](https://proceedings.mlsys.org/paper_files/paper/2026/hash/8ae9cf363ea625161f885b798c1f1f78-Abstract-Conference.html)
- [De-Flake Your Tests (ICSME 2020)](https://conferences.computer.org/icsme/pdfs/ICSME2020-1oOutvkGTwF4GyVvNtr3Mm/561900a736/561900a736.pdf)
- [SWE-bench dataset card](https://huggingface.co/datasets/princeton-nlp/SWE-bench/blob/main/README.md)
- [Code-Compression-Bench](https://github.com/daseinlabs/code-compression-bench)

**Engineering posts and specs**
- [OpenAI: Harness engineering](https://openai.com/index/harness-engineering/)
- [OpenAI: Symphony SPEC](https://github.com/openai/symphony/blob/main/SPEC.md)
- [Anthropic: Effective harnesses for long-running agents](https://anthropic.com/engineering/effective-harnesses-for-long-running-agents)
- [Anthropic: Harness design for long-running apps](https://anthropic.com/engineering/harness-design-long-running-apps)
- [Uber: Bypassing large diffs in SubmitQueue](https://www.uber.com/en-US/blog/bypassing-large-diffs-in-submitqueue/)
- [Plaid: Merge queue](https://plaid.com/blog/merge-queue)
- [Temporal: LangGraph plugin](https://temporal.io/blog/temporal-langgraph-plugin-durable-execution)
- [Your agent needs a supervisor, not a retry loop](https://tianpan.co/blog/2026-07-02-your-agent-needs-a-supervisor-not-a-retry-loop)

**Project documents**
- [GitHub merge queue](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue)
- [Zuul gating](https://zuul-ci.org/docs/zuul/latest/gating.html)
- [Rust rollups](https://forge.rust-lang.org/release/rollups.html)
- [Gas Town architecture](https://github.com/gastownhall/gastown/blob/main/docs/design/architecture.md)
- [Loop Engineering checklist](https://github.com/cobusgreyling/loop-engineering/blob/main/docs/loop-design-checklist.md)
- [Trunk flaky test quarantine](https://docs.trunk.io/flaky-tests/quarantining/index.md)
- [Claude Code sandboxing](https://docs.claude.com/en/docs/claude-code/sandboxing)
- [Node.js pull request guide](https://raw.githubusercontent.com/nodejs/node/main/doc/contributing/pull-requests.md)
- [Testing Go CLIs with test scripts](https://bitfieldconsulting.com/posts/test-scripts)

**Vendor comparisons, used with care**
- [Durable AI agent frameworks compared](https://www.developersdigest.tech/blog/durable-ai-agent-frameworks-compared-2026)
- [Temporal against Inngest, Restate and DBOS](https://www.beri.net/article/temporal-vs-inngest-vs-restate-vs-dbos-durable-execution-agent-cost)
