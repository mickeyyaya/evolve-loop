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
- **De-dup `default_args` vs realized flags:** a manifest's `default_args` (e.g. claude's
  `--dangerously-skip-permissions`) and the realized `permission=bypass` flag are the SAME flag from
  two sources. The wiring must apply ONE — drop `default_args` for the realized launch (the realizer
  is the single source of launch flags) or dedup — never emit both.
- **RealizeFor empty-result caveat:** an empty `Realization` is indistinguishable from a missing
  manifest. Validate the CLI (driver registry / LoadManifest) before trusting an empty result;
  don't infer "no flags needed" from emptiness (a typo'd CLI would launch bare).

## Acceptance

`EVOLVE_BRIDGE_GO=1 evolve loop` with builder→agy-tmux / auditor→codex-tmux / rest→claude-tmux boots
every phase (no EC80) and the cycle progresses; bridge packages stay at 100% coverage, `-race` clean.

## Phase 2b — wiring shipped (tmux drivers)

The three `*-tmux` drivers now build `launchCmd` as `<binary> + Realization.LaunchFlags`
(`launchCmdLine`); the inline model/permission construction is gone. `LaunchArgs` builds the
`LaunchIntent` from the profile (`ModelTier`=effective model, `Permission`=`permissionIntent(permMode)`,
`RawByCLI`=`profile.extra_flags_by_cli`, `SessionMode`) and stores `RealizeFor(cli, intent)` on
`Config.Realization`. Profiles migrated: flat `extra_flags` → `extra_flags_by_cli["claude-tmux"]`
(dropping the print-only `--no-session-persistence`), and `permission_mode` dropped (the bypass posture
is the realized default). builder.json(agy-tmux)/auditor.json(codex-tmux) keep their claude flags under
the `claude-tmux` key intentionally — `RawByCLI[agy/codex]` is nil, so a future switch back to claude
re-activates them without re-editing. Acceptance test: `realizer_wiring_test.go`
(`TestRealizerWiring_NoCrossCLILeak`) proves builder→agy = `agy --dangerously-skip-permissions`,
auditor→codex = `codex -m gpt-5.4`, scout→claude = full claude flags, zero cross-CLI leak.

### Two design shifts (recorded, not silent)

- **Bypass authority moved to the engine boundary.** `engine.Launch` (the programmatic runner entry,
  not human `evolve bridge launch`) unconditionally appends `--allow-bypass`. With `permission_mode`
  dropped from profiles, `cfg.PermissionMode` is `""`, so the tmux safety gates would otherwise block
  every in-process launch. The per-profile `permission_mode:bypassPermissions` opt-in is replaced by an
  always-on grant at the trusted-orchestrator boundary. Headless drivers ignore `AllowBypass` — no effect.
- **`permissionIntent("")` → `"bypass"`.** An empty `permission_mode` realizes to bypass, faithful to
  the drivers' prior default (`--dangerously-skip-permissions` when no mode set). claude-tmux/agy-tmux
  emit the bypass flag; codex is a controller no-op. A non-empty mode (e.g. `plan`) passes through and
  realizes per-manifest (claude maps bypass+plan).

### Phase 2c follow-ups (NOT in this slice)

- **Headless drivers do not yet consume `Realization`.** claude-p/codex/agy(headless) still build argv
  inline and read `Config.ExtraFlags`. Since `phaseflags.Resolve` reads the flat `extra_flags` (now empty
  on migrated profiles), a profile dispatched to **claude-p via the runner** would not receive its
  `extra_flags_by_cli["claude-p"]` flags. Currently benign: all 15 profiles use `*-tmux` CLIs, so no
  runner launch hits a headless driver. Unifying headless onto the Realization (and adding `claude-p`
  keys where needed) completes "unify headless+tmux".
- **Env-override permission-mode leak.** `EVOLVE_<PHASE>_PERMISSION_MODE` flows through
  `phaseflags.Resolve` into `Config.ExtraFlags` as a raw `--permission-mode` flag (not via the intent),
  so for a codex/agy phase it would reach the inner CLI and fail. Pre-existing; route the override
  through `LaunchIntent.Permission` in Phase 2c.
- **Dead flat `extra_flags` path.** `profiles.Profile.ExtraFlags` + the `phaseflags` read are now inert
  for migrated profiles; remove when headless unification lands.

## Addendum (Bug A, 2026-05-29) — agent-keyed env-key contract

Cycle-124 V1 verification surfaced a silent drop of `--model <agent>=<value>` overrides for any phase
whose `PhaseName()` differs from its `AgentPromptName()` minus the `evolve-` prefix: tdd/tdd-engineer,
build/builder, audit/auditor, retro/retrospective.

**Contract (pin this):** per-agent override env keys are **agent-keyed**, NOT phase-keyed.

| Override | Env key written by `cmd_loop.go:1131` | Read site that must agree |
|---|---|---|
| `--cli <agent>=X` | `EVOLVE_<AGENT>_CLI` | `runner.go:259` (`resolveCLIChain(profileName, …)`) ✓ |
| `--model <agent>=X` | `EVOLVE_<AGENT>_MODEL` | `runner.go:284` (FIXED — now `PhaseEnvKey(profileName, "MODEL")`) ✓ |
| `EVOLVE_<AGENT>_PERMISSION_MODE` | n/a (operator env) | `runner.go:310` ✓ already correct |
| `EVOLVE_<AGENT>_INTERACTIVE_POLICY` | n/a (operator env) | `bridge.go:injectPolicyPrefix` ✓ already correct |
| `EVOLVE_<AGENT>_SYSTEM_PROMPT` | n/a (operator env) | `systemprompt.Resolve(profileName, …)` ✓ already correct |

The single drift was at `runner.go:284`, which had used `PhaseEnvKey(phase, "MODEL")` — fixed in PR 1 to
`PhaseEnvKey(profileName, "MODEL")`. The regression guard is `runner_perphase_env_test.go`
(`TestRun_PerAgentModelEnvKey_AgentKeyedNotPhaseKeyed`) which exercises every known phase ≠ profile
pair.

**Why the convention is AGENT not PHASE.** A phase is the runtime stage (scout, tdd, build); a profile
is the deployment identity (scout, tdd-engineer, builder). The same phase could in principle dispatch
to different profiles in different contexts (e.g., a consensus auditor running two profile variants),
and the operator's override should pin to the profile that runs, not the phase that gates it. Aligning
all per-agent env keys to `profileName` keeps the override syntax predictable: `EVOLVE_<AGENT>_<KNOB>`
always reads the override for the profile of that name.
