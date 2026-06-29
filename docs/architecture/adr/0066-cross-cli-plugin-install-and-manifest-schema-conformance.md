# ADR-0066: Cross-CLI plugin install — native plugin projection + platform-manifest schema conformance

Status: Accepted
Date: 2026-06-29
Released: v21.4.1 → v21.4.5 (PRs #280–#284)
Relates to / supersedes:
- **Supersedes the codex section of [ADR-0041](0041-cross-cli-skill-publishing.md)** — codex moved from
  loose `~/.codex/skills/evolve-<name>/SKILL.md` file discovery to a **native plugin + marketplace** install.
- Extends [ADR-0040](0040-skill-naming-and-single-source-projection.md) (single-source-with-projection) —
  the codex manifests are a third projection surface after the phase-facts regions and the `commands/` stubs.
- Inherits [ADR-0065](0065-per-phase-binary-integrity.md) — the `make build → evolve reset-sha -operator`
  self-heal recurs once per release in this work's operational notes.

## Context

### The incident

A new user's `/plugin install evo@evo` **failed on Claude Code 2.1.195** with the misleading error
*"This plugin uses a source type your Claude Code version does not support. Update Claude Code and try
again."* The plugin had shipped fine on prior CC versions.

### Root cause (verified against the CC 2.1.195 binary schema)

CC 2.1.195 tightened plugin-manifest validation and **claimed `binaries` as a native field**, typed as a
record: `binaries: A.record(<basename>, A.object({sha256: /^[0-9a-f]{64}$/, …platforms}))`. evo had
repurposed `binaries` as a **documentation array** (`[{name, module, source_build, …}]`) and added a custom
`compatibility` object. Two failures resulted, in order:

1. **marketplace entry** — CC's marketplace plugin-entry schema is `.strict()`; the unknown
   `binaries`/`compatibility` keys surfaced as the misleading *"source type … not supported"* error (thrown
   in the source resolver's switch default, never reaching the real validator).
2. **plugin.json** — once the entry parsed, the manifest schema reported `binaries: Invalid input: expected
   record, received array`.

The working reference marketplaces (`ecc`, `worktrunk`) carry only standard keys — confirming the two
vendor-extension fields were the sole offenders.

### The platform landscape shifted under ADR-0041

ADR-0041 (2026-06-07, verified on codex 0.136) modelled codex as **loose Agent-Skills file discovery**:
copy `evolve-<name>/SKILL.md` into `$CODEX_HOME/skills`. **codex 0.142.2 ships a native plugin + marketplace
system** — a near-clone of Claude Code's: `codex plugin marketplace add <owner/repo|path>` →
`codex plugin add <plugin>@<marketplace>`, reading `.agents/plugins/marketplace.json` at the repo root and,
for each plugin, `.codex-plugin/plugin.json` (with `"skills": "./skills/"`). Loose `~/.codex/skills` files are
no longer discovered. The three target CLIs have **converged on the same plugin-host shape**:
`.claude-plugin/` / `.codex-plugin/` + `skills/` + a `marketplace.json`.

### The generalizable lesson

A **platform-owned manifest is governed by the platform's evolving schema**. Documentation-only metadata
placed in keys the host may later claim becomes a hard install-blocker the moment the host tightens its
schema. Vendor metadata belongs in vendor-owned files, never in platform manifest keys.

## Decision

### 1. Manifest-schema conformance (platform manifests hold only platform-schema fields)

`.claude-plugin/plugin.json` and `.claude-plugin/marketplace.json` carry only fields the CC schema accepts.
The `binaries` array and `compatibility` object were removed; their content already lived in evo-owned
SSOTs — the release matrix in `.goreleaser.yml` (the manifest comment already said so) and the compatibility
tiers in this doc set. **Gate:** `go/acs/regression/pluginschema` (acs-tagged, runs in CI) asserts `binaries`
is absent-or-record (never an array/string), `compatibility` is absent, and marketplace plugin entries use
only CC-supported keys. It checks the live manifests AND adversarial fixtures, so it is not a tautology.

### 2. Codex native plugin install (supersedes ADR-0041 §codex)

`skillcheck` **projects** two codex manifests from the canonical `.claude-plugin/plugin.json`:

| File | Content | Source |
|---|---|---|
| `.codex-plugin/plugin.json` | name/version/description/author/homepage/repository/license/keywords + codex `"skills":"./skills/"` + `interface` | `.claude-plugin/plugin.json` (shared fields) + codex projection constants |
| `.agents/plugins/marketplace.json` | `name`, `interface.displayName`, one plugin entry: `source.path:"."`, `policy.authentication:"ON_USE"` (codex `.strict()` schema rejects `"NONE"`; only `ON_INSTALL`/`ON_USE`) | same |

A user installs exactly as on Claude Code:

```bash
codex plugin marketplace add mickeyyaya/evolve-loop
codex plugin add evo@evo            # → 23 skills under ~/.codex/plugins/cache/evo/evo/<version>/
```

`source.path:"."` (the repo root **is** the plugin) keeps the 23 canonical skills single-sourced under
`skills/` with **zero duplication**, at the cost of `marketplace add` git-fetching the whole repo. This is the
third projection surface in `skillcheck` (after phase-facts and `commands/`); `evolve skills generate` writes
the manifests and `evolve skills check` gates drift (also run in-process by the cycle audit phase).

### 3. Version-sync via a single source of truth (`versionbump.Paths.Files()`)

`versionbump.Paths.Files()` enumerates the marker files a release writes (plugin.json, marketplace.json,
**codex plugin.json**, SKILL.md, README.md). The release stager (`ship.stageReleaseSet`) consumes it — so a
marker registered in `Paths`/`Files()` is staged into the release commit automatically, which is the fix for
the codex regression. The writer (`versionbump.Run`) still accesses each marker by *named field*
(`BumpJSONVersion(paths.CodexPluginJSON, …)` etc.), so registering a new `Paths` field also requires its
explicit `BumpXxx` call in `Run`: `Files()` is the SSOT for **staging**, not a substitute for wiring the bump.
Three drift gates back this:

- `acs/regression/pluginschema` — codex⟷claude `name`+`version` parity on the **committed** state (CI).
- `releaseconsistency` — a `.codex-plugin/plugin.json` version marker (tolerant of an absent generated file)
  that fails the release → auto-rollback if the codex surface is stale.
- `evolve skills check` — full projection drift (dev + cycle audit).

### 4. agy — stage whole skill trees + prune the pre-rename plugin

`renderAgy` stages each skill's **entire** `skills/<name>/` directory — SKILL.md plus companion files and
`reference/` overlays (the loop SKILL.md body tells the agy runtime to read `reference/agy-tools.md`,
`reference/agy-runtime.md`, `reference/platform-detect.md` first; staging only SKILL.md left them 404, and
`agy plugin validate` only checks SKILL.md so the loss was invisible). The provenance header is injected only
into the entry SKILL.md (companions copied verbatim). `installAgy` best-effort prunes the pre-rename
`evolve-loop` plugin (gated on `--prune`, mirroring `installOllama`).

## The bug chain (incident archaeology — three releases to harden)

| Release | Change | What went wrong |
|---|---|---|
| v21.4.1 (#280) | removed `binaries`/`compatibility` | — (the install blocker; verified `✔ installed`) |
| v21.4.2/21.4.3 (#281–283) | added `CodexPluginJSON` to `versionbump.DefaultPaths` | bumped codex on **disk**, but `ship.stageReleaseSet` hand-listed marker files and **omitted** it → release committed codex one version behind. `releaseconsistency` reads the post-bump **working tree** and PASSED; only `acs/pluginschema` (committed state, in CI) caught it |
| v21.4.4 (#283) | `Paths.Files()` SSOT | root fix — stager consumes the writer's file list |
| v21.4.5 (#284) | agy whole-skill-tree + prune | — |

**Lesson:** a comment claiming `SSOT: versionbump.DefaultPaths` while hand-enumerating a subset of its fields
**is** the drift. Expose a `.Files()` method both the writer and the stager consume, so the list cannot be
copied wrong.

## Considered alternatives (rejected)

1. **Keep `binaries`/`compatibility`; rely on CC ignoring unknown keys** — impossible: CC's marketplace
   plugin-entry schema is `.strict()` and rejects them.
2. **Convert `binaries` to CC's native record shape** (`basename → {sha256, platforms}`) — rejected:
   over-scoped. evo's binary is built and released separately via `.goreleaser.yml`; CC's binary-shipping
   feature is not wanted.
3. **Codex via loose `~/.agents/skills` files** (ADR-0041 / the prior session's plan) — rejected: empirically,
   codex 0.142.2 does not discover loose skills; it requires a plugin. The native marketplace is the only
   path that surfaces all 23 skills.
4. **Vendor the codex skills into a self-contained `plugins/evo/skills/`** — rejected: duplicates the 23
   skills (violates single-source). `source.path:"."` keeps zero duplication.
5. **Bump the codex version by re-running `skills generate` inside the release pipeline** — rejected as
   riskier than extending the versionbump file list; `BumpJSONVersion`'s surgical regex keeps generate and
   release-bump byte-identical, so the simpler approach is also the safe one.

## Deferred (audit-found, deliberately not done)

A read-only conformance audit (an `evo-plugin-conformance-audit` workflow over claude 2.1.195 / codex 0.142.2
/ agy 1.0.13) found two further degradations whose fixes were **deliberately deferred**:

- **D1 — Claude Code loads 0/12 evo agents.** CC 2.1.195's agent-frontmatter schema is `.strict()`; evo's
  `agents/*.md` add `output-format`/`perspective`/`capabilities`/`tools-gemini`/`tools-generic` + a
  non-resolvable `model: tier-2`. `output-format` is **load-bearing** —
  `go/internal/phasecoherence/artifact_coherence.go` reads it, `acs/cycle387` enforces it, and
  `prompts_test.go` asserts it — so a fix is an architectural change to the agent schema entangled with the
  coherence/integrity machinery across ~90 files + gates + tests. The payoff is **cosmetic**: evo's loop
  dispatches agents through the bridge, which reads only the markdown **body** (never the frontmatter), and a
  user runs `/evo:loop`, never `@evolve-scout`. This warrants a dedicated ultrathink + strong-review effort,
  not a cleanup sweep.
- **D6 — agy command-stub dedup.** The audit claims agy folds `commands/` stubs into skills, colliding with
  the same-named real skills. This contradicts the deliberate, tested design
  (`TestPublishAgy_IncludesCommandStubs`, "so /evo:<name> appears in agy's menu"); overturning it needs
  empirical verification of agy 1.0.13's command-menu behaviour first.

## Consequences

- A new user installs evo on **Claude Code, Codex, and agy** from the same canonical repo, on each CLI's
  current version, with the commands documented in `platform-compatibility.md`.
- Every release keeps all manifest surfaces version-synced through one SSOT (`Paths.Files()`), gated three
  ways; a missed marker fails CI (and, for the codex version, the release itself).
- ADR-0041's codex *publish-to-`~/.codex/skills`* target in `cmd_skills_publish.go` is now historical for
  codex 0.142.2+ (which doesn't discover those files); it remains usable for the open-standard
  `--codex-home ~/.agents` surface. Retiring or retargeting that publish target is a documented follow-up.
- **Operational note (recurs every release):** the release in-process binary is the gitignored `go/bin/evolve`;
  a `git checkout` removes it and the per-phase integrity pin (ADR-0065) rejects a stale one — so each release
  is `cd go && make build` → `evolve reset-sha -operator` → `evolve release X.Y.Z`. `marketplace-poll` also
  requires the GitHub `evo` marketplace registered locally (`claude plugin marketplace add
  mickeyyaya/evolve-loop`); a local-directory marketplace makes the poll fail → auto-rollback.
