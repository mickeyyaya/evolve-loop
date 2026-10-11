# Cycle agent messaging: research dossier (2026-10)

> The plan that uses this research: [agent-messaging-2026-10.md](../plans/agent-messaging-2026-10.md). The design: [agent-messaging.md](../architecture/agent-messaging.md). The decision record: [ADR-0129](../architecture/adr/0129-cycle-scoped-agent-messaging.md). The substrate: [event-channels.md](../architecture/event-channels.md) ([ADR-0127](../architecture/adr/0127-push-only-event-channels.md)).
>
> The refinements R1 to R24 at the end map to the plan's decisions D1 to D15.

Date: 2026-10-09. Method: a survey of current practice (2024 to 2026) for a general agent-to-agent message protocol. A read-only probe of the code seams of this repository adds the local evidence. Each claim has a source. "Secondary" marks a blog, a summary or a mirror. "Synthesis" marks our own inference.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Agent-to-agent protocols](#2-agent-to-agent-protocols)
3. [Addresses and scope](#3-addresses-and-scope)
4. [Identity for processes with one uid](#4-identity-for-processes-with-one-uid)
5. [Delivery to an LLM agent in a task](#5-delivery-to-an-llm-agent-in-a-task)
6. [Safety](#6-safety)
7. [Scale and delivery semantics](#7-scale-and-delivery-semantics)
8. [Adopt or reject](#8-adopt-or-reject)
9. [Refinements to adopt](#9-refinements-to-adopt)

## 1. Local evidence

**F1.1** The phases of one cycle run mostly in order. The kernel dispatches one phase at a time, and `cs.ActiveAgent` holds one value (`go/internal/core/cyclerun_dispatch.go:45`). The phase order is `docs/architecture/phase-registry.json` and `go/internal/phaseorder/phaseorder.go`.
*Implication:* the recipient of a message often does not exist yet when the sender sends it. Store and forward is the base case, and live delivery is the special case.

**F1.2** Some phases run at the same time. The evaluate batch runs the consecutive `evaluate` phases between build and audit in goroutines (`go/internal/core/evaluate_batch.go:12`, `:72`). `ParallelEvaluateConcurrency` bounds it (`:86`, default 3). A phase with `parallel_eligible` can fan out workers, and each worker is its own bridge dispatch.
*Implication:* live peers exist, so the design needs live delivery and request/reply too.

**F1.3** Several cycles run at the same time. Each fleet lane runs `evolve cycle run` as its own process, with its own cycle number, worktree and workspace (`go/cmd/evolve/cmd_fleet.go:127`).
*Implication:* "same cycle only" is a real boundary between live processes on one host.

**F1.4** Each dispatch has an id. `mintDispatchID` makes `<run>/<cycle>/<agent>/p<owner-pid>n<nonce>` (`go/internal/bridge/dispatch_reap.go:39`, `go/internal/proctree/dispatchid.go:20`). The bridge exports it as `EVOLVE_DISPATCH_ID` into the pane (`dispatch_reap.go:44`) and into the process environment (`:51`). The key is in `go/internal/ipcenv/ipcenv.go:25`.
*Implication:* the dispatch id names the cycle and the agent. It is a good principal name, but a value in the environment is not a proof.

**F1.5** All panes run as one uid. No code reads `LOCAL_PEERPID` or `SO_PEERCRED`. A manifest cannot set an `EVOLVE_` key (`go/internal/bridge/manifest.go:245`).
*Implication:* a uid check cannot tell two agents apart. The identity needs a second source.

**F1.6** The sandbox profile denies writes under the repository root and then allows the worktree, the workspace and `/tmp` (`go/internal/adapters/sandbox/sandbox.go:181`, `go/internal/bridge/sandbox_wrap.go:185`). The bridge forces `AllowNetwork` for every sandboxed launch (`sandbox_wrap.go:246`). Phases that write no source run with no sandbox (`sandboxPrefixForLaunch`, `sandbox_wrap.go:225`).
*Implication:* the kernel can confine a sandboxed agent to one outbox directory. An agent with no sandbox needs the token as its proof.

**F1.7** ADR-0127 gives append-only channel logs with a lock, a kernel wake and one cursor for each reader. The channel name rule accepts any dotted name (`go/internal/events/channel/channel.go:39`). A record holds exactly one of a signal and a gap (`go/internal/events/channel/record.go:43`).
*Implication:* a per-cycle channel `msg.<cycle>` is a legal name now. A message needs a new record variant, because a signal carries only string fields.

**F1.8** ADR-0127 adds `/.evolve/events/` to the protected surface, so no phase agent writes a channel record. The manifest is in `go/internal/guards/integrity_surface.go:18`.
*Implication:* an agent cannot append its own message. A trusted writer outside the sandbox must do it.

**F1.9** The bridge inbox is the nearest prior art. It keeps one NDJSON file for each agent at `<workspace>/.bridge-inbox/<agent>.ndjson` (`go/internal/bridge/inbox/inbox.go:50`, `:75`). The bridge reads it in its wait loop and types the text into the pane (`go/internal/bridge/tmux_inject.go:37`, `:81`).
*Implication:* that path polls, it reaches only a live tmux agent, and it has no sender identity. The name "inbox" also belongs to the operator inbox (ADR-0007). This design uses "mailbox" and "outbox".

**F1.10** The prompt has a cache-stable prefix and a per-cycle tail. `cycleContextBoundary` splits them (`go/internal/phases/runner/runner.go:260`).
*Implication:* a message section goes after the boundary, so it never breaks the prompt cache.

**F1.11** The cycle owner is the process that calls `Orchestrator.RunCycle` (`go/internal/core/orchestrator.go:722`). `newCycleRun` returns a cleanup stack (`go/internal/core/cyclerun.go:360`), and it starts the run lease (`:533`). The resume path is `RunCycleFromPhase` (`go/internal/core/resume.go:248`, lease at `:289`).
*Implication:* one service for each cycle can start and stop with the cycle run, on both paths.

**F1.12** The project hooks of Claude Code are `PreToolUse` and `PostToolUse` entries in `.claude/settings.json`. No hook carries `additionalContext` today. No hook support for agy was found. A headless driver cannot take input in a turn (`ErrTransportNoInject`, `go/internal/bridge/channel/supervisor.go:21`).
*Implication:* live delivery is an adapter for each CLI, and agy needs another rung.

## 2. Agent-to-agent protocols

**F2.1** Google A2A, now under the Linux Foundation. An agent publishes an Agent Card at `/.well-known/agent-card.json`, with its identity, capabilities, endpoint and auth schemes. Sources: https://a2a-protocol.org/latest/specification/ ; https://tyk.io/learning-center/a2a-protocol-architecture-and-technical-specification/ (secondary).
*Implication:* discovery across a network is not our problem. One host and one roster are enough.

**F2.2** The A2A unit of work is the Task, with active, interrupted and terminal states. A terminal task takes no more messages. Source: https://a2a-protocol.org/latest/specification/ (§4.1.3, §3.1.1).
*Implication:* a request with a deadline plays the task. An expired request is terminal.

**F2.3** Each A2A Message has a required `messageId`, and it can have `contextId`, `taskId` and `referenceTaskIds`. The server can use `messageId` to find duplicates, so a send can be idempotent. Source: https://a2a-protocol.org/latest/specification/ (§4.1.4, §3.3.1).
*Implication:* adopt the envelope fields: `id` as the idempotency key, `context_id` as the cycle, and `correlation_id` for request and reply.

**F2.4** A2A streams with Server-Sent Events. A push notification goes to a webhook that the client registers, and then the client reads the full state. Sources: https://a2a-protocol.org/latest/specification/ ; https://tyk.io/learning-center/a2a-protocol-architecture-and-technical-specification/ (secondary).
*Implication:* reject the transport. HTTP, SSE and webhooks add nothing on one host.

**F2.5** A2A §13.1: the server must authorize every operation. It must limit results to the caller's scope, also when the caller gives no `contextId` filter. It must not show that an out-of-scope resource exists. Source: https://a2a-protocol.org/latest/specification/
*Implication:* the scope check runs before any lookup, and an out-of-scope lookup gives "not found".

**F2.6** A2A §13.2: the receiver validates each notification, processes it idempotently and rate-limits it. The sender retries with exponential back-off. Source: https://a2a-protocol.org/latest/specification/
*Implication:* the receiver must expect duplicates.

**F2.7** IBM ACP was a REST agent protocol (March 2025). It merged into A2A in August 2025. Sources: https://www.hpcwire.com/bigdatawire/2025/05/08/open-source-ai-gets-a-boost-as-ibm-hands-over-key-projects-to-linux-foundation/ (secondary); https://aiwiki.ai/wiki/a2a_protocol (secondary).
*Implication:* ACP is not a live standard.

**F2.8** ANP uses `did:wba` identifiers that resolve to a `did.json` file over HTTPS. The handshake is a signed request, a DID resolve, a signature check and a short-lived token. Sources: https://arxiv.org/pdf/2505.02279 ; https://atlan.com/know/mcp/how-to-choose-mcp-a2a-anp/ (secondary).
*Implication:* ANP solves trust between organizations. It is too heavy for one host.

**F2.9** MCP is agent to tool, not agent to agent. Elicitation asks the user for data. The client must show which server asks, and the user can decline. Source: https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation
*Implication:* MCP does not give a peer protocol.

**F2.10** MCP 2025-11-25 has `resources/subscribe` and sampling. The 2026-07-28 revision makes MCP stateless and removes `notifications/elicitation/complete`. Sources: https://glama.ai/mcp/servers/@MikkoParkkola/mcp-gateway/blob/05b91c64a18dc6d4051bb14ac93342211a2cc1b1/docs/issue-73-impl-plan.md (secondary); https://modelcontextprotocol.io/specification/2026-07-28/changelog
*Implication:* the MCP surface still changes. Do not build on it now.

**F2.11** Claude Code channels push events into a live session through an MCP server. The model gets each event as a `<channel source="...">` block. Each channel keeps a sender allowlist. The feature is a research preview. Source: https://code.claude.com/docs/en/channels
*Implication:* a preview feature for one CLI cannot be the base.

**F2.12** Claude Code agent teams have a lead, teammates, a task list and a mailbox. Each mailbox is a JSON file at `~/.claude/teams/{team}/inboxes/{agent}.json`. The reader validates each entry and removes malformed ones. Source: https://code.claude.com/docs/en/agent-teams
*Implication:* a file mailbox is enough for live LLM agents.

**F2.13** Agent teams deliver with no poll from the lead. A message to a stopped teammate restarts it and becomes its next prompt. Task claims use file locks. Source: https://code.claude.com/docs/en/agent-teams
*Implication:* "the next prompt" is our delivery at dispatch.

**F2.14** In agent teams, the receiver learns that a message came from another session, not from the user. A message cannot approve a permission prompt. In auto mode, a classifier checks each message. Source: https://code.claude.com/docs/en/agent-teams
*Implication:* adopt the label "from an agent, not the operator" and "no authority".

**F2.15** Claude Code cross-session messaging uses one Unix socket for each session, open only to the OS user. A message arrives between tool calls, and an idle session starts a new turn. Source: https://code.claude.com/docs/en/cross-session-messaging
*Implication:* delivery between tool calls never breaks a running tool.

**F2.16** The receiver can accept, hold or refuse. Held messages stop at 100. The receiver rate-limits each sender, drops identical repeats and queues at most 50 messages. The docs say: "A message loop between two sessions therefore stops on its own". Source: https://code.claude.com/docs/en/cross-session-messaging
*Implication:* adopt the guards and their values as the first caps.

**F2.17** Claude Code checks a message from the session's own child (a hook or a Bash command) by process evidence. Else it uses a per-session token in `CLAUDE_CODE_MESSAGING_TOKEN`. Source: https://code.claude.com/docs/en/cross-session-messaging
*Implication:* a token in the environment is accepted practice as a second check.

**F2.18** An OpenAI Agents SDK handoff moves control to another agent. By default the new agent sees the full history, and an `input_filter` can change what it sees. Sources: https://openai.github.io/openai-agents-python/ja/handoffs/ ; https://tessl.io/registry/tessl/pypi-openai-agents/0.6.0/files/docs/handoffs.md (secondary); https://github.com/openai/openai-agents-python/pull/2157/files
*Implication:* the receiver gets a filtered, bounded view, not the sender's transcript.

**F2.19** AutoGen 0.4 Core has direct messages (to an `AgentId`) and broadcast (to a `TopicId`). A `TopicId` is a pair (type, source), and the source "lets publishers create isolated silos". The runtime makes an agent on demand. A topic with no subscription delivers to no one. Source: https://microsoft.github.io/autogen/stable/user-guide/core-user-guide/core-concepts/topic-and-subscription.html
*Implication:* the cycle is the source of a topic. "Made on demand" is our planned phase.

**F2.20** AutoGen group chats stop on `max_round` or on termination conditions such as `MaxMessageTermination(25)`. Sources: https://docs.ag2.ai/latest/docs/user-guide/advanced-concepts/ending-a-chat/ ; https://microsoft.github.io/autogen/0.6.2/user-guide/agentchat-user-guide/selector-group-chat.html
*Implication:* a conversation needs a hard stop.

**F2.21** LangGraph agents share one graph state. Concurrent writes to one key can overwrite each other unless a reducer merges them. Sources: https://towardsdatascience.com/how-agent-handoffs-work-in-multi-agent-systems/ (secondary); https://pypi.org/project/langgraph-supervisor/ ; https://wec.wiline.com/docs/tutorials/langgraph-multi-agent-handoff-state/ (secondary).
*Implication:* an append-only log with one writer for each append has no lost update.

**F2.22** CrewAI gives a delegating agent two tools, "Delegate work to coworker" and "Ask question to coworker", addressed by role. Source: https://docs.crewai.com/en/concepts/collaboration
*Implication:* a message is a tool call that names a role. Our tool is `evolve msg send`.

**F2.23** MetaGPT agents publish to a shared message pool. Each role subscribes to the action types that it needs. A message carries `cause_by`, `sent_from` and `send_to`. Source: https://deepwiki.com/geekan/MetaGPT/2.2-message-passing-system (secondary).
*Implication:* a role address and a kind are enough to route most messages.

**F2.24** CAMEL role-play shows role flips, repeated instructions and infinite message loops. The fixes are prompt rules, a done token and a turn cap. Sources: https://arxiv.org/pdf/2303.17760 ; https://docs.camel-ai.org/key_modules/society.html
*Implication:* a loop needs a limit in code, not only in the prompt.

## 3. Addresses and scope

**F3.1** NATS gives each user publish and subscribe allow and deny lists. Deny wins over allow. With an allow list, every other subject is refused, and the server logs the violation. Source: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/authorization
*Implication:* an ordered rule list where a deny wins, with a default deny.

**F3.2** NATS `allow_responses` gives a responder a temporary right to publish only to the reply subject. Source: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/authorization
*Implication:* a request grants a reply right for its correlation id until it expires.

**F3.3** A NATS account is an isolated tenant with its own subject space. Messages never cross accounts without an explicit export and import, which is "stronger than permissions". Source: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/accounts
*Implication:* make the cycle an account, not only a permission. One service for each cycle cannot deliver to another cycle.

**F3.4** Mosquitto `pattern` ACL lines put `%c` (client id) and `%u` (user name) into a topic, as one full topic level. Sources: https://mosquitto.org/man/mosquitto-conf-5.html ; https://mosquitto.org/documentation/plugins/acl-file/
*Implication:* the scope comes from the sender's identity, never from the message.

**F3.5** Kafka ACLs are LITERAL or PREFIXED. One PREFIXED ACL covers a namespace for each tenant. Sources: https://cwiki.apache.org/confluence/display/KAFKA/KIP-290%3A+Support+for+Prefixed+ACLs ; https://conduktor.io/glossary/kafka-acls-and-authorization-patterns (secondary).
Synthesis: a prefix without its separator matches too much, because `team-a` also matches `team-abc`.
*Implication:* a channel family is `msg.<cycle>` with the dot.

**F3.6** Akka and Pekko `tell` gives at-most-once delivery and order for each sender and receiver pair. Retries give at-least-once, with a correlation id and an ack. Sources: https://pekko.apache.org/docs/pekko/1.1/general/message-delivery-reliability.html ; https://www.lightbend.com/blog/akka-message-delivery-part-2
*Implication:* one log for each cycle gives a total order in the cycle, which is more than the order of each pair.

## 4. Identity for processes with one uid

**F4.1** `getpeereid()` gives the uid and gid of the peer of a Unix socket, fixed at connect time. On macOS, `LOCAL_PEERPID` gives the peer pid. Sources: https://man.freebsd.org/getpeereid(3) ; https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man3/getpeereid.3.html ; https://codebrowser.dev/slint/crates/std/src/os/unix/ucred.rs.html
*Implication:* with one uid, the uid check tells nothing.

**F4.2** A pid can map to a process tree, but the system can use a pid again after an exit. Claude Code reports that on macOS it can check a child "only while the posting process is still running", so it adds a token. Source: https://code.claude.com/docs/en/cross-session-messaging
*Implication:* a pid is a weak identity. Kernel peer credentials cannot tell agents apart under one uid.

**F4.3** A macaroon is a bearer token of chained HMACs. Any holder can add a caveat, for example `cycle = 1850`, and so make the token weaker. Nobody can remove a caveat. The check needs the root key. Sources: https://research.google/pubs/macaroons-cookies-with-contextual-caveats-for-decentralized-authorization-in-the-cloud/ ; https://www.ndss-symposium.org/wp-content/uploads/2017/09/04_3_1.pdf
*Implication:* a per-dispatch token with caveats for the cycle, the dispatch, the agent and the expiry needs only the standard library.

**F4.4** Biscuit uses public-key signatures and a Datalog policy language. The IETF draft "Attenuating Agent Tokens" (2026) proposes simpler constraint types. Sources: https://docs.rs/biscuit-auth ; https://www.ietf.org/archive/id/draft-niyikiza-oauth-attenuating-agent-tokens-01.html
*Implication:* offline checks are not needed. We have one verifier.

**F4.5** SPIFFE and SPIRE attest a workload from its pid and issue an SVID. The guides warn that a selector of the uid alone matches every process of that uid. Sources: https://spiffe.io/docs/latest/deploying/registering ; https://goteleport.com/docs/enroll-resources/workload-identity/workload-attestation
*Implication:* a new daemon and a PKI are too heavy for one host.

**F4.6** OWASP Agentic Top 10 (2026) lists ASI07, "Insecure Inter-Agent Communication": a message without authentication or integrity lets an attacker spoof a trusted agent. Sources: https://www.promptfoo.dev/docs/red-team/owasp-agentic-ai/ (secondary); https://kratikal.com/blog/top-ten-owasp-risk-for-agentic-applications/ (secondary).
*Implication:* the broker stamps the sender. A sender value in the message is data.

Synthesis: the threat is not a hostile local user. The threat is a confused or injected agent. It can try to (a) claim that it is another phase, (b) write to another cycle, or (c) steer its auditor.

| Mechanism | Stops (a) | Stops (b) | Cost | Fit |
|---|---|---|---|---|
| uid peer credentials | no (one uid) | no | low | weak alone |
| pid and process tree | while the sender lives | yes | medium: pid reuse, macOS limits | a second check at most |
| HMAC token for each dispatch, with caveats | if the agent cannot read other tokens | yes | low: standard library | good |
| Biscuit | yes | yes | a library and Datalog | too heavy |
| SPIFFE and SPIRE | yes | yes | a daemon and a PKI | too heavy |
| an outbox path that the sandbox enforces | yes | yes | low: we write the profiles | best fit |

## 5. Delivery to an LLM agent in a task

**F5.1** Claude Code delivers a cross-session message between tool calls in an active turn. An idle session starts a new turn with the message. Source: https://code.claude.com/docs/en/cross-session-messaging
*Implication:* a message arrives at a safe point, never in the middle of a tool.

**F5.2** `additionalContext` in `hookSpecificOutput` works on `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop` and other events. Claude Code wraps the text in a system reminder. Text over 10,000 characters goes to a file. The docs advise factual words, because text in the style of a system order can trip injection defenses. Source: https://code.claude.com/docs/en/hooks
*Implication:* a `PostToolUse` hook can deliver to a busy Claude agent, with factual framing.

**F5.3** `asyncRewake: true` runs a command hook in the background. On exit code 2 it wakes Claude, and stderr shows as a system reminder. Source: https://code.claude.com/docs/en/hooks
*Implication:* a background hook that blocks on a kernel wake can wake an idle Claude agent with no poll.

**F5.4** Gemini CLI and Antigravity hooks: `BeforeAgent` and `AfterTool` can add `additionalContext`. We found no async wake hook. Sources: https://geminicli.com/docs/hooks/reference/ ; https://medium.com/google-cloud/a-developers-guide-to-agent-hooks-in-antigravity-cli-4c1440febd11 (secondary).
*Implication:* agy has no push rung for an idle agent. A fixed-text nudge fills the gap.

**F5.5** `tmux send-keys` types into a pane as if a user typed it. Source: https://man7.org/linux/man-pages/man1/tmux.1.html
*Implication:* typed text gets user authority, races with the CLI's own input and has no provenance. Never type message text.

**F5.6** Store and forward at start: a message to a stopped teammate becomes its next prompt (https://code.claude.com/docs/en/agent-teams). AutoGen makes the recipient on demand (https://microsoft.github.io/autogen/stable/user-guide/core-user-guide/core-concepts/topic-and-subscription.html).
*Implication:* the main path is a prompt section at dispatch.

## 6. Safety

**F6.1** Prompt Infection (Lee and Tiwari, 2024): a malicious prompt copies itself from agent to agent. "LLM Tagging", which marks the source of each message, cuts the spread. Source: https://arxiv.org/abs/2410.07283
*Implication:* each message shows its provenance.

**F6.2** Multi-agent hijacking (Triedman, Jha, Shmatikov, COLM 2025): bad content took over AutoGen, CrewAI and MetaGPT in 45% to 64% of trials. For some pairs, the rate was 100%. Source: https://arxiv.org/abs/2503.12188
*Implication:* each agent must treat a peer message as untrusted.

**F6.3** Agent-in-the-Middle (He and others, ACL Findings 2025): an attacker that only changes the messages between agents can take over the system. Source: https://arxiv.org/abs/2502.14847
*Implication:* only the broker writes the store, and the agents cannot write it.

**F6.4** Open challenges in multi-agent security (Schroeder de Witt and others, 2025): free-form messages allow secret collusion through steganography. Sources: https://arxiv.org/abs/2505.02077 ; https://arxiv.org/html/2402.07510v5
*Implication:* audit independence is a policy rule. Free text from build does not reach the auditor.

**F6.5** Design patterns against prompt injection (Beurer-Kellner and others, 2025): plan-then-execute fixes the control flow before untrusted data arrives. Sources: https://arxiv.org/abs/2506.08837 ; https://simonwillison.net/2025/Jun/13/prompt-injection-design-patterns/ (secondary).
*Implication:* a message is data, and it never changes a phase's control flow.

**F6.6** MAST (Cemri and others, NeurIPS 2025): 14 failure modes from more than 1,600 traces. Most failures come from the system design and from misalignment between agents. Source: https://arxiv.org/abs/2503.13657
*Implication:* the limits and the policy belong in code.

**F6.7** A message from another agent cannot approve a permission prompt, change the configuration or run a slash command. Source: https://code.claude.com/docs/en/cross-session-messaging
*Implication:* a message has no authority.

## 7. Scale and delivery semantics

**F7.1** At-least-once delivery needs a correlation id, a retry and an ack. Source: https://www.lightbend.com/blog/akka-message-delivery-part-2
*Implication:* the id is the idempotency key, and the cursor is the ack.

**F7.2** A guarantee of delivery needs a durable store, so that an accepted message survives a crash. Source: https://discuss.akka.io/t/delivery-guarantee-for-local-messages-in-distributed-pub-sub/10640 (forum opinion, secondary).
*Implication:* the store is the ADR-0127 channel log, not memory.

**F7.3** Enterprise Integration Patterns: the Dead Letter Channel, the Correlation Identifier and the Idempotent Consumer. Sources: https://cwiki.apache.org/confluence/display/CAMEL/Enterprise+Integration+Patterns ; https://docs.redhat.com/en/documentation/red_hat_fuse/7.13/html/apache_camel_development_guide/introtoeip
*Implication:* adopt all three by name.

**F7.4** Claude Code drops an unanswered idle subscription after 12 hours and tells the waiting agent. A2A suggests delivery timeouts of 10 to 30 seconds with back-off. Sources: https://code.claude.com/docs/en/cross-session-messaging ; https://a2a-protocol.org/latest/specification/
*Implication:* a request has a deadline, and an expired message is a dead letter.

**F7.5** NATS accounts and AutoGen topic sources isolate tenants. Kafka prefixed ACLs cover a topic family. Sources: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/accounts ; https://microsoft.github.io/autogen/stable/user-guide/core-user-guide/core-concepts/topic-and-subscription.html ; https://conduktor.io/glossary/kafka-acls-and-authorization-patterns (secondary).
*Implication:* one channel for each cycle is the unit of order, of access and of retention.

## 8. Adopt or reject

| Prior art | Verdict | Reason | Finding |
|---|---|---|---|
| A2A envelope fields (`messageId`, `contextId`, task and reference ids) | adopt | an idempotency key, a cycle and a correlation | F2.3 |
| A2A transport (HTTP, SSE, webhooks) | reject | one host needs no network transport | F2.4 |
| A2A scope rule (§13.1) | adopt | the scope check runs before any lookup | F2.5 |
| A2A idempotent receiver (§13.2) | adopt | duplicates can occur | F2.6 |
| ACP | reject | merged into A2A | F2.7 |
| ANP and DIDs | reject | trust across organizations | F2.8 |
| MCP as the peer protocol | reject | agent to tool; the surface still changes | F2.9, F2.10 |
| Claude Code channels | reject for now | a research preview for one CLI | F2.11 |
| Claude Code agent teams as the transport | reject | scoped by OS user; no agy; no audit rule | F2.12, F2.15 |
| A file mailbox for each agent | adopt | proven for live LLM agents | F2.12 |
| "next prompt" for a stopped agent | adopt | the prompt section at dispatch | F2.13, F5.6 |
| "from an agent, no authority" label | adopt | the frame of each message | F2.14, F6.7 |
| Claude Code guards: rate, repeat drop, queue cap 50 | adopt | the first caps | F2.16 |
| A token in the environment as a second check | adopt | the per-dispatch token | F2.17 |
| A filtered view for the receiver | adopt | the receiver never sees the sender's transcript | F2.18 |
| AutoGen topic (type, source) | adopt | source = the cycle; a planned phase is made on demand | F2.19 |
| A hard stop for a conversation | adopt | the hop limit and the cycle budget | F2.20, F2.24 |
| Shared mutable state | reject | lost updates; use the append-only log | F2.21 |
| A message as a tool call that names a role | adopt | `evolve msg send` with `role:` | F2.22 |
| MetaGPT role and kind routing | adopt | role, phase and kind in the policy | F2.23 |
| NATS deny wins and default deny | adopt | the rule engine | F3.1 |
| NATS `allow_responses` | adopt | the reply right | F3.2 |
| NATS accounts | adopt | one service for each cycle | F3.3 |
| Mosquitto `%u` patterns | adopt | the scope comes from the sender's identity | F3.4 |
| Kafka prefixed ACLs | adopt with the separator | `msg.<cycle>` | F3.5 |
| Akka order for each pair | exceed | one log gives a total order in the cycle | F3.6 |
| uid peer credentials | reject alone | one uid | F4.1 |
| pid and process tree | reject as the main check | pid reuse; macOS limits | F4.2 |
| Macaroon-style HMAC caveats | adopt | standard library; a caveat only narrows | F4.3 |
| Biscuit | reject | one verifier; no Datalog needed | F4.4 |
| SPIFFE and SPIRE | reject | a daemon and a PKI | F4.5 |
| A sender stamp from the broker | adopt | ASI07 | F4.6 |
| An outbox path that the sandbox enforces | adopt | the kernel enforces it | §4 table |
| Delivery between tool calls | adopt | never in the middle of a tool | F5.1 |
| `PostToolUse` `additionalContext` | adopt | the busy Claude rung | F5.2 |
| `asyncRewake` | adopt | the idle Claude rung | F5.3 |
| agy `AfterTool` and `BeforeAgent` | defer | no async wake found; a nudge first | F5.4 |
| `tmux send-keys` of message text | reject | user authority and no provenance | F5.5 |
| LLM tagging of the source | adopt | the frame | F6.1 |
| A policy rule for audit independence | adopt | free text from build never reaches the auditor | F6.4 |
| The store written only by the broker | adopt | Agent-in-the-Middle | F6.3 |
| At-least-once with an idempotency key | adopt | the id and the cursor | F7.1 |
| A durable store | adopt | the channel log | F7.2 |
| Dead Letter Channel | adopt | `msg.dead` | F7.3 |
| Correlation Identifier | adopt | request and reply | F7.3 |
| Deadlines for requests | adopt | an expired message is a dead letter | F7.4 |
| One partition for each tenant | adopt | one channel for each cycle | F7.5 |

## 9. Refinements to adopt

1. **R1** The protocol accepts any cycle in an address. The policy refuses a cross-cycle address. [F3.1, F3.3] → D1
2. **R2** A relative address takes the sender's cycle from its identity, never from the message. [F3.4] → D1
3. **R3** One service for each cycle. It holds only its own cycle, so cross-cycle delivery is impossible by construction. [F3.3, F7.5] → D2
4. **R4** The first identity check is location: the sandbox lets a dispatch write only its own outbox. [§4 table, F1.6] → D3
5. **R5** The second identity check is an HMAC token for each dispatch, with caveats that only narrow it. [F4.3, F2.17] → D3
6. **R6** The broker stamps `from` and ignores a sender value. A uid check is not identity. [F4.1, F4.2, F4.6] → D3
7. **R7** The policy is an ordered rule list from config. A deny wins, and the default denies. [F3.1] → D4
8. **R8** Audit independence is a deny rule for free text from build and tdd to audit. [F6.4, F2.23] → D4, OD1
9. **R9** A request grants a reply right for its correlation id until its deadline. [F3.2] → D4
10. **R10** The scope check runs before any lookup. An out-of-scope lookup gives "not found". [F2.5] → D5
11. **R11** The envelope copies A2A: an id (the idempotency key), a context id (the cycle) and a correlation id. [F2.3] → D6
12. **R12** The kind set is an open registry in config. [F2.23] → D6
13. **R13** The store is one ADR-0127 channel for each cycle, `msg.<cycle>`, with the dot. [F1.7, F3.5, F7.2] → D7
14. **R14** Only the broker writes the store, and the store is protected surface. [F1.8, F6.3] → D7
15. **R15** The main delivery is a prompt section at dispatch, after the cache boundary. [F1.1, F1.10, F5.6] → D8
16. **R16** A busy Claude agent gets a `PostToolUse` hook, and an idle one an `asyncRewake` hook. [F5.1, F5.2, F5.3] → D8
17. **R17** An idle agy agent gets a fixed-text nudge. [F5.4] → D8, OD3
18. **R18** A message is untrusted data in a frame with its provenance and a "no authority" line. Never type message text into a pane. [F2.14, F5.5, F6.1, F6.7] → D9
19. **R19** Config gives the caps: a body cap, a sender rate, a repeat drop and a queue cap of 50. [F2.16] → D10
20. **R20** A hop limit and a cycle budget stop a conversation. [F2.20, F2.24, F6.6] → D10
21. **R21** Delivery is at least once: the id, a cursor for each recipient and a seen set. [F2.6, F7.1] → D11
22. **R22** An expired or undeliverable message goes to the dead-letter channel `msg.dead`. [F7.3, F7.4] → D11
23. **R23** A recipient outside the cycle roster is refused at send time. [F2.19] → D12
24. **R24** Every accepted, refused or dead message is an event with metadata only. [F6.1, F6.4] → D14
