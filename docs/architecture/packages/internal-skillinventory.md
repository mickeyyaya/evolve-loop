# internal/skillinventory

## Purpose

`internal/skillinventory` builds `.evolve/skill-inventory.json`, a cache of all available skills indexed by category. Scout reads the part of the category index that matches the project language, framework and task types, as routing hints (`skillCategories`). The CLI is `evolve skill-inventory build [--ttl 1h]`.

## Design

- **Cache freshness is the mtime of the output file.** When the file is younger than the TTL, `Build` does nothing and returns the cached path. An operator forces a rebuild with `--force` or by deleting the file. `DefaultTTL` is one hour, the cadence of the bash script this package replaced.
- **Discovery** walks `<project_root>/skills/*/SKILL.md` and reads the YAML frontmatter fields `categories` and `name`. A skill without frontmatter is listed under the category `uncategorized`, so it stays discoverable. A missing skills directory is not fatal: the result is an empty inventory.
- **`extractCategories` accepts every shape that `prompts.ParseFrontmatter` can emit.** These are a `[]string` (the common path), a comma-separated string (split as a fallback), and a `[]any` (defensive, for third-party loaders). An empty value or an unsupported type gives nil. The skill then maps to `uncategorized` and the scan does not crash.
- **Skill and category order is sorted,** so that the file diffs deterministically.
- **The write uses `atomicwrite.JSON`,** which creates the parent directory, writes mode 0644 and renames atomically.
- `NowFn` is the time source; a test overrides it for deterministic cache decisions.

## Invariants

- The JSON shape of `Inventory` is the public contract of Scout's `skillCategories` input. Keep it stable.
- `Options.ProjectRoot` is required.
