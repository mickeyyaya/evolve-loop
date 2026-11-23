# Plan: the pipeline files the inbox items that phase agents request

- **Status:** partly built, 2026-11-21 (2 of 5 components on main, 1 PR open, 2 not started or unverified). Earlier status: proposed, 2026-11-17.
- **Research:** the online search of section 2 (P1 to P3). No separate dossier: the facts are local, and the search answers one question.
- **Related:** [cli-contract.md §4.3](../architecture/cli-contract.md) specifies the request kinds. [llm-isolation.md](../architecture/llm-isolation.md) gives the request channel as built. The incident is [the build agent's inbox-add requests were refused](../incidents/2026-11-17-the-build-agents-inbox-add-requests-were-refused.md) (PR #1545).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

## 1. Request

The inbox item `agent-request-inbox-add-has-no-executor`, filed by the console on 2026-11-17. The build agent of cycle 2460 wrote 6 `inbox-add` requests for the move steps of the `cmd/evolve` split. The request channel refused all 6, and no item reached the inbox. The design that the same cycle landed says that the items were filed.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | `inbox add` has the class R and the kind `inbox-add`. A phase agent that runs the verb is told to write a request file. | `go/internal/agentclass/table.go:66`, cli-contract §4.4 |
| F2 | The channel refuses each kind except `dispatch`, with the precondition `kind`. | `go/internal/agentrequest/channel.go:190` |
| F3 | 11 requests were refused in cycles 2191, 2200, 2271 and 2460. No item was filed for any of them. | `.evolve/runs/cycle-*/signals.ndjson`, kind `isolation.request.decision` |
| F4 | Each decision signal has an empty reason. `writeRecord` sets `Summary` on its copy of the record, and `emitDecision` reads the summary of the caller's copy. The signal center marks the drift `SIGNALCENTER_MISSING_REASON`. | `go/internal/agentrequest/record.go:75-79`, `channel.go:103-105`, `signals.go:31` |
| F5 | `Request.What.Args` is a map of strings. The requests of cycle 2460 carry `id` and `title` in it, and the reason in `why`. | `go/internal/agentrequest/request.go`, `.evolve/runs/cycle-2460/requests/` |
| F6 | An inbox item needs the text fields `id`, `title`, `kind`, `summary` and `fix`, and at least one `acceptance` entry. It also needs a `weight` above 0 and a `priority_class` from the policy list. | `go/internal/inboxmover/lifecycle/file.go:31`, `itemFieldRules`, `checkPriorityClass` (`file.go:57`) |
| F7 | The intake of `evolve inbox add` runs the curator admission (overlap, kinds, protected path, probe of the tree) and then `inboxmover.File`. It lives in package `main`. | `go/cmd/evolve/cmd_inbox_add.go:28-62`, `admitInboxItem` |
| F8 | The origin `cycle` exists for items that a cycle files. | `go/internal/inboxbatch/origin.go` |
| F9 | The dispatch judge is an LLM seat. Its decision must be the catalog action `dispatch-phase-agent`. | `channel.go` `judge`, `phasecontract.ActionDispatchPhaseAgent` |
| F10 | The channel has one policy, the dispatch policy. `Handoff` returns early when the dispatch stage is `off`. `decide` reads the dispatch stage, and `check` applies the dispatch roles and depth to each request. | `channel.go:62`, `channel.go:84-86`, `decide` at `channel.go:149`, `check` at `channel.go:188` |
| F11 | The cycle cap is 3 by default and counts every record of the cycle, also the refused ones. Cycle 2460 wrote 6 `inbox-add` requests. | `go/internal/policy/isolation.go:15`, `recordCount` in `record.go` |
| F12 | The retro autofile sets the weight of each item it files from the policy key `default_weight`. | `go/internal/policy/budgets_recovery.go:29-37` |
| F13 | `inboxbatch.Item` has no `source` field. An unknown key is author-owned, and no loader reads it. | `go/internal/inboxbatch/item.go`, `fieldRoles` in `fieldowner.go:87` |
| P1 | Current practice for agent actions: a deterministic policy layer decides if a proposed action runs. An LLM review can advise, but a mechanical refusal overrides an LLM approval. | arXiv 2609.02246; AWS and Netskope on Bedrock AgentCore policy |
| P2 | Structural and scope checks belong to deterministic tools. An LLM judge accepted code that a parser refused. | arXiv 2607.27674; a DEV write-up on deterministic PR checks |
| P3 | Most of these sources are vendor posts or design papers. Treat them as positions, not as measured results. | the same |

## 3. Goals and non-goals

Goals:

1. A valid `inbox-add` request of a phase agent becomes an inbox item at the handoff of its phase.
2. A request that does not become an item is loud: an ERROR signal names the request path and the reason.
3. Each decision signal carries its reason.

Non-goals:

- The kinds `routing-proposal`, `quality-reading` and `code-refresh` keep their refusal. Each gets its own item.
- The dispatch path does not change.
- No item of the 11 lost requests is filed by this change. The console checks the 5 old ones at a boundary.

## 4. Candidate designs

| Design | What it does |
|---|---|
| A. The specified path | The orchestrator seat judges each `inbox-add` request, as it judges a dispatch. On approval the pipeline files the item. |
| B. Deterministic intake | The channel validates the request in Go and runs the intake of `evolve inbox add` on the item. The intake is the judge. No LLM seat runs. |
| C. A boundary queue | The channel writes each valid request to a queue. The console files the queue at the boundary. |
| D. Loud refusal only | The channel keeps the refusal, but emits ERROR and lists the request for the console. |

Scores: 0 is bad, 2 is good.

| Criterion | A | B | C | D |
|---|---|---|---|---|
| The item reaches the inbox with no person (goal 1) | 2 | 2 | 0 | 0 |
| A decision is the same each time for the same request (P1, P2) | 0 | 2 | 2 | 2 |
| Cost for each request | 0 (one seat) | 2 | 1 | 2 |
| Changes to protected catalogs | 0 (a new catalog action) | 2 | 2 | 2 |
| Size of the change | 1 | 1 | 1 | 2 |
| **Total** | 3 | 9 | 6 | 8 |

Goal 1 is a gate: a design with 0 on it is out, whatever its total. So C and D are out.

B wins. A needs a new catalog action in `selfheal` and a seat for each request. Its LLM judgment adds one check that the intake does not have: the scope of the requester's item. D9 makes that check a rule. C and D keep the console in the loop for each item, which is the defect of F3.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Each request kind has one executor in the channel, keyed by its `agentclass` kind. An executor holds the stage, the cycle cap, the preconditions and the run step of its kind. The composition root in `request_channel.go` builds the executors from the policy. `Handoff` returns early only when each kind is `off`. | F10; the three other kinds of cli-contract §4.3 come next | A check of the kind in `Handoff`, `check` and `decide`: each new kind edits the same three places. |
| D2 | The `inbox-add` executor runs no LLM judge. It runs the checks of D9, then the intake of `evolve inbox add`. The intake decides. | F7, F9, P1, P2 | The judge seat (design A): one seat for each request, a new catalog action, and no fact that D9 does not check. |
| D3 | The item comes from the request. `args.id`, `args.title`, `args.kind`, `args.fix`, `args.priority_class` and `args.acceptance-1` are required. More acceptance entries are `args.acceptance-2` and up, and files are `args.files-1` and up. `why` becomes `summary`. The pipeline sets `weight` from the policy key `isolation.inbox_requests.default_weight`, so an agent cannot raise the rank of its item. The origin is `cycle`. | F5, F6, F12 | A separator in one string: a criterion can hold the separator. An agent weight: an agent can steer the rank. |
| D4 | One intake function serves the verb and the executor: `intakeItem(in intakeInput)` returns `(inboxmover.FileResult, error)`. The value `intakeInput` holds `Root`, `Tree`, `Raw`, `Origin` and `Source`. Its errors wrap the overlap error and `inboxmover.ErrInvalidItem`. The verb maps the errors to its exit codes with `intakeExitCode`. For a shadow run, `lifecycle` exports one `Validate` that runs the item rules with no write. | F7 | A copy in the executor: two intakes drift. A dry-run argument: a flag argument. |
| D5 | The policy block `isolation.inbox_requests` holds `stage` (`off`, `shadow`, `enforce`; default `enforce`), `per_cycle` (default 8) and `default_weight` (default 0.5). In `shadow` the executor validates and records "would file", and it files nothing. The block is proposed until C3 lands. | F3, F11: the default must not lose more items | Shadow as the default: each shadow request is one more lost item. One cap for all kinds: the incident's own 6 requests fail it (F11). |
| D6 | The cap of a kind counts only the records of that kind in the cycle. | F11 | The shared count of `recordCount`. |
| D7 | One decision signal for each request. Its severity comes from the executor: a refused or failed `inbox-add` request is ERROR. The item id and the item path are fields of that signal. The registered text of `ISOLATION_REQUEST_FAILED` changes to "the executor of the kind failed". | F3, goal 2 | A second signal for the same failure: two homes for one fact. |
| D8 | The summary is built once, by a pure `withSummary(Record) Record`. The caller passes the result to the writer and to the emitter. | F4 | The summary inside `writeRecord`: the caller emits a copy with no summary (F4). |
| D9 | A precondition `scope` refuses an item whose `files` overlap the `files` of the requester's own item. The cell gives the executor the files of the item and the worktree of the cycle. | the spec gives the judge "the scope of the item" (cli-contract §4.3 step 3); a build agent can move its own work to the inbox | No scope check: audit then sees a smaller diff than the item asked for. |
| D10 | The intake probes the worktree of the cycle, not the main tree. A file that the cycle adds exists there before the ship. | Q1 of the first version; `probeTree` in `cmd_inbox_add.go:175` | The main tree: an item on a new file of the cycle is refused. |
| D11 | `inboxbatch.Item` gets a `source` field that the loop stamps: `request:<cycle>/<request name>`. `inboxmover.Options` and the lifecycle `Mover` get a `Source` field, and `stampIntake` writes it next to `origin`. An authored `source` in the raw item is refused, as each loop-stamped key is. The set `filingStamps` of `origin.go` comes from the keys that `stampIntake` writes. So the set has one home, and withdraw accepts an item with a `source`. | F13 | An author-owned key: no reader, and a console author can forge it. |
| D12 | The record of a filed request holds the item path in `Report`. A try to file that gets `ErrAlreadyFiled` for an item with the same `source` counts as filed. | cli-contract §4.3 step 4; a crash between the file and the record | A new record field: `Report` already holds the result of a dispatch. |
| D13 | `agentclass` gets the constant `KindInboxAdd`. `evolve agent capabilities --json` prints the caps and the required args of each kind. | the literal in `table.go:66` | The literal in two places. |

Consequences:

- Good: an agent files an out-of-scope finding, and it reaches the inbox at the handoff, with the same rules as a console item.
- Good: each decision signal has a reason, and a lost request is an ERROR.
- Good: the next kind (`routing-proposal`, `quality-reading`, `code-refresh`) is one more executor.
- Bad: the required args are stricter than the example of cli-contract §4.3. The 6 requests of cycle 2460 had no `kind`, `fix`, `priority_class` or acceptance, so this executor refuses such requests. The refusal reason names each missing arg, and `agent capabilities` lists them (D13), so the agent can write a valid request.
- Bad: a phase agent can file an item that the curator later rejects. The cap and the overlap refusal limit the number.

What changes our mind: the curator rejects more than half of the items that agents file over 2 waves. Then the stage goes to `shadow`, and design A returns.

## 6. Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| C1 | D8: the decision signal carries its reason. | a refused request emits a decision signal whose `Reason` is not empty | console (the channel is protected) |
| C2 | D1, D6, D13: an executor for each kind. The dispatch executor keeps today's behavior. | each existing channel test stays green; with the dispatch stage `off` and the inbox stage `enforce`, an `inbox-add` request is read | console |
| C3 | D2, D3, D4, D5, D9, D10, D11, D12: the `inbox-add` executor and the shared intake. | a valid `requests/inbox-add-1.json` files the item with origin `cycle` and its `source`. A request with no `fix` is refused, and the reason names `fix`. A filed item carries its `source`, a raw item with an authored `source` is refused, and an item filed with a `source` can be withdrawn. Six requests in one cycle file six items. An item on a file of the requester's item is refused with `scope`. | console |
| C4 | D7: the severity of each kind, and the new text of the failed code. | a refused `inbox-add` request emits one decision signal at ERROR with the item id | console |
| C5 | The spec. In cli-contract §4.3, D5 replaces the "same block" sentence, and step 3 says that `inbox-add` has no judge. The "As built" rows of llm-isolation.md follow. | none (a document) | console, with C3 |

## 7. Order

1. C1 alone. It is small, and it fixes the reason of each decision signal.
2. C2 alone. It changes the shape of the channel and keeps its behavior.
3. C3, C4 and C5 as one change.

## 8. Verification

| Measure | Before | Target | Command |
|---|---|---|---|
| `inbox-add` requests with no item and no ERROR | 11 | 0 | the decision signals of the next 2 waves |
| Decision signals with `SIGNALCENTER_MISSING_REASON` | each decision | 0 | `evolve signals` with that code |
| Agent items that the curator rejects | not measured | under half | `evolve inbox curate` proposals for origin `cycle` |

## 9. Risks

| Risk | Control |
|---|---|
| An agent files many items. | The cap of the kind (D5, D6) and the overlap refusal of the intake. |
| An agent moves work of its own item to the inbox. | The `scope` precondition (D9). |
| An agent raises the rank of its item. | The pipeline sets the weight (D3). The curator reviews the priority class at each boundary. |
| An agent files an item on a protected file to steer the console. | The intake routes it to the console with the checked path. The console decides under the rule "the console builds only what a lane cannot build" of [operating-policy.md §2](../operations/operating-policy.md). |
| A filed item duplicates an open item. | The overlap refusal of the curator admission. |

## 10. Open questions

- **Q1.** Does a cycle that fails still keep the items that its agents filed? This plan keeps them: the finding is true even when the cycle fails. The curator reviews each open item at the boundary. C3 adds a test that pins this case.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| C1 the decision signal carries its reason (D8) | done | [#1553](https://github.com/mickeyyaya/evolve-loop/pull/1553), [`0d74ee9`](https://github.com/mickeyyaya/evolve-loop/commit/0d74ee997); `go/internal/agentrequest/record.go:82` | none |
| C2 an executor for each kind (D1, D6, D13) | done | [#1553](https://github.com/mickeyyaya/evolve-loop/pull/1553), [`0d74ee9`](https://github.com/mickeyyaya/evolve-loop/commit/0d74ee997); `go/internal/agentrequest/executor.go`, `kinds.go`; `KindInboxAdd` in `go/internal/agentclass/table.go:66` | none |
| C3 the `inbox-add` executor and the shared intake | built, PR open | [#1621](https://github.com/mickeyyaya/evolve-loop/pull/1621); on main only a stub: `inboxAdd.run` refuses with the precondition `intake` (`go/internal/agentrequest/executor.go:44`) | `agent-request-inbox-add-has-no-executor` |
| C4 severity of each kind and new failed-code text (D7) | not started | none; `go/internal/agentrequest/signals.go:25` still uses WARN for a refused request, and D7 asks for ERROR. It is unverified if #1621 covers it | `agent-request-inbox-add-has-no-executor` |
| C5 the spec: cli-contract §4.3 and the llm-isolation "As built" rows | not started (unverified) | none found; the cli-contract grep for `inbox_requests` shows no change | none |

The executor frame and the reason on the signal are on main. The intake that files the item is in [#1621](https://github.com/mickeyyaya/evolve-loop/pull/1621). One open item covers C3 and C4. C5 has no open inbox item. A follow-up draft is in `followups/agent-request-inbox-add-spec-c5.json`.
