# Subscription token telemetry: what the loop records, who uses the tokens, and what each CLI can supply (research, 2026-10)

> The plan that uses this research: [token-telemetry-2026-10](../plans/token-telemetry-2026-10.md). The earlier campaign: [token-telemetry-2026-07](../plans/token-telemetry-2026-07.md) and [ADR-0071](../architecture/adr/0071-token-telemetry-attribution-and-clean-boot.md). The spec of the try record: [model-attempt-telemetry.md](../architecture/model-attempt-telemetry.md). The cost policy that reads the same data: [ADR-0158](../architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) and [cost-aware-repair-2026-10](../plans/cost-aware-repair-2026-10.md).

Date: 2026-10-27. Status: research for a proposed plan.

The operator asked on 2026-10-27: "make sure we have proper telemetry to monitor the subscription token consumption to identify which phase or agent or steps burning the most tokens. Then we can improve the token usage. Always research and investigate first."

## Table of contents

- [Purpose](#purpose)
- [Scope](#scope)
- [Method](#method)
- [Terms used throughout](#terms-used-throughout)
- [1. Local evidence: what the loop records](#1-local-evidence-what-the-loop-records)
- [2. Local evidence: the largest consumers in waves 141 to 143](#2-local-evidence-the-largest-consumers-in-waves-141-to-143)
- [3. Current practice: what each CLI and standard supplies](#3-current-practice-what-each-cli-and-standard-supplies)
- [4. Ranked gaps](#4-ranked-gaps)
- [5. Adopt or reject](#5-adopt-or-reject)
- [6. Refinements to adopt](#6-refinements-to-adopt)
- [7. Sources](#7-sources)

## Purpose

This dossier records how the loop measures subscription token use today. It names the largest consumers in waves 141 to 143. It records what each agent CLI and the OpenTelemetry (OTel) standard can supply. It ranks the gaps that stop the operator from finding the largest consumer. It does not decide: the [plan](../plans/token-telemetry-2026-10.md) decides.

## Scope

- In scope: the Claude Code subscription and the agy (Antigravity CLI, Gemini family) subscription. API billing is out of scope.
- Code: `origin/main` at `66b0bcc7e`, and the open PRs #1184 (`feat/cost-measure`) and #1189 (`feat/cycle-spend-limit`). A citation from a PR says "PR #1184 branch" or "PR #1189 branch".
- Data: the run directories `.evolve/runs/cycle-2030` to `cycle-2050`. These 21 cycles are waves 141 to 143 (`run.json` `goal_text`).
- Outside sources: the CLI vendors, the OTel GenAI semantic conventions, and studies of token cost.

## Method

1. We read the writers and the readers of `llm-calls.ndjson`, the token collectors in `go/internal/tokenusage` and the driver manifests. Each cited `path:line` was read again on `origin/main` at `66b0bcc7e` on 2026-10-27.
2. We read the two PR branches with `git show origin/<branch>:<path>`.
3. We merged the 21 `llm-calls.ndjson` files into one scratch file with `jq`. Each row got its cycle, its wave and the `final_verdict` of its cycle. We grouped the rows with `jq -s 'group_by(...)'`. No script was added to the repository.
4. We ran `evolve tokens report --last 3` on the runtime tree and compared it with the sum of `llm-calls.ndjson` for the same cycles.
5. The outside survey used live WebSearch and WebFetch on 2026-10-27, with 12 searches and the official pages that §7 lists. It used no MCP tool.

Each local claim names its file, its command or its data. "Synthesis" marks our own inference.

## Terms used throughout

| Term | Meaning in this document |
|---|---|
| fresh tokens | input + output + cache write. This is the definition of `cyclespend.Sum` (PR #1184 branch, `go/internal/cyclespend/spend.go:30-43`). |
| cache read | tokens that the model read again from the prompt cache. Each turn of a session reads the earlier context again. |
| driver | the launch form of a CLI, for example `claude-tmux` or `agy-tmux`. A driver has one manifest. |
| family | the model vendor behind a driver: `claude` or `gemini`. |
| seat | one role that the pipeline fills with one agent: for example the builder, the auditor or the advisor (`router`). |
| round purpose | the reason for one dispatch: the first try, a retry, a repair round, a contract correction or an audit fix. |
| CWU | cost-weighted units: input + 1.25 × cache write + 0.1 × cache read + 5 × output, in millions. It is a proxy from API list prices, not the subscription rule (§3.6). |

## 1. Local evidence: what the loop records

### 1.1 The row for each model try

**F1.1** The bridge writes one row for each model try (`go/internal/bridge/attempt_telemetry.go:72`). The writer is `llmcalls.AppendWorkspace` (`attempt_telemetry.go:134`). The file is `<run dir>/llm-calls.ndjson` (`go/internal/llmcalls/record.go:17`).
*Implication:* the loop already has one append-only record per call. A new field goes on this row, not into a second ledger.

**F1.2** The row has these fields. The struct is `llmcalls.Record` (`record.go:49`).

| Field | Present | Evidence |
|---|---|---|
| input, output, cache read, cache write | yes | `go/internal/cyclestate/result.go:42-47`, `record.go:66` |
| requested, dispatched and served model | yes | `record.go:61-63`, `record.go:84` |
| `cli` (the driver) and `agent` | yes | `record.go:57-59` |
| `phase` | a copy of `agent` | `attempt_telemetry.go:113-114` (`Phase: req.Agent`) |
| `attempt`, `rung`, `ladder_stage` | yes | `record.go:64`, `record.go:80`, `record.go:86` |
| start, end, `duration_ms` | yes | `record.go:53-54`, `record.go:69` |
| `usage_status`, `source` | yes | `record.go:36-41`, `record.go:67-68` |
| `fill_pct` (the peak context fill) | yes | `record.go:72` |
| cycle, wave, run id | no. The directory name implies the cycle. | not in `Record` |
| profile, seat, round purpose | no | not in `Record` |
| tokens for each turn or each tool call | no | not in `Record` |
| family | no. The report must derive it from `cli`. | not in `Record` |

*Implication:* the row answers "which agent", but not "which round", "which step" or "which wave".

**F1.3** The bridge request already carries `Cycle`, `RunID`, `Profile`, `Attempt`, `FreshSession`, `CorrectionDirective` and the ladder try (`go/internal/core/ports.go:183-216`). The row copies only `Agent`, `CLI` and `Attempt` from it (`attempt_telemetry.go:113-120`).
*Implication:* the bridge can write cycle, run id, profile and part of the round purpose with no change to the request type. `ports.go` is on the fence (`go/internal/guards/integrity_surface.go:126`), and `attempt_telemetry.go` is not.

### 1.2 The readers of the row

**F1.4** These readers use `llm-calls.ndjson`:

| Reader | Granularity | Evidence |
|---|---|---|
| `evolve cycle models`, `evolve models performance` | each model try | `go/cmd/evolve/cmd_cycle_models.go:38` |
| `evolve ladder report` | each rung and failure class | `go/cmd/evolve/registry.go:139` |
| `lanetime` | the wall time of each lane | `go/internal/lanetime/read.go:20` |
| `cyclespend.Sum` | each cycle: `llm_ms`, `fresh_tokens`, `cache_read_tokens`, `unmeasured_calls`, `audit_rounds` | PR #1184 branch, `go/internal/cyclespend/spend.go:22-43` |
| `wavemeasure` cost | each wave: LLM minutes, fresh and cache-read tokens for each shipped cycle, audit rounds | PR #1184 branch, `go/internal/wavemeasure/cost.go:14-23` |
| the cycle spend limit (`shadow`) | each cycle at each phase boundary; code `CYCLE_SPEND_LIMIT` | PR #1189 branch, `go/internal/core/cycle_spend_limit.go:13-16` |

*Implication:* PRs #1184 and #1189 add cycle and wave totals. They add no split by phase, seat or step.

**F1.5** `evolve tokens report` reads the phase timing entries of each cycle, not `llm-calls.ndjson` (`go/cmd/evolve/cmd_tokens.go:137`). Its cache-hit ratio is cache read / (input + cache read) (`cmd_tokens.go:153-158`). For cycles 2044 to 2046 on 2026-10-27, it printed 18.60 M cache-read tokens, against 26.25 M in `llm-calls.ndjson`. It showed build 3.15 M against 7.06 M, scout 1.07 M against 3.44 M, and no `router` row against 1.37 M.
*Implication:* the one token report of the operator under-counts by 29% in this sample. It keeps only the last try of each phase and leaves out the advisor.

**F1.6** `<phase>-usage.json` keeps one total for each phase, with `cost_usd` at 0. `advisor-span-*.json` repeats the `router` row. `cyclecost.SummarizeCycle` reads the `stream-json` result events of each phase (`go/internal/cyclecost/cyclecost.go:39`, `cyclecost.go:103-120`). `token-usage.json` keeps only `peak_tokens`.
*Implication:* four stores hold parts of the same facts. A new report reads the row, and the other stores stay as they are.

### 1.3 The collectors of each driver

**F1.7** The manifest key `collectors.tokens` selects the collector chain (`attempt_telemetry.go:250-256`, `go/internal/tokenusage/defaultresolver.go:40-62`).

| Driver | Collector | Source | Rows in waves 141 to 143 |
|---|---|---|---|
| `claude-tmux` | `claude-jsonl-v1` (`go/internal/bridge/manifests/claude-tmux.json:10`) | the transcript JSONL under `~/.claude/projects`, one entry for each message id (`go/internal/tokenusage/scanner.go:80`, `scanner.go:154-170`); then the events log; then a scrollback peak | 145 measured, 33 partial, 2 unavailable |
| `agy-tmux` | `agy-conversation-v1` (`go/internal/bridge/manifests/agy-tmux.json:9`) | the agy conversation SQLite store, one row for each generation (`go/internal/tokenusage/agy.go:51`, `agy.go:79`) | 62 measured, 2 unavailable |
| `agy-claude-tmux` | the agy collector, through `"base": "agy-tmux"` | the agy conversation store | 4 measured |
| `codex`, `codex-tmux`, `ollama-tmux` | none. The resolver records `no_collector` (`defaultresolver.go:21-25`). | the events log, then the scrollback | 0 rows |

*Implication:* both families that the loop uses have a collector. The codex and ollama gap has no cost now.

**F1.8** The claude scanner reads the usage of each message and keeps a map by message id. It returns only the sum and the peak turn (`scanner.go:117-130`). The agy collector reads the usage of each generation and also returns only the sum (`agy.go:79`).
*Implication:* the data for each turn is already in memory at the end of each call. The loop drops it. A turn record costs no new read.

**F1.9** The 33 partial `claude-tmux` rows came from the scrollback peak, which sees output only. They are `router` 20, `tdd` 6, `audit` 5, `build` 1 and `triage` 1. All agy rows report `cache_write` as 0.
*Implication:* the advisor is the seat with the most unmeasured calls. The agy cache-write field is not known (§4, gap 6).

**F1.10** The post-mortem reader `attemptpostmortem.ClaudeTranscript` reads the tool calls of a transcript: text, start, status and exit code (`go/internal/attemptpostmortem/transcript.go:48`, `go/internal/attemptpostmortem/record.go:28-33`). It reads no tokens.
*Implication:* a reader of tool names from the claude transcript exists. The turn record can use the same parse.

### 1.4 The weekly quota

**F1.11** `clicontrol` parses the Claude `/usage` pane and the codex status pane (`go/internal/bridge/clicontrol/usage.go:49-52`). It has no parser for agy. `quotastate.UsageWindow` keeps `percent_used`, `kind` (5 h or week), the scope and the models (`go/internal/quotastate/windows.go:46-56`).
*Implication:* the loop sees the Claude weekly percent. It does not see an agy quota percent.

**F1.12** The store is `.evolve/usage-windows.json` (`go/internal/usageprobe/windows.go:19`). `RecordObservation` reads the file, replaces the entry of the CLI and writes the file again (`windows.go:31-44`). `evolve clihealth usage` prints the latest windows (`go/cmd/evolve/cmd_clihealth.go:33-34`).
*Implication:* the loop keeps no history of the quota percent. It cannot give the percent that one wave used.

### 1.5 The runs outside a cycle

**F1.13** `evolve agent run` creates a run workspace and removes it at the end unless `--keep` is set (`go/internal/cli/agentcmd/run.go:157-163`). The workspace holds the `llm-calls.ndjson` of the run. The launch record has no token field (`run.go:288-292`).
*Implication:* the tokens of each review run through `evolve agent run` are lost. The commit gate and the docs reviews use this path.

**F1.14** The console subagents do not use the bridge, so they write no row.
*Implication:* console work stays outside this telemetry. The bridge usage probe and the statusline are the only views of it (§3.2).

### 1.6 Signals about tokens

**F1.15** In cycles 2030 to 2050, `signals.ndjson` holds `BRIDGE_CONTEXT_FILL_HIGH` 50 times, `BRIDGE_USAGE_EVIDENCE` 9 times and `BRIDGE_TOKEN_USAGE_WARNING` 4 times. It holds `BRIDGE_TELEMETRY_TRIPWIRE` (`attempt_telemetry.go:51`) and `BRIDGE_TELEMETRY_APPEND_FAILED` (`attempt_telemetry.go:135`) 0 times.
*Implication:* the writer is healthy. The problem is the depth of the record, not its loss.

## 2. Local evidence: the largest consumers in waves 141 to 143

Data: 21 cycles, 248 calls, from the merged `llm-calls.ndjson` (Method, step 3). M is a million tokens. Min is LLM minutes.

**F2.1** The total is 581.3 M tokens and 940 LLM minutes. Cache read is 553.6 M (95.2%), and fresh input is 18.2 M (3.1%). Output is 3.08 M (0.5%), and cache write is 6.45 M (1.1%). The cache-hit ratio of `evolve tokens report` (F1.5) is 96.8% on these rows.
*Implication:* a raw token sum measures how often each turn reads the context again. The size of a prompt is a small part.

**F2.2** By agent:

| Agent | Calls | Input | Output | Cache read | Cache write | Total | Share | Min |
|---|---|---|---|---|---|---|---|---|
| build | 42 | 10.36 | 1.31 | 305.5 | 0.54 | 317.8 | 54.7% | 453 |
| scout | 23 | 4.19 | 0.46 | 100.2 | 0.19 | 105.1 | 18.1% | 144 |
| triage | 22 | 2.57 | 0.21 | 49.0 | 0.28 | 52.0 | 9.0% | 74 |
| tdd | 28 | 0.00 | 0.59 | 43.8 | 1.92 | 46.3 | 8.0% | 103 |
| audit | 26 | 0.00 | 0.25 | 21.5 | 1.19 | 23.0 | 4.0% | 73 |
| router (the advisor) | 89 | 0.00 | 0.09 | 9.5 | 1.74 | 11.3 | 2.0% | 39 |
| retro | 6 | 0.29 | 0.07 | 7.5 | 0.24 | 8.1 | 1.4% | 15 |
| 8 optional phases | 13 | - | - | - | - | 17.7 | 3.0% | 32 |

*Implication:* build and scout use 73% of the tokens. The advisor makes the most calls but uses 2%.

**F2.3** By driver: `agy-tmux` 467.0 M (80.3%, 645 min). `claude-tmux` 109.9 M (18.9%, 287 min). `agy-claude-tmux` 4.4 M (0.8%).
*Implication:* a telemetry source for one family only sees one fifth of the tokens, if that family is Claude.

**F2.4** In CWU, the total is 97.0. The largest cells are build on `agy-tmux` 44.8 (46%) and scout on `agy-tmux` 16.2. Then come tdd on `claude-tmux` 9.7, triage on `agy-tmux` 8.2 and audit on `claude-tmux` 4.9.
*Implication:* the weighted order and the raw order agree on the top two seats. The weights move tdd above triage, because tdd writes more output and more cache.

**F2.5** By wave: wave 141 (cycles 2030 to 2034) 147.6 M. Wave 142 (2035 and 2036) 120.2 M. Wave 143 (2037 to 2050) 313.5 M. 20 cycles have the verdict PASS and 1 cycle (2035) has FAIL.
*Implication:* a wave total needs a cycle count beside it. The rows do not say which PASS cycles landed (gap 8).

**F2.6** Work that did not land, or that ran again:

| Bucket | Tokens | Share |
|---|---|---|
| The FAIL cycle 2035 (4 audit dispatches, 3 tdd and build rounds) | 91.8 M | 15.8% |
| Retry rows (`attempt` > 1): build 21 rows 124.5 M, audit 13 rows 13.4 M, tdd 7 rows 4.5 M, scout 1 row 4.5 M | 146.9 M | 25.3% |
| PASS cycles with `GIT_FLEET_REBASE_NEEDED` at ship (2038, 2039, 2040, 2042, 2044) | 171 M | 29% |
| Calls with an exit code other than 0 | 6.2 M | 1.1% |

The buckets overlap. They are not a sum.
*Implication:* retries and rework at ship are each larger than any seat except build. The row cannot split a retry by its reason (gap 3).

**F2.7** We estimate the first prompt as the bytes of `<phase>-prompt.txt` divided by 4. The mean is 5 k tokens (router), 4 k (scout), 14 k (triage), 11 k (tdd), 11 k (build) and 9 k (audit). One build call on agy reads about 7.3 M cache tokens.
*Implication (synthesis):* the cost comes from the number of turns multiplied by the context that each turn reads again. The first prompt is not the cause. Only a record of each turn can show which turns grow the context.

## 3. Current practice: what each CLI and standard supplies

### 3.1 Claude Code OpenTelemetry

**P3.1** `CLAUDE_CODE_ENABLE_TELEMETRY=1` turns on the export. `OTEL_METRICS_EXPORTER` accepts `otlp`, `prometheus`, `console` and `none`. `OTEL_LOGS_EXPORTER` accepts `otlp`, `console` and `none`. The default intervals are 60000 ms for metrics and 5000 ms for logs. ([monitoring-usage](https://code.claude.com/docs/en/monitoring-usage))

**P3.2** `OTEL_RESOURCE_ATTRIBUTES` adds keys to all metrics and events. A value must not contain a space. (same)

**P3.3** The metric `claude_code.token.usage` has the attribute `type` with the values `input`, `output`, `cacheRead` and `cacheCreation`. The metrics carry `model`, `query_source` (`main`, `subagent`, `auxiliary`), `effort`, `agent.name`, `skill.name` and `mcp_server.name`. (same)

**P3.4** The event `claude_code.api_request` occurs once for each model call. It carries the model, the four token counts, `cost_usd`, `duration_ms` and `request_id`. The event `claude_code.tool_result` carries `tool_name`, `duration_ms`, `tool_input_size_bytes` and `tool_result_size_bytes`. Events carry `session.id`, `prompt.id`, `agent_id` and `parent_agent_id`. (same)

**P3.5** Traces are a beta feature (`CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`). The span tree is `claude_code.interaction`, then `claude_code.llm_request` and `claude_code.tool`. Only `-p` sessions and Agent SDK sessions read an inbound `TRACEPARENT`. (same)

*Implication:* OTel gives Claude Code a turn record and a tool record with no parse of a private format. It covers the `claude` family only (19% of the tokens, F2.3), and it needs new variables in the child environment.

### 3.2 Other Claude Code usage data

**P3.6** The statusline JSON gives `rate_limits.five_hour.used_percentage` and `rate_limits.seven_day.used_percentage`, each with `resets_at`. These fields occur only for Pro and Max subscribers, after the first API response. Claude Code gives no token count for the Max weekly limit. ([statusline](https://code.claude.com/docs/en/statusline))

**P3.7** The statusline JSON also gives `context_window.current_usage`, `cost.total_cost_usd` (an estimate at list price) and `transcript_path`. (same)

**P3.8** On a subscription plan, `/usage` shows how usage divides among skills, subagents, plugins and MCP servers. Its session cost is not a billing value. ([costs](https://code.claude.com/docs/en/costs))

**P3.9** `claude -p --output-format json` returns `total_cost_usd`, `usage` and the cost of each model. In `stream-json`, `parent_tool_use_id` names the subagent of each message. ([headless](https://code.claude.com/docs/en/headless))

**P3.10** Each assistant line of the transcript JSONL has a `message.usage` object. Community readers remove duplicate lines on `requestId` with `uuid` or `message.id`. Some readers do not read the subagent files. ([ccwhy](https://docs.rs/ccwhy), [ccupp](https://pypi.org/project/ccupp/), [aident](https://aident.ai/blog/claude-code-jsonl-token-usage))

*Implication:* the weekly percent is the unit of the subscription. The statusline is the only per-session source of it. The transcript format is not a public contract.

### 3.3 Antigravity CLI and Gemini CLI

**P3.11** Gemini CLI exports OTel from the `telemetry` block of `.gemini/settings.json`. Its metric `gemini_cli.token.usage` has the types `input`, `output`, `thought`, `cache` and `tool`. Its event `gemini_cli.api_response` carries each count. ([Gemini CLI telemetry](https://geminicli.com/docs/cli/telemetry/))

**P3.12** For some users, Antigravity CLI replaced Gemini CLI on 2026-06-18. (same)

**P3.13** Antigravity CLI has no OTLP export, and the `OTEL_*` variables have no effect. Feature request 366 has been open since 2026-06-11. ([agy issue 366](https://github.com/google-antigravity/antigravity-cli/issues/366))

**P3.14** `agy --print --output-format json` returns a `usage` object with input, output, thought, cache-read and total tokens. In `stream-json`, each `agent_response` step carries usage. The report in the same issue says that the conversation SQLite files are not reliable. (same)

**P3.15** On agy 1.2.2, the JSON status showed `usage.total_tokens: 0` ([agy issue 1010](https://github.com/google-antigravity/antigravity-cli/issues/1010)). Hook payloads in agy 1.2.12 carry no usage field ([scion issue 2237](https://github.com/ptone/scion/issues/2237)).

*Implication:* no OTel source exists for 80% of the tokens. The loop reads the agy conversation store (F1.7), which a vendor issue calls unreliable. The headless JSON is the vendor surface, but the loop runs agy in a REPL pane.

### 3.4 Codex CLI and ollama

**P3.16** `codex exec --json` emits `turn.completed` with `usage.input_tokens`, `usage.cached_input_tokens` and `usage.output_tokens` (a community source, [codex exec reference](https://codex.danielvaughan.com/2026/04/08/codex-exec-jsonl-reference/)). Codex sets OTel in the `[otel]` table of `~/.codex/config.toml`, off by default ([codex config](https://learn.chatgpt.com/docs/config-file/config-advanced)).

**P3.17** An ollama response carries `prompt_eval_count`, `prompt_eval_cached_count` and `eval_count` ([ollama generate](https://docs.ollama.com/api/generate)).

*Implication:* a collector for codex and ollama is simple when the loop uses them again (gap 7).

### 3.5 The OTel GenAI semantic conventions

**P3.18** All `gen_ai.*` items have the Development status, so names can change between releases. The conventions moved to `open-telemetry/semantic-conventions-genai` at v1.42.0 on 2026-06-12. ([dash0 explainer](https://www.dash0.com/knowledge/opentelemetry-genai-semantic-conventions-explained), [OTel GenAI](https://opentelemetry.io/docs/specs/semconv/gen-ai/))

**P3.19** The operation names include `chat`, `invoke_agent`, `invoke_workflow`, `plan` and `execute_tool`. The agent attributes are `gen_ai.agent.name` and `gen_ai.agent.id`. The workflow attribute is `gen_ai.workflow.name`. The usage attributes include `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens` and `gen_ai.usage.cache_read.input_tokens`. (same)

**P3.20** For Anthropic, `gen_ai.usage.input_tokens` is the sum of the input, the cache-read and the cache-creation tokens ([OTel Anthropic](https://opentelemetry.io/docs/specs/semconv/gen-ai/anthropic)). The loop's `input` field excludes the cache.

*Implication:* the conventions give stable names for the dimensions (agent, workflow, tool), but the input field has a different meaning. A local record can use the names and keep its own four counts.

### 3.6 Tokens are not cost

**P3.21** A study of 2,908 billed Claude Code runs found a weak correlation between tokens and cost (r = 0.15). Cache creation and cache reads are most of the input cost. A 38.4% cut of tool-output tokens raised the billed cost by 6.8%. The study recommends one metric: the cost of each successful task. ([arXiv 2607.12161](https://arxiv.org/abs/2607.12161))

**P3.22** A cache read costs 0.1 times the base input price, and 0.05 times on Opus 5.5 and Sonnet 5.5. A cache write costs 1.25 times the base price with the 5-minute TTL, and 2 times with the 1-hour TTL. ([prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching))

*Implication:* a raw token sum ranks the seats wrongly when cache reads are 95% of the sum. The metric must weight the token kinds and divide by the work that landed. The subscription publishes no weights, so the weights are a proxy until the quota percent calibrates them.

### 3.7 Hosted tracing

**P3.23** Phoenix traces Claude Code with a hook plugin, with model calls, tool calls and token cost for each turn ([Phoenix](https://arize.com/docs/phoenix/integrations/coding-agents/claude-code)). Langfuse traces Claude Code and Codex with lifecycle hooks ([Langfuse](https://langfuse.com/resources/engineering/coding-agent-tracing)).

*Implication:* both send the data to a service outside the host and add hooks inside the CLI. The CLI-only rule ([cli-only-no-mcp.md](../conventions/cli-only-no-mcp.md)) and the isolation rules of [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) rule them out.

### 3.8 Measured methods that cut tokens

These methods feed the follow-ups of the plan. The plan measures first and adopts none of them yet.

| Id | Method | Measured effect | Source |
|---|---|---|---|
| P3.24 | Mask old tool observations | about 50% less cost, the same solve rate as LLM summaries | [arXiv 2508.21433](https://arxiv.org/abs/2508.21433) |
| P3.25 | Compress observations and keep the next action | 33.0% fewer tokens on SWE-bench Verified | [arXiv 2607.02911](https://arxiv.org/abs/2607.02911) |
| P3.26 | Context editing | 84% fewer tokens in a 100-turn evaluation | [context management](https://www.anthropic.com/news/context-management) |
| P3.27 | Filter test and log output with a hook | from tens of thousands of tokens to hundreds | [costs](https://code.claude.com/docs/en/costs) |
| P3.28 | Multi-agent systems | about 15 times the tokens of a chat | [multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system) |

## 4. Ranked gaps

The rank is how much each gap stops the search for the largest consumer.

| Rank | Gap | Evidence | Effect |
|---|---|---|---|
| 1 | No tokens for each turn or each tool call | F1.8, F2.7 | The operator cannot see which tool call, file read or test run grows the context in build and scout (73% of the tokens). |
| 2 | No quota join | F1.11, F1.12, P3.6 | The operator cannot convert tokens to the weekly percent, or see the agy quota at all. |
| 3 | `phase` is a copy of `agent`, and the row has no seat or round purpose | F1.2, F1.3, F2.6 | Retries (25.3%), repair rounds, contract corrections and the advisor sub-steps are not separate. |
| 4 | No cycle or wave on the row, and the token report under-counts | F1.2, F1.5 | The operator joins directory names by hand. The one report misses 29% of the cache reads. |
| 5 | No durable record for runs outside a cycle | F1.13, F1.14 | The tokens of `evolve agent run` reviews are lost. |
| 6 | Partial and unknown counts | F1.9 | 33 of 180 Claude rows are output-only. The agy cache write is never seen. |
| 7 | No collector for codex and ollama | F1.7 | No effect now: 0 calls in the sample. |
| 8 | No landed outcome | F2.5, F2.6 | 29% of the tokens went to PASS cycles that needed a rebase at ship. The rows cannot say if the work landed. |

## 5. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Keep the four token kinds apart and weight them | adopt | P3.21, P3.22: cache reads are 95% of the tokens and a small part of the cost. |
| Cost of each successful task as the headline metric | adopt, as CWU for each shipped cycle | P3.21. PR #1184 already divides by shipped cycles. |
| A record for each turn from the files that the collectors read | adopt | F1.8: the data is already in memory. It covers both families. |
| Claude Code OTel events to a local receiver | defer | P3.1 to P3.5 cover 19% of the tokens. It adds child variables (I-2), a listener and a 5-second loss window. |
| agy headless `stream-json` for usage | defer | P3.14, P3.15: the loop runs agy in a REPL pane, and the field values differ between versions. |
| Statusline `rate_limits` for the weekly percent | adopt, at `shadow` | P3.6: the only per-session source of the subscription unit. |
| GenAI semconv names for the dimensions | adopt for names only | P3.18 to P3.20: the status is Development, and `input` has another meaning. |
| Hosted tracing (Phoenix, Langfuse) | reject | P3.23: an outside service and in-CLI hooks. |

## 6. Refinements to adopt

- **R1.** Write cycle, run id, profile and a derived round purpose on each row, from the bridge request. (F1.2, F1.3)
- **R2.** Let the core declare the round purpose that the bridge cannot derive: a repair round, an audit fix, a recovery and the advisor sub-step. (F2.6, gap 3)
- **R3.** Keep a sidecar record for each turn, with the four counts, the tool names and the context size. (F1.8, F1.10, gap 1)
- **R4.** Append each quota observation to a history file. Add the Claude statusline `rate_limits` as a second source. (F1.12, P3.6, gap 2)
- **R5.** Import the row of each `evolve agent run` into a durable ledger before the workspace goes. (F1.13, gap 5)
- **R6.** Make `evolve tokens report` read `llm-calls.ndjson`, split by phase, seat, purpose and step, and print CWU for each shipped cycle. (F1.5, P3.21, gap 4, gap 8)
- **R7.** Find why the advisor rows fall back to the scrollback peak. (F1.9, gap 6)

## 7. Sources

Local:

- `go/internal/llmcalls/record.go`, `go/internal/bridge/attempt_telemetry.go`, `go/internal/bridge/manifests/*.json`, `go/internal/core/ports.go`
- `go/internal/tokenusage/defaultresolver.go`, `scanner.go`, `agy.go`
- `go/internal/attemptpostmortem/transcript.go`, `record.go`
- `go/internal/bridge/clicontrol/usage.go`, `go/internal/quotastate/windows.go`, `go/internal/usageprobe/windows.go`
- `go/cmd/evolve/cmd_tokens.go`, `cmd_clihealth.go`, `cmd_cycle_models.go`, `registry.go`
- `go/internal/cyclecost/cyclecost.go`, `go/internal/lanetime/read.go`, `go/internal/cli/agentcmd/run.go`
- PR #1184 branch: `go/internal/cyclespend/spend.go`, `go/internal/wavemeasure/cost.go`
- PR #1189 branch: `go/internal/core/cycle_spend_limit.go`
- `.evolve/runs/cycle-2030` to `cycle-2050`: `llm-calls.ndjson`, `run.json`, `ship-error.json`, `signals.ndjson`, `*-prompt.txt`

Outside (accessed 2026-10-27):

- https://code.claude.com/docs/en/monitoring-usage
- https://code.claude.com/docs/en/statusline
- https://code.claude.com/docs/en/costs
- https://code.claude.com/docs/en/headless
- https://platform.claude.com/docs/en/build-with-claude/prompt-caching
- https://docs.rs/ccwhy
- https://pypi.org/project/ccupp/
- https://aident.ai/blog/claude-code-jsonl-token-usage
- https://geminicli.com/docs/cli/telemetry/
- https://github.com/google-antigravity/antigravity-cli/issues/366
- https://github.com/google-antigravity/antigravity-cli/issues/1010
- https://github.com/ptone/scion/issues/2237
- https://github.com/nickbrett1/agy-telemetry
- https://signoz.io/docs/antigravity-cli-monitoring/
- https://codex.danielvaughan.com/2026/04/08/codex-exec-jsonl-reference/
- https://learn.chatgpt.com/docs/config-file/config-advanced
- https://docs.ollama.com/api/generate
- https://www.dash0.com/knowledge/opentelemetry-genai-semantic-conventions-explained
- https://opentelemetry.io/docs/specs/semconv/gen-ai/
- https://opentelemetry.io/docs/specs/semconv/gen-ai/anthropic
- https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans
- https://arxiv.org/abs/2607.12161
- https://arxiv.org/abs/2508.21433
- https://arxiv.org/abs/2607.02911
- https://arize.com/docs/phoenix/integrations/coding-agents/claude-code
- https://langfuse.com/resources/engineering/coding-agent-tracing
- https://www.anthropic.com/news/context-management
- https://anthropic.com/engineering/advanced-tool-use
- https://www.anthropic.com/engineering/multi-agent-research-system
- https://oneuptime.com/blog/post/2026-02-06-file-exporter-opentelemetry-collector/view
