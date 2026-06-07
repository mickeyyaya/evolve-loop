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
   not repeat the plugin name. `skills/build` → `/evolve-loop:build`. Never `skills/evolve-build`.
3. **Frontmatter `name:` MUST equal the directory name.** A mismatch makes the skill fail to
   load. Enforced by `evolve skills check` / `cmd_skills_drift_test.go`.
4. **Every skill on disk is listed in `.claude-plugin/plugin.json:skills[]`** — filesystem
   discovery is not an excuse for an incomplete manifest.
5. **Skills are the only invocation surface.** No parallel `.claude-plugin/commands/` entries
   (layer deleted in ADR-0040).
6. **Phase skills are projections.** Structured facts (output-contract headings, artifacts,
   gates, fan-out) are generated from their SSOTs by `evolve skills generate`; edit the SSOT,
   not the generated region.

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
