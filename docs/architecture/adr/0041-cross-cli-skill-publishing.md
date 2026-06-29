# ADR-0041: Cross-CLI Skill Publishing (`evolve skills publish`)

- **Status**: Accepted — **codex section superseded by [ADR-0066](0066-cross-cli-plugin-install-and-manifest-schema-conformance.md)** (2026-06-29)
- **Date**: 2026-06-07
- **Extends**: [ADR-0040](0040-skill-naming-and-single-source-projection.md) (skill naming + single-source projection)

> **⚠️ Codex superseded (2026-06-29, ADR-0066):** the codex target below projects loose
> `$CODEX_HOME/skills/evolve-<name>/SKILL.md` files. **codex 0.142.2 no longer discovers loose skills** — it
> ships a native plugin + marketplace system. The primary codex install is now `codex plugin marketplace add
> mickeyyaya/evolve-loop` → `codex plugin add evo@evo`, served by the repo-committed `.codex-plugin/plugin.json`
> + `.agents/plugins/marketplace.json` (projected by `skillcheck`). The agy and ollama targets here remain
> current. See ADR-0066 for the native-plugin decision and the manifest-schema-conformance principle.

## Context

The repo's canonical skills (`skills/<name>/SKILL.md`, enumerated by
`.claude-plugin/plugin.json:skills[]`) were published to exactly one surface:
the Claude Code plugin marketplace. The other three LLM CLIs this project
drives via the native bridge have their own — mutually incompatible —
extension surfaces:

| CLI | Extension surface (verified on codex 0.136 / agy 1.0.6 / ollama) |
|---|---|
| Codex CLI | Native Agent Skills: same SKILL.md frontmatter format, discovered from `$CODEX_HOME/skills/<name>/` (default `~/.codex/skills`). **Flat namespace** — no plugin prefix. Also scans repo-local `.agents/` roots, so in-repo discovery already works via the `.agents/skills/` symlink layer. |
| Antigravity (agy) | Native plugin system: `agy plugin validate|install <dir>` consumes a directory with `plugin.json` (`{"name": …}`) + `skills/<name>/SKILL.md`, installed to `~/.gemini/config/plugins/<name>/`. Its `plugin import claude` path does not recognize `.claude-plugin/`-rooted repos. |
| Ollama | **No skill/plugin system at all** (see `go/internal/bridge/capabilities/catalogs/ollama-tmux.json`: "Customization is limited to Modelfiles and runtime /set overrides"). The only persistence mechanism is `ollama create <model> -f Modelfile`. |

Hand-copying skills into those surfaces would violate the repo's
single-source-with-projection rule: canonical content has one home;
every other surface is a generated view.

## Decision

A new `evolve skills publish` subcommand (joining `generate|check` in
`cmd_skills.go` / `cmd_skills_publish.go`) projects the canonical skills into
per-CLI artifacts:

| Target | Names | Transform | Install step (`--install`) |
|---|---|---|---|
| codex | `evolve-<name>` | rewrite the single frontmatter `name:` line in-place (line splice — never YAML re-serialization, which is lossy) | copy staged mirror → `$CODEX_HOME/skills/` + prune |
| agy | unprefixed | verbatim + provenance marker | `agy plugin install .evolve/publish/agy/evo` |
| ollama | model `evolve-<name>` | Modelfile: `FROM <base>` + skill body as `SYSTEM """…"""` (literal `"""` downgraded to `'''`) | `ollama create evolve-<name> -f Modelfile` per skill |

### Flat-namespace prefixing

Codex skills live in a flat global namespace; names like `build`, `loop`,
`audit` are far too generic to claim there. The `evolve-` prefix is the
**namespace projection** for namespace-less targets — not a violation of
ADR-0040's no-stutter rule, which is namespace-relative (the Claude plugin
name already supplies the prefix there; codex has nothing supplying it).
agy keeps unprefixed names because the `evolve-loop` plugin name namespaces
them, mirroring the Claude layout.

### Safety invariant: stage by default, mutate only on `--install`

Bare `evolve skills publish` writes gitignored mirrors under
`.evolve/publish/{codex,agy,ollama}/` and runs the read-only
`agy plugin validate` — it never touches `$CODEX_HOME`, agy's plugin store, or
the ollama model registry. `--install` performs the mutating steps.
`--dry-run` prints the plan and writes nothing at all.

### Provenance + prune

Every projected artifact embeds the `EVOLVE-PUBLISH:projection` sentinel (HTML
comment after the frontmatter for .md; `#` comment in Modelfiles) naming the
canonical source and the regenerate command. Prune (default on; `--no-prune`)
deletes **only** artifacts that (a) carry the `evolve-` prefix AND (b) contain
the sentinel — a user's hand-authored `~/.codex/skills/evolve-mine` without
the marker is never touched. Installed ollama models cannot be inspected for
provenance, so created model names are recorded in
`.evolve/publish/ollama/manifest.json`; stale-model `ollama rm` runs only for
`evolve-*` names present in the previous manifest and absent from the fresh
render.

### Ollama read-only subset

Plain `ollama run` has no agentic tool use, and the bridge driver
(`driver_ollamatmux.go`) rejects write phases outright. Publishing
write/orchestration skills (tdd, build, loop, ship, commit, release, publish,
phase-create, setup, refactor) as system prompts would advertise capabilities
the runtime cannot honor. Only the reasoning/review subset is projected:
scout, plan-review, audit, retro, intent, evaluator, inspirer,
adversarial-testing, golang-test-review, code-review-simplify,
security-review-scored, verify-release (the `ollamaCompatible` map; skips are
logged, not silent).

### Base model default

No ollama profile exists in `.evolve/profiles/`, so the Modelfile `FROM`
default is the bridge driver's constant `llama3.1:8b`, overridable via
`--ollama-base` / `EVOLVE_OLLAMA_BASE`. TODO: if an ollama profile is added,
resolve the default through the profile + model-catalog tier mapping instead.

### Drift (`--check`)

`evolve skills publish --check` re-renders in-process and diffs against the
`.evolve/publish/*` mirrors (exit 2 on any missing/stale/extra file). It
answers "is the staged projection current?" — *environment* drift detection
(comparing against the installed `~/.codex/skills`, agy plugin store, ollama
registry) is a documented follow-up: agy transforms plugins on install and
ollama stores digests, so environment comparison is not deterministic.

## Alternatives considered

1. **Symlink `~/.codex/skills/<name>` → repo `skills/<name>`** — rejected:
   the frontmatter `name:` must change for the flat namespace, so a live link
   cannot carry the rename; and generic unprefixed names would collide.
2. **`agy plugin import claude`** — rejected: agy 1.0.6's importer scans for
   Claude *extensions*, not `.claude-plugin/`-rooted working trees; it found
   nothing. The native staging-dir + `agy plugin install` path is explicit
   and validateable.
3. **Skip ollama** — rejected by operator decision: the Modelfile projection
   is the one mechanism ollama has, and the read-only subset matches the tier
   this repo already assigns it.
4. **One-time manual copy** — rejected: stale on the next skill edit;
   violates single-source-with-projection.

## Consequences

- Re-running `evolve skills publish --install` after any skill change is the
  one sanctioned update path for all three foreign surfaces.
- `evolve skills publish --check` keeps staged projections honest in CI
  without touching user environments.
- The agy `plugin.json` is intentionally minimal (`{"name":"evolve-loop"}`,
  matching agy's own staged-plugin format); if a future agy version demands
  more fields, `agy plugin validate` (run on every publish) fails loudly.
- Codex in-repo discovery via `.agents/skills/` symlinks continues to work
  unchanged; the publish path adds the *user-global* surface.
