# ADR-0051: Setup onboarding — `evolve setup` + `/setup`

> **Renumbered from 0027** (2026-06-15): ADR number 0027 had been dual-assigned to both this ADR and [ADR-0027 commit-as-evidence](./0027-commit-as-evidence.md). This onboarding ADR is renumbered to 0051 (the next free slot) so each number maps to exactly one decision; commit-as-evidence keeps 0027. Inbound references were repointed accordingly.

> Status: **Accepted** (2026-05-27). Adds a first-launch onboarding flow: detect available LLM CLIs/subscriptions, propose + validate a per-phase model config, and explain the pipeline. Builds on `bridge.Doctor` (ADR-era doctor), `resolvellm` (ADR-0001), profile envelopes (`dynamic-model-routing.md`), and the routing engine's "model proposes, kernel disposes" pattern ([[dynamic-phase-routing]], ADR-0024).

## Context

Evolve-loop assumed the operator already knew the phase pipeline and how to hand-edit `.evolve/llm_config.json`. There was no onboarding: no guidance on which CLIs/subscriptions are available, which model each phase should use, or how the pipeline works and why it is trustworthy. The config surface (`llm_config.json`, profile `model_tier_envelope`, `cross_family_with`, `allowed_clis`) was powerful but undocumented at the point of use.

## Decision

### 1. In-session skill + deterministic Go core (the split)

The judgment work (recommend per-phase models, explain the pipeline) runs in the operator's interactive CLI session via a `/setup` skill — **zero extra API cost, interactive (AskUserQuestion), grounded in real detection**. The deterministic work runs in the Go binary:

| `evolve setup …` | Role | Reuses |
|---|---|---|
| `detect [--json]` | Read-only onboarding digest: per-CLI {binary, auth-mode, subscription, capability tier, verdict} + per-phase {current routing, envelope, cross_family_with, allowed_clis} | `bridge.Doctor`, `capability.Inspect`, `resolvellm.Resolve` |
| `validate [--config P] [--strict]` | Clamp a proposed `llm_config.json` against the floor | `resolvellm` parse + profile reads |
| `complete` | Stamp the first-run marker | lossless raw-merge into `state.json` |

This mirrors the routing kernel: **the skill proposes, `validate` disposes.** Rejected alternative: a Go-side bridge LLM call (headless but costs tokens, can't interactively adjust). Rejected alternative: a deterministic Go recommendation engine (the user wanted LLM judgment for the proposal; Go only validates).

### 2. Validation severities — cross-family is ADVISORY (deviation from the plan)

The approved plan said `validate` would **exit 2** on builder/auditor same-family. Implementation **changed this to a WARN** (exit stays 0; `--strict` promotes it to an error). Reason: the **live production config is intentionally all-Claude** (cycle-100 recovery after the Gemini quota wall), and cross-family is "documented-only / advisory" per `dynamic-model-routing.md`. A hard exit-2 would reject the legitimate all-Claude fallback. Hard (exit 2) checks are reserved for **envelope** bounds and **allowed_clis** — constraints the profile declares unconditionally.

### 3. First launch — non-blocking nudge

`evolve loop` prints one stderr line (`[setup] First run — run /setup …`) when `state.setupCompletedAt` is empty, then proceeds with defaults. It never blocks (respects bypass-permissions "never stop to ask" + CI). No new feature flag — the marker lives in `state.json` (no [[feedback_no_feature_flag_sprawl|sprawl]]).

### 4. Detection depth — auth mode + capability tier only

Detect reports binary presence, auth mode (`SUBSCRIPTION_OAUTH` / `API_KEY` / `CUSTOM_PROXY` / `SUBSCRIPTION` / `MISCONFIGURED`, synthesized from `bridge.Auth` + env per the README precedence), a coarse capability tier (`full`/`delegated`/`n/a` from `capability.Inspect`), and the doctor verdict. Subscription **plan/quota** is out of scope (no reliable local signal).

### 5. Lossless marker write

`core.State` is a SUBSET view of `state.json` — `WriteState` (marshaling the struct) drops unmodeled keys (e.g. `expected_ship_sha`, confirmed present in the live file). So `complete` reads `state.json` into a `map[string]json.RawMessage`, sets the two marker keys, and atomic-writes — preserving every other field. Two struct fields (`SetupCompletedAt`, `SetupVersion`) were added to `core.State` so the nudge can read the marker and the orchestrator round-trips it.

## Consequences

- **Positive:** onboarding with no token cost; deterministic + unit-tested core; the LLM advises with rationale; reuses every existing detection/config building block; no new flags.
- **Known limitation:** `bridge.doctorAuth` checks `~/.claude/.credentials.json` only — on macOS, claude OAuth in the Keychain reads as `MISCONFIGURED` (false negative). The skill is instructed to treat claude as available when run from a Claude session. Fixing the Keychain probe is deferred (a pre-existing doctor limitation).
- **Reversible:** the feature is additive; deleting the skill + subcommand restores prior behavior. The nudge is silenced by `evolve setup complete`.

## References

- [docs/architecture/setup-onboarding.md](../setup-onboarding.md) — subcommand contract + digest shapes + skill flow
- `go/internal/setup/setup.go`, `go/cmd/evolve/cmd_setup.go`, `skills/setup/SKILL.md`
- [[dynamic-phase-routing]] / ADR-0024 — the propose/clamp pattern this follows
- ADR-0001 (llm-config-router) — the `llm_config.json` + resolver this writes to
