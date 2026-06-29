# Skill Naming Convention

> TLDR: single-word = closed builtin phase vocabulary; everything else = `<object>-<action>`;
> never repeat the plugin namespace in a skill directory name. Decided in
> [ADR-0040](../architecture/adr/0040-skill-naming-and-single-source-projection.md).

## Rules

1. **Two-tier vocabulary**
   - **Tier 1 — builtin phase skills** use the closed single-word phase vocabulary, matching the
     phase name in `docs/architecture/phase-registry.json`: `scout`, `plan-review`, `tdd`,
     `build`, `audit`, `ship`, `retro`, `intent`, plus the macro `loop`. This vocabulary is
     closed — new members require an ADR.
   - **Tier 2 — utility / user / minted skills** use `<object>-<action>` kebab-case:
     `verify-release`, `phase-create`, `code-review-simplify`, `security-review-scored`, …
     Single nouns are acceptable when no action applies (`commit`, `setup`, `inspirer`,
     `evaluator`, `publish`, `release`, `refactor`).
2. **No namespace stutter.** The CLI renders skills as `/<plugin>:<dir>`; a directory name must
   not repeat the plugin name. `skills/build` → `/evo:build`. Never `skills/evolve-build`.
3. **Frontmatter `name:` MUST equal the directory name.** A mismatch makes the skill fail to
   load. Enforced by `evolve skills check` / `cmd_skills_drift_test.go`.
4. **Every skill on disk is listed in `.claude-plugin/plugin.json:skills[]`** — filesystem
   discovery is not an excuse for an incomplete manifest.
5. **Skills are the canonical source; `commands/<name>.md` is a generated projection of them.**
   Each skill is mirrored into a thin `commands/<name>.md` stub so it surfaces as `/evo:<name>` in
   Claude Code's slash-command menu (plugin skills alone are not discoverable in the `/` typeahead).
   The stub is generated and drift-gated by `evolve skills generate` / `check` — SKILL.md stays the
   single source; never hand-author a stub. See [ADR-0067](../architecture/adr/0067-command-surface-reintroduction.md).
6. **Phase skills are projections.** Structured facts (output-contract headings, artifacts,
   gates, fan-out) are generated from their SSOTs by `evolve skills generate`; edit the SSOT,
   not the generated region.

## Cross-CLI projection (ADR-0041)

`evolve skills publish` projects canonical skills into foreign CLI surfaces. Naming there is
namespace-relative:

- **Flat-namespace targets (Codex)** get the `evolve-` prefix — dir AND frontmatter `name:`
  become `evolve-<name>` (e.g. `~/.codex/skills/evolve-build`). This is the namespace
  *projection* for a target that has no plugin prefix, not a violation of rule 2: stutter is
  only stutter when the namespace already supplies the prefix.
- **Plugin-namespaced targets (agy)** keep unprefixed names — the `evolve-loop` plugin name
  supplies the namespace, mirroring the Claude layout.
- **Ollama** models are named `evolve-<name>` (flat model registry, same reasoning as Codex).

Projected artifacts carry the `EVOLVE-PUBLISH:projection` provenance marker; edit the
canonical skill and re-run `evolve skills publish`, never the projection.

## Phase → skill → agent → profile mapping

| Phase (registry) | Skill dir | Agent persona | Profile |
|---|---|---|---|
| scout | skills/scout | agents/evolve-scout.md | .evolve/profiles/scout.json |
| plan-review | skills/plan-review | agents/plan-reviewer.md | .evolve/profiles/plan-reviewer.json |
| tdd | skills/tdd | agents/evolve-tdd-engineer.md | .evolve/profiles/tdd-engineer.json |
| build | skills/build | agents/evolve-builder.md | .evolve/profiles/builder.json |
| audit | skills/audit | agents/evolve-auditor.md | .evolve/profiles/auditor.json |
| ship | skills/ship | (native phase — orchestrator) | .evolve/profiles/orchestrator.json |
| retrospective | skills/retro | agents/evolve-retrospective.md | .evolve/profiles/retrospective.json |
| intent | skills/intent | agents/evolve-intent.md | .evolve/profiles/intent.json |
| (macro) | skills/loop | agents/evolve-orchestrator.md | .evolve/profiles/orchestrator.json |

User-minted phases (`.evolve/phases/<name>/`) follow Tier 2 naming (`<object>-<action>`, e.g.
`bug-reproduction`, `account-reconcile`) and have no skill projection — they are invoked via
`evolve phase <name>`.
