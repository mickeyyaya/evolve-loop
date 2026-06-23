# Incident Report: V3 Verification — Codex Per-Edit-Approval Modal + Empty-Fallback-Chain Design Gap — Cycle 123

**Date:** 2026-05-28 | **Severity:** HIGH (V3 end-to-end verification of the cycle-122 remediation revealed TWO orthogonal gaps the 3 shipped fixes did not cover; cycle-123 reproduced the cycle-122 PhasesRun=[scout, triage] truncation despite the remediation) | **Status:** Cycle-123 sealed manually below; 1 of 3 cycle-122 fixes fully defused the failure (Fix 3 observer auto-spawn); 1 of 3 ran correctly but addressed only one codex permission class (Fix 1 codex pre-trust); 1 of 3 was design-irrelevant (Fix 2 extended fallback trigger list — chain is single-element when profiles don't declare `cli_fallback`). Builds on [cycle-122 incident report](./cycle-122-codex-permission-modal-and-wsg-fallback-gap.md).

**Failure summary (cycle-123, V3 verification of cycle-122 remediation).** scout (agy-tmux) → triage (claude-tmux) → **tdd (codex-tmux) ABORTED `exit=81 core: bridge artifact timeout`** after 10 min. PhasesRun=[scout, triage]. The agent actually COMPLETED its TDD work (full report, predicates, reflection, handoff JSON all visible in tmux scrollback) and was hung at the codex per-edit-approval modal — a DIFFERENT modal than cycle-122's workspace-write modal. The bridge artifact-timeout fired, no fallback was attempted (chain was single-element), the cycle aborted.

---

## Part 1: What Happened

**Context.** All 4 cycle-122 remediation commits had shipped to origin/main:
- `8952a1c` docs (incident + ADR-0030 + cycle-121 backlog)
- `314cfc9` Fix 1 codex pre-trust
- `cb4e6fb` Fix 2 extended fallback trigger list to include exit=81
- `b9bac09` Fix 3 phase-observer auto-spawn

Per the V1-V3 verification plan, cycle-122 was sealed via `evolve cycle reset` → `.evolve/runs/cycle-122.reset-20260528T123756.422496000/` (forensic data preserved), `lastCycleNumber` advanced to 122. Cycle-123 launched at 2026-05-28T12:38Z PID 22614 with the EXACT cycle-122 multi-CLI fan-out command (Gemini scout/audit, GPT tdd/build, Claude triage/build-planner/retro) — the regression test.

**Timeline (UTC, ~20 min total wall-clock).**

| Time | Phase | Driver | Outcome |
|---|---|---|---|
| 12:38:22 | scout | agy-tmux (Gemini) | ✅ ~3 min — scout-report.md + 117 KB events file produced; observer attached + clean stop |
| 12:41:39 | triage | claude-tmux | ✅ ~1.5 min — triage-report.md produced; observer attached + clean stop |
| 12:43:18 | tdd | codex-tmux (gpt-5.5) | ❌ Agent completed work but HUNG at `Would you like to make the following edits?` modal |
| 12:53:18 | (observer) | — | **`stall_no_output` INCIDENT fired** (10-min stall threshold) — Fix 3 working as designed |
| 12:58:31 | (loop) | — | bridge artifact-timeout fired exit=81 → cycle aborted, no fallback attempted |
| 12:58:36 | loop exit | — | `stop_reason: error`, `PhasesRun: [scout, triage]`, `FinalVerdict: PASS` (for completed phases) |

**Smoking-gun evidence from cycle-123 forensic state:**

1. **`~/.codex/config.toml` (Fix 1 verification):** entries `[projects."~/ai/claude/evolve-loop/.evolve/worktrees/cycle-123"]` and `[projects."~/ai/claude/evolve-loop/.evolve/runs/cycle-123"]` BOTH present, mtime `2026-05-28 20:43:18` = exactly the tdd phase start. **Fix 1 fired correctly.**

2. **`tdd-observer-events.ndjson` (Fix 3 verification):**
   ```ndjson
   {"ts":"2026-05-28T12:43:18.183468Z","type":"started","severity":"info","cycle":123,"phase":"tdd","agent":"tdd","reason":"observer attached"}
   {"ts":"2026-05-28T12:53:18.16695Z","type":"stall_no_output","severity":"incident","cycle":123,"phase":"tdd","agent":"tdd","reason":"no stdout growth for 10m0s"}
   {"ts":"2026-05-28T12:58:31.877212Z","type":"stopped","severity":"info","cycle":123,"phase":"tdd","agent":"tdd","reason":"context canceled"}
   ```
   **Fix 3 fired at exactly the 10-min mark, on the same EVOLVE_OBSERVER_STALL_S=600 default. This is forensic visibility the cycle-122 incident lacked entirely.**

3. **Dispatch log line for tdd (Fix 2 (non-)verification):**
   ```
   [runner] phase=tdd agent=tdd-engineer cli=codex-tmux (source=env(EVOLVE_TDD_ENGINEER_CLI)) profile=~/ai/claude/evolve-loop/.evolve/profiles/tdd-engineer.json
   ```
   **NO `fallback=` or `triggers=` suffix.** The format-conditional only emits when `chain.candidates > 1`. The `tdd-engineer.json` profile has no `cli_fallback` key — chain resolved to `[codex-tmux]`, single-element. Even though Fix 2 extended the default trigger list to include exit=81, **there was no second CLI in the chain to fall back to.**

4. **tmux pane scrollback (the actual hang):**
   ```
   ...
   ## Reflection
   ### Reflection confidence (required)
   - `confidence: 0.86`
   <!-- END reflection -->

     Would you like to make the following edits?

   › 1. Yes, proceed (y)
     2. Yes, and don't ask again for these files (a)
     3. No, and tell Codex what to do differently (esc)

     Press enter to confirm or esc to cancel
   ```
   The agent finished writing the report content in-conversation; codex's per-edit-approval modal blocked the actual filesystem write. The `tdd-report.md` artifact never materialized — bridge's artifact-wait timed out.

**Forensic state preserved:** cycle-123 artifacts at `.evolve/runs/cycle-123/` (sealed below at V1 of the cycle-124 follow-up plan).

---

## Part 2: Root Cause — TWO orthogonal gaps

### Gap G1 — Codex has multiple permission classes; Fix 1 addressed only one

The cycle-122 incident report Part 2 F1 identified codex's "workspace-write" permission boundary and applied "Fix A" from the cycle-121 research dossier: pre-trust the worktree + workspace paths in `~/.codex/config.toml`. Fix 1 implemented this correctly and **it worked** — the cycle-122 `Working with untrusted contents` modal did NOT fire in cycle-123.

But codex has at least THREE distinct permission boundaries, only one of which is gated by `trust_level`:

| Boundary | Trigger | Modal text | Gated by | Cycle-122 Fix 1 covers? |
|---|---|---|---|---|
| **Workspace trust** | Launch in a path not in `~/.codex/config.toml:projects` | `Working with untrusted contents / Yes, continue` | `trust_level = "trusted"` | ✅ Yes |
| **Workspace-write** | Shell redirect to a path outside the writable worktree | `Do you want to allow writing the X artifact into the cycle workspace outside the writable worktree?` | `trust_level` on workspace path | ✅ Yes (cycle-122 case) |
| **Per-edit-approval** | Codex agent calls `apply_patch` / edits a file in the worktree | `Would you like to make the following edits? / 1. Yes, proceed / 2. Yes, and don't ask again for these files / 3. No, and tell Codex what to do differently` | **NOT** `trust_level` — gated by codex's `approval` policy (`--ask-for-approval=never` or similar; need to verify exact flag for codex 0.134) | ❌ **NO — cycle-123 case** |

The codex-tmux manifest's `interactive_prompts[]` regex list (`internal/bridge/manifests/codex-tmux.json`) does not include any pattern matching the per-edit-approval modal either, so the auto-responder didn't dismiss it.

### Gap G2 — Empty-fallback-chain design (the WS-G design assumption that doesn't hold in practice)

WS-G's design (ADR-0029) requires operators to enumerate `cli_fallback: [...]` in each profile JSON. Without it, `resolveCLIChain` returns a single-element chain regardless of how many CLIs are registered or how complete the trigger list is. Fix 2 extended the default trigger list `[80, 127]` → `[80, 81, 124, 127]` but **the trigger list is only consulted when the chain has more than one candidate**. With profiles that don't declare `cli_fallback`, the trigger list is irrelevant.

Audit of current profiles (as of cycle-123):
- `.evolve/profiles/tdd-engineer.json`: cli=claude-tmux, **no cli_fallback** ← cycle-123 failure
- `.evolve/profiles/scout.json`: (verify)
- `.evolve/profiles/triage.json`: (verify)
- `.evolve/profiles/builder.json`: (verify)
- `.evolve/profiles/auditor.json`: (verify)
- `.evolve/profiles/build-planner.json`: (verify)
- `.evolve/profiles/retro.json`: (verify)

The cycle-122 contract test (`TestRun_FallbackOnArtifactTimeout_DefaultTriggerListIncludes81`) PASSES because the test fixture explicitly writes `"cli_fallback": ["claude-tmux"]` in the test profile. **Unit-tested behavior diverges from real-profile behavior.** The test proved the trigger-list extension WOULD fire fallback — but never tested the profile-loading path that would expose the empty-fallback-chain default.

### How G1 + G2 combined to reproduce the cycle-122 failure shape

```
   codex-tmux tdd starts; Fix 1 pre-trusts workspace (G1.workspace covered) ─── ✅
                │
                ▼
   Agent runs go test, writes report content in-conversation ─── (work completed)
                │
                ▼
   Agent calls apply_patch to materialize tdd-report.md ─── codex per-edit modal fires
                │                                          (G1.per-edit NOT covered)
                ▼
   Bridge waits for tdd-report.md ─── Observer fires stall_no_output at 10m ─── ✅ Fix 3
                │                                          (forensic visibility gained)
                ▼
   Bridge artifact-timeout fires exit=81 ─── (correct WS-B safety net behavior)
                │
                ▼
   Runner checks chain.candidates length ─── chain=[codex-tmux] single-element
                │                                          (G2: no fallback to try)
                ▼
   Cycle aborts, PhasesRun=[scout, triage] ─── same shape as cycle-122
```

Fix 2's extended trigger list (which covered exit=81 specifically) was **necessary but not sufficient** because the chain was empty. Fix 1 covered the workspace-write modal but not the per-edit modal. Only Fix 3's observability — the `stall_no_output` INCIDENT in the events file — actually fired and provided new forensic value vs cycle-122.

---

## Part 3: What Did Get Defused (the partial-win record)

The cycle-122 remediation was NOT a total miss. Three specific cycle-122 failure modes ARE structurally defended now:

1. **The cycle-122 workspace-write modal cannot recur** — Fix 1's pre-trust of the workspace path means that exact text ("Do you want to allow writing the X artifact into the cycle workspace outside the writable worktree?") will not appear again. Confirmed in cycle-123: the workspace-write boundary was crossed without prompting.

2. **"8 hours of silence" cannot recur** — Fix 3 emits a `stall_no_output` INCIDENT to `<workspace>/<phase>-observer-events.ndjson` at the StallS threshold (default 600s, configurable). Operators now have explicit, machine-readable evidence "this phase is stuck right now," not "this phase failed 8 hours ago."

3. **The per-cell trigger list now includes the WS-B sentinel** — when an operator DOES declare `cli_fallback: [...]` in a profile, exit=81 from artifact-timeout will route to the next CLI without requiring per-profile overrides. This unblocks the eventual G2 fix (implicit chain) but already helps any profile that's been hand-configured.

The honest scorecard for the cycle-122 plan: **1 of 3 fixes (Fix 3) was the unambiguous V3-pass-criteria win; 1 of 3 (Fix 1) was a correct fix that addressed one of three permission boundaries; 1 of 3 (Fix 2) was design-correct code that hit a pre-existing data-plane assumption (profiles don't declare `cli_fallback`).**

---

## Part 4: Follow-up Plan (cycle-124 / V3-recovery scope)

> **AMENDMENT (2026-05-28, operator redirect after this report shipped).** The plan
> below was written from the pre-redirect perspective — leading with G2 (cross-CLI
> fallback chain) as the second fix. The operator subsequently directed: *"I don't
> expect fallback to happen. LLM CLIs should complete their assigned tasks. We
> have full tmux control — can always send query and command to ask LLM CLI
> continue and correct its job."* Part 4 below stays in place as the historical
> design record; the REDIRECTED priority sequence the cycle-124 PR actually ships
> is:
>
> | # | Fix | What | Status (cycle-124 PR) |
> |---|---|---|---|
> | 1 | **G1a** | codex `--yolo` + ollama `--experimental-yolo` via manifest `default_args` (wire-up + claude/agy `--dangerously-skip-permissions` migration) | LANDED in PR |
> | 2 | **F4** | `KindKeystroke` envelope — raw `tmux send-keys` channel for operator/observer to dismiss modals, navigate menus, send Ctrl chars (ADR-0023 addendum) | LANDED in PR |
> | 3 | **G1b** | codex per-edit-modal regex in `interactive_prompts` (defense-in-depth behind G1a's `--yolo`) | LANDED in PR |
> | 4 | **G3** | CLIPreflight optional interface — codex moves `pretrustCodexProjects` into `Preflight()`; opt-out by omission, no stubs in drivers that don't need prep | LANDED in PR |
> | 5 | Liveness nudge (Task 6) | Standalone `evolve phase-observer` default flipped 0 → 300s; auto-spawn adapter wire-up DEFERRED with a documented gap | PARTIAL in PR |
> | 6 | **G2** | implicit cross-CLI fallback chain — DEMOTED to LAST RESORT; ships in a follow-up cycle, not in cycle-124 | DEFERRED |
>
> Reasoning for the demotion: the operator's "full tmux control" mechanism (G1a +
> F4 + G1b + active nudging) is the PRIMARY autonomous-completion path. G2 (fallback
> to a different CLI when the primary fails) is only meaningful if items 1-5 fail
> first — by design, they shouldn't, so the fallback becomes a safety net rather
> than a routine recovery path. The cycle-124 PR ships items 1-4 fully + item 5
> partial; G2 ships in a follow-up PR after V1-V3 verification confirms the
> primary mechanisms are sufficient.
>
> Sources for this redirect: the operator's 2026-05-28 message captured in
> `~/.claude/session-data/2026-05-28-cycle122-123-redirect-session.tmp` and the
> `[[project_cycle122_remediation_shipped]]` memory anchor.

This incident must be resolved before the "any CLI × any phase × any model" invariant (`feedback-any-cli-any-phase-any-model-invariant`, internal operator memory) can be claimed. Three concrete fixes (PRE-REDIRECT plan — preserved as historical context; see the AMENDMENT block above for the ACTUAL cycle-124 PR sequence):

### Fix G1a — Launch codex with non-interactive approval flag

Add `--ask-for-approval=never` (or codex 0.134's equivalent) to the codex-tmux launch flags so the per-edit-approval modal never appears. The agent's edits apply unconditionally inside the worktree. The host sandbox (WS-B's sandbox-exec / bwrap profile) already constrains writes to the worktree+workspace boundary, so removing codex's redundant per-edit prompt does NOT widen the actual filesystem footprint.

**Files:** `go/internal/bridge/manifests/codex-tmux.json` (add the flag to `launch_flags`); `go/internal/bridge/driver_codextmux_test.go` (test asserting the flag is present in `launchCmd`).

**Risk:** if the flag's exact spelling has changed in codex 0.134, the launch will fail with `unknown flag`. Mitigation: research current codex CLI flag set first (the dossier `knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md` may already document this).

### Fix G2 — Implicit cross-CLI fallback chain when profile.cli_fallback is empty

When `profile.CLIFallback` is empty AND `primary` is one of the registered tmux CLIs, auto-populate the chain with the other registered CLIs in a deterministic order. Specifically: extend `resolveCLIChain` so when `prof.CLIFallback` is empty/nil, the chain becomes `[primary] + dedup(registeredDriverNames() - primary)` with a stable order (e.g., `claude-tmux → codex-tmux → agy-tmux → ollama-tmux` alphabetical-or-policy).

**Files:** `go/internal/phases/runner/cli_chain.go` (auto-population logic); `go/internal/phases/runner/cli_chain_test.go` (assert auto-population fires when profile is empty); `go/internal/phases/runner/runner_fallback_test.go` (integration test using a real-shape profile with NO `cli_fallback` key, asserting the chain still recovers on exit=81).

**Design choice:** Should empty-fallback be `[primary]` (preserve byte-identical) or `[primary, ...others]` (auto-recover)? The cycle-123 evidence says the auto-recover default is what the "any CLI any phase any model" invariant requires. Make it the default; provide `cli_fallback: []` as the explicit opt-OUT (different from nil/absent which means "auto").

### Fix G3 — Codify per-CLI preflight as an architectural pattern

Fix 1's `pretrustCodexProjects` is currently codex-specific. Each driver needs an analogous preflight hook for its own CLI-native permission state. Define a `type CLIPreflight interface { Preflight(*Config) error }` that drivers can optionally implement; the bridge's Launch trunk calls it once before the driver's Launch. claude-tmux + agy-tmux + ollama-tmux each get a stub preflight; codex-tmux moves `pretrustCodexProjects` into its preflight implementation.

**Files:** `go/internal/bridge/preflight.go` (interface + dispatch); `go/internal/bridge/driver_codextmux.go` (move pretrust into preflight method); driver_claudetmux.go / driver_agytmux.go / driver_ollamatmux.go (add preflight stubs).

**Why this is bigger than codex 0.134:** as soon as another CLI hits a similar permission-state-not-yet-trusted situation, the same pattern applies. Codifying it now prevents a Fix 1c / Fix 1d / Fix 1e cascade.

### Sequencing

| # | Fix | Order | Why |
|---|---|---|---|
| 1 | G1a — codex `--ask-for-approval=never` | First, smallest | Defuses the immediate cycle-123 trigger; ~5 LOC + 1 manifest line + 1 test. Lowest risk per change. |
| 2 | G2 — implicit fallback chain | Second | Defuses the "even if a CLI fails, the cycle keeps going" invariant violation. Higher risk because it changes default behavior for every profile; needs the design-choice decision (`nil = auto`, `[] = opt-out`) explicit. |
| 3 | G3 — per-CLI preflight architectural pattern | Third | Refactor only; behavior-preserving. Best done last when G1a+G2 have stabilized the matrix. |

Cycle-124 V1-V3 verification then re-runs the cycle-123 multi-CLI fan-out and asserts (a) no codex modal of any class fires, (b) PhasesRun reaches `audit` or beyond, (c) observer events file present for every phase.

---

## Part 5: Lessons + Open Items

### Lessons

1. **"Cell" failures mask multi-cell gaps.** Cycle-122's modal was ONE codex permission boundary. The remediation correctly addressed the cell. But the matrix lens — "would another modal class, or another CLI's analogous boundary, recur?" — was reserved for the follow-up memory, not the in-PR fix. Apply the matrix lens during PR design, not after V3 fails.

2. **Test fixtures must be representative of production profile shapes.** The cycle-122 contract test wrote `"cli_fallback": ["claude-tmux"]` and proved the trigger-list extension worked. It did NOT prove production profiles (which lack `cli_fallback`) get any benefit. A "real-profile-shape" integration test would have caught G2 pre-ship.

3. **Observability separated from enforcement is correct architecture, but operators must connect them.** Fix 3 (observer auto-spawn) DID exactly its job: emit `stall_no_output` at 10 min. It did NOT kill the subagent. The bridge artifact-timeout did the kill. Both fire at the same 10-min mark, so the observer didn't speed up detection — it only added structured evidence. Fix 3 v2 (already in the cycle-122 plan's open follow-ups) would have the observer cancel the cycle context on stall, which would shorten the time-to-fail from 10 min to whatever StallS is configured to.

4. **Partial wins still ship.** Of the 4 cycle-122 commits, all 4 went through gated `/commit` + `evolve ship --class manual` + CI green. The fact that V3 exposed orthogonal gaps doesn't invalidate the fixes — Fix 1 + Fix 3 are still permanent improvements; Fix 2 becomes load-bearing the moment G2 (implicit fallback chain) lands. Document the partial-win pattern instead of regretting the partial coverage.

### Open Items

| ID | Item | Owner |
|---|---|---|
| G1a | Codex `--yolo` + ollama `--experimental-yolo` via `manifest.default_args`; claude/agy migration | LANDED in cycle-124 PR |
| F4  | `KindKeystroke` envelope (operator's "full tmux control" mechanism, ADR-0023 addendum) | LANDED in cycle-124 PR |
| G1b | Codex per-edit-modal regex (`Would you like to make the following edits...`) — defense-in-depth behind G1a's `--yolo` | LANDED in cycle-124 PR |
| G3  | CLIPreflight optional interface (codex preflight extracted; opt-out by omission) | LANDED in cycle-124 PR |
| Liveness nudge (Task 6) | Standalone `phase-observer` default 0→300s; auto-spawn adapter wire-up gap documented | PARTIAL in cycle-124 PR |
| G2  | Implicit cross-CLI fallback when `profile.cli_fallback` empty (operator-demoted to LAST RESORT) | DEFERRED to follow-up cycle |
| G4 (deferred) | Fix 3 v2 — observer cancels cycle context on stall (instead of just emitting INCIDENT) | post-cycle-124 |
| G5 (deferred) | Manifest completeness audit for remaining tmux drivers (claude/agy/ollama) | post-cycle-124 |
| Adapter nudge wire-up | Port phaseobserver's nudge logic into adapters/observer's slim Watch loop OR consolidate the two observers | post-cycle-124 |

---

## References

- **Cycle-122 incident report (precursor):** [./cycle-122-codex-permission-modal-and-wsg-fallback-gap.md](./cycle-122-codex-permission-modal-and-wsg-fallback-gap.md)
- **Cycle-122 remediation plan (executed in full):** `~/.claude/plans/iterative-chasing-thunder.md`
- **The matrix invariant memory this incident reaffirms:** `feedback-any-cli-any-phase-any-model-invariant`
- **Codex 0.134 research dossier (Fix A is what Fix 1 implemented; codex flag set for G1a should be researched here first):** [../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md](../../knowledge-base/research/codex-cli-0.134-repl-boot-timeout-2026-05-28.md)
- **ADR-0029 CLI fallback chain (G2 amends this):** [../architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md](../architecture/adr/0029-cli-fallback-chain-and-per-agent-overrides.md)
- **ADR-0030 phase-observer auto-spawn (Fix 3, which DID work):** [../architecture/adr/0030-phase-observer-autospawn-in-evolve-loop.md](../architecture/adr/0030-phase-observer-autospawn-in-evolve-loop.md)
- **Live forensic artifacts (cycle-123, sealed via `evolve cycle reset` after this incident lands):** `.evolve/runs/cycle-123/`
