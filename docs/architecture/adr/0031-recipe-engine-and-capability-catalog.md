# ADR-0031 — Interaction recipe engine + capability catalog

> Status: **Accepted** (2026-05). Adds two layers above the existing
> READ→DECIDE→INJECT→VERIFY tmux primitives so the bridge can drive scripted
> multi-step slash-command sequences (e.g. plugin installs) and carry a
> written-down, drift-checkable record of each CLI's control surface.

## Context

The bridge already gives full human-equivalent control of a single keystroke
(the `keystroke` envelope → raw `tmux send-keys`, ADR-0023 facet A). What it
lacked: (1) a way to drive a *sequence* of slash commands with inter-step
verification — the operator's motivating example is installing a plugin
(`/plugin marketplace add` → `/plugin install` → `/reload-plugins`); and (2) a
durable, machine-readable record of what each CLI can do, cross-checked against
the official docs and the live `/help` surface.

## Decision

### 1. Recipe engine — a decoupled, port-owning sub-package

`go/internal/bridge/recipe/` is a pure state machine over (pane-snapshots-in,
key-tokens-out). It OWNS the small ports it needs (`SessionDriver`, `Clock`)
rather than importing `bridge`, so the dependency arrow is one-directional
(`bridge → recipe`) with no cycle. A recipe is declarative JSON: ordered
`steps[]`, each `{send{kind,body}, await{...}, on_timeout}`, with `{{param}}`
substitution and a `per_cli` map for flows whose mechanics differ per CLI.

The per-step loop is: send → poll `Capture` every interval → run `AutoRespond`
each tick (so modals appearing *between* steps are still dismissed — the
auto-responder is never bypassed) → evaluate the `await` condition
(`prompt_marker` | `regex` | `any_of` | `all_of`, with an optional early
`fail_regex`) → advance / time out. `on_timeout` is `abort` (default) or
`continue`.

Patterns: Repository (`LoadRecipe`, embed + `EVOLVE_BRIDGE_RECIPE_DIR`
override, mirroring `LoadManifest`), Template Method (`Engine.Run`), Strategy
(await kinds, send kinds), Command (each step), Adapter
(`recipeSessionDriver` in `bridge` implements the recipe port over the existing
`injectText` / `SendKeys` / `CapturePane` / `autoResponder.tick` primitives).

### 2. Capability catalog — static record + live drift

`go/internal/bridge/capabilities/` carries one `catalogs/<cli>.json` per CLI
(slash commands, key bindings, extension mechanism, headless entrypoint,
sources), grounded in the research dossier. `ParseHelp` parses a captured
`/help` pane; `Diff` reconciles it against the static catalog. Same embed +
`EVOLVE_BRIDGE_CATALOG_DIR` override pattern.

### 3. CLI surface — the bridge stays independently drivable

- `evolve bridge recipe run|list|show` — run a scripted sequence; no orchestrator.
- `evolve bridge capabilities --cli=X [--json]` — print the catalog.
- `evolve bridge introspect --cli=X [--pane-file=P]` — diff live `/help` vs catalog.

### 4. keyspec — warn-not-block key validation

`go/internal/bridge/keyspec/` classifies `keystroke` bodies (named keys,
modifier combos, literals) and flags tokens that look like mistyped key names
(`Excape`). It WARNs but never refuses the send — the full-control hatch is
inviolate.

## Consequences

- Plugin/skill installation is now a first-class, tested, repeatable bridge
  capability — drivable by an operator or the orchestrator with just a CLI name,
  a workspace, and params.
- The catalog makes capability knowledge auditable and drift-detectable instead
  of tribal.
- Two manifest corrections rode along: agy gains a real `-m` flag channel
  (was wrongly `noop`); codex's plugin/skill surface is recorded.

## Alternatives rejected

- Reusing `runTmuxREPL` for sequences: it is welded to the single-prompt
  artifact-wait contract; a fixed multi-step sequence has no artifact. A
  dedicated engine is cleaner than smuggling N steps through that seam.
- Putting recipes/catalogs inside the manifest JSON: conflates launch-time
  realization with interaction scripting; separate sibling files keep each
  concern single-purpose.

## References

- [docs/architecture/full-tmux-control.md](../full-tmux-control.md)
- [docs/architecture/cli-capability-matrix.md](../cli-capability-matrix.md)
- [knowledge-base/research/llm-cli-control-surfaces-2026-05.md](../../../knowledge-base/research/llm-cli-control-surfaces-2026-05.md)
- ADR-0022 (LaunchIntent/Realizer), ADR-0023 (live injection + keystroke).
