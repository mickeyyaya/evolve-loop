# internal/agentmsg

> The spec of record: [agent-messaging.md](../agent-messaging.md). The plan (component M1, decisions D1 to D15): [agent-messaging-2026-10.md](../../plans/agent-messaging-2026-10.md). The decision: [ADR-0129](../adr/0129-cycle-scoped-agent-messaging.md). The research: [agent-messaging-2026-10.md](../../research/agent-messaging-2026-10.md).

This page covers `internal/agentmsg` and its seven subpackages: `address`, `envelope`, `identity`, `policy`, `broker`, `memstore` and `delivery`. The eighth subpackage, `channelstore` (component M2), has its own page: [internal-agentmsg-channelstore.md](internal-agentmsg-channelstore.md).
This page covers `internal/agentmsg` and its nine subpackages: `address`, `envelope`, `identity`, `policy`, `broker`, `memstore`, `delivery`, `outbox` and `boot`.

## Purpose

`internal/agentmsg` is the skeleton of cycle agent messaging. It is a general protocol between the agents of one cycle, with a policy that keeps each message in its cycle. It has these parts:

| Package | Purpose |
|---|---|
| `agentmsg` | the `messaging` config block (`Config`) and its resolution to the settings of the parts (`Resolve`) |
| `agentmsg/address` | the address grammar `[cycle:<N>/]<selector>`: `Parse`, `String`, `Resolve` and the JSON text form |
| `agentmsg/envelope` | the `msg/1.0` envelope, the open kind registry, `Validate` and the ULID source |
| `agentmsg/identity` | the principal of a dispatch, the `Authenticator` port and the token authority (`Mint`, `Authenticate`) |
| `agentmsg/policy` | the `Authorizer` port, the rule engine (`Compile`, `Authorize`) and the default rules |
| `agentmsg/broker` | the service of one cycle (`Send`, `Read`), its ports (`Store`, `Roster`, `Notifier`) and its typed errors |
| `agentmsg/memstore` | the in-memory adapter of the `Store` port |
| `agentmsg/delivery` | the `Deliverer` port, the frame of a message (`Frame`) and the prompt-section adapter (`Section`) |
| `agentmsg/outbox` | the outbox watcher: `Layout`, `Watcher`, `Answer` and the ports `Sender` and `Waker` (component M3) |
| `agentmsg/boot` | the token mint at boot: `Env` and the port `Minter` (component M3) |

Nothing outside `internal/agentmsg` imports it yet. Component M2 added the channel store. Components M3 to M7 add the other adapters and wire it.

## Design

- **Ports and adapters.** The core has no I/O. Each boundary is a port in the package that uses it: `identity.Authenticator`, `policy.Authorizer`, `broker.Store`, `broker.Roster`, `broker.Notifier` and `delivery.Deliverer`. The tests use the real `identity.Authority`, the real `policy.Engine` and the real `memstore.Store`. Only the roster and the notifier are fakes.
- **The send pipeline** (`broker.Service.Send`) runs these steps in order: authenticate, idempotency, stamp, validate, limits, scope, resolve, authorize, append, record, notify. Each refusal is a typed error and a `msg.denied` event.
- **The service holds one cycle.** `Config.Cycle` is fixed at `New`. A principal of another cycle is unauthenticated. A cross-cycle address is refused before the roster is read: `ErrDenied` by the policy, else `ErrUnroutable`.
- **The broker stamps the header.** `schema`, `from`, `context_id`, `ts` and `hop` come from the broker, never from the sender. The sender's CLI can give the `id`, so a retry is idempotent.
- **One lock for each service.** A send and a read hold the lock of the service, with the store call. The idempotency check, the limits, the append and the ledger record must be one step. Else two sends with one id, or two at the rate limit, can both pass. A read keeps the dead-letter set and the queue in step with the store.
- **Events after the lock.** A call collects its events (`notices`) and notifies them after it releases the lock, so a notifier can call the service again. The roster call is outside the lock too.
- **The identity comes from the roster.** `authenticate` checks the token and the cycle, then finds the roster entry of the dispatch. The entry must be `live` and equal to the principal of the token, or the call is unauthenticated.
- **Roster states.** `Member` adds a `MemberState` to a principal. `selects` skips a `past` entry for `role:`, `phase:` and `*`. Only `dispatch:` selects it.
- **Gaps and acks.** `Batch.Gaps` carries the gaps of a store. `Read` puts each gap in every mailbox, and emits one `msg.dead` event for each distinct gap. `Ack` moves the queue of a reader to a cursor inside the log.
- **The store contract.** `Append` returns the cursor `c`. A later `Read(from)` with `from` at or below `c` holds the record, and its `Next` is above `c`. A read and an ack hold the lock across the store call, so a slow store blocks the sends.
- **Config checks.** `Config.check` reads an ordered list, so the first absent field is the same in each run.
- **The ledger** (`broker/ledger.go`) holds the state of the limits in memory:
  - the accepted ids, with their hop and their parties;
  - the send times of each sender and the repeat digests;
  - the queue of each recipient;
  - the reply grants and the dead letters already sent.
  - The check (`checkLimits`) is pure. The record (`record`) runs only after the store accepted the message.
- **A recipient key** is the dispatch id, or `phase:<p>` for a planned phase with no dispatch. A record names a planned phase, and the later dispatch of that phase reads it (`addressedTo`).
- **The policy decision.** The first deny that matches decides; else the first allow that matches; else the rule `default` denies. A rule matches when each key that it gives holds.
- **The token** is a chain of HMAC-SHA256 values over its caveats, in the style of a macaroon. It holds the six canonical caveats in a fixed order. A holder can add only an earlier expiry. `hmac.Equal` compares the signature. `Mint` needs the full principal and an expiry within the lifetime of the config.
- **The ULID** is 48 bits of milliseconds and 80 bits of entropy in Crockford base32. The clock and the entropy are injected (`IDSource`).
- **The frame** escapes every field with `html.EscapeString`, so a body cannot close its `<agent_message>` block.
- **The config** follows the `ci_watch` template: pointer fields, compiled defaults and a range check for each key. A nil `Rules` gives the defaults, and an empty list closes the messaging.

## Outbox and boot (component M3)

- **The path names the dispatch.** `Layout.Locate` resolves a draft path with `EvalSymlinks`. It accepts only `<root>/.evolve/msg/<cycle>/<key>/out/<ULID>.json`, where `<key>` is the canonical base64url form of the dispatch id. The dispatch of the credential is that key, never a field of the draft.
- **A half-written draft is never read.** The watcher reads only names that are `<ULID>.json`. A writer that uses `atomicwrite` has a temporary name that starts with a dot, so the rename is the moment the draft appears.
- **The wake.** `Watcher.Run` arms the own outbox, reads the entries, and then waits. A wake or a stop ends the wait. It never polls. A wake that finds no draft is harmless.
- **The answer.** A `send` draft goes to `Sender.Send`. An `ack` draft goes to `Sender.Ack`. The watcher writes an `Answer` to `box/receipts/<id>.json`, then it deletes the draft. A crash between the two steps replays the draft, and the idempotent id gives the same receipt. A draft with no envelope id gets the id of its file name.
- **A bad draft never stops the watcher.** A draft that is too large, malformed, not a regular file or of an unknown type gets an error answer. A path outside the exact form gets a `msg.denied` event with the rule `outbox.path`, and the watcher deletes it. A symlink in one outbox that points to the draft of another dispatch gets the same denial: a watcher serves only its own dispatch.
- **A draft that vanishes is skipped.** The watcher raises no event for it.
- **A failed write or delete is reported.** A receipt or a delete that fails gets a `msg.denied` event with the rule `outbox.receipt` or `outbox.remove`, and the watcher goes on. Only a failed mkdir, arm, wait or directory read ends `Run` with an error.
- **Dispatch ids must differ when folded to lower case.** A host that folds case maps two keys that differ only in case to one directory.
- **The token mint.** `boot.Env` mints the token of a dispatch from the cycle authority. It returns a new environment that holds `EVOLVE_MSG_TOKEN` (`ipcenv.MsgTokenKey`) once. It drops any older value and does not change the input. The key is in `ipcenv.ProtocolKeys`, so `Scrub` never forwards it.
- **Unwired.** Nothing calls `outbox` or `boot` yet. Component M7 starts one watcher for each dispatch and calls `boot.Env` at the bridge boot.

## Invariants

- **A message never crosses a cycle.** `TestSend_ACrossCycleAddressIsDeniedBeforeAnyRosterLookup`, `TestSend_ACrossCycleAddressThatThePolicyAllowsIsStillUnroutable`, `TestSend_APrincipalOfAnotherCycleIsRefused` and `TestRead_APrincipalOfAnotherCycleReadsNothing`.
- **The sender cannot choose its identity.** `TestSend_TheBrokerStampsTheHeaderAndIgnoresTheSenderValues`, `TestAuthority_AnAddedCaveatCanOnlyNarrowAndNeverWidens` and `TestAuthority_TheLocationAndTheTokenMustNameOneDispatch`.
- **A deny wins, and the default denies.** `TestEngine_ADenyWinsOverAnEarlierAllow`, `TestEngine_NoMatchingRuleDenies` and `TestDefaults_AReplyGrantNeverOverridesADeny`.
- **Free text from build never reaches audit.** `TestDefaults_DenyFreeTextFromBuildAndTDDToAudit` and `TestSend_BuildCannotSendFreeTextToAudit`.
- **Nothing outside the caller's scope shows.** `TestSend_AReplyToAMessageOutsideTheSenderScopeIsNotFound` and `TestRead_OnlyARecipientSeesAMessage`.
- **A failed send counts nothing.** `TestSend_AStoreFailureIsReturnedAndNothingIsCounted`.
- **A holder cannot add an identity attribute.** `TestAuthority_APhaseAddedToAPhaseLessTokenIsRefused` and `TestSend_TheRosterNotTheTokenStatesWhoTheDispatchIs`.
- **A notifier can call the service.** `TestSend_ANotifierThatCallsTheServiceDoesNotDeadlock`.
- **Only a live entry acts, and a past entry gets only exact addresses.** `TestSend_APastDispatchCannotSendOrRead`, `TestSend_APlannedEntryGetsQueuedMessagesButCannotSend`, `TestSend_APhaseAddressSkipsAPastRetrySoTheRetryGetsOneCopy` and `TestSend_ATokenWhoseIdentityDiffersFromTheRosterIsRefused`.
- **A gap is never silent.** `TestRead_AGapFromTheStoreIsReportedAndNeverSilent`.
- **An ack clears the queue.** `TestAck_ClearsTheQueueWithoutARead`.
- **An event never carries the body.** `TestNotify_EventsCarryMetadataAndNeverTheBody`.
- **A draft is attributed by its path.** `TestOutbox_ThePathNamesTheDispatch` and `TestLayout_LocateAcceptsOnlyTheExactForm`.
- **A half-written draft is never read.** `TestOutbox_AHalfWrittenDraftIsNeverRead`.
- **No poll.** `TestNoPollTimerInTheEventChannelSources` scans `internal/agentmsg`.

## Findings

- **The red runs.** Each package had a stub that compiled, so each red test failed on its assertion. A stub that panicked stopped the rest of a package run, so the broker red was recorded one test at a time.
- **Mutants.** 2 controls and 40 mutants of `policy` and `broker` ran with `go test -overlay`, and a named test killed each one. Three mutants survived the first run and each one got a test:
  - a reply grant that ignores the requester (`TestSend_ARequestGrantsItsRecipientsARightToReplyUntilItExpires` now sends the reply to another agent);
  - a queue ack that drops the message at the cursor itself (`TestSend_AFullRecipientQueueRefusesTheRecipientUntilItReads` now reads from cursor 1);
  - a sender that cannot follow up its own message (`TestSend_ASenderCanFollowUpItsOwnMessage`).
- **One guard was not needed.** The sender check in `resolve` first also tested for an empty dispatch. A sender always has a dispatch, so the check was removed.
- **Fix round 1.** The review found that a holder can add `phase=` or `role=` to a token with no such caveat. The token is now one canonical set, and the roster states the identity. 3 controls and 60 mutants ran. One mutant is equivalent: the queue ack with `<=`, because every queued cursor is below the next cursor of a read.
- **Fix round 2.** The roster got states, the store port got gaps, and the service got `Ack`. 3 controls and 71 mutants ran, and a named test kills 70 of them. The equivalent mutant returns the token principal in place of the roster entry: the check has already made the two equal. The round 1 queue mutant is no longer equivalent, because `Ack` takes any cursor in the log.
