# 2026-11-20: five signal roots publish nothing to the event bus

Related documents: the event channels are specified in `docs/architecture/event-channels.md`. The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** the roots `integration`, `auth`, `queue`, `latency` and `retire` published no event to the root event bus. The 266 MB bus under `.evolve/events/ch` held 0 records with these sources. The batch of PR #1600 printed `[events] WARN: role integration publishes no events`. A console watch on the bus cannot see the main-red release or the batch hold.
- **Root cause:** `publisher.Roles()` lists 15 roles. The CLI builds signal centers under 5 more roles. The publisher refuses an unknown role, and `rootwire.attach` prints one WARN and drops each event.
- **Fix:** the inbox item `signal-roots-under-unregistered-roles-publish-nothing` registers the 5 roles and replaces the hand-written role list of the guard. It is open.
- **Lesson:** a guard that checks a hand-written list does not check the call sites. It stays green when a call site is added.

## Terms used throughout

- **Root:** a signal center that a CLI verb builds under a role name.
- **Role:** the closed set of source names that the publisher accepts.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 14:02 to 14:30 | The batch of #1600 prints the WARN for role `integration`. | item `signal-roots-under-unregistered-roles-publish-nothing` |
| 14:33 | The console files the item. It counted the bus records by source: wave 68, ship 18, quality 7387, and 0 for each of the 5 roles. | item file name; item summary |

The WARN text (`go/internal/events/rootwire/rootwire.go:26`):

```text
[events] WARN: role integration publishes no events: publisher: role "integration" is not one of [...]
```

## Root cause

**F1.1** `Roles()` (`go/internal/events/publisher/publisher.go:55`) returns 15 names. `Config.check` (`:83`) refuses a role that is not in the list.
*Implication:* a new root under a new role publishes nothing.

**F1.2** The 5 call sites are `go/cmd/evolve/cmd_integration_stop.go:19`, `go/cmd/evolve/cmd_auth_seat_token.go:20`, `go/cmd/evolve/cli_queue.go:34`, `go/cmd/evolve/cmd_wave_latency_check.go:119` and `go/cmd/evolve/cmd_cycle_retire.go:66`.
*Implication:* each verb of these roots runs without a bus record.

**F1.3** `rootwire.attach` (`go/internal/events/rootwire/rootwire.go:23`) turns the error into one WARN and returns. The verb does not fail.
*Implication:* the loss is silent in a long run. Only a count of the bus shows it.

**F1.4** The guard `TestRootSignalCenter_EveryRootRunsUnderARoleOfTheClosedSet` (`go/cmd/evolve/cmd_root_guard_test.go:130`) checks a list of 7 roles that the test text holds. It does not read the roles that the call sites pass.
*Implication:* the guard stayed green when the 5 roots were added.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| Register the 5 roles in `publisher.Roles()`. The guard parses the role argument of each `newRoleSignalCenter`, `newRoleSignalCenterWithAttach` and `newRootSignalCenter` call. | inbox item `signal-roots-under-unregistered-roles-publish-nothing` | owed by the item: `TestRootSignalCenter_EveryCallSiteRoleIsAPublisherRole` (red on main with the 5 roles named) and `TestRoles_IsTheClosedSetOfTheSpecRoots` |

## Limits

- The 5 roots stay silent until the item lands. The signal `GATE_REPO_MAIN_RED_STOP_RELEASED` is in `.evolve/signals.ndjson:11438`. The loss is on the event bus (verified for this one signal).
- The item adds the item `integration-batch-emits-its-steps-as-signals` to its dependants. That item needs the role.

## What we learned

- Count the records of the bus by source after a new root ships.
- Let a guard read the call sites, not a copy of the list.
