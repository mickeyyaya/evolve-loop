# Skill Naming Convention

> TLDR: a single word is for the closed vocabulary of builtin phases. All other skills use `<object>-<action>`.
> A skill directory name never repeats the plugin namespace.
> [ADR-0040](../architecture/adr/0040-skill-naming-and-single-source-projection.md) records this decision.

## Rules

1. **Two-tier vocabulary**
   - **Tier 1 — builtin phase skills** use the closed vocabulary of single-word phase names.
     Each name matches the phase name in `docs/architecture/phase-registry.json`:
     `scout`, `plan-review`, `tdd`, `build`, `audit`, `ship`, `retro`, `intent`, and the macro `loop`.
     This vocabulary is closed. A new member needs an ADR.
   - **Tier 2 — utility / user / minted skills** use `<object>-<action>` kebab-case.
     For example: `verify-release`, `phase-create`, `code-review-simplify`, `security-review-scored`.
     A single noun is permitted when no action applies (`commit`, `setup`, `inspirer`,
     `evaluator`, `publish`, `release`, `refactor`).
2. **No namespace stutter.** The CLI shows a skill as `/<plugin>:<dir>`.
   A directory name must not repeat the plugin name. `skills/build` → `/evo:build`. Never `skills/evolve-build`.
3. **Frontmatter `name:` MUST equal the directory name.** If the two are different, the skill does not load.
   `evolve skills check` / `cmd_skills_drift_test.go` enforce this rule.
4. **Every skill on disk is listed in `.claude-plugin/plugin.json:skills[]`.**
   The filesystem discovery does not make an incomplete manifest correct.
5. **Skills are the canonical source; `commands/<name>.md` is a generated projection of them.**
   - Each skill has a thin mirror stub in `commands/<name>.md`.
     The stub makes the skill show as `/evo:<name>` in the slash-command menu of Claude Code.
     (Without a stub, the `/` typeahead does not show a plugin skill.)
   - `evolve skills generate` / `check` generate the stub and gate its drift. SKILL.md stays the single source.
     Never write a stub by hand. See [ADR-0067](../architecture/adr/0067-command-surface-reintroduction.md).
6. **Phase skills are projections.** `evolve skills generate` generates the structured facts
   (output-contract headings, artifacts, gates, fan-out) from their SSOTs.
   Edit the SSOT, not the generated region.

## Cross-CLI projection (ADR-0041)

`evolve skills publish` projects the canonical skills into the surfaces of other CLIs.
There, the names are relative to the namespace:

- **Flat-namespace targets (Codex)** get the `evolve-` prefix. The directory AND the frontmatter `name:`
  become `evolve-<name>` (for example, `~/.codex/skills/evolve-build`). This is the namespace
  *projection* for a target that has no plugin prefix. It is not a violation of rule 2.
  A stutter occurs only when the namespace already supplies the prefix.
- **Plugin-namespaced targets (agy)** keep the names without a prefix. The `evolve-loop` plugin name
  supplies the namespace, the same as in the Claude layout.
- **Ollama** models have the name `evolve-<name>`. Ollama has a flat model registry, so the reason is the same as for Codex.

Each projected artifact has the `EVOLVE-PUBLISH:projection` provenance marker.
Edit the canonical skill and run `evolve skills publish` again. Never edit the projection.

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

User-minted phases (`.evolve/phases/<name>/`) use the Tier 2 names (`<object>-<action>`, for example
`bug-reproduction`, `account-reconcile`). They have no skill projection. To start one, use
`evolve phase <name>`.
