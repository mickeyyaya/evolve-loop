# Incident Report & Remediation: Codex Permission Modal Stall + WS-B/WS-G Integration Gap — Cycle 122

**Date:** 2026-05-28 | **Severity:** HIGH (the first live test of the multi-workstream stack from cycles 119-121 ran scout + triage cleanly on the multi-CLI fan-out, then died at TDD on a single-CLI integration bug — and unlike cycle 121, **no fallback fired**, exposing that two adjacent workstreams shipped without an integration test of their shared seam) | **Status:** Root cause identified + three integrated fixes designed and approved (Fix 1 codex pre-trust + Fix 2 fallback-list extension + Fix 3 observer auto-spawn). This report ships in commit 1 of a 4-commit remediation; code fixes follow in commits 2-4. Verification scheduled as cycle 123 against the same multi-CLI fan-out that failed here. Builds on [cycle-121 incident report](./cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md) and [ADR-0029 CLI fallback chain](../architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md).

**Failure summary (cycle 122, multi-CLI fan-out).** scout (agy-tmux) → triage (claude-tmux) → **tdd (codex-tmux) ABORTED `exit=81 core: bridge artifact timeout`** after ~10 min. No fallback to claude-tmux or agy-tmux was attempted (WS-G's default trigger list `[80, 127]` doesn't include 81). PhasesRun: `[scout, triage]`. FinalVerdict for those phases was PASS but the cycle stopped with `stop_reason=error`. The "any-CLI/any-model/any-phase always executes" invariant from ADR-0029 was silently violated.

---

## Part 1: What Happened

**Context.** After all 7 cycle-119/120/121 hardening PRs (WS-A through WS-G) merged to `main` on 2026-05-27/28, the user requested the first live end-to-end test of the multi-CLI stack with the cycle-122 launch:

```bash
EVOLVE_SCOUT_MODEL=gemini-3.5-flash EVOLVE_AUDIT_MODEL=gemini-3.5-flash \
  EVOLVE_TDD_MODEL=gpt-5.5 EVOLVE_BUILD_MODEL=gpt-5.5 \
  EVOLVE_GO_BIN=/tmp/evolve-g2-hardened \
  EVOLVE_BUILD_PLANNER=1 \
  /tmp/evolve-g2-hardened loop --cycles 1 \
    --cli scout=agy-tmux --cli auditor=agy-tmux \
    --cli tdd-engineer=codex-tmux --cli builder=codex-tmux \
    --goal-text "investigate latest citations on context window management in multi-agent and multi-model environments and propose how to apply them to this evolve-loop pipeline"
```

Three LLM families (Google scout/audit, OpenAI tdd/build, Anthropic triage/build-planner/retro) routed via WS-G's `--cli` flags. The dispatcher launched 2026-05-28 at 07:36 UTC as PID 71277.

**Timeline (UTC, ~16 min total wall-clock).**

| Time | Phase | Driver | Outcome |
|---|---|---|---|
| 07:36:28 | scout | agy-tmux (Gemini) | ✅ ~2.5 min — scout-report.md produced, 87 KB events file |
| 07:39:09 | triage | claude-tmux | ✅ ~2.5 min — triage-report.md produced with `top_n` decisions |
| 07:41:43 | tdd | codex-tmux (gpt-5.5) | ❌ HUNG at codex permission modal, then `exit=81 bridge artifact timeout` |
| 07:52:00 | (loop exit) | — | `stop_reason=error`, PhasesRun=[scout, triage] |
| (never) | build-planner / build / audit / ship / retro | — | gated by tdd failure with no fallback |

**The smoking-gun signal — captured live in the tmux pane `evolve-bridge-codex-c122-tdd-pid71277-1779954103`:**

```
• Ran go test ./internal/compactor ./internal/phases/build ./internal/phases/runner -run '...' -count=1
  └ FAIL    ./internal/compactor [setup failed]
    open /Users/danleemh/Library/Caches/go-build/.../1abf73...d: operation not permitted

• The first RED run hit the sandboxed Go build cache, not the tests themselves.
  I'm rerunning with GOCACHE inside an allowed temp directory so the failure
  signal comes from the test contract.

• Ran env GOCACHE=/private/tmp/evolve-cycle-122-go-cache go test ... -count=1
  └ FAIL    github.com/.../internal/phases/runner    0.415s
    FAIL                                                  ← desired RED outcome

• Running env GOCACHE=/private/tmp/evolve-cycle-122-go-cache go test ...
  > /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/cycle-122/test-red-output.txt 2>&1

  Would you like to run the following command?

  Reason: Do you want to allow writing the TDD RED output artifact into the
          cycle workspace outside the writable worktree?

  $ env GOCACHE=/private/tmp/evolve-cycle-122-go-cache go test ... -count=1
    > /Users/danleemh/ai/claude/evolve-loop/.evolve/runs/cycle-122/test-red-output.txt 2>&1

› 1. Yes, proceed (y)
  2. Yes, and don't ask again for commands that start with `env GOCACHE=...`
  3. No, and tell Codex what to do differently (esc)

  Press enter to confirm or esc to cancel
```

The codex agent correctly chose option 1 (the `›` selection bullet points to it). But **no Enter keystroke was sent**, because the agent has no way to physically press a key. After ~10 min of pane idleness, the bridge's artifact-timeout fired (`tdd-report.md` never appeared) and the cycle aborted with `exit=81 core: bridge artifact timeout`.

**Forensic state preserved.** Per the approved plan, cycle-122 artifacts (`.evolve/runs/cycle-122/`, `.evolve/worktrees/cycle-122/`, `.evolve/cycle-state.json` with `phase=tdd`, the live tmux session) stay intact across the 4 remediation commits so each fix can be re-anchored against this evidence. They are sealed via `evolve cycle reset` only at verification step V1.

---

## Part 2: Root Cause

Three independent flaws stacked to produce the failure. Each is documented in the [approved plan](~/.claude/plans/iterative-chasing-thunder.md). The order matters — F1 is the trigger, F2 is the silent-violation amplifier, F3 is the missed early-detection.

### Flaw F1 — Codex per-cycle workspace path NOT pre-trusted

The codex 0.134.0 binary at `/opt/homebrew/Caskroom/codex/0.134.0/codex-aarch64-apple-darwin` has **its own permission system** (separate from the bridge's `sandbox-exec`/`bwrap` host sandbox) that prompts when a command writes outside its `cwd` boundary. The `~/.codex/config.toml` pre-trusts these paths:

```toml
[projects."/Users/danleemh/ai/apps/kids-math"]
trust_level = "trusted"
[projects."/Users/danleemh/ai/claude/evolve-loop"]
trust_level = "trusted"
[projects."/Users/danleemh/.claude/plugins/marketplaces/evolve-loop"]
trust_level = "trusted"
```

But the cycle's per-run paths — `.evolve/runs/cycle-122/` (the workspace where reports + events go) and `.evolve/worktrees/cycle-122/` (the writable worktree where source edits go) — are **NOT** listed. So when the agent tried to redirect `go test > .../runs/cycle-122/test-red-output.txt`, codex's permission layer flagged "writing to cycle workspace outside the writable worktree" and rendered the modal.

The **bridge's** host sandbox profile (`go/internal/bridge/sandbox_wrap.go:89-139`) correctly grants both `req.Workspace` AND `req.Worktree` as writable to the codex subprocess — so the host sandbox is not the constraint. The constraint is **inside codex itself**.

**This was anticipated.** The cycle-121 research dossier ([`knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md`](../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md)) listed "Fix A — pre-trust worktree paths in `~/.codex/config.toml`" as a known-but-deferred remediation. Cycle-122 made the deferral cost concrete.

### Flaw F2 — WS-G fallback chain excludes `exit=81`

The fix for F1 alone would prevent the modal from appearing in steady state. But the deeper failure is that even though the bridge's safety net DID fire (the artifact timeout from WS-B), **no recovery happened** because the WS-G fallback trigger list defaults to `[80, 127]` — `ExitREPLBootTimeout` and `ExitMissingBinary`. Exit code 81 (`ExitArtifactTimeout`, defined in `go/internal/core/errors.go` as part of WS-B) is not in that list.

The dispatch log proves the gap. The cycle-122 logs show only one tdd attempt — no `[runner] phase=tdd ... source=fallback(...)` line ever appeared. The chain code at `go/internal/phases/runner/runner.go` saw exit=81, looked it up in `triggers={80, 127}`, found no match, and surfaced the failure as-is — exactly the documented design when the exit code "looks like a model decision, not a CLI bug." But 81 IS a CLI-side hang, not a model verdict; it just wasn't in the trigger list.

**This is the most damning finding of the incident.** WS-B (artifact timeout, PR #23) and WS-G (fallback chain, PR #26) shipped in adjacent PRs in the same session, both reviewed in isolation. Neither test asked "does WS-G see WS-B's new exit code?" — and the answer was no. The two workstreams ship correct unit behavior individually, but their integration was never asserted.

### Flaw F3 — Phase-observer is feature-complete but NOT auto-spawned by `evolve loop`

A separate safety net — the phase-observer stall detector — would have killed the hung tdd process at the 600s `EVOLVE_OBSERVER_STALL_S` mark (or 90s if a `FileNeverCreatedGraceS` were configured), well before the bridge's coarse artifact-timeout. But the observer never ran, because:

| Layer | Status | Evidence |
|---|---|---|
| `go/internal/phaseobserver/phaseobserver.go` | ✅ implemented | StallS + MaxNoProgressS + NudgeS all coded |
| `go/cmd/evolve/cmd_phase_observer.go` | ✅ usable as manual `evolve phase-observer` subcommand | Reads `EVOLVE_OBSERVER_STALL_S`, etc. |
| **`go/cmd/evolve/cmd_loop.go:293` orchestrator startup** | ❌ **does not spawn observer** | `orch.RunCycle(context.Background(), req)` — no goroutine launch, no `WithObserver()` |
| `go/internal/core/orchestrator.go:175-186` | ❌ no observer field, only `reviewer: noopReviewer{}` default | Composition root never wires observer |
| `archive/legacy/scripts/dispatch/run-cycle.sh:726-732` (legacy bash dispatcher) | ✅ background-spawned `phase-observer.sh` | This is the pre-v12 behavior the Go port silently dropped |

The [`CLAUDE.md` env-var table](../../CLAUDE.md) reads `EVOLVE_OBSERVER_ENFORCE=1 default-on since v10.18.0` — that documentation is **technically true for the bash dispatcher and the standalone subcommand, but factually false for the modern Go `evolve loop` path**. The v12.0.0 flag day (which deleted the bash dispatcher) restored the observer code as a manual subcommand and forgot to re-wire the auto-spawn at the orchestrator startup. Cycle 122 is the first cycle where that silent regression actually bit, because cycle 121's `exit=80` failure was caught by WS-G's chain (which includes 80) rather than needing the observer.

Additionally, two cousin features WS-E1 `MaxNoProgressS` (babbling-livelock detector) and WS-E2 `DeliverableReviewer.WithReviewer` are also defined-but-unwired in the autonomous loop — neither would have helped here (tdd was waiting on a keystroke, not babbling), but the same composition-root gap explains all three.

### How F1 + F2 + F3 stacked into the visible failure

```
        codex-tmux tdd starts (07:41:43 UTC)
               │
               ▼
    agent runs `go test > <workspace>/test-red-output.txt`
               │
               ▼
   codex permission layer sees write outside worktree ─── F1: workspace not pre-trusted
               │
               ▼
    codex renders "Press enter to confirm" modal ─── F4 (parking spot, Part 3):
               │                                          EVOLVE_INTERACTIVE_POLICY
               │                                          doesn't reach CLI-native UI
               ▼
    agent picks option 1 but cannot inject keystroke
               │
               ▼
    bridge sees no NDJSON event for ~10 min ─── F3: observer would have caught
               │                                     this at 90s but is unwired
               ▼
    bridge artifact-timeout fires `exit=81` ── (WS-B safety net works as designed)
               │
               ▼
    runner checks triggers={80, 127}, no match ─── F2: WS-G default trigger list
               │                                        doesn't include 81
               ▼
    cycle aborts, no fallback CLI tried ─── multi-CLI invariant SILENTLY VIOLATED
```

---

## Part 3: The Fix (three integrated commits)

The [approved plan](~/.claude/plans/iterative-chasing-thunder.md) ships three commits, each targeting one flaw, sequenced to validate fix interaction at every step. F4 (interactive policy + CLI modals) and F5 (dispatcher wall-clock timeout) are intentionally deferred — F1 + F3 together remove the cycle-122 trigger, so F4's deeper architecture (a `KindKeystroke` envelope, per Explore Agent #2's Seam 3 analysis) is worth a follow-up cycle but not urgent. F5 is similarly subsumed by F3's per-phase ceiling.

### Fix 1 — Pre-trust workspace + worktree in codex-tmux launch (defuses F1)

**Files:** `go/internal/bridge/driver_codextmux.go` (new `pretrustCodexProjects` helper invoked before `runTmuxREPL`); `go/internal/bridge/driver_codextmux_pretrust_test.go` (NEW: RED + GREEN). **Reuse:** existing `cfg.Worktree` / `cfg.Workspace` from `bridge.LaunchIntent`.

The driver writes a TOML-safe per-cycle merge into `~/.codex/config.toml`:

```toml
[projects."<cfg.Worktree>"]
trust_level = "trusted"
[projects."<cfg.Workspace>"]
trust_level = "trusted"
```

File-locked atomic merge so concurrent driver launches don't trample. Reverse-cleanup is optional (codex tolerates stale entries; they accumulate but don't break anything).

This is **codex Fix A from the cycle-121 research dossier**, deferred at the time because WS-G's regex-based trust-modal handling (Fix E) was thought sufficient. Cycle-122 proved it isn't — Fix E catches the `Working with untrusted contents` modal at boot but doesn't catch the *runtime* workspace-write modal that fires when the agent shells out a command with a redirect to the workspace path.

### Fix 2 — Extend WS-G default fallback trigger list (defuses F2)

**Files:** `go/internal/config/config.go` (1-line default list edit); `go/internal/phases/runner/runner_fallback_test.go` (NEW cross-workstream contract test); `CLAUDE.md` row "WS-G default fallback triggers".

Two changes:

1. Default `cli_fallback_on_exit` extends from `[80, 127]` to `[80, 81, 124, 127]`:
   - `80` = `ExitREPLBootTimeout` (existing — WS-G original)
   - `81` = `ExitArtifactTimeout` (NEW — WS-B's signal; the cycle-122 case)
   - `124` = coreutils `timeout(1)` exit code (defensive — if anything wraps a CLI in `timeout` and trips the limit, retry on another CLI)
   - `127` = `ExitMissingBinary` (existing — WS-G original)

2. A contract test in `go/internal/phases/runner/runner_fallback_test.go` asserts: when the bridge returns `core.ErrArtifactTimeout` from the primary CLI, the runner advances to the next CLI in the resolved chain. This is the WS-B↔WS-G integration test the workstreams should have shipped with originally. The test instantiates a fake bridge that returns ErrArtifactTimeout on attempt 1 and success on attempt 2, then asserts both: (a) attempt 2 happened, (b) the ledger has two phase-attempt entries with the right CLI attribution.

### Fix 3 — Auto-spawn phase-observer goroutine from `evolve loop` (defuses F3)

**Files:** `go/internal/core/orchestrator.go` (add `startPhaseObserver` invoked from `RunCycle`); `go/internal/phaseobserver/phaseobserver.go` (add `FileNeverCreatedGraceS` config field + handling in `tail()` at line 333-335); `go/cmd/evolve/cmd_cycle.go:295-304` (wire `WithObserver(...)` option); `go/internal/core/orchestrator.go:175-186` (add `noopObserver{}` default for byte-identical opt-out). **Reuse:** `phaseobserver.Run`, `phaseobserver.Config` — already implemented.

The composition-root wiring restores the pre-v12 bash-dispatcher behavior. For each phase, before the runner launches, a goroutine is spawned equivalent to `evolve phase-observer --enforce` watching `<workspace>/<agent>-stdout.log`. The new `FileNeverCreatedGraceS` (default 90s) handles cycle-122's exact shape: after the phase starts, if the log file doesn't appear within 90s, emit `stuck_no_output` INCIDENT and SIGTERM the subagent. This kills the hung tdd process at minute 1:30 instead of waiting 10 min for the bridge-side artifact-timeout to fire.

**Rollout flag:** `EVOLVE_OBSERVER_AUTOSPAWN=1` default-on, opt-out `=0` for the rollback hatch. Documented in ADR-0030.

---

## Part 4: Lessons + Follow-ups

### Lessons

1. **Cross-workstream integration tests are not optional when adjacent PRs share a control plane.** WS-B added a new exit code; WS-G dispatched on exit codes; neither tested the seam. The contract test in Fix 2 is the deliverable that pins the seam — and the lesson is to ship at least one such test per pair of workstreams that touch the same control surface (here, the exit-code → recovery routing).

2. **CLI-native UI modals are a distinct class of failure from agent-emitted prompts.** `EVOLVE_INTERACTIVE_POLICY` was designed to defuse `AskUserQuestion` (a tool the model can choose to invoke). Codex's runtime workspace-write modal lives in codex's UI layer, never reaches the model, and cannot be resolved by a policy block in the prompt body. The two need distinct defenses: (a) eliminate-at-source via driver preflight config (Fix 1 for codex), and/or (b) keystroke injection via a new bridge envelope (deferred — F4 parking spot).

3. **The v12 flag day silently regressed phase-observer auto-spawn.** Deleting the bash dispatcher dropped an auto-launch behavior that the Go port preserved as a manual subcommand only. The CLAUDE.md table reads as if the observer is auto-on, but the autonomous Go loop never spawns it. This is the "documented behavior diverges from shipped behavior" footgun — and the lesson is to validate documentation against the autonomous path during major refactors.

4. **`exit=81` was the correct safety-net signal — but a coarse one.** The bridge artifact-timeout (10 min) is the right backstop when no other detector fires, but it's wall-clock budget, not signal-of-stall. The phase-observer (Fix 3) provides the FIRST line of stall detection at 90 s; the artifact-timeout becomes the second line at 10 min; WS-G's fallback (Fix 2) becomes the third line that recovers rather than aborts. Three concentric belts, not one.

5. **Forensic-alive cycles are valuable.** Per the operator decision, cycle-122 stays unsealed across the four remediation commits so each fix can be tested against the real failure evidence (the tmux pane content, the per-phase NDJSON, the exact path strings). The reset happens only at verification step V1. This pattern is worth codifying for future high-information incidents.

### Follow-ups (open)

| ID | Item | Status |
|---|---|---|
| F4-keystroke | Add `KindKeystroke` envelope to the bridge inbox + driver drain handler for raw keystroke injection (e.g., `\r`). Operator can then send via `evolve bridge send --kind=keystroke --body=$'\r'`. ~10 LOC + ADR-0023 addendum. | Deferred — Fix 1 prevents the modal from appearing in steady state. |
| F5-wallclock | Add `evolve loop --max-duration <seconds>` flag with `context.WithTimeout` propagated to all phases. Belt-and-braces above Fix 3's per-phase ceiling. | Deferred — Fix 3's auto-spawn provides per-phase ceiling sufficient for the cycle-122 shape. |
| F6-sessionfile | Resolve the `/save-session` + `session-end.js` hook dual-writer (one canonical file vs. auto-stub overwriting curated handoff). Per Explore Agent #3's analysis. | Deferred — flagged in CLAUDE.md as known operator hazard; fix is a session-tooling change separate from evolve-loop core. |
| G2-polish | Reconcile the `--model` flag's PHASE/AGENT key inconsistency (existing TaskList #12 from cycle-121). | Still pending. |

---

## References

- **Plan that motivated this incident report + fix:** `~/.claude/plans/iterative-chasing-thunder.md`
- **Cycle-121 incident report:** [cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md](./cycle-121-codex-repl-boot-timeout-and-ws-g-multi-cli.md)
- **ADR-0029 CLI fallback chain (the WS-G doc this incident extends):** [../architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md](../architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md)
- **ADR-0030 Phase-observer auto-spawn (this incident's new architectural decision):** [../architecture/adr/0030-phase-observer-autospawn-in-evolve-loop.md](../architecture/adr/0030-phase-observer-autospawn-in-evolve-loop.md)
- **ADR-0023 live-injection and launch rules:** [../architecture/adr/0023-live-injection-and-launch-rules.md](../architecture/adr/0023-live-injection-and-launch-rules.md) (relevant to the F4 deferred follow-up)
- **Codex 0.134 root-cause dossier:** [../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md](../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md) (Fix A in section 0 is what Fix 1 here implements)
- **Ollama control-surface dossier:** [../../knowledge-base/research/ollama-control-surface-2026.md](../../knowledge-base/research/ollama-control-surface-2026.md)
- **Live forensic artifacts (sealed at V1):**
  - `.evolve/runs/cycle-122/scout-events.ndjson` (87 KB)
  - `.evolve/runs/cycle-122/triage-events.ndjson` (41 KB, ends with the "Perusing… (1m 34s · ↑ 4.7k tokens)" turn that triggered the tdd handoff)
  - `.evolve/runs/cycle-122/tdd-prompt.txt` (19 KB — proves the `## Subagent Interactive Policy (recommended_or_first)` block was injected as expected; the failure is structural, not policy-injection)
  - tmux session `evolve-bridge-codex-c122-tdd-pid71277-1779954103` (contains the verbatim `Press enter to confirm` modal text quoted in Part 1)
- **Forensic evidence post-V1:** moves to `.evolve/runs/cycle-122.reset-<UTCnano>/` (never deleted per `evolve cycle reset` contract).
