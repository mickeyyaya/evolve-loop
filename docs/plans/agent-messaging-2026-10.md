# Plan: cycle agent messaging, a general protocol with a policy scoped to one cycle

- **Status:** design and skeleton, opened 2026-10-09 in the lane `cl-agentmsg` (branch `feat/agentmsg-skeleton`, base `origin/main` at `99b9c896d`). The operator gave decisions OD1 to OD4 on 2026-10-09. This lane delivers M0 (the documents) and M1 (the skeleton).
- **Spec of record:** [agent-messaging.md](../architecture/agent-messaging.md). It holds the protocol: the envelope, the addresses, the policy, the identity, the flow and the delivery. It also holds the limits, the exit codes and the threat model. This plan holds the decisions, the components and the tests.
- **Decision record:** [ADR-0129](../architecture/adr/0129-cycle-scoped-agent-messaging.md).
- **Research:** [agent-messaging-2026-10.md](../research/agent-messaging-2026-10.md) (findings F1.1 to F7.5, refinements R1 to R24).
- **Substrate:** [event-channels.md](../architecture/event-channels.md) ([ADR-0127](../architecture/adr/0127-push-only-event-channels.md)).

## Table of contents

1. [Request](#1-request)
2. [Facts: the seams](#2-facts-the-seams)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Operator decisions](#4-operator-decisions)
5. [Candidate designs](#5-candidate-designs)
6. [Decisions](#6-decisions)
7. [The skeleton](#7-the-skeleton)
8. [The command surface](#8-the-command-surface)
9. [Wiring](#9-wiring)
10. [Components](#10-components)
11. [TDD protocol](#11-tdd-protocol)
12. [Verification](#12-verification)
13. [Patterns and forces](#13-patterns-and-forces)
14. [Risks and limits](#14-risks-and-limits)
15. [Open questions](#15-open-questions)
16. [Status](#16-status)

## 1. Request

The operator wrote this on 2026-10-09:

1. *"Could we research and design a way allowing each phase agent send message to any other agents within the same cycle. (The protocol should allow any messaging, but our system should apply the permission only allowing the agent within the same cycle to communicate), ultrathink for the research and design, start with the skeleton with architecture that is flexible and able to scale"*
2. *"Make sure all the research and plan and decision are properly written in details and strictly following the rules and policy defined under docs/"*

The requirements that follow from the request:

- **R-A.** Any phase agent can send a message to any other agent.
- **R-B.** The protocol is general: any sender, any address, any message kind.
- **R-C.** The policy of this system permits only traffic inside one cycle. The protocol does not hard-code that rule.
- **R-D.** The architecture is flexible and scales: fleet lanes, many cycles at once, more phases and more CLIs.
- **R-E.** The standing rules apply: push and never poll, one command surface, config and not flags, Go only, 100% coverage, zero comments, and STE documents.

## 2. Facts: the seams

| Fact | Evidence |
|---|---|
| The kernel dispatches one phase at a time; most recipients are not alive when a message is sent | `go/internal/core/cyclerun_dispatch.go:45`; research F1.1 |
| The evaluate batch runs phases at the same time, bounded by `ParallelEvaluateConcurrency` | `go/internal/core/evaluate_batch.go:12`, `:72`, `:86`; research F1.2 |
| A fleet lane runs each cycle in its own process | `go/cmd/evolve/cmd_fleet.go:127`; research F1.3 |
| Each dispatch has an id that names the run, the cycle and the agent | `go/internal/bridge/dispatch_reap.go:39`; `go/internal/proctree/dispatchid.go:20`; research F1.4 |
| The bridge exports `EVOLVE_DISPATCH_ID` into the pane and the process | `dispatch_reap.go:44`, `:51`; `go/internal/ipcenv/ipcenv.go:25` |
| All panes share one uid; no code reads a peer credential | research F1.5 |
| The sandbox denies writes under the repository root, then allows the worktree, the workspace and `/tmp` | `go/internal/adapters/sandbox/sandbox.go:181`; `go/internal/bridge/sandbox_wrap.go:185` |
| Phases that write no source run with no sandbox | `sandbox_wrap.go:225`; research F1.6 |
| ADR-0127 channel names accept any dotted name; a record holds a signal or a gap | `go/internal/events/channel/channel.go:39`; `record.go:43`; research F1.7 |
| `/.evolve/events/` is protected surface | `go/internal/guards/integrity_surface.go:18`; research F1.8 |
| The bridge inbox polls and reaches only a live tmux agent; "inbox" names two other things | `go/internal/bridge/inbox/inbox.go:50`, `:75`; `go/internal/bridge/tmux_inject.go:37`, `:81`; research F1.9 |
| The prompt has a cache-stable prefix before `cycleContextBoundary` | `go/internal/phases/runner/runner.go:260`; research F1.10 |
| The cycle run owns a cleanup stack, on the normal path and on a resume | `go/internal/core/orchestrator.go:722`; `go/internal/core/cyclerun.go:360`, `:533`; `go/internal/core/resume.go:248`, `:289` |
| The project hooks of Claude Code are `PreToolUse` and `PostToolUse`; no agy hook support exists; a headless driver cannot take input in a turn | `.claude/settings.json`; `go/internal/bridge/channel/supervisor.go:21`; research F1.12 |

## 3. Goals and non-goals

Goals:

- **G1.** An agent sends a message to any agent of its cycle: one dispatch, a phase, a role or all.
- **G2.** A message reaches a recipient that starts later, a busy recipient and an idle recipient. No rung polls.
- **G3.** A message never crosses to another cycle: the policy denies it, and the design cannot deliver it.
- **G4.** The broker proves the sender. A sender cannot claim another identity.
- **G5.** The audit stays independent: free text from build or tdd never reaches the auditor.
- **G6.** A conversation stops: caps on size, rate, repeats, queue, hops and the cycle total.
- **G7.** Every send, refusal and dead letter is on the event log, with metadata only.
- **G8.** Each part is a port with an adapter, so a new CLI, a new store or a federation adds an adapter and changes no core.

The spec holds the non-goals ([agent-messaging.md](../architecture/agent-messaging.md) §16 and §Limits). They are:

- no delivery across cycles now (a federation adapter is a later seam);
- no network transport and no daemon;
- no authority transfer through a message;
- no exactly-once delivery;
- no message that starts an agent.

The plan adds one non-goal: no feature flag and no env flag. The config is the `messaging` block of `policy.json`.

## 4. Operator decisions

| # | Decision | Source |
|---|---|---|
| OD1 | Audit independence: evidence only. The policy denies free text from build to audit. Build messages reach the auditor only as quoted evidence that is attached to the build record. | operator, 2026-10-09 |
| OD2 | The command surface: `evolve msg send\|read\|wait\|reply` for agents. Operators watch the same records with `evolve events watch --channel msg.<cycle>`, through the one filter grammar. | operator, 2026-10-09 |
| OD3 | An idle agy agent gets a fixed-text nudge through the bridge: "N messages wait: run evolve msg read". The bridge never types message text. Claude gets a `PostToolUse` hook while it works and an async wake hook while it is idle. | operator, 2026-10-09 |
| OD4 | The first lane builds the document set, the core and the ports. The real store is a later component. | operator, 2026-10-09 |

The decisions supersede the console draft v0 where they differ. The main change: an outbox for each dispatch, not a socket.

## 5. Candidate designs

### 5.1 A: an outbox for each dispatch, which the sandbox enforces (the winner)

Each dispatch writes drafts only to its own outbox. The sandbox enforces this. A service for each cycle runs outside the sandbox. The kernel wakes it, and it appends each accepted message to the channel `msg.<cycle>`.

**Why it wins:**
- The location identity comes from the kernel sandbox, not from a pid or a secret alone (research §4).
- It reuses ADR-0127: the log, the lock, the cursors and the wake (research F1.7).
- It covers the main case, a recipient that starts later, with a prompt section and no live part (research F1.1, F5.6).
- It is neutral to the CLI. Only the live rung differs for each CLI (research F5.2 to F5.4).

### 5.2 B: a broker daemon on a Unix socket, with peer credentials and tokens

One broker binds a socket. Agents connect through `evolve msg`. The broker checks `getpeereid` and `LOCAL_PEERPID` and a token.

**Steelman.** It has the lowest latency for live peers, and the socket is a natural push channel. It is close to the cross-session design of Claude Code (research F2.15).

**Rejected:**
- It duplicates the log: it needs its own durability, order and cursors, or it writes to the log anyway.
- A daemon is a new failure point to supervise.
- The peer pid races: the system can use a pid again, and macOS cannot attribute an exited peer (research F4.2).
- The sandbox must allow the socket, so every sandboxed agent can reach it, and the token is the only check.

### 5.3 C: the native features of Claude (agent teams, `SendMessage`, MCP channels)

**Steelman.** Almost no code for messages between Claude agents, and the guards come with it (research F2.12 to F2.16).

**Rejected:**
- Agent teams and channels are previews (research F2.11).
- The OS user sets the scope, not the cycle (research F2.15).
- They do not cover agy, so they break "any CLI, any phase".
- They cannot enforce audit independence, because the policy is outside our control.

### 5.4 D: each agent appends signed envelopes to a shared log

**Steelman.** No service: each agent signs and appends, and each reader checks the signature.

**Rejected:**
- The agent then writes the protected store, which ADR-0127 and ADR-0064 forbid (research F1.8).
- The signature key is in the agent's process, so the key leaks to the agent (research F6.3).

### 5.5 Scores

| Force | A: outbox (winner) | B: socket broker | C: Claude features | D: shared log |
|---|---|---|---|---|
| Correct boundary of the cycle | yes: one service for each cycle, a location and a token | yes, with a scope check in the broker | no: the OS user is the scope | no: the agent writes the store |
| Push only | yes: the kernel wakes the service and the readers | yes | yes | yes |
| Reuse | the ADR-0127 log and wake | a second transport and store | vendor code | the log, but with keys in agents |
| Operations cost | none: it lives in the cycle run | a daemon to supervise | a preview to track | none |
| CLI coverage | every CLI; only the live rung differs | every CLI | Claude only | every CLI |
| Audit independence | a policy rule | a policy rule | not enforceable | not enforceable |

## 6. Decisions

| # | Decision | Reason | Research |
|---|---|---|---|
| D1 | The protocol is general, and the policy is scoped. The grammar `[cycle:<N>/]<selector>` accepts any cycle. A relative address resolves to the sender's cycle. The policy refuses a cross-cycle address. | R-B and R-C: the operator asked for a general protocol and a scoped permission. | R1, R2 |
| D2 | One service for each cycle, hosted in the cycle run: `RunCycle` → `newCycleRun` cleanup stack, and `RunCycleFromPhase` for a resume. Its HMAC key is only in memory. It appends only to `msg.<cycle>`. | Cross-cycle delivery is then impossible by construction, as well as by the rule. A federation adapter is a later seam. | R3 |
| D3 | Two checks prove the dispatch: the outbox location, which the sandbox enforces, and a token `EVOLVE_MSG_TOKEN`. The token holds the canonical caveats cycle, dispatch, expiry, agent, phase and role. A holder can add only an earlier expiry. The roster entry of the dispatch states its agent, phase and role. The broker stamps `from`. | Peer credentials cannot tell agents apart under one uid. The token covers the panes with no sandbox. A token attribute that the holder can add must never decide a policy. | R4, R5, R6 |
| D4 | Authorization is an ordered rule list from `policy.json` `messaging.rules`. A deny wins, and the default denies. The compiled defaults: allow `same_cycle`; deny free text from build and tdd to audit (OD1); a reply right while a request lives. | Config, not code; the NATS model. | R7, R8, R9 |
| D5 | The scope check runs before any lookup. An out-of-scope target or lookup gives "not found" (`ErrNotFound`), the same as an unknown recipient. | A2A §13.1: never show that an out-of-scope resource exists. | R10 |
| D6 | The envelope `msg/1.0` has these fields: `id` (a ULID and the idempotency key), `ts`, `kind` (an open registry) and `from` (stamped). It also has `to`, `subject`, `body` (capped), `refs`, `reply_to`, `correlation_id`, `expires_at`, `hop` and `context_id` (the cycle). | A2A fields, with no A2A transport. | R11, R12 |
| D7 | The store is the ADR-0127 channel log: one channel `msg.<cycle>` for each cycle, and a new `message` record variant. The dead letters of a cycle go to `msg.dead.<cycle>`. The store is under the protected `/.evolve/events/`. The seal archives it, and gc removes it. Each recipient has its own cursor. The skeleton defines only the `Store` port and an in-memory adapter. | One log format; a durable store; one writer. | R13, R14 |
| D8 | Delivery adapters sit behind a `Deliverer` port. The main path is the prompt section after `cycleContextBoundary`. The live rungs are Claude `PostToolUse` and `asyncRewake`, the agy fixed-text nudge (OD3) and `evolve msg read\|wait`, which the kernel wakes. | Most recipients start later. Each live rung is a push. | R15, R16, R17 |
| D9 | A message is untrusted data. The recipient sees it in a frame with its provenance and the statement that it has no authority. Never type message text into a pane. | Prompt infection and hijack studies; `send-keys` gives user authority. | R18 |
| D10 | Config gives the limits: a body cap, a sender rate, a repeat drop and a queue cap of 50 for each recipient. A hop limit and a cycle budget tie it to ADR-0126. | The guards of Claude Code; a conversation converges or stops. | R19, R20 |
| D11 | Delivery is at least once: the ULID, a cursor for each recipient and a seen set. A request carries a `correlation_id` and a deadline. An expired or undeliverable message goes to `msg.dead.<cycle>`. | The Idempotent Consumer, the Correlation Identifier and the Dead Letter Channel. | R21, R22 |
| D12 | A send to a recipient outside the cycle roster (past, live or planned dispatches) is refused at send time. | Never store a message that can never be delivered. | R23 |
| D13 | The command surface is OD2. Its exit codes follow the style of `evolve events`. | One surface for agents; one filter grammar for operators. | — |
| D14 | Observability: the events `msg.sent`, `msg.denied` and `msg.dead` carry metadata only, never the body. `msg.denied` is WARN and goes to the `errors` channel. | An audit log makes misuse visible; a body in a log leaks it. | R24 |
| D15 | The names are "mailbox" and "outbox", never "inbox". | The operator inbox (ADR-0007) and the bridge inbox use that name. | research F1.9 |

More decisions that the skeleton made, each with its reason:

- **D3a** `Send` and `Read` take an `identity.Credential`, not a `Principal`. Authentication is the first step of the send pipeline, so the service owns it.
- **D4a** A deny rule also wins over the reply right. Thus build answers an auditor's request with `evidence`, never with free text (OD1).
- **D10a** A failed store append counts nothing: the broker records the rate, the repeat digest and the queue only after the append.
- **D3b** (fix round 1) The token authority mints only for a full principal, with an expiry after now and at most `token_ttl_s` later. A credential with no location is refused.
- **D6a** (fix round 1) A repeat of an `id` with other content is `ErrIDConflict`, not the first receipt.
- **D10b** (fix round 1) A read refuses a cursor past the head (`ErrCursor`). The queue of a reader moves only to the next cursor that a read returns.
- **D10c** (fix round 1) The service collects its events and notifies them after it releases its lock. The store append stays inside the lock, because the idempotency check, the limits and the append must be one step.
- **D3c** (fix round 2) The roster entry has a state: `planned`, `live` or `past`. Only a `live` entry can send, read or ack. A token whose agent, phase or role differs from the entry is refused, never ignored. The roster comes from the memory of the kernel, never from `runs/`.
- **D7a** (fix round 2) `Batch.Gaps` carries the gaps of the store. A read reports each one in the mailbox with a `msg.dead` event (reason `gap`), so the M2 adapter needs no change to the core.
- **D7b** (M2) A `message` record line has its own limit, `MaxMessageBytes`, of 65,536 bytes. The signal limit of 4,608 bytes is below the default body cap of 8,192 bytes. A body of `<` escapes to six bytes for each byte, which is 49,152 bytes. The log keeps `message` as raw JSON, so `internal/events/channel` imports nothing of `agentmsg`.
- **D8a** (fix round 2) The transport is drafts in the outbox: `send` and `ack{cursor}`. The token is a field of the draft, and the service deletes the draft after ingest. `Service.Ack` gives each delivery rung an ack path. For the prompt section, the kernel reads the queue with a credential that it mints itself.
- **D12b** (fix round 2) `role:`, `phase:` and `*` skip a `past` entry. Only an exact `dispatch:` address selects it. Thus a retried phase gets one copy.
- **D3d** (kept) A holder can add an earlier expiry to a token. The consumer is the M5 `asyncRewake` hook: the pane can give the hook a token with a shorter life. The rule is small, and a test pins it.
- **D12a** A recipient that two addresses select gets one copy. `*` skips the sender, and an address that selects only the sender is an unknown recipient.

## 7. The skeleton

M1 is unwired: nothing outside `go/internal/agentmsg` imports it. The packages and their exported API:

| Package | Exported API |
|---|---|
| `agentmsg` | `Config{Rules, Kinds, BodyBytes, SubjectBytes, SenderPerMinute, RepeatWindowS, RecipientQueue, MaxHops, CycleBudget, TokenTTLS}`, `(*Config).Resolve() (Settings, error)`, `Settings{Policy, Envelope, Limits, TokenTTL}`, `ErrConfig` |
| `agentmsg/address` | `Address{Cycle, Selector, Value}`, `Selector` and `SelectRole`, `SelectPhase`, `SelectDispatch`, `SelectAll`, `Parse`, `(Address).String`, `Relative`, `Resolve`, `MarshalText`, `UnmarshalText`, `MaxValueBytes`, `ErrSyntax` |
| `agentmsg/envelope` | `Envelope`, `Kind` and the seven kinds, `DefaultKinds`, `Limits{BodyBytes, SubjectBytes, Kinds}`, `(Envelope).Validate`, `IDSource{Now, Entropy}`, `(IDSource).New`, `ValidID`, `SchemaVersion`, `ErrInvalid`, `ErrID` |
| `agentmsg/identity` | `Principal{Cycle, Dispatch, Agent, Phase, Role}`, `Credential{Token, Location}`, `Authenticator`, `Authority`, `NewAuthority`, `(*Authority).Mint`, `(*Authority).Authenticate`, `MinKeyBytes`, `ErrUnauthenticated`, `ErrKey`, `ErrPrincipal`, `ErrConfig`, `ErrExpiry`. `NewAuthority(key, now, maxTTL)` |
| `agentmsg/policy` | `Rule{Name, Effect, When}`, `Match`, `Effect`, `Scope`, `Request`, `Decision{Allow, Rule, Reason}`, `Authorizer`, `Engine`, `Compile`, `Defaults`, `DefaultRule`, `ErrRule` |
| `agentmsg/broker` | the service: `Service`, `New`, `(*Service).Send`, `(*Service).Read`, `(*Service).Ack`, `Config`, `Limits`, `Receipt`, `Refusal` and `Mailbox`. The ports: `Store`, `Roster`, `Notifier`, `Record`, `Batch`, `Gap`, `Dead`, `Member`, `MemberState` (`MemberPlanned`, `MemberLive`, `MemberPast`), `Event` and `EventKind`. The rule and reason names. The errors: `ErrConfig`, `ErrDenied`, `ErrUnroutable`, `ErrUnknownRecipient`, `ErrNotFound`, `ErrCursor`, `ErrIDInUse`, `ErrIDConflict`, `ErrHopLimit`, `ErrBudget`, `ErrRateLimited`, `ErrRepeat` and `ErrQueueFull`. |
| `agentmsg/memstore` | `Store`, `New`, `Append`, `Read`, `DeadLetter`, `Dead`, `ErrCursor` |
| `agentmsg/delivery` | `Deliverer`, `Frame`, `Section`, `(*Section).Deliver`, `(*Section).Text` |

- **The dependency direction.**
  - `address` and `identity` are leaves, and `envelope` imports them.
  - `policy` imports `envelope` and `identity`, and `broker` imports the four.
  - `memstore` and `delivery` are adapters.
  - The root `agentmsg` imports `broker`, `envelope` and `policy`.
- **Two packages that the brief allowed to merge stay apart.** `memstore` is an adapter, and the hexagonal rule keeps an adapter out of the core. `delivery` holds the frame, which the M5 adapters share.
- **The token is in `identity`.** It is small: one file with the chain, the caveats and the check.

## 8. The command surface

The spec of record is [agent-messaging.md](../architecture/agent-messaging.md) §11: the verbs, the flags, the exit codes and the output form. This plan does not repeat it.

## 9. Wiring

Component M7 wires the service. The seams:

| Seam | Change |
|---|---|
| `Orchestrator.RunCycle` → `newCycleRun` (`go/internal/core/cyclerun.go:360`) | start the service and push its stop on the cleanup stack |
| `RunCycleFromPhase` (`go/internal/core/resume.go:248`) | start the service on a resume; the store keeps the messages |
| the bridge boot (`go/internal/bridge/dispatch_reap.go:44`) | export `EVOLVE_MSG_TOKEN` beside `EVOLVE_DISPATCH_ID`; add the key to `ipcenv.ProtocolKeys()` |
| the runner (`go/internal/phases/runner/runner.go:260`) | render the prompt section after `cycleContextBoundary` |
| the sandbox (`go/internal/bridge/sandbox_wrap.go:185`) | allow writes to the own outbox; deny the mailboxes of others and the store (M6) |
| `policy.json` `messaging` | `internal/policy` decodes it strictly and calls `agentmsg.Config.Resolve` |
| the Signal Center | the `Notifier` adapter emits `msg.sent`, `msg.denied` and `msg.dead` |
| the protected surface (`go/internal/guards/integrity_surface.go:18`) | add `/.evolve/msg/` |

## 10. Components

Each component is small and lands unwired first. Each new package is at 100 in `go/.cover-strict` and listed in `go/.apicover-enforce`.

| # | Component | Reuses |
|---|---|---|
| M0 | The research dossier, this plan, the spec and ADR-0129 (this lane) | the research lane |
| M1 | The skeleton (this lane): the envelope, the address grammar, the identity and the token, and the policy engine. Also the broker core, the ports, the in-memory store, the frame and the config struct. | the standard library only |
| M2 | The channel-log `Store` adapter and the `message` record variant of ADR-0127 | `internal/events/channel` |
| M3 | The outbox watcher, the location identity, and the token mint at boot | `internal/events/wake`, `atomicwrite` |
| M4 | The `evolve msg` CLI | the dispatch style of `cmd_wave.go` |
| M5 | The deliverers: the prompt section, the Claude hooks and the agy nudge | `delivery.Frame`, `bridge/inbox` (fixed text only) |
| M6 | The sandbox rules. Write only the own outbox, and deny the store. Deny `file-read*` on the `.evolve/msg/` tree of other cycles and on the mailboxes of other dispatches. | `internal/adapters/sandbox` |
| M7 | The wiring into `RunCycle` and `RunCycleFromPhase`, and the roster of the kernel with its states. Also the policy block, the signals, and at the seal the archive hash in the ledger and the dead letters. | §9 |
| M8 | A live check in a wave | the wave tools |

- Each component is its own lane and pull request, merged at a wave boundary.
- Each one follows the usual chain: red tests first, then the simplifier, then the Go and architecture reviewers, then the full floor.
- M1 to M6 ship unwired. M7 and M8 change live behavior.
- **The order of M6 and M7.** M6 comes after the fix of the bug `sandbox-cross-cycle-run-dir-write`. M7 is blocked until M6 and that fix land, because the location identity needs a sandbox that confines writes to the cycle.

## 11. TDD protocol

The rules for each component:

1. Write one test for one acceptance criterion. Run it, and see it fail on its named assertion. Keep the red output in the lane scratchpad.
2. Write the smallest code that passes it.
3. Run mutants of the core in a scratch overlay (`go test -overlay`), and see a test fail for each one.
4. Unit tests fake the clock, the entropy, the roster and the notifier. No test sleeps.
5. New packages reach 100% line and API coverage.
6. Run `go test -count=1 -race` and the full floor before the review.

The M1 tests for each decision:

| Decision | Tests |
|---|---|
| D1 | `TestParse_EveryProductionOfTheGrammar`, `TestParse_RefusesEveryMalformedAddress`, `TestResolve_ARelativeAddressTakesTheSenderCycle`, `TestResolve_AnAbsoluteAddressKeepsItsCycle`, `TestSend_ARelativeAddressResolvesToTheSenderCycle`, `TestDefaults_DenyARecipientInAnotherCycle` |
| D2 | `TestSend_ACrossCycleAddressThatThePolicyAllowsIsStillUnroutable`, `TestSend_APrincipalOfAnotherCycleIsRefused`, `TestRead_APrincipalOfAnotherCycleReadsNothing` |
| D3 | `TestAuthority_AMintedTokenAuthenticatesItsPrincipal`, `TestAuthority_TheLocationAndTheTokenMustNameOneDispatch`, `TestAuthority_ATokenFromAnotherKeyIsRefused`, `TestAuthority_AChangedCaveatIsRefused`, `TestAuthority_AnAddedCaveatOtherThanAnEarlierExpiryIsRefused`, `TestAuthority_ATokenExpiresAtItsDeadline`, `TestAuthority_RefusesATokenThatIsNotTheCanonicalSet`, `TestSend_TheBrokerStampsTheHeaderAndIgnoresTheSenderValues`, `TestSend_AnUnauthenticatedSenderIsRefusedAndRecorded`, `TestSend_AnOutboxOfAnotherDispatchIsRefused`, `TestAuthority_APhaseAddedToAPhaseLessTokenIsRefused`, `TestAuthority_AnAddedEarlierExpiryNarrowsTheToken`, `TestAuthority_ACredentialWithoutALocationIsRefused`, `TestMint_RefusesAPrincipalWithoutTheFullIdentity`, `TestMint_RefusesAnExpiryOutsideTheLifetime`, `TestNewAuthority_RefusesABrokenConfig`, `TestSend_ATokenWhoseIdentityDiffersFromTheRosterIsRefused`, `TestSend_ADispatchOutsideTheRosterIsUnauthenticated`, `TestSend_APrincipalOfAnotherCycleOnAWrongRosterIsRefused`, `TestSend_APastDispatchCannotSendOrRead` |
| D4 | `TestDefaults_AllowEveryKindInsideTheCycle`, `TestDefaults_DenyFreeTextFromBuildAndTDDToAudit`, `TestDefaults_AReplyGrantNeverOverridesADeny`, `TestEngine_ADenyWinsOverAnEarlierAllow`, `TestEngine_TheFirstMatchingAllowNamesTheDecision`, `TestEngine_NoMatchingRuleDenies`, `TestEngine_EachConditionNarrowsTheMatch`, `TestSend_BuildCannotSendFreeTextToAudit`, `TestSend_BuildEvidenceReachesAudit`, `TestSend_ABroadcastSkipsTheRecipientsThatThePolicyDenies`, `TestSend_ARequestGrantsItsRecipientsARightToReplyUntilItExpires`, `TestSend_ARequestToAPlannedPhaseGrantsItsLaterDispatch` |
| D5 | `TestSend_ACrossCycleAddressIsDeniedBeforeAnyRosterLookup`, `TestSend_ARecipientOutsideTheRosterIsRefused`, `TestSend_AReplyToAMessageOutsideTheSenderScopeIsNotFound`, `TestRead_OnlyARecipientSeesAMessage` |
| D6 | `TestValidate_RefusesEachBrokenRule`, `TestValidate_ARequestCarriesACorrelationAndADeadline`, `TestValidate_AcceptsAKindThatTheConfigAdds`, `TestIDSource_EncodesTheTimeAndTheEntropyInCrockfordBase32`, `TestIDSource_IDsSortByTime`, `TestSend_MintsAnIDWhenTheSenderGivesNone`, `TestSend_TheSameIDIsAcceptedOnce`, `TestSend_AnIDOfAnotherSenderIsRefused`, `TestSend_AnInvalidEnvelopeIsRefused`, `TestSend_TheSameIDWithOtherContentIsAConflict` |
| D7 | `TestStore_EachCycleHasItsOwnIncreasingCursors`, `TestStore_ReadFromACursorReturnsTheRestAndTheNextCursor`, `TestStore_KeepsItsOwnCopyOfEachRecord`, `TestRead_StartsAtTheCursorAndGivesTheNext`, `TestRead_AGapFromTheStoreIsReportedAndNeverSilent` |
| D8 | `TestSection_CollectsTheFramedMessagesOfEachDelivery` (the prompt-section adapter), `TestAck_ClearsTheQueueWithoutARead`, `TestAck_RefusesACursorOutsideTheLog`, `TestAck_AStoreFailureIsAnError` |
| D9 | `TestFrame_StatesTheProvenanceAndThatAMessageHasNoAuthority`, `TestFrame_EscapesTextThatCouldCloseTheBlock` |
| D10 | `TestSend_TheHopLimitStopsAReplyChain`, `TestSend_TheCycleBudgetCapsAcceptedMessages`, `TestSend_TheSenderRateIsCappedForEachMinute`, `TestSend_AnIdenticalRepeatInsideTheWindowIsDropped`, `TestSend_AFullRecipientQueueRefusesTheRecipientUntilItReads`, `TestSend_AStoreFailureIsReturnedAndNothingIsCounted`, `TestSend_ANotifierThatCallsTheServiceDoesNotDeadlock`, `TestRead_ANegativeCursorIsRefusedBeforeTheStore`, `TestNew_NamesTheFirstMissingFieldEveryTime`, `TestResolve_AnAbsentBlockGivesTheCompiledDefaults`, `TestResolve_RefusesAValueOutOfRange` |
| D11 | `TestRead_ADuplicateRecordIsDeliveredOnce`, `TestRead_AnExpiredMessageGoesToTheDeadLetterOnce`, `TestSend_ASenderCanFollowUpItsOwnMessage` |
| D12 | `TestSend_ARecipientOutsideTheRosterIsRefused`, `TestSend_APlannedPhaseAndALiveDispatchAreValidRecipients`, `TestSend_APhaseAddressSkipsAPastRetrySoTheRetryGetsOneCopy`, `TestSend_APlannedEntryGetsQueuedMessagesButCannotSend`, `TestSend_ARecipientNamedTwiceGetsOneCopy`, `TestSend_ASenderIsNeverItsOwnRecipient` |
| D14 | `TestNotify_EventsCarryMetadataAndNeverTheBody` |
| D4, D10 config | `TestResolve_ConfiguredRulesReplaceTheDefaults`, `TestResolve_EachKeyOverridesItsDefault` |

D13 (the CLI) and D15 (the names) have no M1 test. M4 owns the CLI, and D15 is a naming rule for the documents and the code.

The red tests of the later components, named by their acceptance criteria:

| # | Red tests |
|---|---|
| M2 | `TestChannelStore_AppendsAMessageRecordToTheCycleChannel`, `TestRecord_HoldsExactlyOneOfSignalGapAndMessage`, `TestChannelStore_ReadFollowsRotation`, `TestChannelStore_DeadLettersGoToMsgDead` |
| M3 | `TestOutbox_AKernelWakeIngestsADraft`, `TestOutbox_ThePathNamesTheDispatch`, `TestOutbox_AHalfWrittenDraftIsNeverRead`, `TestOutbox_TheReceiptLandsInTheMailbox`, `TestBoot_ExportsTheMessageToken`, `TestProtocolKeys_HoldTheMessageToken` |
| M4 | `TestMsgCLI_ExitCodesMatchTheTable`, `TestMsgSend_ReadsTheBodyFromStdin`, `TestMsgSend_NoReceiptByTheDeadlineExitsTwo`, `TestMsgWait_WakesOnTheMailboxWithoutAPoll`, `TestMsgReply_OutOfScopeExitsFour` |
| M5 | `TestRunner_PutsTheMessagesAfterTheCycleContextBoundary`, `TestHookPostToolUse_PrintsAdditionalContext`, `TestHookRewake_ExitsTwoWithTheFrame`, `TestAgyNudge_IsFixedTextWithTheCount`, `TestAuditRunner_AttachesEvidenceToTheBuildRecord` |
| M6 | `TestSBPL_AllowsOnlyTheOwnOutbox`, `TestSBPL_DeniesTheStoreAndOtherMailboxes`, `TestSBPL_DeniesReadsOfOtherMailboxesAndOtherCycles`, `TestSandbox_ARealProcessCannotWriteAnotherOutboxOfItsCycleOrOfAnotherCycle` |
| M7 | `TestRoster_HoldsAtMostOneEntryThatIsNotPastForEachPhase`, `TestRoster_APlannedEntryBecomesLiveInPlace`, `TestSeal_DeadLettersTheQueuesOfPastEntries`, `TestSeal_WritesTheArchiveHashToTheLedger`, `TestRunCycle_StartsAndStopsTheService`, `TestRunCycleFromPhase_StartsTheService`, `TestPolicyMessaging_DecodesStrictly`, `TestNotifier_EmitsTheThreeKinds`, `TestSeal_DeadLettersTheMessagesOfPhasesThatDidNotRun`, `TestProtectedSurface_CoversTheMessageTree` |
| M8 | a live wave. Scout sends a `handoff` to build, and build sees it in its prompt. Build sends `evidence` to audit. A `note` from build to audit gets `msg.denied`. |

How "no poll" is proved:

- **The AST guard** `TestNoPollTimerInTheEventChannelSources` (`go/test/structure/nopoll_guard_test.go`) scans `internal/agentmsg` too, from M1. It bans `time.Sleep`, `time.Tick`, `time.NewTicker`, `time.After` and `time.NewTimer`.
- **M3 and M4** use the ADR-0127 `Waiter`: arm, catch up, wait.

## 12. Verification

- **The boundary of the cycle:**
  - a cross-cycle address is denied before any roster lookup;
  - a token of another cycle is refused;
  - a federation rule still gives "unroutable".
- **Identity:** a changed caveat, a token of another key, an added caveat that widens, an expired token and a location of another dispatch are refused. The broker stamps `from`.
- **Policy:**
  - a deny wins, and the default denies;
  - build cannot send free text to audit, and evidence reaches audit;
  - the reply right holds only for the requester, only for its correlation id and only before the deadline.
- **Limits:** the hop limit, the cycle budget, the sender rate, the repeat window and the queue cap refuse at their exact boundaries. A failed store append counts nothing.
- **Delivery:** a read returns only the reader's messages, from its cursor, once for each id. An expired message is a dead letter once.
- **Mutants (M1, fix round 2):** 3 controls and 71 mutants of the `policy`, `identity` and `broker` cores. A named test kills 70 mutants. One mutant is equivalent: `authenticate` returns the token principal instead of the roster entry, but the check has already made the two equal. The lane scratchpad keeps the table.
- **End to end (M8):** a real wave, with the events on the `errors` and `signals` channels.

## 13. Patterns and forces

| Pattern | Where | Force |
|---|---|---|
| Ports and adapters | `Authenticator`, `Authorizer`, `Store`, `Roster`, `Notifier`, `Deliverer` | The core is pure. A new store, CLI or federation is an adapter. |
| Specification | `policy.Match` | Rules compose by AND inside a rule and by order across rules. |
| Strategy | `Deliverer`: the prompt section, the hooks, the nudge | One interface for each delivery rung. |
| Transactional outbox | the outbox of each dispatch and the channel log | The store is the record, and the id is the idempotency key. |
| Idempotent consumer | the seen set and the idempotent send | At-least-once delivery with no duplicate effect. |
| Correlation identifier | `correlation_id` and the reply right | A reply finds its request. |
| Dead letter channel | `msg.dead.<cycle>` | An undeliverable message is never silent. |
| Account isolation (NATS) | one service for each cycle | Cross-cycle delivery is impossible by construction. |

Not abstracted:

- The kinds are data in config, not types.
- The default rules are data, not branches.
- No transport interface: the outbox is the only transport, and a socket is not planned.

## 14. Risks and limits

The limits of record are in the spec: [agent-messaging.md](../architecture/agent-messaging.md) §Limits. The main risks:

- **A sandbox profile error** lets an agent write another outbox. M6 adds a test that runs a real sandboxed process and proves the denial.
- **An `asyncRewake` hook that hangs or leaks.** M5 ties its life to the dispatch, and the dispatch tag lets gc reap it.
- **Injected text through a hook.** The frame, the caps and the typed kinds reduce it. The recipient still reads the text, so a message stays untrusted.
- **The token in the environment.** Another process of the same uid can read it. The location check is the boundary for a sandboxed agent.
- **A sandbox that lets an agent write another cycle's run directory** breaks the location identity. The bug `sandbox-cross-cycle-run-dir-write` must be fixed first, and M6 and M7 wait for it (§10).
- **An input from `runs/`.** An agent can write `runs/`, so no roster, archive or evidence input of messaging is read from there. The roster lives in the memory of the kernel.
- **The seal archive.** The retro reads `msg.<cycle>` from the protected store, and the seal writes the archive hash into the ledger. A copy in `runs/` is for people only.
- **The proof of M6** must cover the outbox of another dispatch in the same cycle and the outbox of a dispatch in another cycle.
- **The ledger never shrinks during a cycle** (review item 7, accepted). The maps of accepted ids, digests, grants and dead letters stay in memory until the service stops. The cycle budget bounds them.

## 15. Open questions

| # | Question | Recommendation |
|---|---|---|
| Q1 | The default cycle budget is 200 accepted messages, and the default hop limit is 4. The research gives no number for the budget. | Keep 200 and 4 in shadow. M8 measures the real use of a wave, and then the operator sets the values. |
| Q2 | A deny wins over the reply right, so build cannot answer an auditor's request with free text (D4a). | Keep it: build answers with `evidence`, which keeps OD1 strict. |
| Q3 | agy has no async wake hook (research F5.4). | Use the fixed-text nudge (OD3). Check each new agy release in the `evolve cli update` smoke test. |
| Q4 | The reply grants and the token key are in memory, so a restart of the service loses them. | Accept it: a resume mints new tokens for the new dispatches, and a lost grant needs only an allow rule. |
| Q5 | At the seal, a message to a phase that did not run goes to `msg.dead.<cycle>` (M7). | Confirm it at M7. |

## 16. Status

| # | Status |
|---|---|
| M0 | ☑ the research dossier, this plan, the spec and ADR-0129, in this lane. Merged in PR #845 on 2026-10-12. |
| M1 | ☑ `internal/agentmsg` and its seven subpackages: built, unwired, at 100% coverage, and in both gate files. The no-poll guard scans the tree. Fix rounds 1 and 2 closed the review items. 70 of 71 mutants killed, with 3 controls, and one is equivalent (§12). Package notes: [internal-agentmsg.md](../architecture/packages/internal-agentmsg.md). Merged in PR #845 on 2026-10-12. |
| M2 | ☑ `internal/agentmsg/channelstore`: the `Store` adapter on the channel log (`msg.<cycle>` and `msg.dead.<cycle>`). The `message` record variant has its own line limit (D7b). The package is built, unwired, at 100% coverage and in both gate files. The four red tests of §11 pass, and the real broker runs over the adapter, and a named test kills each of 43 mutants. Package notes: [internal-agentmsg-channelstore.md](../architecture/packages/internal-agentmsg-channelstore.md). Not merged. |
| M3 to M8 | ☐ not started |
