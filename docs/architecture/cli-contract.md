# CLI contract: the `evolve` verbs that other services call

> [ADR-0156](adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) · plan: [llm-isolation-2026-10](../plans/llm-isolation-2026-10.md) §12 · research: [llm-isolation-2026-10](../research/llm-isolation-2026-10.md) §6 (F4.1 to F4.11) · design of the change: [llm-isolation](llm-isolation.md) Part III.
> This file is the spec of record for the service contract of the `evolve` CLI. It holds the envelope, the exit codes, the availability classes and the list of core verbs. [runtime-reference.md](../operations/runtime-reference.md) holds the operator detail of each verb.
>
> Status: proposed, 2026-10-25. The columns "today" describe `origin/main` at `603389ebd`. The target columns are the work of component X17. Before X17b, no verb emitted the envelope. The X17b amendment below changes this.
>
> Amended on 2026-10-26 by the operator: each verb also has an agent class (§4.1 to §4.5). The column "agent" and the evidence of §4.1 describe `origin/main` at `5c010d7a4`. The work is component X17e.
>
> Amended on 2026-10-27 by component X17b: each core verb prints the envelope of §2 and exits with a code of §3. The columns "json" and "root" of §5 describe the branch of X17b. The guards of X17b are in place (§9).
>
> Amended on 2026-10-27 by component X17c: the `queued` verbs queue during a wave, and `gc` takes a lease (§4.6). The stage of the queue is the policy key `isolation.i15.stage`, compiled default `shadow`.
>
> **Terms used throughout:**
> - **core verb**: a verb in the table of §5. A service can depend on it.
> - **envelope**: the one JSON object that a core verb prints on stdout with `--json`.
> - **schema id**: the `schema` field of the envelope, `evolve.<verb>/<major>`.
> - **availability class**: when a verb works while a wave runs (§4).
> - **live lease**: a run whose lease heartbeat is fresh and whose owner process is alive (`runlease.LiveRuns`).
> - **queue**: a durable file of requested changes that the pipeline applies at the next boundary.
> - **cell**: the sandbox around one phase agent ([llm-isolation.md](llm-isolation.md) §4).
> - **agent class**: what a verb does when an agent in a cell calls it: `A`, `R` or `D` (§4.1).
> - **request**: a typed JSON file in which an agent asks the pipeline to run an `R` verb for it (§4.3).
> - **dispatch credential**: a secret that the pipeline holds and that no cell can read (item `isolation-dispatch-capability-token`).

## Table of contents

1. [Goals](#1-goals)
2. [The envelope](#2-the-envelope)
3. [Exit codes](#3-exit-codes)
4. [Availability classes](#4-availability-classes) and [agent classes](#41-agent-classes)
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

- A verb prints the envelope on each exit, also on a usage error.
- The dispatcher builds the envelope, not the verb (`runEnveloped`, `go/cmd/evolve/cli_contract.go`). With `--json`, a core verb prints only its `data` document on stdout. The dispatcher reads that document and adds `schema`, `ok`, `error` and `warnings`. One registry holds the path, the class, the major and the extra codes of each core verb (`coreVerbs`, `go/cmd/evolve/cli_contract_verbs.go`).
- A verb can print `data` on a failure too. For example, `cli-routing check` prints its findings with exit 1.
- When the verb writes no usable stderr line, the message is `evolve <verb> exited <code> (<kind>)`.
- Text that a verb prints on stdout in place of JSON goes to stderr, `data` is `null`, and `warnings` names the fault. Two or more JSON values become one list, and `warnings` names that fault too. Each one is a defect of the verb.
- `error.message` is the last usable stderr line. A line is not usable when it is indented, ends with `:`, holds `WARN` or starts with `usage:`.
- The dispatcher reads `--json` as a word before `--`. A text value that is exactly `--json`, for example `wave next --note --json`, also selects the envelope.
- Exit 4 (`queued`) has `ok: false` and `error.kind: "queued"`, as the rule for `ok` says. Its `data` is the receipt of the queue.
- A stream verb (`signals tail --follow`, `wave watch`, the `events` verbs) prints one envelope for each line. Each line has the same `schema` field (JSON Lines).
- The human text without `--json` is not a contract. It can change at any time.

## 3. Exit codes

One table holds for each core verb. The Go constants are `cliExit*` in `go/cmd/evolve/cli_contract_verbs.go`. Before X17b, some verbs used 1 or 2 for usage (F4.3).

| Code | Name | Meaning | Before X17b |
|---|---|---|---|
| 0 | `ok` | The verb did its work. A read found what it read. | none |
| 1 | `failed` | The work ran and the result is negative: a check failed, a gate is red, a verdict is FAIL. | `exitRefused` was also 1 (`c/cmd_pr.go:27`) |
| 2 | `io` | A file, a process or `gh` failed. The result is unknown. | `exitIO` (`c/cmd_pr.go:26`) |
| 3 | `refused` | A rule stopped the verb before it changed anything: a live lease, a boundary-only verb, a phase marker. Retry later. | `exitRoutingRefused` was 2 (`c/cli_routing_root.go:122`) |
| 4 | `queued` | The pipeline accepted the change into a queue. It applies at the next boundary. | none |
| 10 | `usage` | The call is wrong: an unknown flag, a missing operand, a stray word. | `exitUsage` (`c/cmd_pr.go:25`) |
| 11 | `input` | The input is wrong: a bad item file, a bad JSON body. | `agent run` uses 11 today (`c/registry.go:43`) |
| 130 | `interrupted` | A signal (SIGINT or SIGTERM) stopped the verb. It is 128 plus the signal number of SIGINT. | `cli update` and `loop` used it already |
| 21 to 99 | verb-specific | A verb can add a code here. The verb section of runtime-reference.md names it. | for example, the dispatch error of `agent run` moves from 3 to 21 |

- `refused` moved from 1 or 2 to 3. A caller can then retry a refusal and not retry a failure.
- `agent run` used 2 for a refusal and 3 for a dispatch error. X17b moved them to 3 and 21.
- The verb codes of X17b: `agent run` 21 `dispatch`; `sync-main`, `boundary run` and `wave next` 22 `index-dirty` and 23 `index-fetch`, from the code index refresh ([code-index-cli.md](code-index-cli.md) §4).
- A changed code takes effect at once, with no release of overlap. The design said one release with a WARN line. The operator asked for one usage code everywhere at once (2026-10-27), and no caller in the repository read the old codes. Each move is a line in the CHANGELOG entry of X17b.

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
2. A `queued` verb never refuses only because a lease is live. Before X17c, five inbox verbs and the `cli-routing` writers refused (F4.5). §4.6 has the design as built.
3. The queue is `.evolve/queue/<verb>-<utc>.json`, one file for each request. The boundary step `queue-apply` applies the files in time order, right after `sync-main`. Each applied file writes one `cli.queue.applied` signal.
4. A `leased` verb holds the lease only for its own write. `gc` takes the `gc` lease and skips each resource that a live run owns. Before X17c, `gc` took no lease (F4.7).
5. A `boundary` verb keeps its refusal. These refusals are correct (F4.6).
6. Inside a phase pane, each writing verb refuses with exit 3. This is the I-3 rule of ADR-0154. The agent class of §4.1 sets which verbs a cell can run.

### 4.1 Agent classes

The operator said on 2026-10-26: "make sure that LLM cli is able to justify how to interact with the evo loop pipeline as it can decide how to leverage evo loop cli to help it to complete its tasks."

The availability class says when a verb works, relative to a wave. The agent class says if a phase agent in a cell can run the verb. The two classes are independent. For example, `inbox add` is `leased` for the operator, but an agent in a cell must send a request for it.

| Agent class | Name | In a cell | Credential | Examples |
|---|---|---|---|---|
| `A` | allowed | The verb runs. | not needed | `phase verify`, `selfcheck build`, `code find`, `inbox show`, `docs ste-lint` |
| `R` | request only | The verb refuses with exit 3. The agent writes a request (§4.3), and the pipeline runs the verb after the orchestrator approves it. | needed | `agent run`, `phase <name>`, `inbox add`, `quality report` |
| `D` | denied | The verb refuses with exit 3. No request can ask for it. | needed | `loop`, `wave next`, `sync-main`, `pr merge`, `reset-sha`, `release`, `gc`, `cli update`, `cli-routing set` |

Rules:

1. The list of `A` and `R` verbs is an allowlist. Each verb that the list does not name is `D`. A new verb is thus `D` until a change of this file gives it another class.
2. An `A` verb changes no shared state: no inbox, no policy, no signal, no lease and no git ref. It can write telemetry in the own workspace and a cache that a content key addresses.
3. An `A` verb execs no LLM CLI. A verb form that execs one is `D` in a cell (I-3).
4. An `R` verb changes shared state or starts an agent. The pipeline runs it, never the cell.
5. A `D` verb is a verb of the operator or of the boundary. An agent has no reason to run it, so no request kind maps to it.
6. In the subagents of its own harness, an agent can use each `A` verb. The subagents run in the cell, so the same classes apply to them ([llm-isolation.md](llm-isolation.md) §7.2 on the branch of #1126).

The column "Agent" of §5 gives the class of each core verb. These verbs are outside §5, but a cell meets them:

| Verb | Agent class | Why |
|---|---|---|
| `phase verify <phase>` | A | It reads the deliverables. For `build` it runs the handoff probe (F5). |
| `selfcheck build` | A | the same probe as `phase verify build` (F5) |
| `code callers`, `api`, `find`, `map` | A | one usage record in the own workspace (F4) |
| `code refresh` | R (`code-refresh`, proposed) | It moves the shared index worktree to `origin/main` and rebuilds it ([code-index-cli.md](code-index-cli.md) §4, D5.5). In a cell it refuses with exit 3. |
| `agent capabilities` (proposed, §4.2) | A | It reads the class table. |
| `context index`, `context open` | A | They read the cell index and its bodies, and they write nothing ([cell-context-index.md](cell-context-index.md) §7). |
| `context stats` | D | It reads the access log of other cycles, which I-5 denies to a cell. |
| `phase <name>` | R (`dispatch`) | It starts a phase agent. |
| `subagent run` | D | The pipeline starts fan-out workers. A cell uses the subagents of its own harness. |
| `reset-sha`, `release` | D | operator verbs |

#### Findings

**F1** The profiles tell an agent which verbs it can run in Claude Code tool syntax, for example `Bash(evolve phase verify:*)` in 93 profiles ([llm-isolation.md](llm-isolation.md) §28). *Implication:* an agent of another family gets no list it can read, and the list does not say why a verb is absent.

**F2** The refusals of X4 read the markers `EVOLVE_DISPATCH_DEPTH` and `EVOLVE_PHASE` (`go/internal/cli/agentcmd/run.go:93`, `go/cmd/evolve/boundary_write.go:14`). `env -u` removes them (the review of #1126). *Implication:* a class that the markers alone enforce is advice, not a rule.

**F3** `quality report` publishes its readings to the Signal Center (`go/cmd/evolve/cmd_quality.go:186`). `quality compare` publishes one PQ3 reading too (`go/cmd/evolve/cmd_quality_compare.go:44-47`). *Implication:* both change shared state. A reading from a cell can steer a stage flip that other cells depend on.

**F4** Each `code` verb appends one usage record (`go/cmd/evolve/cmd_code_usage.go:46-48`). In a phase the record goes to the cycle workspace, else to the user cache (`go/cmd/evolve/registry.go:134`). *Implication:* the write is telemetry in the own workspace, not shared state.

**F5** `phase verify` and `selfcheck build` read the deliverables and run the build handoff probe (`go/internal/cli/phasecmd/phase_verify.go:22-60`, `go/cmd/evolve/cmd_selfcheck.go:21-52`). For `build`, the probe runs `go test` and can put pass records in the test-verdict cache `.evolve/landing/testcache` (`go/internal/phases/ship/scanshadow.go:75`, `:228`). *Implication:* the only write is a cache that a content key addresses. A cached verdict is valid for each reader with the same key, so an agent can run both verbs in a cell. The profiles already allow `evolve phase verify` (F1).

**F7** `clihealth usage` opens an interactive session of each LLM CLI to read its usage (`go/internal/usageevidence/production.go:58`). `cli-routing show` and `cli-routing explain` without `--static` run each installed LLM CLI with `--version` (`go/internal/bridge/doctor.go:188`). *Implication:* in a cell these forms exec an LLM binary, which I-3 forbids. The `--static` forms exec none.

**F8** I read the other verbs that §5 marks `A` on `5c010d7a4`. They change no file, signal, lease, git ref or `gh` state. They read files, or they run `git`, `gh`, `go list` and `ps` reads. Examples: `status` runs `gh pr list` (`go/cmd/evolve/cmd_status.go:158`). `ledger verify` takes a shared `flock` on a lock file that exists (`go/internal/adapters/ledger/ledger.go:142`). `ci classify` can file an inbox item only when a rerun result is known, and only `--rerun` gets one (`go/internal/ciwatch/classify.go:153`, `:168`).

**F6** `agent capabilities` is not a verb on `origin/main` (`go/cmd/evolve/registry.go:43`: `agent run` and `agent list` only).

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Three classes, and `D` is the default | F1, F2 | Two classes, allow and deny: an agent then has no way to ask for a justified dispatch, and the operator asked for that way. A deny list: a new verb is open until someone adds it. |
| D2 | `quality report` and `quality compare` are `R`. The read form `quality show` (§6, X17d) is `A`. | F3 | `A`: a cell then publishes readings to the Signal Center, which is a channel to other cells outside I-4. `D`: a PQ3 reading is "on request" by design ([llm-quality-indexes.md](llm-quality-indexes.md) §5.4), so an agent needs a way to ask for it. |
| D3 | `code callers`, `api`, `find` and `map` are `A` | F4 | `R`: the usage record is telemetry of the own cell, and a request for each code query costs a judgement for no gain. |
| D4 | The cell reads its classes from one verb, `agent capabilities --json` (§4.2) | F1, F6 | A list in each persona: it is CLI text above the driver, which I-17 forbids. A tool server for each family: the house rule is CLI only ([cli-only-no-mcp.md](../conventions/cli-only-no-mcp.md)). |
| D5 | `R` and `D` verbs need the dispatch credential (§4.4) | F2 | The markers only: `env -u` passes them. A sandbox deny of the `evolve` binary: the profiles must run `evolve phase verify`. |
| D6 | `cli-routing set` is `D`. A routing change from an agent is a request of the kind `routing-proposal` (§4.3). | D1 rule 4 | `R` for `cli-routing set` itself: the verb writes the policy file at once. A change from a cell must wait for a judgement and for the boundary queue (§4 rule 3). |
| D7 | `clihealth usage` is `D`. `cli-routing show` and `explain` are `A` only with `--static`. `ci classify --rerun` is `D`. | F7, F8 | `R` for `clihealth usage`: the operator and the boundary already read the usage, and a cell has no task that needs it. |

What changes our mind: the orchestrator can approve more than 95% of the requests of one kind for two weeks. Then that kind can move to `A` with a cap. If an `A` verb gets a shared write, it moves to `R` in the same change.

### 4.2 The capabilities verb

`evolve agent capabilities --json` (proposed, X17e) tells the agent of the current cell what it can do. It is `A` and `always`. Each family reads the same JSON, so no persona needs CLI-specific text.

```json
{
  "schema": "evolve.agent.capabilities/1",
  "ok": true,
  "data": {
    "cell": {"cycle": 2101, "phase": "build", "workspace": ".evolve/runs/cycle-2101"},
    "verbs": [
      {"verb": "phase verify", "class": "A", "input": "args/1", "output": "evolve.phase.verify/1"},
      {"verb": "code find", "class": "A", "input": "args/1", "output": "text/1"},
      {"verb": "inbox add", "class": "R", "request": {"kind": "inbox-add", "schema": "evolve.request/1", "path": ".evolve/runs/cycle-2101/requests/inbox-add-<n>.json"}},
      {"verb": "agent run", "class": "R", "request": {"kind": "dispatch", "schema": "evolve.request/1", "path": ".evolve/runs/cycle-2101/requests/dispatch-<n>.json"}},
      {"verb": "wave next", "class": "D"}
    ],
    "default_class": "D",
    "caps": {"dispatch": {"per_cycle": 2, "chain_depth": 1}, "inbox-add": {"per_cycle": 3}}
  },
  "error": null,
  "warnings": []
}
```

| Field | Rule |
|---|---|
| `cell` | The pipeline fills it from its own record of the dispatch, not from the environment of the caller. Outside a cell it is `null`, and each verb shows its class for a cell. |
| `verbs[].class` | the class of §4.1 and of the column "agent" of §5 |
| `verbs[].input`, `verbs[].output` | the schema id of the arguments and of the `--json` envelope (§2). `text/1` marks a verb whose output is lines, not an envelope. |
| `verbs[].request` | only for `R`: the request kind, the request schema and the path to write |
| `default_class` | always `D` (§4.1 rule 1) |
| `caps` | the caps for each request kind that the validator applies (§4.3). The values in the example are not defaults. |

The neutral TaskAssignment (I-17) carries a pointer to this verb ([llm-isolation.md](llm-isolation.md) §28). The agent calls it once at the start, and again when it needs a verb that it has not used.

### 4.3 Requests

An agent asks for an `R` verb through a request file. It writes one file for each request, in its own workspace:

```json
{
  "schema": "evolve.request/1",
  "kind": "inbox-add",
  "what": {"verb": "inbox add", "args": {"id": "parser-nil-guard", "title": "..."}},
  "why": "The build found a nil dereference in a parser that this item does not own. The fix is out of scope.",
  "inputs": ["build-report.md"],
  "expected_output": {"artifact": "an inbox item", "schema": "inbox-item/1"}
}
```

| Kind | Verbs | What the pipeline does after approval |
|---|---|---|
| `dispatch` | `agent run`, `phase <name>` | It dispatches the phase agent ([llm-isolation.md](llm-isolation.md) §7.2 on the branch of #1126). |
| `inbox-add` | `inbox add` | It files the item with `source` set to the request id. |
| `routing-proposal` | `cli-routing set`, `cli-routing unset` | It writes the change to the queue of §4 rule 3. The boundary applies it after `cli-routing check` passes. |
| `quality-reading` | `quality report`, `quality compare` | It runs the verb and publishes the reading. |
| `code-refresh` (proposed) | `code refresh` | It runs `evolve code refresh` ([code-index-cli.md](code-index-cli.md) §4, D5.5). |

The path from a request to a result:

1. The agent writes `requests/<kind>-<n>.json` in its workspace. The file is a declared output of the cell.
2. The pipeline validates the file in Go. It checks the schema, the kind, the allowlist of the seat and the caps. The caps for `dispatch` are the proposed policy key `isolation.dispatch_requests` of item `isolation-dispatch-request-channel`. The other kinds get a cap for each cycle in the same block (proposed).
3. The orchestrator agent of [ADR-0155](adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) judges if the request is reasonable. It reads the `why`, the `inputs` and the scope of the item. It proposes "approve" or "refuse" with a reason, and the pipeline disposes.
4. After approval the pipeline runs the verb with its own credential. The pipeline adds the result to the lineage index of the requester ([llm-isolation.md](llm-isolation.md) §8).
5. A refusal records its reason in the same index. The agent never runs the verb itself.

The agent does not wait in its turn for the decision. The pipeline reads the requests at the handoff of the phase. A later phase, a retry or the next item of the lineage reads the result through the index.

### 4.4 Enforcement

The class is a rule, not advice. Three layers enforce it:

1. **The verb.** An `A` verb needs no credential. An `R` or `D` verb reads the dispatch credential first.
   When the read fails, the verb exits 3. It prints one line that names the request path:
   `refused inside a phase: inbox add is class R; write the request to .evolve/runs/cycle-2101/requests/inbox-add-<n>.json (schema evolve.request/1, see evolve agent capabilities --json)`
   A `D` verb names no path: `refused inside a phase: wave next is class D; it is an operator or boundary verb`.
2. **The credential.** The pipeline holds it, and the sandbox denies its read to each cell. The operator and the pipeline run outside a cell, so their verbs can read it. The design is item `isolation-dispatch-capability-token` of X4 (#1126). That item covers the dispatch verbs. X17e extends the same check to each `R` and `D` verb.
3. **The sandbox.** A cell can write only in its own workspace and its worktree ([llm-isolation.md](llm-isolation.md) §9).
   Past the first two layers, an `R` or `D` verb cannot write the inbox, the policy or the main tree.

The markers of X4 stay as a fast first check with a clear message. They are not the guard (F2).

As built (item `isolation-dispatch-capability-token`): layers 1 and 2 cover each command that starts a dispatch ([llm-isolation.md](llm-isolation.md) §7.1.1). Examples are `agent run`, `phase <name>`, `loop`, `wave next`, `cycle run` and `boundary run`. Each LLM launch through the bridge launch API checks the credential too. The refusal exits 3 with the texts above. X17e adds the other `R` and `D` verbs when it builds the class table.

### 4.5 Signals and quality readings

Each request and each decision is a signal of the module `isolation` ([llm-isolation.md](llm-isolation.md) §15):

| Kind | Severity | Code | When | Fields |
|---|---|---|---|---|
| `isolation.request` | INFO | none | the validator reads a request | `request`, `kind`, `verb`, `cell`, `family` |
| `isolation.request.decision` | INFO | none | the orchestrator approves, or the pipeline runs the verb | `request`, `kind`, `decision`, `result` |
| `isolation.request.decision` | WARN | `ISOLATION_REQUEST_REFUSED` (proposed) | the validator or the orchestrator refuses | `request`, `kind`, `decision`, `reason`, `stage` |
| `isolation.violation` | WARN | `ISOLATION_I3_VIOLATION` | an `R` or `D` verb refuses in a cell (§4.4) | `rule` is `verb.class.r` or `verb.class.d`, and `target` is the verb |

The last row closes a gap of X4: the refusal of `agent run` emits no signal yet.

Three proposed readings of the quality panel ([llm-quality-indexes.md](llm-quality-indexes.md), ADR-0153) use these signals. An amendment of ADR-0153 adds them at stage `shadow`:

| Reading | Unit | Window | What it shows |
|---|---|---|---|
| request rate | requests for each cycle, for each kind and family | wave | if agents use the request path, and which family asks the most |
| approval rate | approved requests over decided requests, for each kind | wave | if the requests are reasonable. A low rate means that the persona text or the classes need a fix. |
| refusal outcomes | refusals, split by reason: schema, cap, judged unreasonable, verb refused with no credential | wave | where agents try to go past the boundary |

### 4.6 The queue and the `gc` lease (as built, X17c)

The operator said on 2026-10-27: "the CLI is the service interface and must be invocable at any time".

**F9.1** Before X17c, `inbox edit`, `withdraw`, `verify`, `route-console` and `route-lane` refused while a run lease was live (`refusedMidWaveAt`). The `cli-routing` writers refused too (`refuseBoundaryWrite`).
*Implication:* a service had to poll for a boundary to change one inbox field.

**F9.2** `sync-main` refuses a tracked change that is not a stamp of the loop (`refuseDirtBeyondInboxStamps`, `go/cmd/evolve/cmd_syncmain.go`). An edit of a curator field is such a change, and so is a write of `.evolve/policy.json`.
*Implication:* a queue applied before `sync-main` stops the boundary at `sync-main`.

**F9.3** The file sweeps of `gc` already judge each resource. A worktree, a run dir or a claim dir of a live run stays (`internal/gc/worktrees.go`). The sweeps of sessions, sockets, cycle processes and dispatch processes stop processes.
*Implication:* the process sweeps are the risk while lanes run.

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D8 | A `queued` verb under a live lease writes one request file and exits 4. At stage `shadow` it refuses with exit 3 and names the file that `enforce` writes. Inside a phase pane it refuses with exit 3. | F9.1 | A lease for each item: a lane reads the item, so a write during a wave changes the input of a lane. |
| D9 | The boundary step `queue-apply` runs right after `sync-main`, not before it. | F9.2 | Before `sync-main`: an applied edit stops `sync-main`. A boundary commit of the applied files: `.evolve/policy.json` is not a path that a boundary commit can hold. |
| D10 | `gc` takes the `gc` lease (`.evolve/gc.lock`, a `flock`). The boundary step waits up to 2 minutes for it (`--lease-wait 2m`); a call with no wait refuses at once. While a run lease is live, `gc` skips the sessions, sockets, cycle processes and dispatch processes. The other sweeps run. | F9.3 | Only the `cache` scope while lanes run: the file sweeps already skip each live resource. |

The request file:

```json
{"schema": "evolve.queue.request/1", "verb": "inbox edit", "args": ["cli-inbox-show", "--set", "weight=0.4"], "queued_at": "2026-10-27T01:00:00.000000000Z"}
```

- The file name is `<verb with dashes>-<queued_at without colons>-<pid>.json`. The args have no `--project-root` and no `--json`.
- `queue-apply` first moves each request to `.evolve/queue/processing/`, so a request never runs twice. Then it runs the request through the verb itself, with `--project-root` of the plane.
- A request that exits 0 moves to `applied/`. A request that the verb queues again, because a lease became live, moves to `requeued/`. Any other request moves to `failed/` with a WARN signal `CLI_QUEUE_FAILED`.
- A failed request never stops the boundary. The step exits 0 unless it cannot list the queue.
- While a run lease is still live, `queue-apply` applies nothing and keeps the queue, so a request is never queued again by its own replay.
- The time in the file has a fixed width (`2006-01-02T15:04:05.000000000Z`), so the order of the text is the order of the time.
- An applied edit leaves the same tracked change that an operator edit at a boundary leaves. `queue-apply` names each such path on stdout and in one WARN signal `CLI_QUEUE_UNLANDED`.
- The operator or the console lands each such change, for example with `evolve ship --class manual`, before the next `wave next`. Otherwise the next boundary stops at `sync-main` with exit 3. Set the stage `enforce` only when that owner exists.
- `--json` on a queued call prints the receipt `{"verb", "path"}` as `data`, with exit 4.

Limits:

- The `gc` lease covers each run of `evolve gc`. The `gc` hook of the loop (`gc.mode`) takes no lease.
- The check for a live run is a check at one time. A lane that starts after the check is not seen.
- The live runs are the runs of this project. The session and socket sweeps act on the whole host, as before X17c.
- At stage `shadow` the verb writes only a stderr line, so no signal counts the requests before the flip to `enforce`. A follow-up can add one.
- A relative item path in the args resolves against the directory of `queue-apply`. Queue an item by its id.

## 5. The core verbs

"json" and "root" say if the verb has `--json` and `--project-root` on the branch of X17b. `TestCLIContract_DocMatchesTheRegistry` requires "yes" in both columns. "Agent" is the agent class of §4.1, checked on `5c010d7a4`. The name in brackets is the request kind of §4.3.

| Verb | Class | Agent | json today | root today | Schema id (target) | Note |
|---|---|---|---|---|---|---|
| `status` | always | A | yes | yes | `evolve.status/1` | one report of loop, cycles, PRs and CI |
| `loop status` | always | A | yes | yes | `evolve.loop.status/1` | fix §8 first |
| `wave status` | always | A | yes | yes | `evolve.wave.status/1` | |
| `wave supply` | always | A | yes | yes | `evolve.wave.supply/1` | |
| `wave watch` | always | A | yes | yes | `evolve.wave.watch/1` | a stream; it polls every 5 s today (`c/cmd_wave.go:27`) until E13 |
| `inbox list`, `show`, `rank`, `lint`, `batches` | always | A | yes | yes | `evolve.inbox.<verb>/1` | `evolve inbox` takes `--project-root` for each verb (F4.8) |
| `signals tail` | always | A | yes | yes | `evolve.signals.tail/1` | a stream |
| `ledger verify` | always | A | yes | yes | `evolve.ledger.verify/1` | the text form prints on stderr; `--evolve-dir` still wins over the root |
| `dossier verify` | always | A | yes | yes | `evolve.dossier.verify/1` | |
| `cli-routing show`, `check`, `explain` | always | A (`show` and `explain` with `--static` only, F7) | yes | yes | `evolve.cli-routing.<verb>/1` | |
| `routing explain` | always | A | yes | yes | `evolve.routing.explain/1` | |
| `clihealth list`, `usage` | always | `list` A, `usage` D (F7) | yes | yes | `evolve.clihealth.<verb>/1` | `usage` starts a usage probe of the CLI, not a model turn |
| `agent list` | always | A | yes | yes | `evolve.agent.list/1` | ADR-0152 |
| `ci classify` | always | A (`--rerun` is D) | yes | yes | `evolve.ci.classify/1` | `--rerun` makes it `leased` |
| `scan secrets` | always | A | yes | yes | `evolve.scan.secrets/1` | |
| `docs ste-lint` | always | A | yes | yes | `evolve.docs.ste-lint/1` | |
| `quality report` | leased | R (`quality-reading`) | yes | yes | `evolve.quality.report/1` | it publishes signals |
| `quality compare` | leased | R (`quality-reading`) | yes | yes | `evolve.quality.compare/1` | it publishes one PQ3 reading |
| `agent run` | leased | R (`dispatch`) | yes | yes | `evolve.agent.run/1` | read-only agent; it writes a ledger entry with cycle 0 |
| `inbox add`, `consume` | leased | `add` R (`inbox-add`), `consume` D | yes | yes | `evolve.inbox.<verb>/1` | no lease check; `add` reads stdin only with `--file -` |
| `inbox edit`, `withdraw`, `verify`, `route-console`, `route-lane` | queued | D | yes | yes | `evolve.inbox.<verb>/1` | queue during a wave at stage `enforce`, exit 4 (§4.6) |
| `cli-routing set`, `unset`, `init`, `migrate` | queued | D (`set` and `unset` through `routing-proposal`) | yes | yes | `evolve.cli-routing.<verb>/1` | queue during a wave at stage `enforce`; `--dry-run` runs at once (§4.6) |
| `wave note` | leased | D | yes | yes | `evolve.wave.note/1` | |
| `checkpoint save`, `list`, `restore`, `prune` | leased | D | yes | yes | `evolve.checkpoint.<verb>/1` | ADR-0122 |
| `gc` | leased | D | yes | yes | `evolve.gc/1` | the `gc` lease; a live run stops the process sweeps (§4.6) |
| `loop-stop` | leased | D | yes | yes | `evolve.loop-stop/1` | it sets the brake; `--wait` blocks |
| `wave next` | boundary | D | yes | yes | `evolve.wave.next/2` | it runs the boundary. X17b moved the old document, which had its own `schema` field, into `data`, so the major is 2. |
| `boundary run` | boundary | D | yes | yes | `evolve.boundary.run/1` | |
| `pr merge` | boundary | D | yes | yes | `evolve.pr.merge/1` | refuses while a loop runs (`c/cmd_pr.go:265-276`) |
| `sync-main` | boundary | D | yes | yes | `evolve.sync-main/1` | (`c/cmd_syncmain.go:58-63`) |
| `cycle reset` | boundary | D | yes | yes | `evolve.cycle.reset/1` | `--force` overrides (`c/cmd_cycle.go:109`, refusal at `:131-133`) |
| `cli update` | boundary | D | yes | yes | `evolve.cli.update/1` | `--dry-run` is `always` (`c/cmd_cli.go:57`) |
| `loop` | boundary | D | yes | yes | `evolve.loop/1` | one owner only (`c/cmd_loop_detach.go:55`) |

`inbox curate` is not a core verb yet. It has no `--json`. Without a flag it is read-only. `--event` publishes signals, and `--apply` refuses with exit 1 while a run lease is live. [inbox-curator.md](inbox-curator.md) is its specification.

Verbs outside this table are tools for the operator or for the pipeline itself. A caller must not depend on them. Examples are `inbox-mover`, `fanout-dispatch`, `serve-phase` and `phase-watchdog`.

## 6. Verbs to add

| Verb | Class | Today | Why |
|---|---|---|---|
| `inbox claim <id> --cycle N` | leased | only `inbox-mover claim` (`c/cmd_inbox_mover.go:29`) | a service that runs one item needs a public claim |
| `dossier show <cycle>` | always | no read of one dossier; `dossier` has 4 other subverbs (`c/cmd_dossier.go:19-32`) | the result of one cycle in one call |
| `cycle resume [--cycle N]` | boundary | only `loop --resume` | resume one cycle without the loop flags |
| `wave stop [--wait D]` | leased | only `loop-stop --wait` | the wave verb set is complete |
| `events channels`, `kinds`, `watch`, `subscribe` | always (`subscribe` is leased) | absent: plan E9 is not started | a service waits on an event, not on a poll ([event-notification-protocol-2026-10](../plans/event-notification-protocol-2026-10.md) E9) |
| `quality show --scope S` | always | absent | the readings with no publish. Its agent class is `A` (§4.1 D2). |
| `agent capabilities [--json]` | always | absent (`c/registry.go:43`) | the agent of a cell reads its classes, schemas and request paths (§4.2). Its agent class is `A`. Component X17e. |
| `guidance index [--audience X] [--phase P] [--cli F] [--json]`, `guidance check` | always | both exist (#1138); `index` prints an envelope and `check` has no `--json` | an agent of any family lists the rules of its case from the rule registry ([guidance-layers.md](guidance-layers.md) §10.3). The agent class of both is `A`. `guidance build` writes tracked files, so it stays an operator tool outside the core verbs. |

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
| This file lists exactly the core verbs that the registry has | `TestCLIContract_DocMatchesTheRegistry` (built). It parses §3, §5 and §6. It checks each verb against `coreVerbs` and the command registry. | X17b | enforce |
| Each core verb prints one envelope with a `schema` field on each exit | `TestCLIContract_EachCoreVerbPrintsOneEnvelope` (built, integration tier), a table test over §5 with a temp root | X17b | enforce |
| Each core verb exits with a code of §3 | the same table test, with a usage case and a negative case (a refusal, else a failure) for each verb | X17b | enforce |
| An `always` verb writes nothing | `TestCLIContract_AlwaysVerbsWriteNothing` (built, integration tier): a tree hash of the temp root before and after | X17b | enforce |
| A `queued` verb does not refuse during a wave | `TestInboxEdit_DuringAWaveQueuesAndExitsFour` (built) | X17c | shadow, then enforce |
| The boundary applies the queue in time order with one signal each | `TestQueueApply_AppliesTheRequestsInTimeOrderAndEmitsOneSignalEach` (built) | X17c | enforce |
| `gc` skips a resource that a live run owns | `TestGC_TakesItsLeaseAndSkipsLiveRuns` (built) | X17c | enforce |
| The agent class of each verb in Go matches §4.1 and the column "Agent" of §5 | `TestAgentClasses_MatchTheContract` | X17e | enforce |
| An `R` verb with no credential exits 3 and names the request path. A `D` verb exits 3. | `TestAgentClass_RefusesWithoutTheCredentialAndNamesTheRequestPath` | X17e | shadow one wave, then enforce |
| A valid request is run by the pipeline, and its result reaches the lineage index | `TestAgentRequest_AnApprovedRequestIsRunByThePipeline` | X17e | enforce |

## Limits

- The CLI starts a process for each call. A caller that needs many calls a second needs a daemon, which is not in this design.
- The dashboard server has no authentication (`i/dashboard/server.go:90-98`). This file does not change it.
- A `queued` change takes effect at the next boundary, not at once. A caller that needs it at once must wait for the `cli.queue.applied` signal.
- The human text is not a contract. A script that parses it breaks without notice.
- This file covers the core verbs only. About 80 other verbs keep their own usage text.
