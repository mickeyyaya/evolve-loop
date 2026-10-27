# Plan: each subscription token has a phase, a seat, a round purpose and a step, so the operator can find and cut the largest consumer

- **Status:** proposed, 2026-10-27.
- **Research:** [token-telemetry-2026-10](../research/token-telemetry-2026-10.md). **Decision record:** none yet. If the operator accepts this plan, an ADR records D1 to D10. **Spec of the try record:** [model-attempt-telemetry.md](../architecture/model-attempt-telemetry.md). **Earlier campaign:** [token-telemetry-2026-07](token-telemetry-2026-07.md), [ADR-0071](../architecture/adr/0071-token-telemetry-attribution-and-clean-boot.md). **Related cost policy:** [cost-aware-repair-2026-10](cost-aware-repair-2026-10.md), [ADR-0158](../architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md).

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Metrics](#4-metrics)
- [5. Candidate designs](#5-candidate-designs)
- [6. Decisions](#6-decisions)
- [7. Components](#7-components)
- [8. Order](#8-order)
- [9. Reduction follow-ups](#9-reduction-follow-ups)
- [10. Verification](#10-verification)
- [11. Risks](#11-risks)
- [12. Open questions](#12-open-questions)

## 1. Request

The operator, on 2026-10-27:

> "make sure we have proper telemetry to monitor the subscription token consumption to identify which phase or agent or steps burning the most tokens. Then we can improve the token usage. Always research and investigate first."

The operator rule: each request and each result follows the docs policy, in ASD-STE100.

## 2. Facts

| Fact | Research id |
|---|---|
| The bridge writes one row for each model try to `llm-calls.ndjson`. | F1.1 |
| The row has no cycle, wave, profile, seat or round purpose. `phase` is a copy of `agent`. | F1.2 |
| The bridge request already carries the cycle, the run id, the profile, the try number and the correction directive. | F1.3 |
| `evolve tokens report` reads the phase timing entries, not the row. It missed 29% of the cache reads in cycles 2044 to 2046. | F1.5 |
| The collectors read the usage of each turn and keep only the sum. | F1.8 |
| 33 of 180 Claude rows are output-only. The advisor has 20 of them. agy never reports a cache write. | F1.9 |
| The quota store keeps the latest percent only. agy has no quota parser. | F1.11, F1.12 |
| `evolve agent run` removes its run workspace with its rows. | F1.13 |
| Cache reads are 95.2% of 581.3 M tokens in waves 141 to 143. | F2.1 |
| Build and scout use 73% of the tokens. Build on agy is 46% of the CWU. | F2.2, F2.4 |
| agy uses 80.3% of the tokens. | F2.3 |
| Retries are 25.3%. PASS cycles with a rebase at ship are 29%. The FAIL cycle 2035 is 15.8%. | F2.6 |
| The first prompt is 4 k to 14 k tokens. One agy build call reads about 7.3 M cache tokens. | F2.7 |
| Claude Code OTel gives a record for each call and each tool, for the Claude family only. | P3.1 to P3.5 |
| agy has no OTel export. Its usage fields differ between versions. | P3.13 to P3.15 |
| The statusline gives the 5-hour and 7-day percent for Pro and Max plans. | P3.6 |
| Tokens and cost correlate weakly (r = 0.15). The recommended metric is the cost of each successful task. | P3.21 |

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1. Each token has a phase, a seat and a round purpose | share of row tokens with `cycle`, `profile` and `purpose` set | 100% of the rows written after T1 and T2 land |
| G2. Each token has a step | share of row tokens covered by a turn record | 95% or more for `claude-tmux` and `agy-tmux` |
| G3. The report is complete | `evolve tokens report` total against the sum of `llm-calls.ndjson` for the same cycles | equal (from 71% today, F1.5) |
| G4. The tokens join the quota | waves with a quota percent for each family | each wave after T4, for the families that have a source |
| G5. No run outside a cycle is lost | `evolve agent run` calls with a durable row | 100% |
| G6. No risk to the live pipeline | launch failures caused by telemetry | 0. Telemetry never fails a launch ([internal-tokenusage.md](../architecture/packages/internal-tokenusage.md) invariants). |

Non-goals:

- This plan cuts no tokens. It gives the numbers that the cuts need (§9).
- This plan adds no hosted service and no MCP server.
- This plan changes no routing and no model tier.
- This plan does not add a codex or ollama collector. Those drivers had 0 calls (research gap 7).

## 4. Metrics

Each metric has one home: `evolve tokens report` (T7). The baselines come from waves 141 to 143 (research §2).

| Id | Metric | Definition | Baseline |
|---|---|---|---|
| M1 | CWU | input + w_cw × cache write + w_cr × cache read + w_out × output, in millions. The weights of each family are policy data (T8). The default is w_cw 1.25, w_cr 0.1 and w_out 5 (P3.22). | 97.0 for 248 calls |
| M2 | CWU for each shipped cycle | the CWU of a wave / the cycles of that wave that landed on `main` | 4.85 for each PASS cycle (97.0 / 20). The landed count is not known today (gap 8). |
| M3 | cache-hit ratio | cache read / (input + cache read), the definition of `cmd_tokens.go:153-158` | 96.8% (`agy-tmux` 96.2%) |
| M4 | tokens of each phase for each shipped cycle | the four counts of a phase in a wave / the shipped cycles of the wave | build 15.9 M, scout 5.3 M for each PASS cycle |
| M5 | quota percent for each wave | the 7-day percent at the end of a wave − the percent at its start, for each family. A reset inside the wave adds the percent before the reset. | not measurable today (gap 2) |
| M6 | waste shares | the share of tokens in: failed cycles, rows with `purpose` other than `first`, and cycles that needed a rebase at ship | 15.8%, 25.3%, 29% |

Why CWU and not raw tokens: cache reads are 95.2% of the tokens (F2.1). The study of P3.21 found that tokens and cost correlate weakly. A raw sum ranks the seats by how often they read the cache. CWU ranks them by an estimate of cost.

Why a proxy: the subscriptions publish no weights. When T4 gives M5 for two waves or more, a fit of the quota percent against the four counts of each family calibrates the weights. The fit replaces the default weights in the policy.

## 5. Candidate designs

### 5.1 The candidates

- **A. Extend the current collectors and the row.**
  - Add cycle, run id, profile and round purpose to the row.
  - Keep a turn sidecar from the per-message data that the claude and agy collectors already read.
  - Append each quota observation to a history file, and add the statusline `rate_limits`.
  - Keep a durable ledger for `evolve agent run`.
  - Make one `evolve tokens` report by phase, seat, purpose and step.
- **B. Claude Code OTel and a local receiver.** An `evolve` subcommand receives OTLP/HTTP on `127.0.0.1`. The bridge injects `CLAUDE_CODE_ENABLE_TELEMETRY`, the exporter variables and `OTEL_RESOURCE_ATTRIBUTES` with the cycle, the phase and the seat. agy stays on its collector.
- **C. Hybrid.** A now. B later, as a second source for Claude only, with a cross-check against the transcript sums.
- **D. Hosted traces (Langfuse or Phoenix).** A hook in each CLI sends traces to a service.
- **E. agy headless `stream-json`.** Change the agy transport from the REPL pane to `agy --print --output-format stream-json` and read the usage of each `agent_response` step.

### 5.2 The criteria

Each criterion has a score from 1 (bad) to 5 (good). Family coverage counts twice, because agy is 80.3% of the tokens (F2.3) and has no OTel (P3.13).

| Criterion | What 5 means |
|---|---|
| Depth | phase, seat, round purpose, step (turn) and tool, for each call |
| Family coverage (×2) | both `claude` and `gemini` at the same depth |
| Quota join | the tokens join the subscription percent of each family |
| Build cost | small: no new process, no new protocol |
| Pipeline risk | no change to the launch path, the transport or the child environment |
| Isolation fit | holds [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) I-1 (single launcher) and I-2 (hermetic environment allowlist), no MCP, CLI only, each LLM query through the bridge. A 1 is a veto. |

### 5.3 The scores

| Design | Depth | Family coverage (×2) | Quota join | Build cost | Pipeline risk | Isolation fit | Weighted sum |
|---|---|---|---|---|---|---|---|
| A. Extend the collectors and the row | 4 | 5 (10) | 4 | 3 | 4 | 5 | **30** |
| B. Claude OTel and a local receiver | 3 | 2 (4) | 2 | 2 | 2 | 2 | 15 |
| C. Hybrid, A then B | 5 | 5 (10) | 4 | 2 | 3 | 3 | 27 |
| D. Hosted traces | 4 | 2 (4) | 1 | 3 | 2 | 1 (veto) | 15, rejected |
| E. agy headless `stream-json` | 4 | 4 (8) | 1 | 3 | 1 | 4 | 21 |

How each score comes from the facts:

- **A.** Depth is 4, not 5. The claude transcript gives the tool names of each turn (F1.10), but the agy store has no published schema for tool names (Q1). Coverage is 5, because both collectors already read the data of each turn (F1.8). The risk is 4: the row is additive and the turn record reads files that the collectors already read after the launch. The isolation fit is 5: no new child variable, no listener, no outside service.
- **B.** OTel covers the Claude family only: 19% of the tokens (F2.3, P3.13). OTel gives no quota percent (P3.6). It needs a receiver process and per-dispatch child variables. Under I-2, a per-dispatch value is a launcher variable, which the allowlist refuses today, so the change touches `hermetic_env.go` on the fence. The 5-second log flush can lose the last batch of a pane that stops (P3.1).
- **C.** C adds the depth of OTel for Claude: `query_source`, `agent_id` and the size of each tool result (P3.3, P3.4). It pays the cost and the isolation change of B for 19% of the tokens.
- **D.** The data leaves the host, and a hook runs inside each CLI. The CLI-only rule and I-2 veto it (P3.23).
- **E.** The transport change hits the driver of 80% of the tokens. agy versions report different or empty usage fields (P3.14, P3.15). It gives no quota percent.

**Why A wins.** A is the only design with full family coverage at a low risk. Coverage decides: a design that sees Claude only measures one fifth of the tokens. A also costs least, because the collectors already hold the data (F1.8). C scores 3 points less, because its extra depth covers 19% of the tokens and costs an isolation change.

**What changes our mind.** We start B as the second stage of C if one of these occurs:

- agy ships an OTLP export (issue 366 closes), so OTel covers both families.
- The Claude share of the tokens rises above 50% for two waves.
- A Claude Code release breaks the transcript reader, and the transcript stays without a public contract (P3.10).

**Worked example.** Cycle 2040 is a PASS cycle with `GIT_FLEET_REBASE_NEEDED` at ship. Today, its one build row says `agent=build`, `attempt` 1, `cli=agy-tmux` and 8.76 M cache read, and nothing more. With A, the row also says `cycle=2040`, `profile=builder` and `purpose=first`. T3 gives each turn of the agy build call: the turn index and the cache read of the turn.

The report of wave 143 then prints the CWU of build for each shipped cycle. It splits retries from first tries, and it lists the turns with the largest growth of the cache read. With T4, it also prints the 7-day percent that the wave used.

## 6. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | The bridge writes `cycle`, `run_id`, `profile` and a derived `purpose` on each row, from the current request. | R1, F1.3 |
| D2 | The core declares the purpose that the bridge cannot derive: repair, audit fix, recovery and the advisor sub-step. | R2, F2.6 |
| D3 | A turn sidecar `llm-turns.ndjson` holds one line for each turn, keyed by `call_id`. The row stays one line for each call. | R3, F1.8 |
| D4 | Each quota observation is appended to a history file. The current snapshot file stays for its readers. | R4, F1.12 |
| D5 | The Claude statusline `rate_limits` is a second quota source, at `shadow`. | R4, P3.6 |
| D6 | `evolve agent run` imports its rows into a durable ledger before it removes the workspace. | R5, F1.13 |
| D7 | `evolve tokens report` reads `llm-calls.ndjson` and owns the metrics M1 to M6. No second report. | R6, F1.5 |
| D8 | The CWU weights of each family are policy data, with the defaults of M1. | P3.21, P3.22 |
| D9 | Find and fix the cause of the output-only advisor rows before a rank of the seats uses them. | R7, F1.9 |
| D10 | Use the GenAI semconv names for the dimensions in the report output. Keep the four counts of the row as they are. | P3.18 to P3.20 |

The row keeps no cycle outcome. The ledger is append-only, and the outcome of a cycle is known only after the last call of the cycle. The report joins the outcome from `run.json` and the ship result (T7). This choice replaces the "outcome on the row" option, because a later update of an append-only row is not allowed.

Each component lands at `shadow`: it writes its data and the report shows it with the mark `shadow`. At `enforce`, an absent field fires a signal, and the report uses the data in its headline metrics. The flip follows one wave with complete data and an operator decision ([ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md)).

## 7. Components

The draft inbox items carry the full acceptance. Each id is the inbox id.

| Id | Component | Closes gap | Files | Acceptance (red test first) | Stage | Route |
|---|---|---|---|---|---|---|
| T1 `token-row-attribution-from-request` | D1 | 3 (part), 4 | `go/internal/llmcalls/record.go`, `go/internal/bridge/attempt_telemetry.go` | A row from a request with `Cycle` 2040, `Profile` builder and `Attempt` 2 has `cycle` 2040, `profile` builder and `purpose` `retry`. A request with a correction directive gives `contract_correction`. An old row decodes unchanged. | shadow, then enforce: `TOKENS_ROW_UNATTRIBUTED` when a cycle row has no `cycle` | lane |
| T2 `token-row-declared-purpose` | D2 | 3 | `go/internal/core/ports.go`, the repair, audit-fix and recovery call sites in `go/internal/core`, the advisor launch | A repair round of tdd writes `purpose` `repair`. An `audit_fix` patch writes `audit_fix`. An advisor plan writes `router` with `step` `plan`. | shadow, then enforce | console (fence: `ports.go`, `integrity_surface.go:126`) |
| T3 `token-turn-sidecar` | D3 | 1 | `go/internal/tokenusage/scanner.go`, `go/internal/tokenusage/agy.go`, a turn type in `go/internal/llmcalls`, `go/internal/bridge/attempt_telemetry.go` | A claude transcript fixture with 3 turns gives 3 lines with the counts and the tool names of each turn, in time order. The cycle-1967 agy fixture gives 111 lines whose sum equals the row. A call with more than the line cap writes the cap and a `truncated` count. | shadow, then enforce | lane |
| T4 `usage-observation-history` | D4 | 2 | `go/internal/usageprobe/windows.go`, a history file beside it | Two probes write two lines to `.evolve/usage-history.ndjson`, and `usage-windows.json` still holds the latest only. A write failure warns and does not fail the probe. | shadow, then enforce | lane |
| T5 `claude-statusline-rate-limits` | D5 | 2 | an `evolve usage statusline` verb in `go/cmd/evolve`, the statusline configuration of the `claude-tmux` pane | A statusline JSON with `rate_limits.seven_day.used_percentage` 41.5 appends one history line with source `statusline`. A JSON without `rate_limits` appends nothing and exits 0. | shadow only, until Q3 is closed | console (the pane setting is driver data on the launch path) |
| T6 `agent-run-durable-token-ledger` | D6 | 5 | `go/internal/cli/agentcmd/run.go` | A run without `--keep` leaves its rows in `.evolve/agent-runs/llm-calls.ndjson` with the run label. A second import of the same run adds no row (`llmcalls.Import`). | shadow, then enforce | lane |
| T7 `tokens-report-from-llm-calls` | D7, D10 | 4, 8 | `go/cmd/evolve/cmd_tokens.go`, a report package under `go/internal` | On the cycles 2044 to 2046 fixture, the total equals the row sum (26.25 M cache read). `--by phase,profile,purpose,step` splits it. The report prints M1 to M6 and the share of unmeasured rows. A wave with no landed cycle prints M2 as not defined, not as 0. | shadow, then enforce: the old phase-timing view stays as `--source timing` until the flip | lane |
| T8 `token-weights-policy-key` | D8 | 4 | `go/internal/policy/strict_keys.go` and the policy reader | `telemetry.token_weights.<family>` with four weights loads. An unknown family or a negative weight fails the strict load with the key name. | enforce at once (a key, no behavior) | console (fence: `strict_keys.go`) |
| T9 `claude-advisor-rows-measured` | D9 | 6 | `go/internal/tokenusage/scanner.go`, `go/internal/tokenusage/locate.go` | A recorded advisor transcript fixture resolves to `transcript`, not `scrollback_peak`. The agy cache-write field is found or Q2 records that it does not exist. | enforce at once (a fix) | lane |

T2, T5 and T8 are console items. Each one changes a fence path or the launch path. The other six are lane items: their files are not on the fence (`go/internal/guards/integrity_surface.go`), and `attempt_telemetry.go` is not on it.

## 8. Order

1. T1. It is cheap, and each later component and report needs the cycle and the profile on the row.
2. T9. A rank of the seats needs complete advisor rows first.
3. T7 with T8. The operator gets a correct report on the current data, with CWU. T7 reads `cyclespend.Sum` from PR #1184 when that PR lands, so the two cannot disagree.
4. T4. The history starts early, because M5 needs two waves of data to calibrate the weights.
5. T6. It is small and closes the loss of the review runs.
6. T3. It is the largest lane item, and it answers the "which step" part of the request.
7. T2. It needs a console window, and T1 already gives `first`, `retry` and `contract_correction`.
8. T5. It waits for Q3.

After T1, T9, T7 and T4 run one wave at `shadow`, the operator reads the report and flips them to `enforce`. T3, T6 and T2 follow the same rule.

## 9. Reduction follow-ups

The data decides each cut. No cut starts before its metric has one wave of data. Each follow-up becomes an inbox item only when its trigger holds.

| Id | Candidate | Metric that decides | Trigger to start | Candidate methods |
|---|---|---|---|---|
| RF1 | Build turn count and context growth on agy | turns for each build call; cache read of each turn against the turn index (T3) | build on agy stays above 40% of the CWU of a wave | a mask on old observations (P3.24), compressed observations (P3.25), a filter on test and log output (P3.27) |
| RF2 | Retry rounds | M6 retry share by `purpose` (T1, T2) | the retry share stays above 15% after the ADR-0158 items land | owned by [cost-aware-repair-2026-10](cost-aware-repair-2026-10.md); this plan only measures |
| RF3 | Scout and triage context | CWU of scout and triage for each shipped cycle; the tool names of their largest turns (T3) | scout and triage together stay above 20% of the CWU | a smaller first context, a code index query in place of a file read |
| RF4 | Cache-hit ratio and cache writes | M3 by family and seat; cache write by seat | M3 for a seat falls below 90%, or tdd stays the largest cache writer | a stable prompt prefix, the 5-minute TTL in place of the 1-hour TTL (P3.22) |
| RF5 | Rework at ship | the CWU of cycles with `GIT_FLEET_REBASE_NEEDED`, and if they landed (T7) | the rework share stays above 15% | owned by the landing track ([ADR-0134](../architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md)) |

## 10. Verification

| Goal | Before (measured 2026-10-27) | After (how to measure) |
|---|---|---|
| G1 | 0% of the row tokens have `cycle`, `profile` or `purpose` | `evolve tokens report --by profile,purpose` on the first wave after T1 and T2; the share with the fields set |
| G2 | 0% have a turn record | the sum of `llm-turns.ndjson` against the row, for each call, on one wave |
| G3 | the report gave 18.60 M cache read against 26.25 M in the row (cycles 2044 to 2046) | the same three cycles with T7: the two numbers are equal |
| G4 | no quota percent for any wave | M5 for each family on the first wave after T4 |
| G5 | the rows of each `evolve agent run` are removed | the commit gate review runs of one week are in the durable ledger |
| G6 | 0 launch failures from telemetry | `BRIDGE_TELEMETRY_APPEND_FAILED` and launch exits on the shadow wave stay at 0 |

## 11. Risks

| Risk | Control |
|---|---|
| The turn sidecar is large. A build call on agy has about 111 generations. | One short line for each turn and a line cap for each call, with a `truncated` count (T3). |
| The agy store has no published schema, and a vendor issue calls it unreliable (P3.14). | T3 checks that the sum of the turns equals the row. A difference fires a WARN and marks the call `partial`. |
| The CWU weights give a wrong rank. | The weights are policy data. M5 calibrates them after two waves. The report prints raw counts beside CWU. |
| A telemetry error fails a launch. | Each writer warns and returns, as the current writer does (`attempt_telemetry.go:134-136`). |
| The statusline command writes from inside the pane sandbox. | T5 stays at `shadow` until Q3 is closed. No other component depends on it. |
| Two reports disagree. | T7 replaces the timing view at `enforce`. It reads `cyclespend.Sum` for the cycle totals. |

## 12. Open questions

- **Q1.** Does the agy `gen_metadata` row hold the tool names of a generation? If not, agy turns have counts and no tool names. A tool split for agy then needs the headless `stream-json` of design E.
- **Q2.** Which agy field holds the cache write, if any? Today all agy rows report 0 (F1.9).
- **Q3.** Can the `claude-tmux` pane run a statusline command that writes outside its cell? The sandbox grants no write outside the cell. A file inside the cell that the bridge reads after the launch is the likely answer.
- **Q4.** Which source gives the agy quota percent? `clicontrol` has no agy parser (F1.11). Without it, M5 covers the Claude family only.
- **Q5.** Does the operator want an ADR for D1 to D10 before T1 starts, or after the shadow wave?
