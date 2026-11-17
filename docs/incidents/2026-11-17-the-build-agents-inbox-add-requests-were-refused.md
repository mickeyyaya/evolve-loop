# 2026-11-17: the build agent's inbox-add requests were refused, and six planned items were lost

Related documents: the request channel is specified in section "Request file" of [llm-isolation.md](../architecture/llm-isolation.md). The lost items are steps 1 to 6 of [the split design for cmd/evolve](../plans/split-cmd-evolve-2026-11.md). The plan that found the loss is section 11 of [the concurrent-edit plan](../plans/concurrent-edit-layout-2026-11.md).

## Summary

- **Impact:** the build agent of cycle 2460 filed 6 move items as `inbox-add` requests. The pipeline refused all 6, and no item reached the inbox. The design said that the items were filed. In cycles 2191, 2200 and 2271, 5 more requests were lost in the same way.
- **Root cause:** the CLI gives `inbox add` the class R and tells a phase agent to write a request file. The request channel runs only the kind `dispatch`, and refuses each other kind. The refusal is a WARN signal with no reason code, and no report shows it.
- **Fix:** the console filed the 6 items from the request files. Item `agent-request-inbox-add-has-no-executor` adds the executor, and an ERROR signal for each refused request until the executor exists.
- **Lesson:** a verb that tells an agent to write a request needs an executor for that request. A refusal that drops work must be loud.

## Terms used throughout

- **Request:** a file `requests/<kind>-<n>.json` that a phase agent writes in its workspace. The pipeline reads it at the handoff.
- **Request channel:** `agentrequest.Channel`, the code that validates each request and runs it.
- **Class R:** a verb that an agent cannot run itself. The agent writes a request, and the pipeline runs the verb.

## Timeline

| Time (UTC, 2026-11-17) | Event | Evidence |
|---|---|---|
| 11:22 | The build agent of cycle 2460 writes `inbox-add-1.json` to `inbox-add-6.json`. The channel refuses each one with precondition `kind`. | `.evolve/runs/cycle-2460/signals.ndjson`, kind `isolation.request.decision` |
| 12:12 | Cycle 2460 lands `docs/plans/split-cmd-evolve-2026-11.md`. Its section 6 says that the agent filed the 6 items. | commit `206c028fd` |
| 12:30 | The console reads the plan, finds no item in the inbox, and reads the refusals. | `find .evolve/inbox -name '*split-cmd-evolve*'` |
| 12:32 | The console files the 6 items in their order of dependency. | `evolve inbox add`, rc 0 for each |
| 12:33 | The console files item `agent-request-inbox-add-has-no-executor`. The protected check routes it to the console. | `evolve inbox add`, rc 0 |

## Root cause

**F1.1** `go/internal/agentclass/table.go:66` gives `inbox add` the class `Request` with the kind `inbox-add`.
*Implication:* a phase agent that runs `evolve agent capabilities` is told to write an `inbox-add` request.

**F1.2** `go/internal/agentrequest/channel.go:190` refuses each kind except `dispatch`. The specification says so: the executors of the other kinds come with item `cli-agent-tool-surface`. That item is consumed, but the executors do not exist.
*Implication:* each `inbox-add` request is refused. The refusal is by design, but the work in the request is lost.

**F1.3** The refusal is a WARN signal at stage `shadow`, with the drift code `SIGNALCENTER_MISSING_REASON`. No boundary report or `evolve wave supply` line lists it.
*Implication:* no person and no agent saw the loss for 11 requests over 4 cycles.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The 6 move items are filed, each with the dependency on the step before it | none (`evolve inbox add`) | none |
| The executor for `inbox-add`, and an ERROR signal for a refused request | inbox item `agent-request-inbox-add-has-no-executor` | open: a workspace with a valid `inbox-add-1.json` files the item, red before the fix |
| The 5 older lost requests (cycles 2191, 2200, 2271) | for the console at the next boundary: check each against main, and file the ones that are still true | none |

## What we learned

- A verb of class R needs an executor before the CLI tells an agent to use it. (F1.1, F1.2)
- A refusal that drops the work of an agent is an ERROR, and a report lists it. (F1.3)
- When a document says "filed", check the inbox. (F1.3)
