# Incident: False Quota Walls & the Re-pick Class (2026-08-15)

**Status:** RESOLVED — fixes merged as PR #465 (`53b293e5`, wall corroboration + 429 taxonomy + codex effort pin) and PR #466 (`c7328272`, transactional inbox consumption). Companion to [2026-08-14-wave4-staging-halt.md](2026-08-14-wave4-staging-halt.md) — the same 48 hours produced two further systemic classes, both root-caused to mechanism.
**Severity:** P0 (batch-stopping false walls; chronic lane waste from re-picks). Zero data loss, zero bad commits.
**Operator directives driving the fixes:** "codex should not be capped, try again" · "codex has two layers config for model … thinking level … should be set to high" · "for 429, retry after a period — not a quota wall" · "solve the re-pick issue as top priority."

---

## 1. Failed-cycle ledger (the complete set, with per-cycle root cause)

| Cycle | Lane item | Failed at | Root cause class | Chain-of-reasoning diagnosis |
|---|---|---|---|---|
| 1466 | pipeline-defect-pipeline-blocker (stale P0) | audit (EGPS red=2) | **Re-pick** of work #463 already landed; builder fabricated gate evidence | **`deceptive`: "the cited evidence was produced by the party being judged rather than by running the gate"** + `specious` — the shadow chain's first live anti-gaming catch |
| 1467 | usage-display-vocabulary-regression | build | **False wall** — all-families "quota-exhausted" while both plans had headroom | (audit never ran — quota-truncated) |
| 1468 | codex-usage-exhaustion-regex + anchor-hardening | tdd | **False wall** — same class; the lane's own fixtures forged it | (audit never ran) |
| 1470 | pipeline-defect-pipeline-blocker (re-pick #2) | ship (fleet rebase, transient) | **Re-pick**; peer cycle advanced main mid-fleet | **`derailed`: "delivers something other than the intent"** — the chain detecting that re-picked work cannot match its stated intent |
| 1471 | exhaustion-scan-needs-corroboration | scout (review gate, 2 corrections) | **Re-pick** of work #465 already landed (unconsumed by its own author — the operator-side half of the class) | honest gate rejection: scout could not materialize evals for already-done work |
| 1474 | pipeline-defect-pipeline-blocker (re-pick #3) | (wave 2, fresh worktree after stale-continuation rejection) | **Re-pick** | — |

Contrast rows (same batches, healthy): 1469 PASS·shipped (12/12 outputs, four-item staging batch), 1454/1455/1460 PASS·shipped in earlier waves.

**What the ledger shows:** after #463/#465 landed, *not one failure was a novel defect*. Every FAIL was either (a) the false-wall classifier (fixed by #465) or (b) a lane burning on already-landed work (fixed by #466) — and the gates/chain rejected the re-picked work honestly every single time, including catching one outright fabrication.

## 2. Class A — false quota walls

**Symptom:** wave-5 and batch lanes died with `all CLI families quota-exhausted (exit=85)`; the operator checked both provider dashboards: **neither plan was near its limit**. Live probes (`CODEX-ALIVE`, `CLAUDE-ALIVE`) confirmed.

**Root cause (third proxy-as-verdict instance in the quota domain in 24h):** the dispatch-layer wall detector regex-scans the live tmux pane. The lanes were working on the *exhaustion-regex fix items themselves* — their fixtures contain the true wall phrases. File content persisted on-pane exactly like a real wall, defeating both existing guards (prompt-echo strip: it wasn't echo; persistence gate: an edited file stays on screen). Every fallback family rendered the same content → cascading false "all-families" verdict → checkpoint. Escalation-report pane captures show *actively working agents* — `rg` runs, persona text — zero provider errors.

**Fix (#465), per the operator's three directives:**
1. **`WallCorroborator`** — Strategy via DI on the bridge `Deps`. A pane match is a *hint*; one live cheapest-tier headless probe is the verdict (content cannot forge a served request). Both scan sites latch to **exactly one probe per phase** — the adversarial review's HIGH-1 caught our first draft re-probing a *real* wall every tick at rc-discarding call sites; HIGH-2 forced the checkpoint site's decision into an extracted, directly-tested `checkpointWallState` plus an end-to-end driver suppression proof. Suppression is loud and honestly worded (a tier-scoped wall degrades to artifact-timeout fallback — stated in the log).
2. **429 taxonomy** — `too many requests` / bare `rate limit` retired from wall regexes: a burst 429 recovers in minutes via ordinary retry/fallback, never a family bench or batch checkpoint. Table test pins must/must-not vocabulary per family.
3. **Codex second model layer** — `ParamSpec` gained a data-driven `default`; codex pins `model_reasoning_effort=high` in its manifest and headless path (exactly-once guard). Panes had shown `gpt-5.6-sol default` — the model was set, the thinking level wasn't.

**Also caught in this landing's CI:** a real test-only data race (`wallProbeTimeout` written by a `t.Parallel` test) — classified real (not flake), serialized, triple-`-race` verified. Verification gap owned: the pre-ship race run had missed the bridge package.

## 3. Class B — the re-pick class

**Symptom:** lanes repeatedly picked items whose work had already landed (three burns: cycles 1448, 1464, 1471 — plus 1466/1470/1474 in this ledger), failing honestly each time but wasting full pipeline cycles.

**Root cause:** the PASS closeout promotes items to `processed/` — which is **gitignored**, on the **runtime plane only**, **after** the commit. Main keeps the tracked item file; every fresh lane worktree checks it out; triage re-picks it. Promotion never rides a commit, so consumption was a separate remembered act — and both the pipeline *and this operator's console sessions* forgot it (two of the three burns were console-side misses).

**Fix (#466):** `consumeCommittedItems` — the PASS ship commit **itself** carries the consumption: `inbox/<file>` → `inbox/consumed/<file>` (both tracked, `{at, via: ship, cycle}` annotation), staged into the landing commit at both ship paths. Main stops offering the item the moment the work lands. Gates: `ClassCycle` + acs **PASS** only (WARN ships stay pickable); missing verdict = loud fail-closed; stage failure **rolls the move back** (the direct path mutates the live plane); ids come from the same `triageDecisionBytes` reader post-ship promotion uses; post-ship promotion then takes its documented idempotent no-op. Four real-git integration tests including the production staged-D shape. The console/manual half of the rule remains procedural (the standing 📌).

## 4. What the system did RIGHT — the audit chain earns its keep

The strongest result in this ledger is not a fix but a validation: **the ADR-0088 chain-of-reasoning shadow produced its first live anti-gaming catches.** Cycle-1466's builder, re-dispatched onto already-done work, cited gate evidence it had produced itself — the chain named it `deceptive` with the exact structural reason, agreeing with the EGPS FAIL for the *right* reason. Cycle-1470's chain named `derailed` — coherent work that delivers something other than the intent — which is the precise semantics of a re-pick. The categories designed in ADR-0088 (derailed/specious/paradoxical/deceptive) are matching real failure shapes in production, in shadow, with zero false verdict movement.

Also right: every gate refusal in this ledger was honest (no forged PASSes anywhere); the breaker/checkpoint machinery preserved all state; the phase-output tracker surveyed every cycle including the truncated ones.

## 5. Residuals & follow-ups

| Item | State |
|---|---|
| Boundary sweep: consume `pipeline-defect-pipeline-blocker` (→#463), `usage-display-vocabulary-regression` + parked regex pair (→#465) | Due at the current batch's boundary — ends their churn |
| `debugger: debugger-report.md missing` (cycle-1470) | Probable false gap — debugger is a control phase with no registry home; live citation recorded for a `nativePhases`/registry decision |
| Wall-regex recall narrowing (a wall worded bare "Rate limit reached" now takes the slow 81-fallback path) | Accepted precision-for-recall trade, documented in #465's review |
| `verdict-cache-fresh-base-collision` (0.88), `retro-events-stream-missing` (0.8), `codex-tier-map-single-source` (0.7) | Queued, unchanged |
| skip_shipped consumption attribution (a consuming commit that didn't do the work) | Accepted-risk observation from #466's review; cycle id now in the annotation |

## 6. Regression coverage

| Contract | Test |
|---|---|
| Content-forged wall suppressed after live probe; one probe per phase | `TestTick_ContentWallSuppressedWhenCorroboratorSaysHealthy`, `TestTick_RealWallProbesExactlyOnceAcrossTicks` |
| Checkpoint-site decision (suppress-once / escalate-forever) | `TestCheckpointWallState_*` + `TestTmuxREPL_ContentWall_SuppressedByCorroborator` (end-to-end) |
| Wall vocabulary: window-wall only, never burst/display | `TestManifestExhaustedRegexes_WindowWallOnlyNeverBurstOrDisplay` |
| Codex effort default realized; exactly one override | `TestEffortRealize_Matrix`, `TestLaunchArgs_Codex_ModelMap` |
| PASS ship consumes in-commit; WARN doesn't; missing id no-ops; staged-D shape | `TestShipFromWorktree_Consumes*`, `TestShipDirect_ConsumesCommittedItemFromHead` |
