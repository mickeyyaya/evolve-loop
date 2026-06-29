# ADR-0067: Command-surface re-introduction — project skills to `commands/<name>.md` for the `/evo:` slash-command menu

Status: Accepted
Date: 2026-06-29 (records decisions shipped v21.0.0 → v21.4.0)
Relates to / supersedes:
- **Supersedes [ADR-0040](0040-skill-naming-and-single-source-projection.md) §1's "delete the commands/
  layer" decision.** ADR-0040's *no-stutter skill naming* and *SKILL.md-as-projection* decisions stand
  unchanged; only its "skills are the **only** invocation surface, the `commands/` layer is deleted" clause is
  reversed here.
- Extends [ADR-0066](0066-cross-cli-plugin-install-and-manifest-schema-conformance.md) — the `commands/`
  stubs are one of `skillcheck`'s projection surfaces (alongside the phase-facts regions and the codex
  manifests).
- Reference: `reference_cc_plugin_skill_typeahead_bug` (plugin skills are absent from Claude Code's `/`
  typeahead — CC issues #18949 / #17271 / #21125).

## Context

### ADR-0040 deleted the commands/ layer; reality made that untenable

ADR-0040 (v16.9.0) collapsed two parallel invocation surfaces into one: it **deleted** the
`.claude-plugin/commands/*.md` layer and declared *skills the only invocation surface*, on the premise that
Claude Code skills "carry slash invocation natively." That premise proved false in practice: **plugin skills
do not reliably appear in Claude Code's `/` typeahead menu** (CC #18949 / #17271 / #21125). A user who
installs evo sees the skills via the Skill tool but cannot discover `/evo:loop`, `/evo:tdd`, … in the menu —
the exact discoverability the deleted commands layer had provided.

### The Claude Code command-namespace timeline drove a two-step fix

| CC version | Command-file behaviour | Consequence for evo |
|---|---|---|
| ≤ 2.1.193 | A plugin's `commands/<name>.md` surfaces as a **bare** `/<name>` — no plugin prefix | `commands/loop.md` → `/loop`, **colliding** with the built-in `/loop`. A bare layer is unusable. |
| 2.1.195 (#50486) | `commands/<name>.md` surfaces as **`/<plugin>:<name>`** — the plugin name namespaces it | `commands/loop.md` → `/evo:loop` natively. The collision is structurally impossible. |

evo's namespace had already been renamed `evolve-loop` → `evo` in **v21.0.0** (so the target surface is
`/evo:<name>`, matching `.evolve/naming.json:canonical.commandPrefix`). The command layer was then
re-introduced in two steps tracking the CC behaviour above.

## Decision

**Re-introduce a generated `commands/<name>.md` projection** so every skill surfaces as `/evo:<name>` in the
Claude Code menu. The projection is single-source (ADR-0040's principle, now applied to the command surface
instead of deleting it):

- **SKILL.md stays the source.** Each `commands/<name>.md` is a thin generated stub carrying only the skill's
  `description` + `argument-hint` (for the menu) and a body that delegates back to the skill. It bears a
  generated-by provenance marker (`commandGenMarker`) naming its source skill. `evolve skills generate` writes
  the stubs **and reaps orphans** (a marker-bearing stub whose backing skill is gone, e.g. the legacy
  `evo-*.md`); `evolve skills check` is read-only — it gates drift, exiting 2 on any orphan or stale stub.
- **`plugin.json` declares `commands: ["./commands/"]`** so Claude Code loads the layer.

### The bare-vs-prefixed filename (the v21.3.0 → v21.4.0 step)

| Release | Stub filename | Surfaces as | Why |
|---|---|---|---|
| **v21.3.0** (#276) | `commands/evo-<name>.md` | `/evo-loop` on CC ≤2.1.193 | the `evo-` prefix dodged the bare-`/loop` collision with the built-in command |
| **v21.4.0** (#278) | `commands/<name>.md` (**bare**) | `/evo:loop` on CC 2.1.195 | once #50486 namespaced commands as `/<plugin>:<name>`, the `evo-` prefix **double-namespaced** to `/evo:evo-loop`; the bare filename is correct |

`skillcheck.CommandFileName` is the single source for the filename convention, shared by the Claude Code
projection and the agy cross-CLI projection (both `/<plugin>:<basename>` hosts), so the convention cannot
drift between surfaces. The legacy `evo-*.md` stubs are reaped as orphans on the next `generate`.

## Considered alternatives (rejected)

1. **Keep skills as the only surface (hold ADR-0040)** — rejected: skills are not discoverable in the `/`
   menu (CC #18949 et al.); a user cannot find `/evo:loop` without already knowing it exists.
2. **Hand-author `commands/*.md`** — rejected: re-creates the duplication ADR-0040 removed (prose drifts from
   SKILL.md). The stubs are *generated* and drift-gated, so SKILL.md remains the single source.
3. **Keep the `evo-` prefix after 2.1.195** — rejected: double-namespaces to `/evo:evo-loop`. The bare
   filename + the plugin-name namespace is the only form that yields `/evo:loop`.

## Consequences

- A Claude Code user discovers every evo skill as `/evo:<name>` in the slash-command menu, while SKILL.md
  remains the single authored source — ADR-0040's anti-duplication goal is preserved by *projection*, not by
  *deletion* of the command surface.
- The same `CommandFileName` convention + `RenderCommandStub` renderer feed the agy projection (ADR-0066 §4),
  so the `/evo:<name>` surface is consistent across plugin-host CLIs.
- **ADR-0040 §1 is partially superseded** — its naming rules and SKILL.md-projection stand; its "commands
  layer deleted / skills-only" clause does not stand. This ADR is the current record for the command surface.
- Drift guard: `acs/regression/pluginnamespace` pins that `plugin.json`/`marketplace.json` `name` is `evo` (so
  commands resolve as `/evo:*`), and `evolve skills check` keeps the stubs in sync with the skills.
