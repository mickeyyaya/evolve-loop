# Live E2E test plan — real LLM CLIs

> Companion to [../TEST_PLAN.md](../TEST_PLAN.md) (offline deterministic E2E). This
> doc designs the **live** layer: exercising real `claude` / `codex` / `agy` /
> `ollama` binaries against real models. Status as of 2026-05-30: only unproven
> happy-path scaffolding exists (`EVOLVE_E2E_LIVE=1` in the two matrix tests).
> This plan hardens and extends it.

## Why live at all (what offline can't catch)

The offline fake-CLI seam proves orchestration, fallback, and gate logic
deterministically. It cannot prove the **integration contract with the real
binary**:

- The real CLI boots, accepts a pasted/`-p` prompt, and writes a **parseable**
  artifact at the canonical path.
- Real auth/billing path works (subscription OAuth, API key, or local).
- Real REPL **boot-marker detection** against the production glyphs (`❯`, `›`,
  `? for shortcuts`, `>>> `) — offline used a synthetic marker.
- Real **model-tier resolution** (haiku/sonnet/opus, gpt-5.x, gemini-3.5) yields
  output a phase classifier accepts.
- Provider quirks the real auto-responder must dismiss (codex trust modal, agy
  permission prompt, model-deprecation notices).
- Cross-family adversarial audit produces **genuine disagreement** (a real
  different-family auditor catches a real builder defect).

## Probe-grounded reality (this host, 2026-05-30)

`evolve setup detect`:

| CLI | binary | auth | tier | live usable? |
|---|---|---|---|---|
| `ollama` | ✓ | local | full | **yes — FREE, CI-safe** |
| `codex` | ✓ | SUBSCRIPTION | full | yes (subscription quota) |
| `agy` | ✓ | SUBSCRIPTION | full | yes (subscription quota) |
| `claude` | ✓ | **MISCONFIGURED** | full | yes **but** detect false-negatives the macOS keychain creds (works via the launching Claude session's OAuth) |

Two design consequences:
1. **`ollama` is the zero-cost smoke target** — the harness mechanics get
   validated before any paid CLI runs, and this tier can run in hosted CI.
2. The **claude keychain false-negative** means the auth preflight must
   *skip-or-trust*, never hard-fail, on a `MISCONFIGURED` claude when the binary
   is present (the `/setup` skill already treats claude as available inside a
   Claude session — mirror that).

## Determinism strategy: assert structure, never content

Real LLM output varies run-to-run. Every live assertion is **structural**:
- artifact **exists** at the canonical path and **parses** (markdown sections /
  JSON fields), not exact bytes;
- the cycle reaches a **terminal state** and the final verdict ∈ {PASS, WARN,
  FAIL} (a *valid* verdict, not a *specific* one);
- the ledger has the expected **phase roles** and a hash-chain that verifies;
- a PASS cycle **ships** (commit lands) OR a non-PASS cycle is **cleanly
  blocked** (no commit + retro role present).

A live test fails only on a **broken contract** (missing/malformed artifact,
crash, unparseable verdict, chain break), never on model wording.

## Tiered design

Cost and latency forbid running the full CLI×phase×model matrix live on every
push. Four tiers, each its own env gate, escalating cost/rarity:

### Tier 0 — Live smoke (free, frequent, CI-safe) · `EVOLVE_E2E_LIVE_SMOKE=1`
- Target: **`ollama`** (free) + optionally `claude` haiku (cheapest paid).
- Work: one minimal phase (scout) OR a trivial micro-cycle.
- Asserts: real boot + artifact written + parses + exit 0.
- Runtime: seconds–1 min. The canary that the live path is wired at all.

### Tier 1 — Per-CLI happy-path cycle (hardens existing scaffold) · `EVOLVE_E2E_LIVE=1`
- Targets: every available CLI × {headless, tmux} = claude-p, codex, agy,
  claude-tmux, codex-tmux, agy-tmux. Each runs **one full cycle** at its
  **cheapest tier**, hard `--budget-usd` cap.
- Asserts: structural (above). Per-CLI **auth/binary preflight → `t.Skip`**.
- Robustness: retry-on-transient, artifact capture on failure, cost summary.

### Tier 2 — Model-tier matrix (periodic, gated) · `EVOLVE_E2E_LIVE_MATRIX=1`
- For each CLI, run **one cheap phase (scout)** at each supported tier
  (fast/balanced/deep); assert the intended model was invoked (ledger/cost
  fields) AND the artifact parses. Catches tier-resolution + model-deprecation
  drift (e.g. codex ChatGPT-safe-model clamping).

### Tier 3 — Cross-family adversarial soak (rare, observational) · `EVOLVE_E2E_LIVE_SOAK=1`
- Seed a task with a **known planted defect**; run real builder (family A) +
  real auditor (family B, e.g. claude builder × codex auditor). Observe whether
  the auditor FAILs it. Non-deterministic → assert weakly (verdict is valid +
  cycle terminal), **log the catch/miss**, and track catch-rate across runs as a
  quality signal (not a hard gate). Guards against same-family sycophancy
  regressions in the live system.

## Cross-cutting robustness harness (shared helpers)

1. **Auth preflight** (`liveCLIAvailable(cli) (ok bool, why string)`): binary on
   PATH + auth check; `claude` present ⇒ treated available despite a
   `MISCONFIGURED` detect verdict (keychain false-negative). Unavailable ⇒
   `t.Skip(why)`, never fail.
2. **Budget enforcement**: per-cycle `--budget-usd`; a cumulative
   `EVOLVE_E2E_LIVE_BUDGET_USD` suite ceiling; abort remaining sub-tests once
   exceeded; always emit a per-CLI **cost report** parsed from ledger cost
   fields.
3. **Flake policy** (the heart of "robust"): classify each failure —
   - *transient* (HTTP 429 / network / provider `overloaded` / bridge exit 81 /
     boot timeout) → retry ≤ N with exponential backoff; if still failing,
     **quarantine-skip with a loud log** (do not red the suite on a provider
     outage);
   - *contract* (malformed/missing artifact, crash, unparseable verdict, chain
     break) → **hard fail**.
   Reuse the existing classification vocabulary (`infrastructure-transient` vs
   `code-*`) where possible.
4. **Failure-artifact capture**: on any non-skip failure, copy the cycle
   workspace (`*-stdout.log`, `*-stderr.log`, artifacts, ledger) to a retained
   `testdata/live-failures/<cli>-<ts>/` and print the path + cost.
5. **Timeouts**: generous, per-tier, env-tunable (`EVOLVE_E2E_TMUX_TIMEOUT_S`
   exists; add `EVOLVE_E2E_LIVE_TIMEOUT_S`). Rely on the phase-observer for
   stalls rather than only the hard wall-clock.

## CI strategy

| Tier | Hosted CI (GitHub-hosted) | Self-hosted / local |
|---|---|---|
| T0 ollama | ✅ (pull model in setup; free) | ✅ |
| T0 claude-haiku / T1 / T2 / T3 | ❌ subscription OAuth can't be headless-authed in hosted CI | ✅ **operator-triggered** (manual `workflow_dispatch` or local `make live-e2e`) |

Subscription-OAuth CLIs (claude/codex/agy) are **local/self-hosted-runner only**,
behind a manual dispatch + explicit budget input. Document loudly that a green
hosted-CI run does **not** imply live coverage of the paid CLIs — only T0-ollama.

## Env-gate summary

| Gate | Tier | Default | Cost |
|---|---|---|---|
| `EVOLVE_E2E_LIVE_SMOKE` | T0 | off | ~free (ollama) |
| `EVOLVE_E2E_LIVE` | T1 | off | low (cheapest tier × CLIs) |
| `EVOLVE_E2E_LIVE_MATRIX` | T2 | off | medium |
| `EVOLVE_E2E_LIVE_SOAK` | T3 | off | medium, rare |
| `EVOLVE_E2E_LIVE_BUDGET_USD` | all | unset | suite spend ceiling |
| `EVOLVE_E2E_LIVE_TIMEOUT_S` | all | per-tier default | — |

## What live does NOT replace

Offline (`TEST_PLAN.md` Phase 2) remains the **deterministic gate** for
orchestration, fallback codes, audit-FAIL→retro, and validation. Live is an
**integration confidence layer** on top — it can be flaky/skipped without
blocking merges (except the free T0 ollama smoke, which is a hard gate).

## Rollout

1. Land the shared harness (auth preflight + flake classifier + budget/cost
   report + capture) — usable by all tiers.
2. Tier 0 ollama smoke → wire into hosted CI as a required check.
3. Harden the existing Tier 1 scaffold onto the harness; run once locally per
   CLI to **prove the live path** (first real validation) and record results.
4. Tiers 2–3 as periodic/manual.

## Validation log

**2026-05-31 — T1 codex headless, live (proved the classifier + capture path).**
Codex booted + authenticated (`OpenAI Codex v0.135.0`, model `gpt-5.5`) and
fast-failed at the first LLM call with `You've hit your usage limit … try again
at Jun 4th` — the account is quota-capped until Jun 4. The first run **red-failed
the suite** (mis-graded as a contract break), exposing a two-part classifier gap:

1. `transientMarkers` matched the literal `"quota"` but **not** codex's real
   phrasing (`usage limit`, `upgrade to plus`). Added those markers.
2. More importantly, `isTransient` was classifying only the `evolve cycle run`
   subprocess stdout, which carries just `bridge: launch exit=1` — the real
   provider text lives in the per-phase `*-stderr.log` **artifact**. Added
   `phaseStderrTail()` to fold those artifacts into the classified string.

After both, the re-run correctly **SKIPs** (`$0.00`, ~3.5s, "quarantined after
transient retries"). This is the robustness contract working as designed: a
provider quota cap is a skip, not a failure. The failure-artifact capture path
was validated for free (it wrote the real stderr to `testdata/live-failures/`).

**2026-05-31 (later, quota upgraded) — T1 codex headless, live: FULL CYCLE PASS.**
With the codex subscription quota restored, `EVOLVE_E2E_LIVE=1` ran a complete
real cycle: **`--- PASS … (380.36s)`**, `roles=[scout triage tdd build-planner
build audit retro]` — real codex (gpt-5.5) drove **all seven phases** end-to-end
with parseable artifacts throughout. `shipped=false` is the correct outcome (the
synthetic task did not yield a PASS-and-ship verdict; the test asserts the CLI
reached the core phases scout→build→audit, not a specific verdict). `cost=$0`
because subscription usage is not reported in per-entry ledger `cost_usd` fields.
This is the **first genuine live validation of the full per-CLI cycle path** — the
integration contract offline can't prove.

Open / deferred from this investigation:
- The "worktree provisioning already exists" failure from an earlier session was
  **not reproduced** — codex quota blocks the path before worktree state matters.
  Treat as an open, currently-unreproducible observation, not a fixed bug.
- A process-group kill for `runWithTimeout` (reap an orphaned CLI grandchild on
  timeout) was considered and **deferred**: the only live failure observed
  fast-failed in ~3.5s with no timeout and no orphan, so there was no evidence to
  justify it. Revisit if a live full-cycle is ever seen to time out with a
  surviving child. (Host caution: `pgrep -f codex` also matches MCP servers whose
  `PATH` contains the macOS cryptex `codex.system` string — never `pkill -f codex`.)
- A quota/usage-limit failure is currently *transient* (retry-then-skip). Since
  it won't recover within the backoff window, a refinement could skip it
  immediately without spending a retry.
- claude-p cannot be live-tested from inside a Claude Code session (nested hang);
  agy is the one subscription CLI available for an on-demand live full cycle.
