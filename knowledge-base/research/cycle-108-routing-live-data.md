# Cycle-108 — First live LLM-router run: data + findings (2026-05-27)

> Forensic harvest of the first real `EVOLVE_DYNAMIC_ROUTING=advisory EVOLVE_ROUTING_MODE=llm` cycle.
> Cycle **errored at scout** (`bridge: launch exit=81`, `total_cost_usd: 0` — subscription billing not metered).
> Routing brain proved working end-to-end on Haiku/Max; cycle blocked by a **bridge artifact-path bug**, not a routing fault.

## TL;DR

- ✅ **LLM router proposer works live** — produced valid strict JSON on Haiku 4.5 / Claude Max, sound reasoning.
- ✅ **Scout works** — Sonnet 4.6 / Claude Max, 2m40s, 8 turns, correct task + build plan + acceptance criteria.
- ⛔ **Cycle died at scout with `exit=81` = `ExitArtifactTimeout`** — artifact-path contract bug (below). Blocks *every* real cycle, so we never reached the high-value post-build / post-audit routing decisions.
- Only **1 routing decision** captured (the start transition).

## Root cause of exit=81 (bridge artifact-path mismatch)

| | Path |
|---|---|
| Bridge polls (runner.go:210 `filepath.Join(req.Workspace, hooks.ArtifactFilename)`) | `.evolve/runs/cycle-108/scout-report.md` |
| Scout agent actually wrote | `.evolve/runs/cycle-108/`**`workspace/`**`scout-report.md` |

`req.Workspace` is already the cycle dir; the scout prompt's literal `workspace/scout-report.md` convention made the agent create a **nested `workspace/` subdir**, so the file never appears where the bridge waits → `ExitArtifactTimeout` (81) → orchestrator marks scout failed → cycle errors. Same class as the [[project_v12_2_2_session_handoff]] "workspace-pollution" incident; also why the orchestrator archived 19 "polluted" files at cycle start. **Fix candidates:** (a) scout prompt/skill emit the bare filename (no `workspace/` prefix); (b) runner polls both `<ws>/scout-report.md` and `<ws>/workspace/scout-report.md`; (c) set agent CWD = workspace so `workspace/`-relative writes land correctly. (a) is the cleanest contract fix.

## exit=81 deep-dive (2026-05-27, "investigate deeper")

**It's intermittent agent-compliance variance, not a hard regression:**

| Cycle | scout-report.md location | bridge sees it? |
|---|---|---|
| 103, 104, 107 | `cycle-N/scout-report.md` (root) | ✅ |
| **106, 108** | `cycle-N/workspace/scout-report.md` (subdir) | ❌ timeout |

**Contract divergence:** `agents/evolve-scout.md:134` → `### Workspace File: `workspace/scout-report.md`` (a bash-era `workspace/` scratch-subdir convention; also `workspace/builder-notes.md`, `workspace/agent-mailbox.md`, `workspace/next-cycle-brief.json`). Go runner polls `filepath.Join(req.Workspace, "scout-report.md")` where `req.Workspace` = cycle dir (orchestrator.go:203, no subdir). `auditor.md` has the same split (reads `workspace/*-report.md` inputs L48-49; writes bare `audit-report.md` L186). Models comply inconsistently → some cycles land in the subdir → `ExitArtifactTimeout`(81).

**Durable fix is two-sided (you can't trust an LLM to never make the subdir):**
1. **Doc:** drop the `workspace/` prefix from the *polled artifact* write instruction (scout writes `scout-report.md`). Audit which scratch files (builder-notes, mailbox, brief) the Go side actually expects + where.
2. **Runner:** poll BOTH `<ws>/X` and `<ws>/workspace/X` (fallback), and on timeout emit a louder diagnostic (list what *did* appear) instead of a bare exit=81 — the silent-failure surface here is poor.

## Routing data — start transition (the only decision captured)

**Proposer prompt** (`router-prompt.txt`): bridge interactive-policy block prepended, then the ROUTER prompt. Signals **empty** (no phases completed), `mandatory_spine: scout, build, audit, ship`, optional available: `tester` only, budget $7.

**LLM response (Haiku 4.5 / Max), valid strict JSON:**
```json
{"next_phase":"scout","insert_phases":[],
 "justification":"Start mandatory spine; last cycle PASS with no instability signals; tester insertion deferred pending objective signal."}
```

**Recorded decision after kernel clamp** (`routing-decision-1.json`):
```json
{"next_phase":"scout","skip_phases":["intent"],"reason":"spine:scout"}
```
Ledger: `routing_decision` entry_seq 1875, exit_code 0.

## Tuning findings

1. **Skip the proposer on empty-signal transitions.** At `start` the digest is empty and the spine forces `scout` next; the Haiku call's output was discarded in favor of `reason:"spine:scout"`. Pure wasted spend. Gate proposer invocation on "≥1 objective signal present" (or "next is not fully spine-determined").
2. **Preserve the LLM justification in the recorded decision.** `routing-decision-1.json.reason` = static `"spine:scout"`; the model's `justification` is dropped. The Off→Shadow soak (item 3) needs the would-have-routed rationale to diff — capture it (e.g. `llm_justification` field) whenever the proposer ran.
3. **Optional-phase universe is tiny** (`tester` only). Most "routing" today is the fixed spine; the advisor only meaningfully acts post-build/post-audit (insert tester on `acs_red`/`severity`). Reinforces the design discussion about widening the advisor's remit (see below / ADR TBD).

## Scout output (harvested)

Task proposed: **`parse-proposal-unit-tests` [S, ~3 turns]** — add a direct table-driven `TestParseProposal` (5 happy: plain/fenced/leading-prose/leading+trailing/insert-only; 5 error: empty/no-brace/malformed/`{}`/justification-only). System health at scout time: **4/4 `TestRoutingProposer_*` PASS, full core package green (0.895s)**, HEAD c9302d7, carryover empty. Full report: `.evolve/runs/cycle-108/workspace/scout-report.md`; reflection: `scout-reflection.yaml` (8 turns, no friction).

## Artifacts (on disk, cycle-108)
`router-prompt.txt`, `resolved-prompt.txt`, `routing-decision-1.json`, `scout-prompt.txt`, `tmux-final-scrollback.txt` (router + scout sessions), `workspace/scout-report.md`, `workspace/scout-reflection.yaml`.
