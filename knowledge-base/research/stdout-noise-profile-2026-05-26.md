# Stdout noise profile and filter design

**Date:** 2026-05-26
**Cycle source:** cycle-106 build phase (1.4 MB stream-json)
**Status:** Shipped as `go/internal/logfilter/` (v12.2.0 candidate)

## Why this was needed

Operators tail `.evolve/runs/cycle-N/<phase>-stdout.log` to debug failed
cycles. Files are 0.25 – 1.4 MB each (3 – 4 MB per cycle workspace, 118
workspaces on disk). The actual signal is buried under stream-json
envelopes, empty content-block lifecycle pairs, hook events, and
duplicated tool-result payloads. `tail -100` returns redraw frames
instead of the agent's reasoning.

## Method

Inspected `cycle-106/build-stdout.log` end-to-end with `jq` histograms:

```bash
jq -r 'if .type=="stream_event"
       then "stream_event."+(.event.type//"unknown")
       elif .type=="user" then "user.tool_result"
       elif .type=="assistant" then "assistant.tool_use_or_text"
       else .type+"."+(.subtype//"?") end' build-stdout.log \
  | sort | uniq -c | sort -rn
```

## Results (cycle-106 build-stdout.log, 2008 lines, 1.4 MB)

| Lines | Event | Signal value |
|---|---|---|
| 1294 | `stream_event.content_block_delta` | **None** — the assembled `assistant.message.content[]` carries the same payload fully formed |
| 131 | `stream_event.content_block_start` | None — pure structural marker |
| 131 | `stream_event.content_block_stop` | None — pure structural marker |
| 131 | `assistant` (thinking / text / tool_use) | **Full signal** — the agent's actual thinking, replies, decisions |
|  78 | `user.tool_result` | Partial — the result is also in workspace files; first/last bytes recognizable |
|  57 | `system.status` | Small — state transition markers |
|  57 | `stream_event.message_start` | Usage stats — partially in cyclecost summary |
|  57 | `stream_event.message_stop` | None |
|  57 | `stream_event.message_delta` | Usage diff — partially in cyclecost summary |
|   4 | `system.hook_started` | Lifecycle marker — name+event useful, payload not |
|   4 | `system.hook_response` | Lifecycle marker — exit code useful, full output not (in guards.log) |
|   2 | `system.task_started` / `task_notification` | Rare but unique |
|   1 | `system.init` | None — tool registry is project config |
|   1 | `result.success` | **Full signal** — final cost + summary |
|   1 | `rate_limit_event` | **Full signal** — small but unique |

Inner `stream_event.content_block_delta` subtypes:

| Count | Delta type | Disposition |
|---|---|---|
| 745 | `input_json_delta` | Drop (assembled in `assistant.tool_use.input`) |
| 415 | `thinking_delta` | Drop (assembled in `assistant.thinking.thinking`) |
| 113 | `text_delta` | Drop (assembled in `assistant.text.text`) |
|  21 | `signature_delta` | Drop (opaque crypto, also redundant) |

## The key insight

**The entire `stream_event.*` family — 1727 of 2008 lines (86%) — is
redundant with the assembled `assistant` events.** The stream-json
format emits both the streaming chunks AND the fully assembled message;
the assembled form is strictly more compact and more readable.

This collapses the filter design from "complex multi-rule classifier"
to "drop all stream_event, keep all assistant, compress hooks, truncate
tool_results, drop init."

## Filter rules (shipped)

| Event `type` | Action | Rationale |
|---|---|---|
| `stream_event` (all subtypes) | DROP | Fully redundant with assembled `assistant` events |
| `system.init` | DROP | Tool registry is project config |
| `system.hook_*` | COMPRESS to one line | Full output in `guards.log` |
| `system.status` | KEEP one line | Unique state transition marker |
| `assistant` | KEEP whole (strip `signature` field) | The actual signal |
| `user.tool_result` | TRUNCATE head 200 + tail 100 | Full result in workspace files |
| `result` | KEEP whole | Final cost + summary |
| `rate_limit_event` | KEEP one line | Telemetry |
| unknown | KEEP raw (truncated to 500 chars) | Safety — never silently drop new signal |

For tmux-scrollback lines (plain text, not JSON):

| Pattern | Action |
|---|---|
| Spinner glyphs (`⠋⠙⠹…` and ASCII `|/-\`) | DROP |
| Empty box-border lines (`╭─╮ │ │ ╰─╯`) | DROP |
| Identical consecutive lines | COLLAPSE → `<line> (× N times)` |
| Other plain text | KEEP |

## Measured outcome

| File | Raw | Clean | Retention |
|---|---|---|---|
| `cycle-106/build-stdout.log` (first 200 KB slice) | 200,000 B | 15,281 B | **7.6%** (13× reduction) |

Target was ≤30% retention. Real-world data delivers far better because
86% of bytes were strictly redundant rather than just noisy.

## Why this changed mid-design

Initial plan assumed `stream_event` thinking-token deltas were the only
copy of the agent's reasoning. The jq histogram revealed the `assistant`
events also carry the fully assembled content. This shifted the rule
from "aggregate stream chunks into paragraphs" to "drop stream entirely
and keep the assembled form" — much simpler and more compressive.

## Files shipped

| File | Role |
|---|---|
| `go/internal/logfilter/logfilter.go` | Public `Process(workspace, phase)` + format dispatch |
| `go/internal/logfilter/streamjson.go` | JSON event classifier + formatters per rule table |
| `go/internal/logfilter/plaintext.go` | Plain-text classifier with spinner/box drop + dedup |
| `go/internal/logfilter/logfilter_test.go` | 17 behavioral tests |
| `go/internal/logfilter/edges_test.go` | 18 edge / error-path tests |
| `go/internal/logfilter/testdata/streamjson-input.log` | 200 KB cycle-106 fixture |
| `go/internal/phases/runner/runner.go` | One-line wire-up after `bridge.Launch()` succeeds |
| `go/internal/phases/runner/stdout_filter_test.go` | 4 runner-integration tests (success, failure, off-switch, E2E) |
| `CLAUDE.md` | `EVOLVE_STDOUT_FILTER` row in env-var table |

## What was deliberately NOT done

- Raw file replacement (would break cyclecost — companion-file pattern instead)
- NDJSON event stream (deferred; the clean.txt covers the operator pain)
- Auto-prune of raw after filter (deferred until shadow-mode telemetry confirms safe)
- Per-phase filter tuning (all phases share rules in MVP)
- Feeding filtered output back into agent context (separate token-optimization workstream)

## Caveats for future tuning

- The `truncateMiddle` head/tail values (200/100) were picked for
  recognizability, not measured optimum. If a future tool-result audit
  shows operators routinely need more head, bump it; the slack room
  before this becomes a problem is large.
- The `isSpinnerRune` / `isBorderRune` sets cover what Claude Code's
  current TUI emits. If a CLI ships new TUI glyphs they'll pass through
  as "real text" until added — safe-fail.
