# internal/installer

## Purpose

`internal/installer` ports `install.sh` and `uninstall.sh` to Go. It copies the evolve-loop agents and the loop skill into `~/.claude/`. It also holds the CI-mode validator of the plugin layout. `evolve install` and `evolve uninstall` use it.

## Design

- **`Validate` is CI mode and copies nothing.** One pass replaces the legacy CI validation steps, with no python or bash. It asserts these points:
  - `plugin.json` exists and parses. It has `name`, `version`, `description`, `agents` and `skills`. The last two are JSON arrays.
  - `marketplace.json` exists and parses. Its `plugins` array is not empty.
  - Each `agents/evolve-*.md` opens with a `---` fence. Its frontmatter has `name:` and `description:`.
  - The required loop skill files and reference docs exist.
  
  Each hard failure increments `Errors`, and the command exits 1.
- **Reference docs are exempt from frontmatter.** `*-reference.md` files are layer-3 documents by design.
- **A manifest that does not parse is reported once.** It gets one JSON warning. The field checks return early and do not add more findings.
- **Frontmatter keys are read only inside the leading fence.** `frontmatterMissingKeys` stops at the closing fence. Body text can never satisfy a key. A key matches a line whose first non-blank token is `<key>:`.
- **`Install` and `Uninstall` take an explicit `homeDir`.** Agents go to `<home>/.claude/agents`. The loop skill goes to `<home>/.claude/skills/loop`. The package never reads `$HOME`. A test passes a temp directory and never touches the real home.
- **All filesystem changes go through `atomicwrite`.** A partial copy never leaves a half-written agent file in `~/.claude`.
- **`Uninstall` removes the agent files and the whole `skills/loop` directory. It removes nothing else.** It never touches the `.evolve/` workspace of the project. `UninstallDryRun` lists the same removals, deletes nothing and returns the same counts.
- **The caller warns about duplicate plugins.** `PluginAlreadyInstalled` reports if evolve-loop is already a CLI plugin, in a cache or marketplace directory. The cmd layer then warns about duplicate `/evo:loop` entries. The package does not prompt.
- **Two strings are pinned by evals.** `Version` is the banner substring that the `update-install-sh-version-string` eval asserts. `UsageLine` is the three-argument `[cycles] [strategy] [goal]` form that the `fix-install-usage-and-ci-docs-check` eval asserts.
- **The required skill set is a superset.** `loopSkillFiles` requires `online-researcher.md`. The legacy CI did not.

## Invariants

- **Tests never leave `t.TempDir()`.** The installer writes to the system of a user. Tests use injected source and home directories only. They never touch the real home, the `PATH` directories or `~/.claude`.
