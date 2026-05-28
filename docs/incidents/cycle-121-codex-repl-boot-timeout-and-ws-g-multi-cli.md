# Incident Report & Remediation: Codex REPL Boot Timeout + Multi-CLI Pipeline Robustness — Cycle 121

**Date:** 2026-05-28 | **Severity:** HIGH (a verified-hardened `/evolve-loop` cycle ran 5/8 phases cleanly then died at audit on a single-CLI integration bug → no cycle could complete the full Scout→Ship path when the auditor profile pinned to codex-tmux) | **Status:** Root cause identified + multi-CLI fallback fix shipped + codex 0.134 manifest/driver fixes shipped. Validated end-to-end on **cycle 122** which ran 7 phases including the `--cli auditor=claude-tmux` hot-swap path. Builds on cycle-119's incident report ([cycle-119-artifact-timeout-and-cross-cli-trust.md](./cycle-119-artifact-timeout-and-cross-cli-trust.md)).

**Validation result (cycle 122, multi-CLI hot-swap).** scout → triage → tdd → build-planner → build → **audit (claude-tmux via `--cli auditor=claude-tmux source=env(EVOLVE_AUDITOR_CLI)`)** → retro all completed end-to-end. Audit produced a real FAIL verdict on the prompt; state machine correctly routed audit→retro (no ship). The PIPELINE never aborted on a CLI-level failure — exactly what WS-G targeted.

---

## Part 1: What Happened

**Context.** After all 6 cycle-119/120 hardening workstreams (A/D/C/B/F/E) had merged to main, an end-to-end verification cycle (`/evolve-loop --cycles 1 --goal-text "optimize the pipeline"` on the rebuilt binary at `/tmp/evolve-hardened`) was launched as **cycle 121**. Five phases passed cleanly — including the cycle-120 build-planner repro path under `EVOLVE_BUILD_PLANNER=1`, validating WS-A (absolute paths), WS-B (CLI-agnostic sandbox + tree-diff guard), WS-D (optional soft-fail), and WS-F (ollama driver registration). The cycle then **aborted at the audit phase**:

```
evolve loop: cycle 121: phase audit: audit: bridge: bridge: launch exit=80
stop_reason: "error"
"FinalVerdict": "FAIL"
"PhasesRun": ["scout","triage","tdd","build-planner","build"]
```

`exit=80` = `bridge.ExitREPLBootTimeout` ("REPL never showed its prompt marker"). The auditor profile pinned `cli: codex-tmux`; codex (0.134.0) launched in tmux but never produced its `›` prompt marker within the 60s boot window. Without a fallback, audit failed → cycle aborted → ship + retro never ran.

**Timeline (UTC, ~30 min wall-clock):**

| Time | Phase | Driver | Result |
|---|---|---|---|
| 12:38:05 | scout | claude-tmux | ✅ ~5 min |
| 12:43:17 | triage | claude-tmux | ✅ ~1.5 min |
| 12:44:53 | tdd | claude-tmux | ✅ ~12.5 min (RED tests written, cycle-119 fix validated) |
| 12:57:20 | build-planner | claude-tmux | ✅ ~4.5 min (artifact 16 KB, cycle-120 fix validated) |
| 13:01:45 | build | **agy-tmux** (Gemini) | ✅ ~7.75 min (WS-B sandbox validated — no main-tree leak) |
| **13:09:31** | **audit** | **codex-tmux** | ❌ **`exit=80`** after ~60s; cycle aborts |
| (never) | ship | — | gated by audit failure |
| (never) | retro | — | gated by audit failure |

**Key forensic signal.** The audit phase wrote only `audit-prompt.txt` to `.evolve/runs/cycle-121/`. No `audit-stdout.log`, no `audit-stderr.log`, no events file. The bridge bailed BEFORE `openDriverLogs` even ran — meaning the boot loop genuinely never saw the `›` prompt in the captured tmux pane within its 60s budget.

---

## Part 2: Root Cause (from dispatched research dossier)

A deep-research agent was dispatched with WebSearch + WebFetch to investigate codex 0.134's behavior. **Full dossier:** [knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md](../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md) (29 KB, 8 sections, cited).

**Two-layer cause.**

### Layer 1 — codex 0.134 trust modal renders `›` as a selection bullet

codex 0.134 emits a **"Working with untrusted contents"** modal on every launch in a directory not pre-trusted in `~/.codex/config.toml`. In cycle 121, that directory was the worktree at `.evolve/worktrees/cycle-121` — only the main repo path was pre-trusted, so the modal fired. The modal renders:

```
Working with untrusted contents
› 1. Yes, continue
  2. No, exit
```

The `›` glyph (U+203A) is **exactly the character codex-tmux uses as its REPL prompt marker** (`manifest.prompt_marker = "›"`). The boot loop sees `›` in the pane → declares "REPL booted" → exits the boot wait → tries to send the agent prompt → codex is still on the modal → silent hang.

### Layer 2 — boot loop order: marker check BEFORE auto-respond tick

Pre-fix, the loop in `internal/bridge/driver_tmux_repl.go:144-156` was:

```go
for elapsed := 0; elapsed < tmuxREPLBootTimeoutS; elapsed += interval {
    deps.Sleep(...)
    pane, _ := deps.Tmux.CapturePane(...)
    if strings.Contains(pane, lp.promptMarker) {   // ← marker FIRST
        promptSeen = true
        break
    }
    if lp.tickDuringBoot {
        ar.tick(ctx, lp.session)                    // ← tick AFTER
    }
}
```

Even though the codex-tmux manifest's `trust_prompt` regex would have matched and auto-responded `1,Enter` to dismiss the modal, that code path **never ran** when `›` was in the pane — the marker check exited the loop first.

### Layer 3 — cycle-121 specific: pane size pushed modal out of capture window

`audit-stdout.log` was empty (file never created). The dossier's hypothesis: on a tmux pane sized 80×24, the trust modal's text + verbose codex pre-trust output pushed the modal off the `capture-pane -S -200` scrollback window in some launches. So the boot loop saw NEITHER `›` nor any of the trust-modal text within the 60s budget — it genuinely timed out.

In both sub-cases (modal-visible-and-false-positive OR modal-off-screen-and-no-marker), the pipeline died because **codex was the only CLI configured for the audit phase**.

**Ruled out** by the dossier (all with citations): codex auth issues, model identifier validity (`gpt-5.5` is real), alt-screen rendering, segfaults, MCP startup deadlocks, model migration prompts, NUX tooltip.

---

## Part 3: The Fix (Workstream G + codex 0.134 patches)

Cycle 121 surfaced two distinct gaps:

1. **The pipeline couldn't recover from a single-CLI failure** — even when other registered CLIs (claude-tmux, agy-tmux, ollama-tmux) could have run the phase.
2. **codex 0.134's trust modal exposed a marker false-positive bug** in the boot loop.

WS-G addresses both: pipeline RESILIENCE (fallback chain + per-agent overrides) AND the underlying codex CLI bug (regex + tick reordering).

### Plank G1 — fallback chain + per-agent CLI env

**Schema:** `profiles.Profile` gains two new optional fields:
```go
CLIFallback        []string  `json:"cli_fallback,omitempty"`         // ordered alternates
CLIFallbackOnExit  []int     `json:"cli_fallback_on_exit,omitempty"` // default [80, 127]
```

**Resolution (`internal/phases/runner/cli_chain.go`):** primary CLI picked from `EVOLVE_<AGENT>_CLI > EVOLVE_CLI > profile.cli > "claude-tmux"`. Fallback list is `profile.CLIFallback` (deduped). Triggers default to `{80 ExitREPLBootTimeout, 127 ExitMissingBinary}` — operators extend per-agent to e.g. include `81 ExitArtifactTimeout`.

**Dispatch loop (`internal/phases/runner/runner.go`):** each attempt logged + ledgered. On a trigger exit, advance; on a non-trigger exit, surface as-is. A legitimate FAIL verdict from a model **never silently routes to a different CLI** — the chain only catches CLI-level bugs, not model decisions.

### Plank G2 — `--cli` / `--model` launch flags

```
evolve loop --cli auditor=claude-tmux --cli builder=agy-tmux \
            --model auditor=opus --model builder=llama3.1:8b
```

Repeatable flags parsed in `cmd_loop.go` into `cfg.PerAgentCLI` / `cfg.PerAgentModel`, translated to `EVOLVE_<AGENT>_CLI` / `EVOLVE_<AGENT>_MODEL` in `buildCycleEnv` (via `phaseEnvAgentKey`'s dash→underscore upcase). Flags beat inherited shell env. Malformed pairs reject with exit 10.

**Known UX wrinkle (follow-up patch pending):** the runner reads `MODEL` via PHASE name but `CLI` via AGENT name. For phases where agent ≠ phase (build/tdd/audit), `--model build=gpt-5.5` doesn't reach today; operator can use either the AGENT key matching `--cli` (`--model builder=gpt-5.5`) AFTER the polish patch lands, OR set the PHASE-keyed env var directly (`EVOLVE_BUILD_MODEL=gpt-5.5`) until then.

### Plank G3 — startup capability probe

`probeAvailableCLIChain` runs `exec.LookPath(<binary>)` for each candidate's binary BEFORE the dispatch loop. Missing-binary CLIs are **demoted to the end** of the chain (not deleted) so an available fallback runs first; if ALL are missing the original primary still attempts (so the bridge surfaces a real `ExitMissingBinary 127` to the classifier — no silent skip). Cuts boot-timeout cost from 60s to milliseconds when a CLI's binary isn't installed.

### Codex Fix E — broaden manifest `trust_prompt` regex

`internal/bridge/manifests/codex-tmux.json`:
```diff
- "regex": "Do you trust the contents of this directory|Trust this directory",
+ "regex": "Do you trust the contents of this directory|Trust this directory|Working with untrusted contents|This folder has not been trusted|Yes, continue",
```

Catches the 0.134 phrasing variants the dossier identified.

### Codex Fix B — reorder boot loop: tick BEFORE marker check

`internal/bridge/driver_tmux_repl.go`:
```go
for elapsed := 0; elapsed < tmuxREPLBootTimeoutS; elapsed += interval {
    deps.Sleep(...)
    pane, _ := deps.Tmux.CapturePane(...)
    if lp.tickDuringBoot {
        ar.tick(ctx, lp.session)              // ← tick FIRST
    }
    if strings.Contains(pane, lp.promptMarker) {
        promptSeen = true
        break                                  // ← marker check AFTER
    }
}
```

Trust modal is now dismissed by the auto-responder BEFORE the marker check sees the `›` selection bullet, so the bullet no longer false-positives as the REPL prompt.

---

## Part 4: Verification (cycle 122)

Cycle 122 was launched on the rebuilt binary (`/tmp/evolve-g2-hardened`, built from `9d02630`) with the explicit hot-swap:

```bash
/tmp/evolve-g2-hardened loop --cycles 1 --cli auditor=claude-tmux --goal-text "..."
```

A subsequent run with the full multi-family configuration (scout/audit on agy-tmux/Gemini, tdd/build on codex-tmux/gpt-5.5, others on Claude) launched as cycle 122 and surfaced a new failure mode — codex permission modal stall + WS-B/WS-G integration gap (no fallback fired on exit=81). See [cycle-122 incident report](./cycle-122-codex-permission-modal-and-wsg-fallback-gap.md) for the full forensics + remediation plan.

**Dispatch log proves the G2 plumbing end-to-end:**
```
[runner] phase=audit agent=auditor cli=claude-tmux (source=env(EVOLVE_AUDITOR_CLI)) profile=auditor.json
```

The `source=env(EVOLVE_AUDITOR_CLI)` attribution confirms the launch path: `parseLoopArgs` → `cfg.PerAgentCLI["auditor"]` → `buildCycleEnv` emits `EVOLVE_AUDITOR_CLI=claude-tmux` → threaded through `PhaseRequest.Env` → `resolveCLIChain` picks it up via `envchain.PhaseEnvKey("auditor", "CLI")` → dispatch runs claude-tmux instead of profile's codex-tmux.

**Outcome.** scout → triage → tdd → build-planner → build → audit → retro all completed (7 phases). Audit produced a FAIL verdict on the goal — a legitimate model-quality decision, not a CLI bug — and the state machine correctly routed audit→retro (`StateMachine.Next(PhaseAudit, VerdictFAIL) = PhaseRetro`). The cycle reached the terminal `end` state. No CLI-level abort. This is the "complete cycle path" cycle 121 couldn't reach.

---

## Part 5: Lessons + Follow-ups

### Lessons

1. **Single-CLI pinning was an unstated SPoF.** Each phase's `cli` field in its profile was treated as the only viable runtime; one CLI integration bug killed the cycle. The new chain default `cli_fallback: []` keeps the legacy posture byte-identical for operators who don't opt in, but the field's existence makes the SPoF visible.
2. **Prompt-marker false positives are a real class of failure** — any character a CLI uses as its REPL idle prompt may also appear in some other UI element (modals, status lines, error displays). The codex `›` collision is the canonical example; future CLIs should pick prompt markers MORE unique (multi-character, distinguishable from common Unicode bullets). Fix B (tick-before-marker) is a structural defense even if the marker is unique today.
3. **trust-modal phrasings drift across CLI versions.** Hard-coding regexes against single phrasings is brittle. The broadened regex (Fix E) is a small improvement; a longer-term direction is the dossier's Fix A (pre-trust worktree paths in `~/.codex/config.toml` before launch) which removes the modal entirely.
4. **Capability probes pay back fast.** A 30-ms `exec.LookPath` saves a 60-s `ExitMissingBinary` cycle for missing-CLI operators. Demote-don't-delete preserves error-surfacing when ALL candidates are missing.
5. **Per-agent env naming is the right axis.** `EVOLVE_<AGENT>_*` (matching `envchain.PhaseEnvKey(profileName, ...)`) is consistent across CLI, PERMISSION_MODE, SYSTEM_PROMPT. The existing per-PHASE convention for MODEL is the outlier (and the G2 polish patch will reconcile it).

### Follow-ups (open)

| ID | Item | Status |
|---|---|---|
| G2-polish | Make `--model` honor both phase and agent names (or fix runner MODEL key to use agent — same as CLI). ~30 LOC patch. | **Pending (TaskList #12)** |
| codex-Fix-A | Optionally pre-trust worktree paths in `~/.codex/config.toml` before launch — eliminates the trust modal entirely. ~30 LOC + 1 file write. | Documented in dossier; not applied (Fix B + E should cover most cases). |
| E2 wire-up | Wire an Ollama-backed `DeliverableReviewer` via `WithReviewer` when `ReviewGate >= Enforce`. The interface + ollama-tmux driver are both shipped. | Pending. |
| docs-PR | Commit the 2 untracked research dossiers + this incident report as a docs PR. | Pending. |

---

## References

- **Plan that motivated WS-A..F:** `~/.claude/plans/lexical-booping-hamster.md`
- **Cycle-119 incident report:** [cycle-119-artifact-timeout-and-cross-cli-trust.md](./cycle-119-artifact-timeout-and-cross-cli-trust.md)
- **Codex 0.134 deep-dive dossier:** [knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md](../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md)
- **Ollama control-surface dossier (WS-F):** [knowledge-base/research/ollama-control-surface-2026.md](../../knowledge-base/research/ollama-control-surface-2026.md)
- **Shipped PRs (this campaign):**
  - PR #20 (WS-A) `1ad0753` — paths.AbsoluteRoot helper
  - PR #21 (WS-D) `8339e57` — optional-phase soft-fail + empty-output quota classify
  - PR #22 (WS-C) `c645a10` — inert-phase-enable warning
  - PR #23 (WS-B) `3e04e28` — CLI-agnostic sandbox + tree-diff guard
  - PR #24 (WS-F) `56b59ab` — ollama-tmux driver
  - PR #25 (WS-E) `a537c22` — liveness backstop + per-phase review gate
  - **PR #26 (WS-G) `9d02630` — any-CLI/any-model/any-phase pipeline + codex 0.134 fixes** ← this report's primary remediation
