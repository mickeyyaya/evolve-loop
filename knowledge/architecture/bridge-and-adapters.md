# Bridge & Adapters — CLI-agnostic agent launch

> The bridge is the seam that lets *any phase* run on *any LLM CLI* with *any
> model*. A phase says **what** kind of launch it wants in high-level terms; a
> per-CLI realizer translates that into the **how** for one specific CLI — never
> leaking one CLI's argv vocabulary into another. This document covers the
> LaunchIntent→Realizer model (`go/internal/bridge`), the per-CLI drivers, the
> interactive-policy prefix, live injection, the fallback chain, and the recipe +
> capability layers. ADRs: **0021, 0022, 0023, 0029, 0031**. Current design (v13.0.0).

Related: [phase-pipeline.md](phase-pipeline.md) ·
[trust-kernel-and-egps.md](trust-kernel-and-egps.md) ·
[routing-and-advisor.md](routing-and-advisor.md) ·
[glossary](../00-overview/glossary.md)

---

## 1. The invariant: any CLI × any phase × any model

The pipeline drives seven pluggable CLI drivers — `claude-p`, `claude-tmux`,
`codex`, `codex-tmux`, `agy` / `agy-tmux` (Gemini/Antigravity), `ollama-tmux`. The
design goal (stated after the cycle-121 incident, ADR-0029):

> "Allow any combination of LLM CLIs + any model to be adapted for any phase (even
> new customized user / LLM-constructed phases) — always executed in the pipeline."

That invariant is *why* the bridge exists as a distinct layer. A phase must not
know whether it's running on claude or codex; an operator must be able to pin the
auditor to Opus-on-codex and the builder to Sonnet-on-claude without editing Go
code. The bridge is the indirection that makes the trust kernel's cross-family
auditor (see [trust-kernel-and-egps.md](trust-kernel-and-egps.md) §3) physically
possible.

---

## 2. The port and the adapter

The orchestrator depends only on the `core.Bridge` port (`core/ports.go`):

```go
type Bridge interface {
    Launch(ctx, BridgeRequest) (BridgeResponse, error)
    Probe(ctx) (BridgeProbe, error)
}
```

The production implementation is `adapters/bridge.Adapter`
(`go/internal/adapters/bridge/bridge.go`). It adds the one concern the in-process
engine deliberately does *not* own — **interactive-policy injection** — and then
delegates to the native-Go `bridge.Engine`. (The bash `tools/agent-bridge`
subprocess and the `EVOLVE_BRIDGE_GO` toggle were removed in the v12 flag-day
cutover, ADR-0021; the Go bridge is now the only implementation.)

`BridgeRequest` (`core/ports.go`) carries `CLI`, `Profile`, `Model`, `Prompt`,
`Workspace`, `Worktree`, `ProjectRoot`, `ArtifactPath`, `Completion`,
`PermissionMode`, `SystemPrompt`, `Env`, and `ExtraFlags`. Note `PermissionMode` is
passed as **typed config**, not a raw flag — so it never leaks into a non-claude
launch command (only Claude drivers honor `--permission-mode`).

---

## 3. LaunchIntent → Realizer (ADR-0022)

The core abstraction. A phase describes its launch in CLI-agnostic terms
(`bridge.LaunchIntent`, `launchintent.go`):

```go
type LaunchIntent struct {
    ModelTier     string // abstract: fast | balanced | deep
    Permission    string // bypass | plan | default
    SettingsScope string // project | all
    SessionMode   string // "ephemeral" | "named:<name>"
    AllowedTools  []string
    RawByCLI      map[string][]string // escape hatch, applied ONLY to the matching CLI
}
```

A per-CLI **Realizer** maps that intent to a concrete `Realization` (launch flags
*this* CLI defines, post-boot REPL input, controller hints) using the CLI's
declarative `params` table in its manifest (`go/internal/bridge/manifests/*.json`).
The engine is `Realize` (`realizer.go`); adding a CLI is a JSON file, never Go code.

Channels (ADR-0022): `flag` (argv at launch) · `repl` (keystrokes after boot) ·
`controller` (tmux lifecycle) · `noop` (this CLI ignores the intent). The policy is
**flags-first**: prefer a launch flag when the CLI defines one, fall back to REPL
injection, and controller-only intents emit nothing to the CLI. **An intent param
with no manifest entry is a no-op** — which is the property that makes a
foreign/unsupported parameter unable to abort a launch.

### Why this exists — the cycle-1 boot failure

The old model stored raw, *claude-shaped* argv in profiles (`--no-session-persistence`,
`--bare`) and forwarded it verbatim to whatever CLI a phase routed to. That fused an
*intent* with one CLI's *realization* and broke two ways (observed live 2026-05-26):
interactive claude rejected `--no-session-persistence` ("can only be used with
--print mode") → boot timeout; `agy` rejected the claude flags outright → boot
timeout. The reclassification that dissolves it: `--no-session-persistence` is not a
parameter, it is claude's *headless realization* of "ephemeral session" — which on a
tmux REPL is realized by the **controller killing the session on exit** (zero CLI
flags). Those belong on the controller, not the CLI argv.

### Abstract model vocabulary

`ModelTier` is provider-neutral — `fast | balanced | deep` — and each manifest's
`model_tier_map` translates it to that CLI's native model
(`balanced → sonnet` on claude, `→ gpt-5.4` on codex, `→ qwen3:30b` on ollama).
**Why:** a profile that wants "the medium-effort tier" should never need to know
which CLI runs it. The translation is declarative config (Strategy pattern), not a
Go-side mapping table (ADR-0022 PR-2).

---

## 4. The drivers

Each CLI has a driver (`driver_*.go`). They split into two families:

- **Headless** (`claude-p`, `codex`, `agy`): exit after one prompt. Realize the
  intent to argv only; `repl`/`controller` channels are no-ops.
- **tmux-REPL** (`claude-tmux`, `codex-tmux`, `agy-tmux`, `ollama-tmux`): keep a
  persistent tmux session alive, drive it via `tmux send-keys` / `load-buffer` +
  `paste-buffer`, and run a poll loop while the agent works. They build `launchCmd`
  as `<binary> + Realization.LaunchFlags`, inject `Realization.REPLInput` *after*
  the boot marker and *before* the task prompt, and honor the session-lifecycle
  hints (Ephemeral / SessionName).

The **completion contract** (`BridgeRequest.Completion`, ADR-0027): `""`/`"artifact"`
polls the artifact file (default); `"stdout"` completes on REPL-idle for agents
that print their answer and write no file (the router/advisor). Only the `*-tmux`
drivers honor it.

OS-level **sandboxing** wraps every launch when `EVOLVE_SANDBOX=1`: `bridge.Deps.SandboxWrap`
calls `adapters/sandbox.GenerateSBPL` (macOS `sandbox-exec`) / `BwrapPrefix` (Linux
`bwrap`), setting the repo root read-only while allowing writes to the worktree +
workspace. This is the OS belt to the trust kernel's role-gate suspenders.

---

## 5. Interactive-policy prefix

An autonomous loop must never hang waiting for a human to answer a CLI's
`AskUserQuestion` or `y/N` prompt. The bridge adapter prepends a deterministic
**policy block** to every phase prompt (`adapters/bridge/bridge.go`, ADR-0023
facet B seam), so subagents self-resolve interactive prompts. Values
(`EVOLVE_INTERACTIVE_POLICY`, default `recommended_or_first`):

- `recommended_or_first` — pick the option labeled "(Recommended)", else the first;
  record `Auto-picked: <choice>`.
- `auto_yes` — binary y/N → yes; multi-option falls back to recommended-or-first.
- `escalate` — no block, fail loudly on ambiguity (legacy posture).

Per-agent override: `EVOLVE_<AGENT>_INTERACTIVE_POLICY` (e.g. pin the auditor to
`escalate` while every other phase stays autonomous). The block is kept under ~200
tokens and is deterministic to preserve the Claude prompt-prefix cache. A separate
**launch-time system prompt** (`EVOLVE_SYSTEM_PROMPT` / `EVOLVE_<AGENT>_SYSTEM_PROMPT`,
resolved by `systemprompt.Resolve`) prepends per-agent rules as a `## Rules` block;
order in the prompt is `rules < policy < body`.

---

## 6. Live injection — a runtime control channel (ADR-0023 facet A)

After `bridge.Launch()` hands a prompt to a tmux-REPL CLI, the launch was
historically fire-and-forget. Live injection adds a control channel: an append-only
NDJSON **inbox** at `<workspace>/.bridge-inbox/<agent>.ndjson`, drained inside the
driver's existing artifact-wait poll loop — **no async refactor, no orchestrator
change, the blocking Launch model is untouched**.

`evolve bridge send --workspace --agent [--kind] <body>` (or the phase-observer's
soft-stall nudge) appends an envelope. The cursor **seeks to EOF on driver entry**,
so a resumed named session never replays a pre-launch backlog. Kind vocabulary:

| Kind | Timing | Use |
|---|---|---|
| `command` / `nudge` / `system_rule` | idle-gated (wait for the prompt marker) | a correction / a "summarize-and-continue" nudge / a new rule |
| `interrupt` | sends ESC + settle, then injects | pre-empt a pending turn |
| `keystroke` | **no idle-gate, no pre-send** — raw `tmux send-keys`, verbatim | unblock a modal blocking the turn (`--body=Enter` confirms a y/N; `--body=C-c`) |

`keystroke` is the operator's full-control hatch: it exists precisely to send keys
*when the agent is not idle* (a modal blocking the turn) — the case where neither
`command` (idle-gated) nor `interrupt` (sends ESC, which would dismiss the modal)
fits. The bridge does not interpret the body; the operator owns everything that
reaches the REPL. **Scope:** Go-tmux-only by physics — headless drivers exit after
one prompt, so there is nothing to inject into.

---

## 7. Fallback chain + per-agent overrides (ADR-0029)

Pre-ADR-0029, each phase pinned exactly one CLI, making it a single point of
failure: cycle-121's auditor pinned `codex-tmux` and codex 0.134 hit a REPL-boot bug
that killed the whole cycle even though three other registered CLIs could have run
the phase. The fix is a **fallback chain**:

- **CLI precedence** (`runner/cli_chain.go:resolveCLIChain`):
  `EVOLVE_<AGENT>_CLI` > `EVOLVE_CLI` > `profile.cli` > `claude-tmux`.
- **Candidates** = `[primary] + dedup(profile.cli_fallback − primary)`.
- **Trigger codes** = `profile.cli_fallback_on_exit` or the default `[80, 81, 124,
  127]` (REPL-boot-timeout, artifact-timeout, observer-kill, missing-binary).

The dispatch loop (`runner/runner.go`) logs and ledger-records each attempt; on a
*trigger* exit it advances to the next candidate; on a *non-trigger* exit it
surfaces as-is. **A legitimate model FAIL never silently retries on a different
CLI** — the chain catches CLI-level integration bugs only, never substitutes a
different judge for a real verdict. A startup **capability probe** avoids burning a
60s boot timeout on a missing binary. Per-agent overrides
(`--cli <agent>=X`, `--model <agent>=X`) are **agent-keyed** (`EVOLVE_<AGENT>_…`),
not phase-keyed (ADR-0022 addendum) — pin to the profile that runs.

---

## 8. Recipe engine + capability catalog (ADR-0031)

Two layers above the raw tmux primitives let the bridge be driven independently of
the orchestrator:

- **Recipe engine** (`go/internal/bridge/recipe/`). A pure state machine over
  (pane-snapshots-in, key-tokens-out) that drives scripted *multi-step* slash-command
  sequences — the motivating example is a plugin install (`/plugin marketplace add`
  → `/plugin install` → `/reload-plugins`). A recipe is declarative JSON: ordered
  `steps[]` each `{send, await, on_timeout}` with `{{param}}` substitution and a
  `per_cli` map. The per-step loop is send → poll `Capture` → run the auto-responder
  each tick (so modals between steps are still dismissed) → evaluate the `await`
  condition → advance/timeout. It **owns** its small ports (`SessionDriver`, `Clock`)
  so the dependency arrow is one-directional (`bridge → recipe`, no cycle).
- **Capability catalog** (`go/internal/bridge/capabilities/`). One
  `catalogs/<cli>.json` per CLI (slash commands, key bindings, extension mechanism,
  headless entrypoint), grounded in a research dossier. `ParseHelp` parses a captured
  `/help` pane; `Diff` reconciles it against the static catalog — making capability
  knowledge **auditable and drift-detectable** instead of tribal.
- **keyspec** (`go/internal/bridge/keyspec/`) classifies `keystroke` bodies and
  WARNs on tokens that look like mistyped key names (`Excape`) — but never refuses
  the send, because the full-control hatch is inviolate.

CLI surface: `evolve bridge recipe run|list|show`, `evolve bridge capabilities
--cli=X`, `evolve bridge introspect --cli=X` (diff live `/help` vs catalog). Both
the recipe and catalog use the same embed + `EVOLVE_BRIDGE_*_DIR` override pattern
as the manifests.

---

## 9. Putting it together

```
phase (runner)                bridge layer                          CLI
──────────────                ────────────                          ───
PhaseName ──→ profile ──→ LaunchIntent ──Realize(manifest)──→ Realization
                          {fast,bypass,…}                      {flags, repl, ctrl}
              + policy prefix (adapter)
              + system prompt (systemprompt.Resolve)
                                      │
              resolveCLIChain ────────┤ primary + fallbacks
                                      ↓
                              driver_<cli>.go ──launch──→ claude / codex / agy / ollama
                                      │  ← live inbox (.bridge-inbox/<agent>.ndjson)
                                      ↓
                              artifact poll / stdout-idle → BridgeResponse
```

The phase never names a flag; the manifest never knows about phases; the
orchestrator never branches on a CLI. That separation is what makes "any CLI × any
phase × any model" a property of the architecture rather than a per-cycle hope.
