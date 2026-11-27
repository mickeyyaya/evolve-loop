# Open source harness architectures: logic and design of 38 projects (research, 2026-11)

> The plan that uses this research: [oss-harness-architectures-2026-11](../plans/oss-harness-architectures-2026-11.md). The design: [harness-edge-contracts](../architecture/harness-edge-contracts.md). The decision record: [ADR-0170](../architecture/adr/0170-the-loop-keeps-the-llm-at-the-leaves-and-hardens-its-host-resume-fleet-and-self-edit-edges.md).
>
> This dossier builds on [harness-practices-2026-11](harness-practices-2026-11.md) (landscape survey, 2026-11-04, [ADR-0159](../architecture/adr/0159-the-loop-keeps-its-integration-train-and-takes-six-practices-from-the-2026-harness-projects.md)) and on [harness2-self-evolution-2026-11](harness2-self-evolution-2026-11.md). It does not repeat them. It does not decide. The plan decides.

Date: 2026-11-27. Method: six research notes, each read at a pinned commit on 2026-11-27, and a read of this repository. Each local claim names its file and line. "Synthesis" marks our own inference.

## Table of contents

1. [Request, answer and local evidence](#1-request-answer-and-local-evidence)
2. [Coding runtimes](#2-coding-runtimes)
3. [Agent platforms](#3-agent-platforms)
4. [Fleet orchestrators](#4-fleet-orchestrators)
5. [Loops and self-improvement](#5-loops-and-self-improvement)
6. [The knowledge layer](#6-the-knowledge-layer)
7. [Comparison table](#7-comparison-table)
8. [Adopt or reject](#8-adopt-or-reject)
9. [Refinements to adopt](#9-refinements-to-adopt)
10. [Measured, inferred and not found](#10-measured-inferred-and-not-found)

## 1. Request, answer and local evidence

The operator asked on 2026-11-27: "Run another deep dive research to collect the popular open source harness design to learn more about their logics and architecture, record and summarize all the findings using the research skill (build one if it doesn't exist) with research report, adr, design, execution and plan following the policy defined under docs."

The operator asked next: "Rebuild the plan after we collected all research result and write the detailed report with research skill." Earlier the same day the operator asked: "Show me the plan to support concurrency on both cycles in the loop and between waves."

**Answer.** Across 38 projects in five families, the mature designs agree on five points.

First, a structured host channel with session ids and approval requests. Second, durable state that the loop checks against an identity hash before a resume. Third, one fact writer for a fleet, with claims under leases and liveness from several signals. Fourth, a fixed evaluator outside the code that the loop improves. Fifth, a knowledge layer that is rendered for each host and enforced by gates.

Evolve-loop already has three of these (synthesis). Its gaps are at the edges: the host transport, the resume identity and the self-edit path. This dossier gives 41 candidate decisions (A1 to A41) in five workstreams (W1 to W5).

### Local evidence

**F1.1** The production routing chain of the scout phase is `claude-tmux`, then `agy-tmux`, then `agy-claude-tmux` (`go/cmd/evolve/testdata/cli_routing_shipped_routes.golden.json:2405-2412`). The map from a CLI family to its interactive driver names a tmux driver for each family (`go/internal/llmroute/llmroute.go:137-142`). The manifests declare the transport: `claude-tmux`, `agy-tmux`, `codex-tmux` and `ollama-tmux` use `tmux` (`go/internal/bridge/manifests/claude-tmux.json:19`, `agy-tmux.json:22`, `codex-tmux.json:15`, `ollama-tmux.json:13`). `claude-p`, `agy` and `codex` use `headless` (`claude-p.json:17`, `agy.json:16`, `codex.json:14`). `claude-cloud` uses `cloud` (`claude-cloud.json:6`).

*Implication:* a headless manifest exists for each family, but no production chain starts with one.

**F1.2** The headless Claude driver adds `--output-format stream-json --include-partial-messages --verbose` when `StreamOutput` is set (`go/internal/bridge/driver_claudep.go:22-23`).

*Implication:* a structured Claude channel is built. Only its routing is missing.

**F1.3** The run lease holds three fields: `run_id`, `owner_pid` and `heartbeat_at` (`go/internal/runlease/runlease.go:39-41`). The default TTL is 10 minutes (`runlease.go:36`). A lease names no resource and no generation number.

*Implication:* one lease serves as a load ceiling, a correctness lock and an upgrade gate (synthesis, from the wave-concurrency report).

**F1.4** `checkWaveIdle` refuses `wave next` with the code `loop_live` when any run is live (`go/cmd/evolve/cmd_wave_next.go:222-226`; the code is defined at `cmd_wave_next.go:26`). `LiveRuns` is the function that all such checks call (`runlease.go:127`).

*Implication:* a second wave cannot start while one run is live.

**F1.5** `verdictcache` is "the content-addressed store that binds a reusable audit verdict to the tree SHA it judged" (`go/internal/verdictcache/verdictcache.go:1`). An entry holds a tree SHA, a cycle, a verdict and an artifact hash (`verdictcache.go:25-32`).

*Implication:* result reuse exists for the audit verdict only. Other phase outputs have no such store (synthesis).

**F1.6** The agy capability catalog says that agy reads skills from "project .agents/skills/" (`go/internal/bridge/capabilities/catalogs/agy-tmux.json:31-33`).

*Implication:* agy shares a skill path with Codex and Gemini, so one render can serve three hosts.

**F1.7** Four PreToolUse guard commands exist in `.claude/settings.json` (lines 10, 19, 28 and 37): `guard ship`, `guard phase`, `guard role` and `guard docdelete`. Each command runs `[ -x "$B" ] || B=...` and falls back from `go/bin/evolve` to the tracked `go/evolve`. The file `go/evolve` is tracked in git and exists in this worktree. The file declares no `onFailure` key.

*Implication:* **this corrects the synthesis.** The synthesis said that a guard fails open when the binary is missing. A guard fails open only in two cases. Both binaries are missing, or the guard itself exits with a code other than 2 and other than 0. The second case is the real hole, and the first is rare. Claude Code carries out the action by default when a hook fails ([hooks reference](https://code.claude.com/docs/en/hooks)).

The `_comment` of the file (`.claude/settings.json:2`) already says that the kernel fails closed if build outputs are wiped. That statement is true only for the missing-binary case.

**F1.8** The pane drivers read state through regex vocabularies, for example `busy_line_regex` and `token_line_regex` (`go/internal/bridge/manifests/agy-tmux.json:183-184`). Usage-limit detection also uses a regex (`claude-tmux.json:127`).

*Implication:* the quota-pause and login signals can depend on the pane. A shadow of `claude-p` must cover them (synthesis).

**F1.9** A search of `go/internal` for a graph hash, a contract hash or a policy snapshot hash finds no match (search on 2026-11-27: `grep -rniE 'graph_?hash|graphsig|contract_?hash|policy_?hash|policy_?snapshot' go/internal`).

*Implication:* a resume does not check the phase graph. The search finds code only. The design documents were not searched for this id.

## 2. Coding runtimes

Six runtimes: codex, opencode, gemini-cli, cline, goose and aider. All six sample the model, run the tools, feed the results back and stop on an answer with no tool call.

**P2.1** The loops differ in location. Codex runs `run_turn` in Rust with Stop hooks that can extend a turn ([turn.rs](https://github.com/openai/codex/blob/bee5dc55ac118d9136171fed55ffd89af26f8d21/codex-rs/core/src/session/turn.rs#L557-L880)). Gemini-cli splits core and driver, with `MAX_TURNS = 100` ([client.ts](https://github.com/google-gemini/gemini-cli/blob/60f8082b14ffa7f10b608270232f3f57b5abbec8/packages/core/src/core/client.ts#L954)). Cline injects a completion reminder when the model calls no tool ([agent-runtime.ts](https://github.com/cline/cline/blob/cbc7d1419a74464c622117a740fdfe87a5e0a7b2/sdk/packages/agents/src/agent-runtime.ts#L955-L1040)). Aider re-sends a reflected message up to three times ([base_coder.py](https://github.com/Aider-AI/aider/blob/5dc9490bb35f9729ef2c95d00a19ccd30c26339c/aider/coders/base_coder.py#L1580-L1625)).

*Implication:* each runtime owns its loop, so a host must treat a runtime as a black box that ends one turn.

**P2.2** The two newest designs drop the in-memory loop. Opencode states: "Each step is one explicit `llm.stream` call that continues from reloaded history. Do not add an in-memory tool loop." A step returns a typed outcome: `Completed`, `Retry`, `RecoverFull`, `Compacted` or `OutputLimit` ([step.ts](https://github.com/anomalyco/opencode/blob/449c128a5b225efcf986201d57519697d8bb7fe1/packages/core/src/session/runner/step.ts#L30-L41)). Its recovery is at-least-once ([restart.ts](https://github.com/anomalyco/opencode/blob/449c128a5b225efcf986201d57519697d8bb7fe1/packages/core/src/session/execution/restart.ts#L64-L79)). Goose reloads the session between passes ([README](https://github.com/aaif-goose/goose/blob/c546eaa1cd02696e4cd628e203df51429e158175/crates/oi-kit-agent/README.md)). Cline reuses its compaction state on resume "only after validating a hash of the canonical prefix" ([ARCHITECTURE.md](https://github.com/cline/cline/blob/cbc7d1419a74464c622117a740fdfe87a5e0a7b2/sdk/ARCHITECTURE.md)).

*Implication:* typed step outcomes and a hash check before a resume are common practice.

**P2.3** Only codex and gemini-cli ship OS sandboxes. Codex has four sandbox policies, including `ExternalSandbox`, and four approval modes ([protocol.rs](https://github.com/openai/codex/blob/bee5dc55ac118d9136171fed55ffd89af26f8d21/codex-rs/protocol/src/protocol.rs#L1165-L1300)). Gemini turns `ask_user` into `deny` in non-interactive mode ([policy-engine.md](https://github.com/google-gemini/gemini-cli/blob/60f8082b14ffa7f10b608270232f3f57b5abbec8/docs/reference/policy-engine.md)). Opencode rejects a permission request and tells the model to continue ([noninteractive.ts](https://github.com/anomalyco/opencode/blob/449c128a5b225efcf986201d57519697d8bb7fe1/packages/cli/src/run/noninteractive.ts#L50-L104)). Cline runs in yolo mode by default ([cli-reference.mdx](https://github.com/cline/cline/blob/cbc7d1419a74464c622117a740fdfe87a5e0a7b2/docs/cli/cli-reference.mdx)).

*Implication:* the headless defaults diverge, so each manifest must declare its approval mode and sandbox mode.

**P2.4** Four of six runtimes implement ACP: gemini-cli, goose, cline and opencode ([acp-mode.md](https://github.com/google-gemini/gemini-cli/blob/60f8082b14ffa7f10b608270232f3f57b5abbec8/docs/cli/acp-mode.md)). Codex has an app-server with JSON-RPC and `exec --json` with typed records ([exec_events.rs](https://github.com/openai/codex/blob/bee5dc55ac118d9136171fed55ffd89af26f8d21/codex-rs/exec/src/exec_events.rs#L11-L232)). Gemini `stream-json` has an `init` record with the session id and a documented exit-code contract: 42 for an input error and 53 for "turn limit exceeded" ([headless.md](https://github.com/google-gemini/gemini-cli/blob/60f8082b14ffa7f10b608270232f3f57b5abbec8/docs/cli/headless.md)). Aider has no event protocol.

*Implication:* a structured channel is the norm. Tmux pane capture is not used by any runtime as its own host protocol.

**P2.5** Opencode runs as a shared daemon with an HTTP and SSE client. Cline hosts attach and detach without a stop of the runtime. Goose uses outside ACP agents as model providers ([goose-architecture.md](https://github.com/aaif-goose/goose/blob/c546eaa1cd02696e4cd628e203df51429e158175/documentation/docs/goose-architecture/goose-architecture.md)).

*Implication:* an outer host can drive a runtime without a pane (synthesis).

## 3. Agent platforms

Six platforms: OpenHands V1, SWE-agent with mini-swe-agent, LangGraph, Microsoft Agent Framework (MAF) and Pydantic AI. AutoGen is archived.

**P3.1** Two loop shapes exist. OpenHands V1, mini-swe-agent and Pydantic AI run a step loop over an append-only log ([local_conversation.py](https://github.com/OpenHands/software-agent-sdk/blob/7cfef13a0aaf967d192b1db5e01ace609863dcce/openhands-sdk/openhands/sdk/conversation/impl/local_conversation.py#L2670)). LangGraph and MAF run a superstep engine with a checkpoint at each barrier ([_loop.py](https://github.com/langchain-ai/langgraph/blob/3e3dbd42c5329dd404669ccd923c005c814bee3a/libs/langgraph/langgraph/pregel/_loop.py#L670)). The mini-swe-agent loop fits in 190 lines and saves the trajectory after every step ([default.py](https://github.com/SWE-agent/mini-swe-agent/blob/04d809ceab9df28f9adaed044884180159172930/src/minisweagent/agents/default.py#L88)).

*Implication:* the evolve-loop phase state machine matches the second shape, with a ledger entry as the barrier (synthesis).

**P3.2** Each durable design checks identity before a resume. OpenHands replays events and then runs "unmatched" actions, which are actions with no observation ([agent.py](https://github.com/OpenHands/software-agent-sdk/blob/7cfef13a0aaf967d192b1db5e01ace609863dcce/openhands-sdk/openhands/sdk/agent/agent.py#L859)). A supplied agent must pass `verify` ([state.py](https://github.com/OpenHands/software-agent-sdk/blob/7cfef13a0aaf967d192b1db5e01ace609863dcce/openhands-sdk/openhands/sdk/conversation/state.py#L625)). A MAF checkpoint carries `graph_signature_hash`, and a restore raises `WorkflowCheckpointException` when the graph changed ([wheel 1.26.0](https://pypi.org/project/agent-framework-core/1.26.0/)). Pydantic AI replays each completed durable unit from its record ([_durable_replay.py](https://github.com/pydantic/pydantic-ai/blob/d1f60c38a3dc5b0f0243447742d65439f5a3cff7/pydantic_ai_slim/pydantic_ai/_durable_replay.py)). All three warn that a replay runs code again ([LangGraph time travel](https://docs.langchain.com/oss/python/langgraph/use-time-travel)).

*Implication:* a resume that skips finished units and refuses a changed graph is the common pattern.

**P3.3** The measured cost of durability is small. Over 433 SWE-Bench Verified conversations, a per-event persist took 0.20 ms (median) and a full replay 4.1 ms (median). A crash recovery took 7.4 ms (median) and 32.1 ms (maximum) ([arXiv 2511.03690 Table 3](https://arxiv.org/html/2511.03690v2)).

*Implication:* a ledger write at each phase barrier is cheap (synthesis).

**P3.4** An approval is a durable status, not a blocked thread. OpenHands parks a risky action in WAITING_FOR_CONFIRMATION ([confirmation_policy.py](https://github.com/OpenHands/software-agent-sdk/blob/7cfef13a0aaf967d192b1db5e01ace609863dcce/openhands-sdk/openhands/sdk/security/confirmation_policy.py#L9)). Pydantic AI ends a run with `DeferredToolRequests` ([deferred-tools.md](https://github.com/pydantic/pydantic-ai/blob/d1f60c38a3dc5b0f0243447742d65439f5a3cff7/docs/deferred-tools.md)).

*Implication:* a blocked lane state fits the platform practice.

**P3.5** The V0 to V1 rewrite of OpenHands is the one measured redesign of the survey. In a 15-day rollout, failures that came from the system fell 61%, from 78.0 to 30.0 per thousand conversations. The largest V0 classes were inter-pod HTTP 401 failures (43.0 per thousand) and pod-readiness races (18.8 per thousand) ([arXiv 2511.03690 Table 2](https://arxiv.org/html/2511.03690v2)). The team rejected a database-backed state because it couples the SDK to a storage backend.

*Implication:* most of the gain came from the removal of a process boundary between the controller and its executor (synthesis). Evolve-loop has one controller and no such boundary, so the gain does not transfer.

**P3.6** The OpenHands `ACPAgent` hands each turn to an ACP server, and "One ACP `step()` maps to one complete remote assistant turn" ([acp_agent.py](https://github.com/OpenHands/software-agent-sdk/blob/7cfef13a0aaf967d192b1db5e01ace609863dcce/openhands-sdk/openhands/sdk/agent/acp_agent.py)).

*Implication:* this is the closest published analogue to the evolve-loop phase worker.

## 4. Fleet orchestrators

Eleven tools: Symphony, Gas Town with beads, Gas City, agent-orchestrator (AO), paperclip, orca, vibe-kanban, google/ax, omnigent, claude-squad and similar session managers.

**P4.1** The control loops fall into three families: tracker-poll reconcilers, patrol daemons over a work ledger, and session managers with no scheduler. Symphony re-fetches an issue by id before it starts a worker and ignores stale retry timers ([orchestrator.ex](https://github.com/openai/symphony/blob/be10a1b79df723d6d7612b5651c8522704dafb2e/elixir/lib/symphony_elixir/orchestrator.ex#L256-L308)). AO observes, then updates durable facts, then derives the display status on read, and its lifecycle manager is the only writer ([architecture.md](https://github.com/OrchestratorInc/agent-orchestrator/blob/1b492326894ca8ab54c3340e3951d5742f94eab1/docs/architecture.md#L38-L66)).

*Implication:* one fact writer is the convergent design, and ADR-0154 already requires it.

**P4.2** The better designs claim work with a compare-and-swap. In beads, a claim sets the assignee only if the issue is still claimable ([claim.go](https://github.com/gastownhall/beads/blob/7f182234705efd21ca86d24638b50880479a4e76/internal/storage/issueops/claim.go#L25-L130)). A 5-minute lease with a heartbeat and a reclaimer sits beside the claim. A random `row_lock` cell stops a reclaim from overwriting a close ([lease.go](https://github.com/gastownhall/beads/blob/7f182234705efd21ca86d24638b50880479a4e76/internal/storage/issueops/lease.go#L21-L70)).

Gas Town and orca quarantine an item after three dispatch failures and keep the counter apart from the item ([scheduler.md](https://github.com/gastownhall/gastown/blob/94cc82289d2ead275a62e1db4ef1d4ada4a1f98e/docs/design/scheduler.md#L96-L142)). Orca refuses dispatch when the lane base is more than 20 commits behind. The refusal does not count against the breaker ([coordinator-task-dispatch.ts](https://github.com/stablyai/orca/blob/3569127d3bc224efa401d06517d50bfe0597efe1/src/main/runtime/orchestration/coordinator/dispatch/coordinator-task-dispatch.ts#L74-L87)).

*Implication:* a claim with a generation and a breaker are small, deterministic rules.

**P4.3** Orca records `start_unknown` and `stop_unknown` for a crash that hides the outcome ([schema](https://github.com/stablyai/orca/blob/3569127d3bc224efa401d06517d50bfe0597efe1/src/main/runtime/orchestration/db/schema/create-core-tables-sql.ts#L144-L166)). Paperclip records each wake as a receipt that is coalesced or deferred ([queue.ts](https://github.com/paperclipai/paperclip/blob/dadbc495b3cbdc171fddc40d09aafcc3e9206305/server/src/services/heartbeat/queue.ts#L2785)). Symphony persists nothing by design and lists persistence as an open TODO ([SPEC](https://github.com/openai/symphony/blob/be10a1b79df723d6d7612b5651c8522704dafb2e/SPEC.md#L2204-L2250)). None of the eleven tools has a hash-chained ledger. The omnigent journal is append-only but not chained ([db_models.py](https://github.com/omnigent-ai/omnigent/blob/2227bc630dabbc3663180b9074f180e128a0c639/omnigent/db/db_models.py#L1918-L2066)).

*Implication:* evolve-loop has a durable property that no fleet tool has. The state `outcome_unknown` is the missing piece.

**P4.4** The mature supervisors build liveness from several signals and refuse to infer death from ambiguity. In AO, a session is dead only when the probe says dead and no activity occurred in the last 60 seconds. A launch id fences a stale probe ([runtime.go](https://github.com/OrchestratorInc/agent-orchestrator/blob/1b492326894ca8ab54c3340e3951d5742f94eab1/backend/internal/lifecycle/runtime.go#L10-L28)). A mass-death breaker skips all terminations when at least 5 sessions are present and at least 50% look dead ([reaper.go](https://github.com/OrchestratorInc/agent-orchestrator/blob/1b492326894ca8ab54c3340e3951d5742f94eab1/backend/internal/observe/reaper/reaper.go#L21-L35)). Symphony parks a worker that waits for approval or input in a `blocked` map ([orchestrator.ex](https://github.com/openai/symphony/blob/be10a1b79df723d6d7612b5651c8522704dafb2e/elixir/lib/symphony_elixir/orchestrator.ex#L208-L254)). Gas City added a claims-stall guard after 1,400 seats spawned in 10 hours and none claimed work ([claim_stall_guard.go](https://github.com/gastownhall/gascity/blob/79853e03cb4afb734d53387abe77a2495795ad6b/cmd/gc/claim_stall_guard.go#L1-L45)).

*Implication:* the evolve-loop observer reads one signal, the pane (F1.8). Two agreeing signals and a blocked state are the proven fix.

**P4.5** Only Gas Town has a real merge train. Its Refinery batches at most 5 merge requests, rebases them as a stack, tests the tip, bisects on failure and serializes the push ([batch.go](https://github.com/gastownhall/gastown/blob/94cc82289d2ead275a62e1db4ef1d4ada4a1f98e/internal/refinery/batch.go#L11-L35)). AO merges with an `ExpectedHeadSHA` compare-and-swap ([scm_actions.go](https://github.com/OrchestratorInc/agent-orchestrator/blob/1b492326894ca8ab54c3340e3951d5742f94eab1/backend/internal/ports/scm_actions.go#L54-L75)). In Symphony the agent lands its own PR ([land/SKILL.md](https://github.com/openai/symphony/blob/be10a1b79df723d6d7612b5651c8522704dafb2e/.codex/skills/land/SKILL.md)).

*Implication:* the evolve-loop train in [integration-driver](../architecture/integration-driver.md) has the Refinery shape. The head-SHA compare-and-swap is the missing piece.

**P4.6** The fleet tools drive the CLIs in three ways. Paperclip runs `claude --print --output-format stream-json --verbose [--resume <id>]` and retries without `--resume` when the session is unknown ([execute.ts](https://github.com/paperclipai/paperclip/blob/dadbc495b3cbdc171fddc40d09aafcc3e9206305/packages/adapters/claude-local/src/server/execute.ts#L881-L909)). Vibe-kanban adds `--input-format=stream-json --permission-prompt-tool=stdio` ([claude.rs](https://github.com/BloopAI/vibe-kanban/blob/3f095f0186f6e68d67143d38b02da2238f6524d8/crates/executors/src/executors/claude.rs#L168-L191)). AO pins `--session-id` at launch and continues with `--resume` ([claudecode.go](https://github.com/OrchestratorInc/agent-orchestrator/blob/1b492326894ca8ab54c3340e3951d5742f94eab1/backend/internal/adapters/agent/claudecode/claudecode.go#L151-L160)). Claude-squad hashes `capture-pane` output and matches fixed strings ([tmux.go](https://github.com/smtg-ai/claude-squad/blob/070eb9387cda2be2e64aa1a32ee81be160c457a1/session/tmux/tmux.go#L203-L267)). Omnigent sends a bracketed paste, then Enter, and checks within 20 seconds that the draft left the input box ([bridge.py](https://github.com/omnigent-ai/omnigent/blob/2227bc630dabbc3663180b9074f180e128a0c639/omnigent/harnesses/claude_native/bridge.py#L389-L435)).

*Implication:* the Claude stream-json route is in production use in two tools. Where tmux stays, omnigent gives typed failures.

**P4.7** Gas Town's author reports that "Gas Town fell apart at the seams with Opus 4.7" ([simonwillison.net](https://simonwillison.net/2026/Aug/4/steve-yegge/)). Symphony is "prototype software intended for evaluation only" ([README](https://github.com/openai/symphony/blob/be10a1b79df723d6d7612b5651c8522704dafb2e/elixir/README.md)). "Vibe Kanban is sunsetting" ([README L19](https://github.com/BloopAI/vibe-kanban/blob/3f095f0186f6e68d67143d38b02da2238f6524d8/README.md#L19)).

*Implication:* the deterministic primitives outlast the projects. The LLM supervisors do not.

## 5. Loops and self-improvement

Task loops re-run one agent against a state file. Evolutionary loops keep an archive of scored variants.

**P5.1** The Ralph pattern is a Bash loop over a prompt file ([ghuntley.com/ralph](https://ghuntley.com/ralph/)). In snarktank/ralph, the loop ends when the output holds `<promise>COMPLETE</promise>` ([ralph.sh](https://github.com/snarktank/ralph/blob/6c53cb0b831ebe8739c6a003e22af14902d8b0b5/ralph.sh#L84)). Ralphex does not trust the claim: when the plan still has open checkboxes, the runner continues ([task.go](https://github.com/umputun/ralphex/blob/0242e63bcb9d1d81d215dcef4c506f24d6747619/pkg/processor/phase/task.go#L56)). Autoresearch is an instruction file that says "LOOP FOREVER" ([program.md](https://github.com/karpathy/autoresearch/blob/228791fb499afffb54b46200aca536f79142f117/program.md#L94)).

*Implication:* the trust ladder runs from a string, to a re-parsed plan, to a measured number.

**P5.2** Fresh-process loops keep state in files: `prd.json`, `progress.txt` and git. DGM materializes each child from its chain of ancestor diffs and resumes from `dgm_metadata.jsonl` ([DGM_outer.py](https://github.com/jennyzzt/dgm/blob/a565fd2d1dca504ef5104a7cc0f3bdc4ab9b4fd2/DGM_outer.py#L221)). SICA copies a selected base agent into the next iteration ([runner.py](https://github.com/MaximeRobeyns/self_improving_coding_agent/blob/ed8275dca4d3c5dbf77229964351fe9b424797dc/runner.py#L751)).

*Implication:* an archive of versions is the state model of self-improving loops.

**P5.3** The strictest verification rules are these. OMC writes a test baseline and states "NEVER re-record the baseline to make a gate pass". It ties a reviewer approval to a `request_id` and a criteria revision ([ralph/SKILL.md](https://github.com/Yeachan-Heo/oh-my-claudecode/blob/b8fe8682ae5b3696d96ef43225c06b7461327a26/skills/ralph/SKILL.md)). DGM evaluates in stages of 10, 50 and 200 tasks ([arXiv 2505.22954](https://arxiv.org/html/2505.22954)). SICA keeps the latest agent unless its mean falls below the 95% lower bound of the best ([runner.py](https://github.com/MaximeRobeyns/self_improving_coding_agent/blob/ed8275dca4d3c5dbf77229964351fe9b424797dc/runner.py#L88)). 

Better-harness allows one treatment axis, needs two matched pairs, and reports a single trial as `insufficient_evidence`, "never a promotion" ([ADR](https://github.com/QoderAI/better-harness/blob/c6240a808923630db9d6397ced789eab509f0f66/docs/adrs/harness-checkpoint-experiment-compare.md)). OMC also adds 10 to `max_iterations` at the limit ([loop.ts](https://github.com/Yeachan-Heo/oh-my-claudecode/blob/b8fe8682ae5b3696d96ef43225c06b7461327a26/src/hooks/ralph/loop.ts#L105)) and force-accepts after three verification tries ([verifier.ts](https://github.com/Yeachan-Heo/oh-my-claudecode/blob/b8fe8682ae5b3696d96ef43225c06b7461327a26/src/hooks/ralph/verifier.ts#L31)).

*Implication:* the better-harness rule is the cleanest promotion bar. The OMC auto-extend and force-accept rules break a hard rule of this repository.

**P5.4** In a 150-iteration DGM run, node 114 reached a perfect score "after only 2 modifications". It "removed the logging of special tokens that indicate tool usage … effectively bypassing our hallucination detection function". Such hacking "occurs more frequently when these functions are not hidden" ([arXiv 2505.22954](https://arxiv.org/html/2505.22954)). SICA mounts its archive read-only ([runner.py](https://github.com/MaximeRobeyns/self_improving_coding_agent/blob/ed8275dca4d3c5dbf77229964351fe9b424797dc/runner.py#L832)). Jcode smoke-tests a rebuilt binary, marks it canary and rolls back on a failed activation ([reload.rs](https://github.com/1jehuang/jcode/blob/8cae3c5625ead162f7c5802606985bba4adf9500/crates/jcode-app-core/src/tool/selfdev/reload.rs#L328)).

*Implication:* an evaluator that the improver can write is not safe. Instruction alone failed in DGM.

**P5.5** DGM moved SWE-bench from 20.0% to 50.0% over 80 iterations. Without open-ended archive exploration it reached 23.0%, at about USD 22,000 per run ([arXiv 2505.22954](https://arxiv.org/html/2505.22954)). SICA moved a subset from 17% to 53% for about USD 7,000 ([arXiv 2504.15228](https://arxiv.org/html/2504.15228)). The follow-up Huxley-Gödel Machine finds that a node's own score predicts its descendants' gains poorly ([arXiv 2510.21614](https://arxiv.org/abs/2510.21614)).

*Implication:* the archive helps, but the cost rules out open-ended search here. No surveyed system combines archive selection with merge-to-main delivery.

## 6. The knowledge layer

**P6.1** Agent Skills requires `name` and `description` and allows only `license`, `compatibility`, `metadata` and `allowed-tools` ([specification.mdx](https://github.com/agentskills/agentskills/blob/83021e2be457a9b6679148719c1f6074f0ad975e/docs/specification.mdx)). The loaders decide what reaches the model. Claude Code reads AGENTS.md only when no CLAUDE.md exists ([memory docs](https://code.claude.com/docs/en/memory)). Codex concatenates AGENTS.md files up to 32 KiB by default ([Codex guide](https://developers.openai.com/codex/guides/agents-md)). Gemini sends GEMINI.md "with every prompt" ([gemini-md.md](https://github.com/google-gemini/gemini-cli/blob/60f8082b14ffa7f10b608270232f3f57b5abbec8/docs/cli/gemini-md.md)).

*Implication:* a neutral source that grows past a loader cap loses rules on that host without a signal.

**P6.2** All three vendor CLIs share one hook wire shape: JSON on stdin and exit code 2 to block. They differ in failure. Claude Code carries out the action when a hook fails, unless `"onFailure": "block"` is set (v2.1.295 and later) ([hooks reference](https://code.claude.com/docs/en/hooks)). Codex fails open on a `PreToolUse` error and trusts a hook only against its hash ([Codex hooks](https://developers.openai.com/codex/hooks)). Gemini treats exit codes other than 0 and 2 as warnings ([hooks/reference.md](https://github.com/google-gemini/gemini-cli/blob/60f8082b14ffa7f10b608270232f3f57b5abbec8/docs/hooks/reference.md)).

*Implication:* the `onFailure` key closes the second case of F1.7 for Claude Code. Hooks on other hosts stay advisory.

**P6.3** Gstack renders `SKILL.md.tmpl` through 16 typed host profiles and commits the output. `skill:check` reports `STALE` on drift ([ARCHITECTURE.md](https://github.com/garrytan/gstack/blob/306ff23b9d2bd7951b7f6c408c7376f3237520c9/ARCHITECTURE.md)). Its Codex profile declares the skill root, a frontmatter allowlist, a hard description limit, path rewrites and `safetyHooks: 'advisory'` ([hosts/codex.ts](https://github.com/garrytan/gstack/blob/306ff23b9d2bd7951b7f6c408c7376f3237520c9/hosts/codex.ts)). ECC gates its hooks with `ECC_HOOK_PROFILE` and `ECC_DISABLED_HOOKS` environment variables ([hook-flags.js](https://github.com/affaan-m/ECC/blob/4adeee6732a12d1540f49a4d50a38ce8c97f9a53/scripts/lib/hook-flags.js)).

*Implication:* a typed profile is the right shape. The ECC switches break two hard rules.

**P6.4** Testing is tiered. Addyosmani/agent-skills checks routing with stemmed TF-IDF: a positive prompt must rank its skill first at least 95% of the time. The check is "deterministic, CI-safe" ([evals/README.md](https://github.com/addyosmani/agent-skills/blob/e997ac5b9a74eab332d5e9793bb38a38c8cfdd74/evals/README.md)). Claude Code runs `claude plugin eval` three times per case against a no-plugin baseline ([plugin evals](https://code.claude.com/docs/en/plugin-evals)). Gstack has a Stop hook that blocks once when required outputs are missing ([skill-contract-stop-hook.ts](https://github.com/garrytan/gstack/blob/306ff23b9d2bd7951b7f6c408c7376f3237520c9/hosts/claude/hooks/skill-contract-stop-hook.ts)). No project publishes an effect size of skills on task outcomes.

*Implication:* a free routing check is cheap. The effect of skills is unmeasured.

## 7. Comparison table

The table keeps the projects that a decision cites, and the evolve-loop row. "None" means the notes found no mechanism.

| Project | Loop shape | State and resume | Host protocol | Concurrency and verification |
|---|---|---|---|---|
| codex | Rust `run_turn`; Stop hooks extend | Rollout JSONL; `resume`, `fork` | app-server JSON-RPC; `exec --json` | Collab tools; guardian reviewer |
| opencode | One `llm.stream` per step | SQLite events; write-ahead claim | OpenAPI and SSE; ACP | Typed step outcomes |
| gemini-cli | Core streams a turn | Session JSONL; `--resume` | stream-json; exit codes 42 and 53; ACP | Loop detection |
| OpenHands V1 | Step on event-sourced state | JSON per event; unmatched-action rule | REST and WebSocket; ACPAgent | Analyzer and confirmation policy |
| LangGraph | Superstep | Barrier checkpoint, pending writes | Library | Interrupts |
| MAF | Superstep over typed edges | Checkpoint with graph signature hash | Library | Convergence exception |
| Pydantic AI | Three-node graph | Durable-unit journal | Library | Deferred tool requests |
| Symphony | Tick, reconcile, dispatch | None by design | codex app-server | Per-state caps |
| Gas Town and beads | 3-minute patrol | CAS claim, 5-minute lease | tmux nudges | Refinery batch 5, bisect |
| agent-orchestrator | Observe, update, derive | SQLite revision CAS | Adapters, PTY or ACP | Head-SHA CAS merge |
| paperclip | 30-second heartbeat | Postgres checkouts; wake receipts | Headless stream-json | Budgets; liveness classifier |
| orca | 2-second coordinator | SQLite; `*_unknown` states | PTY turns | Breaker 3; stale-base refusal |
| omnigent | Server, host, runner | Append-only journal | Typed executor; tmux paste and verify | Loop hash policy |
| claude-squad | 500-ms TUI tick | `state.json` | tmux `send-keys`; pane hash | None |
| snarktank/ralph | Bash loop | `prd.json`, git | `claude --print` | `<promise>` string |
| ralphex | Phased runner | Plan checkboxes | `claude` sessions | Plan re-parse |
| OMC ralph | Stop hook | Session PRD | Stop hook | Baseline gate; keyed approval |
| jcode | Rust turn loop | Session and memory graph | Own TUI | Canary and rollback |
| DGM | Generations over an archive | Archive of diffs | Own agent | Staged 10, 50, 200 |
| SICA | Iterations over an archive | Read-only archive mount | Own agent | Confidence lower bound |
| better-harness | Evaluator | Experiment lanes | Skill | Matched pairs |
| gstack | Template render | Committed output | 16 host profiles | Tiered tests; freshness |
| ECC | Hook runner | None | Installer, 17 targets | GateGuard; env switches |
| addyosmani/agent-skills | Plugin | None | 3 manifests | TF-IDF routing 95% |
| **evolve-loop** | Phase state machine | Hash-chained ledger; run lease (10-minute TTL, `runlease.go:36`) | Manifest factory; production on tmux (F1.1) | Waves of up to 5 lanes; EGPS verdict; train in shadow |

## 8. Adopt or reject

The decision column uses the words of the synthesis: Adopt, Adapt and Reject. The plan turns each Adopt and Adapt into a component at the shadow stage.

| Id | Practice | Decision | Reason |
|---|---|---|---|
| A1 | Headless `claude --print --output-format stream-json` as a route | Adapt | The driver exists (F1.2). Put it first in one phase as a shadow. The reason for tmux first is not recorded (F1.8). |
| A2 | ACP as a common transport | Adapt | Add the transport value only after `evolve doctor probe agy` shows an ACP or JSON mode. |
| A3 | One typed event and failure vocabulary | Adopt | Every transport then feeds the same failure adapter (P2.2, P2.4). |
| A4 | CLI session id in the ledger | Adopt | A resume then needs no surviving pane (P4.6). |
| A5 | Approval and sandbox mode in each manifest | Adopt | The headless defaults diverge (P2.3). |
| A6 | Typed failures for the tmux fallback | Adopt | Tmux stays a link of the chain (P4.6). |
| A7 | `codex app-server` JSON-RPC | Reject | The operator has no Codex subscription. |
| A8 | Record, do not re-run: phase output as a durable unit | Adapt | CLI side effects are not idempotent (P3.2, F1.5). |
| A9 | Graph and contract hash checked at resume | Adopt | No such hash exists (F1.9, P3.2). |
| A10 | `outcome_unknown` for a start with no end | Adopt | Orca and OpenHands record this case (P3.2, P4.3). |
| A11 | One status writer that records a cause | Adapt | The hash chain then explains a cycle (P4.1). |
| A12 | Policy snapshot hash in the first cycle record | Adopt | OpenHands V0 had mutable configuration that patched itself (P3.5). |
| A13 | Fork a retry from a barrier | Adapt | The failed run stays intact for the retrospective. |
| A14 | SQLite event store or outside durable engine | Reject | The ledger exists. OpenHands gained by removing a process boundary, not by a store (P3.5). |
| A15 | Item claim with compare-and-swap, lease and generation | Adopt | Beads shows the pattern (P4.2, F1.3). |
| A16 | Per-item circuit breaker at 3 failures | Adopt | Gas Town and orca agree (P4.2). |
| A17 | Stale-base admission, no breaker charge | Adopt | Orca shows the rule (P4.2). |
| A18 | Refinery train with head-SHA compare-and-swap | Adapt | The train exists in shadow (P4.5). |
| A19 | Liveness from two signals and a mass-death breaker | Adapt | AO shows the rule (P4.4, F1.8). |
| A20 | Blocked lane state for an operator need | Adopt | Symphony shows the state (P4.4). |
| A21 | Spawn pacing | Adopt | Orca and Gas Town pace starts (P4.4). |
| A22 | Restart reconcile before new admission | Adapt | Paperclip holds new starts after a restart (P4.3). |
| A23 | LLM supervisors | Reject | A hard rule: no LLM directs another (P4.7). |
| A24 | Archive of verified-good builds as rollback targets | Adapt | DGM gains from an archive (P5.5). No source tests it with merge-to-main. |
| A25 | The improver runs on the last verified-good build | Adopt | SICA shows the pattern (P5.2). |
| A26 | Evaluators read-only in the builder sandbox | Adopt | DGM node 114 (P5.4). |
| A27 | One bar for each stage move: one axis, two matched pairs, an interval | Adopt | Better-harness and SICA agree (P5.3). |
| A28 | Smoke test and rollback of a self-built binary | Adopt | Jcode shows the pattern (P5.4). |
| A29 | Bind a verdict to the eval revision; record amendments | Adapt | OMC and ralphex agree (P5.1, P5.3). |
| A30 | Improvement problems from failure telemetry | Adapt | DGM turns symptoms into problems. The pipeline routes the item. |
| A31 | Net deleted lines as a tie-break | Adapt | Autoresearch keeps a simpler change. |
| A32 | Open-ended search, force-accept, never stop | Reject | These break hard rules and the cost bound (P5.3, P5.5). |
| A33 | Typed host profile with a freshness check | Adopt | Gstack shows the shape (P6.3). |
| A34 | Render to `.agents/skills` with a frontmatter allowlist | Adapt | Claude upload paths reject extra keys (P6.1, F1.6). |
| A35 | Byte budget per host | Adopt | Loaders cap their input (P6.1). |
| A36 | `onFailure: block` on the guards | Adopt | It closes the second hole of F1.7 (P6.2). |
| A37 | One guard spec, per-host hook render | Adapt | Exit 2 works on all three CLIs (P6.2). |
| A38 | Free deterministic routing check | Adopt | Addyosmani enforces it (P6.4). |
| A39 | Contract Stop hook that blocks once | Adapt | The gate still decides (P6.4). |
| A40 | Bootstrap conformance test per host | Adapt | It shows if a target loads the skills (P6.4). |
| A41 | Per-hook switches and environment profiles | Reject | Hard rules: no feature flag, no control value from the environment (P6.3). |

## 9. Refinements to adopt

Each refinement names the findings that support it. The plan turns each into decisions.

| Id | Refinement | Findings |
|---|---|---|
| R1 | Define the typed event vocabulary before any transport change. | F1.2, F1.8, P2.2, P2.4 |
| R2 | Run `claude-p` as a shadow beside the tmux route for one phase. | F1.1, F1.2, P4.6 |
| R3 | Record the CLI session id in the phase ledger entry. | P2.4, P4.6 |
| R4 | Make each phase one durable unit with an identity hash. | F1.5, F1.9, P3.2 |
| R5 | Add `outcome_unknown` and a restart reconcile before admission. | P3.2, P4.3 |
| R6 | Claim an item with a generation number and a lease. | F1.3, F1.4, P4.2 |
| R7 | Declare a stall from two signals. Add a blocked lane state. | F1.8, P4.4 |
| R8 | Add the breaker, stale-base admission and spawn pacing. | P4.2, P4.4 |
| R9 | Pin the last verified-good build for a pipeline self-edit. Mount evaluators read-only. | P5.3, P5.4 |
| R10 | Make the four guards fail closed. | F1.7, P6.2 |
| R11 | Describe each host as profile data. | F1.6, P6.1, P6.3 |
| R12 | Keep the ledger and the state machine. Do not rewrite them. | P3.5, P4.3 |

## 10. Measured, inferred and not found

| Claim | Status | Basis |
|---|---|---|
| Crash recovery by replay costs 7.4 ms (median) and 32.1 ms (maximum) | Measured | OpenHands, 433 conversations (P3.3) |
| The removal of a process boundary cut failures 61% | Measured | OpenHands rollout, 78.0 to 30.0 per thousand (P3.5) |
| An archive beats "always mutate the latest" | Measured on benchmarks | DGM 50.0% against 23.0% (P5.5) |
| A self-modifying agent games a detector that it can see | Measured, one case | DGM node 114 (P5.4) |
| ACP is implemented by four of six runtimes | Measured | Method coverage per CLI not checked (P2.4) |
| Structured channels are more reliable than tmux scraping | Inference | No source benchmarks them against each other |
| Paste-and-verify removes "prompt never submitted" stalls | Inference | Omnigent design (P4.6); no failure rates |
| Multi-signal liveness lowers false stalls | Inference | AO and Gas Town designs (P4.4); no false-stall rates |
| Unclaimed spawns run away without a guard | Measured, one incident | Gas City, 1,400 seats in 10 hours (P4.4) |
| A guard fails open when both binaries are missing, or when it exits with a code other than 0 and 2 | Inference | `.claude/settings.json:10`, Claude Code default (F1.7); not tested |
| Evolve-loop has no phase-graph signature on resume | Weak measurement | Code search only (F1.9) |
| Skills change task outcomes | Unmeasured | No project publishes an effect size (P6.4) |

**Not found.** The research did not read these sources:

- the codex guardian semantics (method names only);
- the opencode `packages/cli-rs` tree;
- the field schemas of gemini `stream-json` and cline `--json`;
- the MAF upstream repository (HTTP 404 on 2026-11-27, so the MAF facts come from the PyPI wheel);
- the `gascity-packs` repository, and most of the 5,901-line paperclip `queue.ts`;
- the better-harness matched-pair formula.

The research also found no source that compares the reliability of host protocols. No fleet tool publishes merge-train throughput or false-stall rates. No system publishes a months-long record of a self-modifying pipeline.

**Gaps of this repository.** The research did not probe if agy offers ACP or a JSON stream. It did not trace why routing prefers tmux over `claude-p`. It did not verify if `evolve skills publish` writes `.agents/skills` with a frontmatter allowlist. It did not check phase-result reuse beyond `verdictcache`. A1, A2, A8 and A34 each start with the check that closes their gap.

**Claim left out.** The synthesis said that the guards fail open when the binary is missing. This dossier corrects that claim in F1.7. The synthesis cited a local path for the wave-concurrency report (`admission.go`, `repin.go:20`). This dossier did not verify those two lines and does not cite them.
