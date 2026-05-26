# ADR-0022: CLI-agnostic LaunchIntent → per-CLI Realizer (flag/REPL/controller mapping)

> Status: **Proposed** (2026-05-26). Implements the framework that unblocks multi-CLI tmux
> dispatch (agy build / codex audit / claude rest), surfaced by the cycle-1 boot-failure in the
> bridge-port worktree. Supersedes the implicit "profiles store raw claude argv" model.

## Context

The bridge drives six drivers (claude-p, codex, agy + their `-tmux` REPL variants). Phase agents
carry launch parameters in `profile.extra_flags` — raw, **claude-shaped** argv such as
`--no-session-persistence`, `--bare`, `--strict-mcp-config`, `--setting-sources project`. The
bridge forwards them verbatim to whatever CLI a phase routes to.

That fuses an *intent* with one CLI's *realization* of it. It breaks two ways, both observed live
2026-05-26 in a real `evolve loop` cycle:

- **Interactive claude:** `--no-session-persistence` is print-mode-only → the REPL exits with
  `Error: --no-session-persistence can only be used with --print mode` → no `❯` → EC80 boot timeout.
- **Non-claude CLI:** `agy` rejects claude flags outright (`flags provided but not defined: -bare
  -no-session-persistence -strict-mcp-config`) → prints usage, exits → EC80.

The reclassification that dissolves it: `--no-session-persistence` is not a parameter, it is
claude's *headless realization* of the intent **"ephemeral session."** On a tmux REPL, "ephemeral"
is realized by the **controller killing the session on exit** — it emits zero CLI flags. Likewise
`--bare`/`--output-format` realize "structured output," which on the REPL path is the controller
reading the artifact file. These belong on the controller, not the CLI argv.

## Decision

Introduce a CLI-agnostic **LaunchIntent** and a factory-selected per-CLI **Realizer** that maps it
to a **Realization** across explicit channels. Operator decisions (2026-05-26):

| Axis | Decision |
|---|---|
| Realizer impl | **Hybrid** — Go factory/engine + declarative per-CLI `params` tables in each manifest (generalizes the existing `tier_aliases`). Add a CLI = JSON. |
| Channel policy | **Flags-first** — prefer a launch flag when the CLI defines one; use post-boot REPL injection only when there is no flag; controller-only intents emit nothing to the CLI. |
| Scope | **Unify** — all six drivers consume LaunchIntent → Realization (headless realizes to argv; tmux to flags + REPL injection + controller hints). |
| Profiles | **Migrate** common cases to high-level intent fields; keep a per-CLI raw `extra_flags` escape hatch applied ONLY to the matching CLI. |

### Types (package `bridge`)

```go
type LaunchIntent struct {
    ModelTier     string   // haiku|sonnet|opus  (abstract)
    Permission    string   // bypass|plan|default
    SettingsScope string   // project|all|""
    SessionMode   string   // "ephemeral" | "named:<name>"
    AllowedTools  []string
    RawByCLI      map[string][]string // escape hatch; applied only to the matching cli
}

type Realization struct {
    LaunchFlags []string // only flags THIS cli defines
    REPLInput   []string // typed after boot, e.g. "/model gpt-5.4"
    Ephemeral   bool     // controller: kill on exit
    SessionName string   // controller: named/resumable session
}
```

CLI-specific flags with no high-level intent yet (e.g. claude's `--strict-mcp-config`,
`--plugin-dir`) ride the `RawByCLI` escape hatch keyed by the cli name; promote them to first-class
intent fields if/when a second CLI needs the same concept.

### Channels

`flag` (argv at launch) · `repl` (keystrokes after boot) · `controller` (tmux lifecycle) ·
`noop` (this CLI ignores the intent). An intent param with **no manifest entry → no-op**, so a
foreign or unsupported param can never abort a launch again. (An `env` channel — for credential
vars like `ANTHROPIC_BASE_URL` — is a planned extension; not implemented in Phase 1.)

### Manifest `params` table (declarative, per CLI)

Each entry is a flat `ParamSpec`: a `channel` plus either a `values` map (enum intent value → the
full flag-token list) or a dynamic `flag`/`template` with `from:"tier_alias"`.

```jsonc
"params": {
  "permission":     { "channel":"flag", "values": {
                        "bypass": ["--dangerously-skip-permissions"],
                        "plan":   ["--permission-mode","plan"] } },
  "model_tier":     { "channel":"flag", "flag":"--model", "from":"tier_alias" }, // claude
  "settings_scope": { "channel":"flag", "values": { "project": ["--setting-sources","project"] } },
  "session_mode":   { "channel":"controller" },
  "allowed_tools":  { "channel":"flag", "flag":"--allowedTools" }               // flag once, then each tool
}
```
agy `model_tier` → `{"channel":"noop"}` (no model selector). codex `model_tier` →
`{"channel":"repl","template":"/model {alias}","from":"tier_alias"}`. The `tier_aliases` block
(previously unparsed by the Go `Manifest` struct) is formalized and consumed by `from:"tier_alias"`.

## Consequences

- Adding a CLI = one manifest (`params` + `tier_aliases`); never touch profiles or the controller.
- Adding an intent = one field + each manifest declares its mapping (default no-op).
- The cycle-1 boot failure is fixed by construction: claude-tmux emits no print-only flags; agy/codex
  emit only flags they define; "ephemeral"/"structured output" become controller hints.
- Migration: profiles gain high-level fields; the raw `extra_flags` becomes a per-CLI escape hatch.

### Phase 2 wiring notes (for the implementer)

- **LaunchFlags → launchCmd:** `runTmuxREPL` builds `launchCmd` as a string and `SendKeys` it.
  Join `Realization.LaunchFlags` into that string. The existing `Config.ExtraFlags` append at
  `driver_tmux_repl.go:106-107` must be reconciled: drain `Config.ExtraFlags` through
  `RawByCLI[cli]` (so it's CLI-scoped) and fold it into the realization BEFORE the single join —
  do not also append it afterwards.
- **REPLInput injection ordering:** deliver `Realization.REPLInput` AFTER the boot marker is
  detected and BEFORE the task prompt is pasted (`driver_tmux_repl.go` between marker-detect and
  prompt-deliver). A `/model` command must take effect before the task runs. Decide the ack model:
  send each REPL line, then re-wait for the prompt marker before the next (synchronous), rather
  than fire-and-forget — otherwise a slow `/model` re-render races the prompt paste.
- **Headless drivers:** realize the same intent to argv only (no REPL/controller channels); the
  `repl`/`controller` channels are no-ops there.

## Acceptance

`EVOLVE_BRIDGE_GO=1 evolve loop` with builder→agy-tmux / auditor→codex-tmux / rest→claude-tmux boots
every phase (no EC80) and the cycle progresses; bridge packages stay at 100% coverage, `-race` clean.
