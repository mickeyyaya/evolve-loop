# ADR-0129: Agents of one cycle send messages through a broker of that cycle, with a general protocol and a scoped policy

- **Status:** Proposed (2026-10-09). The operator gave decisions OD1 to OD4 on 2026-10-09 ([plan](../../plans/agent-messaging-2026-10.md) §4). This ADR moves to Accepted as components M2 to M8 land ([plan](../../plans/agent-messaging-2026-10.md) §16).
- **Spec of record:** [agent-messaging.md](../agent-messaging.md). It holds the protocol: the envelope, the addresses, the policy, the identity, the flow and the delivery. It also holds the limits, the exit codes and the threat model.
- **Plan:** [agent-messaging-2026-10.md](../../plans/agent-messaging-2026-10.md).
- **Research:** [agent-messaging-2026-10.md](../../research/agent-messaging-2026-10.md).
- **Amends:**
  - [ADR-0127](0127-push-only-event-channels.md), the event channels:
    - a channel record holds exactly one of `signal`, `gap` and a new `message` variant;
    - a channel family `msg.<cycle>` holds the messages of one cycle, and `msg.dead.<cycle>` holds the dead letters of that cycle;
    - the service of a cycle is the only writer of its channel, outside the closed catalog of the `Publisher`.
  - [ADR-0064](0064-pipeline-integrity-boundary.md), the protected surface: it adds `/.evolve/msg/`, the outboxes and the mailboxes. Only `evolve msg` writes the outbox of its own dispatch, through the grant of the sandbox.
- **Evidence:**
  - Phases talk only through artifacts. The bridge inbox polls, reaches only a live tmux agent and has no sender identity (research F1.9).
  - The kernel dispatches one phase at a time, so most recipients start after the send (research F1.1).
  - All panes share one uid, so a peer credential cannot tell agents apart (research F1.5, F4.1, F4.2).
  - Fleet lanes run several cycles at the same time (research F1.3).
  - Multi-agent studies show hijack rates of 45% to 64% through messages between agents (research F6.2).

## Context

On 2026-10-09 the operator asked for messages between the agents of a cycle: "Could we research and design a way allowing each phase agent send message to any other agents within the same cycle. (The protocol should allow any messaging, but our system should apply the permission only allowing the agent within the same cycle to communicate)". The operator also asked for a skeleton "with architecture that is flexible and able to scale".

The same day, the operator gave four decisions:
- the audit receives evidence only from build (OD1);
- the command surface is `evolve msg` for agents and `evolve events watch` for operators (OD2);
- an idle agy agent gets a fixed-text nudge, and Claude gets hooks (OD3);
- the first lane builds the documents, the core and the ports (OD4).

## Decision

1. **A general protocol.** The envelope is `msg/1.0`. An address is `[cycle:<N>/]<selector>`, and it can name any cycle. A relative address takes the sender's cycle.
2. **A scoped policy.** An ordered rule list in `policy.json` `messaging.rules` decides each recipient. A deny wins, and the default denies.
   - The defaults allow `same_cycle`, deny free text from build and tdd to audit, and give a reply right while a request lives.
3. **One service for each cycle.** The cycle run hosts it, on the normal path and on a resume. It holds only its own cycle and its own channel.
   - Cross-cycle delivery is thus impossible by construction, as well as by the rule.
4. **Two identity checks prove the dispatch.** The sandbox lets a dispatch write only its own outbox. A per-dispatch HMAC token covers a pane with no sandbox.
   - The token holds the canonical caveats, and a holder can add only an earlier expiry.
   - The roster entry of the dispatch states its agent, phase and role. The entry must be `live`, and the token must agree with it. The broker stamps `from`.
   - The roster lives in the memory of the kernel, never in `runs/`. Its entries are `planned`, `live` or `past`, and only `dispatch:` selects a `past` entry.
5. **The scope check comes first.** It runs before any lookup. An out-of-scope target or lookup gives "not found".
6. **The store is the ADR-0127 log.** One channel `msg.<cycle>`, under the protected `/.evolve/events/`. The store port reports its gaps, and the broker never hides one.
   - The retro reads the protected store, and the seal writes the archive hash into the ledger. gc removes the channel.
7. **Delivery rungs, each a push.** The prompt section at dispatch is the main path. Claude hooks, the agy nudge and `evolve msg read|wait` serve live agents.
   - The transport is drafts in the outbox (`send`, `ack`). Each rung ends in a read or an ack.
8. **A message is untrusted data.** A frame shows its provenance and states that it has no authority. No message text is ever typed into a pane.
9. **Limits from config:** a body cap, a sender rate, a repeat drop and a queue cap. A hop limit and a cycle budget follow ADR-0126.
10. **At-least-once delivery.** The id is the idempotency key. Each recipient has a cursor. An expired message goes to `msg.dead.<cycle>`.
11. **Observe the traffic.** `msg.sent`, `msg.denied` and `msg.dead` carry metadata only, never the body.
12. **The names** are "mailbox" and "outbox". "Inbox" stays with ADR-0007 and the bridge.

## Alternatives considered

| Alternative | Why not |
|---|---|
| A broker daemon on a Unix socket, with peer credentials and tokens | It duplicates the log, adds a daemon to supervise, and its peer pid races with pid reuse. |
| The native features of Claude: agent teams, `SendMessage`, MCP channels | They are previews, the OS user sets their scope, they do not cover agy, and they cannot enforce audit independence. |
| Each agent appends signed envelopes to a shared log | The agent then writes the protected store, and the signature key leaks to the agent. |
| One broker for the host | A host-wide process outlives its cycles and needs a supervisor. A cycle is the natural unit of order and access. |
| A2A, ANP or MCP as the wire protocol | They solve discovery and trust across a network. The envelope keeps the A2A fields, so a later bridge is cheap. |
| Delivery by `tmux send-keys` | Typed text gets the authority of the user and has no provenance. |

## Consequences

- **Agents can talk inside a cycle.** A phase can send a question, a handoff or a finding to another phase. A later phase gets it in its prompt.
- **Audit independence becomes a policy rule** in code and config, not a prompt rule.
- **New surfaces:** the `evolve msg` verbs, the `messaging` block of `policy.json`, the env key `EVOLVE_MSG_TOKEN`, the module `msg` and its three kinds.
- **New paths:** `.evolve/events/ch/msg.<cycle>/`, `.evolve/events/ch/msg.dead.<cycle>/` and `.evolve/msg/`. The `.evolve/*` rule of `.gitignore` already ignores them.
- **The sandbox profile** grows one write grant for the own outbox and denies for the store and other mailboxes (M6).
- **Accepted limits:**
  - an idle agy agent gets only a nudge;
  - a token in the environment is readable by the same uid;
  - the reply grants are in memory.

  The spec §Limits holds the full list.
