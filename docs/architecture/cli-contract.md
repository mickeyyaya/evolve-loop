# CLI contract: the `evolve` verbs that other services call

> [ADR-0156](adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) · plan: [llm-isolation-2026-10](../plans/llm-isolation-2026-10.md) §12 · research: [llm-isolation-2026-10](../research/llm-isolation-2026-10.md) §6 (F4.1 to F4.11) · design of the change: [llm-isolation](llm-isolation.md) Part III.
> This file is the spec of record for the service contract of the `evolve` CLI. It holds the envelope, the exit codes, the availability classes and the list of core verbs. [runtime-reference.md](../operations/runtime-reference.md) holds the operator detail of each verb.
>
> Status: proposed, 2026-10-25. The columns "today" describe `origin/main` at `603389ebd`. The target columns are the work of component X17. No verb emits the envelope yet.
>
> **Terms used throughout:**
> - **core verb**: a verb in the table of §5. A service can depend on it.
> - **envelope**: the one JSON object that a core verb prints on stdout with `--json`.
> - **schema id**: the `schema` field of the envelope, `evolve.<verb>/<major>`.
> - **availability class**: when a verb works while a wave runs (§4).
> - **live lease**: a run whose lease heartbeat is fresh and whose owner process is alive (`runlease.LiveRuns`).
> - **queue**: a durable file of requested changes that the pipeline applies at the next boundary.

## Table of contents

1. [Goals](#1-goals)
2. [The envelope](#2-the-envelope)
3. [Exit codes](#3-exit-codes)
4. [Availability classes](#4-availability-classes)
5. [The core verbs](#5-the-core-verbs)
6. [Verbs to add](#6-verbs-to-add)
7. [Inputs and the project root](#7-inputs-and-the-project-root)
8. [The first defect: `loop status <word>`](#8-the-first-defect-loop-status-word)
9. [Guards](#guards)
10. [Limits](#limits)

## 1. Goals

The operator said on 2026-10-25: "Make sure evo loop cli can be invoked for most of the core functions all the time. It's the interfaces for other services to use it's functions anytime."

| Goal | Measure | Today | Target |
|---|---|---|---|
| A caller finds each core function | core verbs in this file that the registry also has | this file is new | each core verb, by guard test |
| A caller parses each result | core verbs with a versioned `--json` envelope | 0 | each core verb |
| A caller reads one exit table | core verbs that follow §3 | some verbs only (F4.3) | each core verb |
| A caller knows when a verb works | core verbs with a class in §5 | 0 | each core verb |
| A read never starts work | read forms that start a loop | 1 (`loop status <word>`, F4.4) | 0 |

### A worked example

A dashboard service polls the state of the pipeline every minute from another directory.

| Step | Today | After X17 |
|---|---|---|
| Call | `evolve loop status --json --project-root /p` | the same |
| Output | `{"loop": {...}}`, the raw `dashboard.LoopStatus` | `{"schema": "evolve.loop.status/1", "ok": true, "data": {...}}` |
| A wrong extra word | `evolve loop status x` starts a loop with the goal `status x` | exit 10, usage, nothing starts |
| The inbox | `evolve inbox list --json` reads the inbox of the current directory, not `/p` | `--project-root /p` reads `/p` |

## 2. The envelope

Each core verb with `--json` prints exactly one JSON object on stdout, and nothing else. Messages go to stderr.

```json
{
  "schema": "evolve.inbox.list/1",
  "ok": true,
  "data": { "items": [] },
  "error": null,
  "warnings": []
}
```

| Field | Rule |
|---|---|
| `schema` | `evolve.<verb path with dots>/<major>`. A new optional field keeps the major. A removed or renamed field adds 1 to the major. |
| `ok` | `true` when the exit code is 0, `false` in each other case. |
| `data` | the result of the verb. Its shape is fixed for each schema id. |
| `error` | `null`, or `{"code": <exit code>, "kind": "<word>", "message": "<one line>"}`. |
| `warnings` | a list of one-line strings. It is never absent. |

- A verb prints the envelope on each exit, also on a usage error. `evolve wave next --json` already does this ([wave-operations.md](../operations/wave-operations.md)).
- A stream verb (`signals tail --follow`, `wave watch`, the `events` verbs) prints one envelope for each line. Each line has the same `schema` field (JSON Lines).
- The human text without `--json` is not a contract. It can change at any time.

## 3. Exit codes

One table holds for each core verb. Today some verbs use these values and others use 1 for usage (F4.3).

| Code | Name | Meaning | Today's constant |
|---|---|---|---|
| 0 | `ok` | The verb did its work. A read found what it read. | none |
| 1 | `failed` | The work ran and the result is negative: a check failed, a gate is red, a verdict is FAIL. | `exitRefused` is also 1 today (`c/cmd_pr.go:27`) |
| 2 | `io` | A file, a process or `gh` failed. The result is unknown. | `exitIO` (`c/cmd_pr.go:26`) |
| 3 | `refused` | A rule stopped the verb before it changed anything: a live lease, a boundary-only verb, a phase marker. Retry later. | `exitRoutingRefused` is 2 today (`c/cli_routing_root.go:122`) |
| 4 | `queued` | The pipeline accepted the change into a queue. It applies at the next boundary. | none |
| 10 | `usage` | The call is wrong: an unknown flag, a missing operand, a stray word. | `exitUsage` (`c/cmd_pr.go:25`) |
| 11 | `input` | The input is wrong: a bad item file, a bad JSON body. | `agent run` uses 11 today (`c/registry.go:43`) |
| 21 to 99 | verb-specific | A verb can add a code here. The verb section of runtime-reference.md names it. | for example, the dispatch error of `agent run` moves from 3 to 21 |

- `refused` moves from 1 or 2 to 3. A caller can then retry a refusal and not retry a failure.
- `agent run` uses 2 for a refusal and 3 for a dispatch error today (`c/registry.go:43`). X17b moves them to 3 and 21.
- A verb that has its own codes today keeps them for one release with a WARN line, then moves to this table. Each move is a line in the CHANGELOG.

## 4. Availability classes

Each core verb has one class. The class says what the verb does while a wave runs.

| Class | While a wave runs | Writes | Example |
|---|---|---|---|
| `always` | works | never | `status`, `inbox list`, `signals tail`, `cli-routing explain` |
| `queued` | accepts the change into a queue and exits 4; the boundary applies it | the queue file only | `inbox edit`, `inbox withdraw`, `cli-routing set` |
| `leased` | takes a short lease on its own resource and works | its own resource | `gc`, `inbox add`, `inbox claim`, `checkpoint save` |
| `boundary` | refuses with exit 3 and names the next boundary | the main tree, the loop | `pr merge`, `sync-main`, `cycle reset`, `cli update`, `loop` |

Rules:

1. An `always` verb never writes a tracked file, a lease or a signal. `quality report` publishes signals today (`c/cmd_quality.go:116-118`), so it is `leased`, and a read-only form `quality show` is added.
2. A `queued` verb never refuses only because a lease is live. Today five inbox verbs and the `cli-routing` writers refuse (F4.5).
3. The queue is `.evolve/queue/<verb>-<utc>.json`, one file for each request. The boundary step applies the files in time order, before `sync-main`. Each applied file writes one `cli.queue.applied` signal.
4. A `leased` verb holds the lease only for its own write. `gc` takes the `gc` lease and skips each resource that a live run owns. Today `gc` takes no lease (F4.7).
5. A `boundary` verb keeps its refusal. These refusals are correct (F4.6).
6. Inside a phase pane, each writing verb refuses with exit 3. This is the I-3 rule of ADR-0154.

## 5. The core verbs

"json" and "root" say if the verb has `--json` and `--project-root` today. Each target is "yes".

| Verb | Class | json today | root today | Schema id (target) | Note |
|---|---|---|---|---|---|
| `status` | always | yes | yes | `evolve.status/1` | one report of loop, cycles, PRs and CI |
| `loop status` | always | yes | yes | `evolve.loop.status/1` | fix §8 first |
| `wave status` | always | yes | yes | `evolve.wave.status/1` | |
| `wave supply` | always | yes | yes | `evolve.wave.supply/1` | |
| `wave watch` | always | yes | yes | `evolve.wave.watch/1` | a stream; it polls every 5 s today (`c/cmd_wave.go:27`) until E13 |
| `inbox list`, `show`, `rank`, `lint`, `batches` | always | yes | no | `evolve.inbox.<verb>/1` | root from env or cwd today (F4.8) |
| `signals tail` | always | yes | no | `evolve.signals.tail/1` | a stream; root from env or cwd today |
| `ledger verify` | always | no | `--evolve-dir` only | `evolve.ledger.verify/1` | prints on stderr today (`c/cmd_ledger.go:60`) |
| `dossier verify` | always | no | yes | `evolve.dossier.verify/1` | |
| `cli-routing show`, `check`, `explain` | always | yes | yes | `evolve.cli-routing.<verb>/1` | |
| `routing explain` | always | no | yes | `evolve.routing.explain/1` | |
| `clihealth list`, `usage` | always | yes | yes | `evolve.clihealth.<verb>/1` | `usage` starts a usage probe of the CLI, not a model turn |
| `agent list` | always | yes | yes | `evolve.agent.list/1` | ADR-0152 |
| `ci classify` | always | yes | yes | `evolve.ci.classify/1` | `--rerun` makes it `leased` |
| `scan secrets` | always | no | yes | `evolve.scan.secrets/1` | |
| `docs ste-lint` | always | yes | yes | `evolve.docs.ste-lint/1` | |
| `quality report` | leased | no | yes | `evolve.quality.report/1` | it publishes signals |
| `quality compare` | leased | no | yes | `evolve.quality.compare/1` | it publishes one PQ3 reading |
| `agent run` | leased | yes | yes | `evolve.agent.run/1` | read-only agent; it writes a ledger entry with cycle 0 |
| `inbox add`, `consume` | leased | no | no | `evolve.inbox.<verb>/1` | no lease check today |
| `inbox edit`, `withdraw`, `verify`, `route-console`, `route-lane` | queued | no | no | `evolve.inbox.<verb>/1` | refuse during a wave today (F4.5) |
| `cli-routing set`, `unset`, `init`, `migrate` | queued | no | yes | `evolve.cli-routing.<verb>/1` | refuse during a wave today (`c/boundary_write.go:13-27`) |
| `wave note` | leased | yes | yes | `evolve.wave.note/1` | |
| `checkpoint save`, `list`, `restore`, `prune` | leased | yes | yes | `evolve.checkpoint.<verb>/1` | ADR-0122 |
| `gc` | leased | no | yes | `evolve.gc/1` | no lease today (F4.7) |
| `loop-stop` | leased | no | yes | `evolve.loop-stop/1` | it sets the brake; `--wait` blocks |
| `wave next` | boundary | yes | yes | `evolve.wave.next/1` | it runs the boundary |
| `boundary run` | boundary | no | yes | `evolve.boundary.run/1` | |
| `pr merge` | boundary | no | yes | `evolve.pr.merge/1` | refuses while a loop runs (`c/cmd_pr.go:265-276`) |
| `sync-main` | boundary | no | yes | `evolve.sync-main/1` | (`c/cmd_syncmain.go:58-63`) |
| `cycle reset` | boundary | no | yes, default `.` | `evolve.cycle.reset/1` | `--force` overrides (`c/cmd_cycle.go:109`, refusal at `:131-133`) |
| `cli update` | boundary | yes | yes | `evolve.cli.update/1` | `--dry-run` is `always` (`c/cmd_cli.go:57`) |
| `loop` | boundary | no | yes | `evolve.loop/1` | one owner only (`c/cmd_loop_detach.go:55`) |

Verbs outside this table are tools for the operator or for the pipeline itself. A caller must not depend on them. Examples are `inbox-mover`, `fanout-dispatch`, `serve-phase` and `phase-watchdog`.

## 6. Verbs to add

| Verb | Class | Today | Why |
|---|---|---|---|
| `inbox claim <id> --cycle N` | leased | only `inbox-mover claim` (`c/cmd_inbox_mover.go:29`) | a service that runs one item needs a public claim |
| `dossier show <cycle>` | always | no read of one dossier; `dossier` has 4 other subverbs (`c/cmd_dossier.go:19-32`) | the result of one cycle in one call |
| `cycle resume [--cycle N]` | boundary | only `loop --resume` | resume one cycle without the loop flags |
| `wave stop [--wait D]` | leased | only `loop-stop --wait` | the wave verb set is complete |
| `events channels`, `kinds`, `watch`, `subscribe` | always (`subscribe` is leased) | absent: plan E9 is not started | a service waits on an event, not on a poll ([event-notification-protocol-2026-10](../plans/event-notification-protocol-2026-10.md) E9) |
| `quality show --scope S` | always | absent | the readings with no publish |

`events` is the E9 component of the events plan. X17 does not build a second `events` verb. The X17 item depends on the E9 item, and the E9 item takes the envelope and the exit table of this file.

## 7. Inputs and the project root

1. The root is `--project-root P`. When the flag is absent, the verb reads `EVOLVE_PROJECT_ROOT`, then the current directory. The verb prints the root that it used in `data.project_root`.
2. A relative root is made absolute once, at the start. `quota-reset` and `fanout-dispatch` join an unset variable with a relative path today (`c/cmd_quota_reset.go:26`, `c/cmd_fanout_dispatch.go:44`).
3. No core verb reads stdin unless the caller passes `-` as a file name. `inbox add` reads stdin today when `--file` is absent. After X17 it needs `--file -`.
4. No core verb prompts. `install` asks for `y` unless `--ci` is set (`c/cmd_install.go:103`). It is not a core verb, and it gets `--yes`.
5. No core verb needs tmux, a terminal or a Claude Code session. Today none does (research F4.9). `postmortem collect` reads a Claude transcript, so it is not a core verb.

## 8. The first defect: `loop status <word>`

Before X17a, `isLoopStatusInvocation` (removed) sent `loop status` to the read-only report only when `status` was alone, or when the next word started with `-`. Any other form went to the loop parser. `parsePositional` made the rest of the words a goal (`c/cmd_loop_args.go:307-323`). The reserved-word guard refused only a goal that was exactly a reserved word. The line references of this paragraph were on main at `9d8e8f174`, before #1125.

| Call | Before X17a | After X17a |
|---|---|---|
| `evolve loop status` | report | report |
| `evolve loop status --json` | report | report |
| `evolve loop status foo` | **starts a loop** with the goal `status foo` | exit 10, nothing starts |
| `evolve loop --project-root P status` | exit 10: the reserved-word guard refuses the goal `status` | report |
| `evolve loop 3 status foo`, `evolve loop stop now` | **starts a loop** | exit 10: the goal starts with a reserved word |
| `evolve loop --goal-text status` | starts a loop (a goal text, by design) | the same |

The fix has two rules. When the first positional word is `status`, the call goes to the status reader. The status reader refuses each extra word with exit 10. The red test: `evolve loop status foo` with a fake loop runner records a launch.

X17a (#1125) applies the rules. `loopStatusArgs` replaces `isLoopStatusInvocation`, which is removed. It reads the loop flags, so a `status` after leading flags goes to the reader too. The reader also refuses a loop flag with exit 10.

A third rule closes the other forms: `reservedGoalVerb` refuses a positional goal whose first word is a reserved word (`status`, `stop`, `help`, `plan`, `watch`). Thus `plan the refactor` needs `--goal-text`. The tests swap the `loopLaunchFn` seam and count the launches (`TestLoopStatus_AnExtraWordNeverStartsALoop`).

## Guards

| Rule | Guard (proposed) | Component | Stage |
|---|---|---|---|
| A read form never starts a loop | `TestLoopStatus_AnExtraWordNeverStartsALoop` | X17a | enforce |
| This file lists exactly the core verbs that the registry has | `TestCLIContract_DocMatchesTheRegistry`. It parses §5 and §6. It checks each verb against `registry.go` and the subverb tables. | X17b | enforce |
| Each core verb prints one envelope with a `schema` field on each exit | `TestCLIContract_EachCoreVerbPrintsOneEnvelope`, a table test over §5 with a temp root | X17b | enforce |
| Each core verb exits with a code of §3 | the same table test, with a usage case and a refusal case for each verb | X17b | enforce |
| An `always` verb writes nothing | `TestCLIContract_AlwaysVerbsWriteNothing`: a tree hash of the temp root before and after | X17b | enforce |
| A `queued` verb does not refuse during a wave | `TestInboxEdit_DuringAWaveQueuesAndExitsFour` | X17c | shadow, then enforce |
| `gc` skips a resource that a live run owns | `TestGC_TakesItsLeaseAndSkipsLiveRuns` | X17c | enforce |

## Limits

- The CLI starts a process for each call. A caller that needs many calls a second needs a daemon, which is not in this design.
- The dashboard server has no authentication (`i/dashboard/server.go:90-98`). This file does not change it.
- A `queued` change takes effect at the next boundary, not at once. A caller that needs it at once must wait for the `cli.queue.applied` signal.
- The human text is not a contract. A script that parses it breaks without notice.
- This file covers the core verbs only. About 80 other verbs keep their own usage text.
