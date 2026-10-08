# Build Explanation — Cycle 1842

## Build Binding
- Cycle: 1842
- Base SHA: 8f46ad383ab22225c7015c60cf7f3afa2e14c06e

## Summary
Generated command wrappers in `commands/` now instruct readers to read `${CLAUDE_PLUGIN_ROOT}/skills/<name>/SKILL.md` directly rather than naming the registered command identifier `evo:<name>`, preventing infinite recursive self-loads in the Skill tool.

## Rationale
Claude Code commands and skills share the `/evo:<name>` namespace. When `RenderCommandStub` generated command wrappers instructing the Skill tool to load `evo:<name>`, Claude Code resolved `evo:<name>` back to the command wrapper itself, causing an infinite recursive dispatch loop. Pointing directly to the installed plugin file path `${CLAUDE_PLUGIN_ROOT}/skills/<name>/SKILL.md` ensures the underlying skill instructions are read immediately and cleanly without circular recursion.

## Changed Areas
- `go/internal/skillcheck/commands.go` — updates `RenderCommandStub` to generate direct `${CLAUDE_PLUGIN_ROOT}` skill path instructions and omit self-referential `evo:<name>` IDs.
- `commands/adversarial-testing.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/architecture-review.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/audit.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/build.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/code-review-simplify.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/commit.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/diff-review.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/engineering-craft.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/evaluator.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/explain.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/fable.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/golang-test-review.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/inspirer.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/intent.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/loop.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/minimalism.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/phase-create.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/plan-review.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/publish.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/quality-index.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/refactor.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/release.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/retro.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/scout.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/security-review-scored.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/setup.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/ship.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/solution-audit.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/solution-build.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/solution-scout.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/tdd.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `commands/verify-release.md` — regenerated stub pointing to the plugin SKILL.md path without self-ID reference.
- `skills/evaluator/SKILL.md` — removed description clause referencing the slash command ID to prevent self-ID insertion into the generated stub.
- `skills/inspirer/SKILL.md` — removed description clause referencing the slash command ID to prevent self-ID insertion into the generated stub.
- `skills/loop/SKILL.md` — removed description clause referencing the slash command ID to prevent self-ID insertion into the generated stub.
- `skills/publish/SKILL.md` — removed description clause referencing the slash command ID to prevent self-ID insertion into the generated stub.
- `skills/setup/SKILL.md` — removed description clause referencing the slash command ID to prevent self-ID insertion into the generated stub.
- `skills/verify-release/SKILL.md` — removed description clause referencing the slash command ID to prevent self-ID insertion into the generated stub.

## Design Decisions
Direct path resolution through `${CLAUDE_PLUGIN_ROOT}` was chosen over modifying the Skill tool or introducing an alias mechanism. This keeps command wrapper stubs simple, self-contained, and portable across Claude Code and compatible runtimes without altering the plugin registration schema.

## Verification
Unit tests in `go/internal/skillcheck/commands_test.go` and publisher tests in `go/cmd/evolve/cmd_skills_publish_test.go` verify that stubs emit `${CLAUDE_PLUGIN_ROOT}` and reject circular `evo:<name>` IDs. ACS predicates `TestC1842_001` through `TestC1842_004` verify generator properties, stub idempotence, repo-wide zero drift, and legacy stub drift detection.

## Compatibility
Command names, arguments, and slash command invocations remain fully backward compatible. Stubs continue to project the same frontmatter description and argument hints.

## Limitations
The command stub relies on `${CLAUDE_PLUGIN_ROOT}` being defined in the plugin runtime environment.
