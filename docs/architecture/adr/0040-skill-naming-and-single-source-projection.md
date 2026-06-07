# ADR-0040: Skill Naming Normalization + Single-Source Skill Projection

- Status: Accepted
- Date: 2026-06-07
- Extends: ADR-0033 (verdict single source), ADR-0034/0035 (deliverable contracts), ADR-0038 (phase plugins)

## Context

The plugin (≤ v16.9.0) exposed **two parallel surfaces** for the same pipeline phases:

1. `.claude-plugin/commands/*.md` (9 files: scout, plan-review, tdd, build, audit, ship, retro,
   loop, intent) → `/evolve-loop:scout`, `/evolve-loop:build`, … These duplicated prose from the
   skills and still referenced the legacy bash dispatcher
   (`legacy/scripts/dispatch/subagent-run.sh`) removed at the v12 flag day.
2. `skills/evolve-*/SKILL.md` (9 phase skills) → stuttering names `/evolve-loop:evolve-build`,
   `/evolve-loop:evolve-loop` — the directory name repeated the plugin namespace.

Concrete duplication: the single-writer-invariant paragraph appeared nearly verbatim in
`commands/build.md` AND `skills/evolve-build/SKILL.md`; output-contract section headings were
**triplicated** across SKILL.md, `agents/evolve-*.md`, and
`go/internal/phasecontract/contract.go`. Skill `evolve-spec` actually served the *scout* phase.
Three on-disk skills (`adversarial-testing`, `golang-test-review`, `setup`) were absent from
`plugin.json:skills[]`. The two-tier naming rule (commit `0149d81`) was applied but never
codified.

## Decision

### 1. One surface per phase; no-stutter naming

- The `.claude-plugin/commands/` layer is **deleted** (all 9 files + the `commands[]` manifest
  array). Skills are the only invocation surface; they carry `argument-hint` and slash
  invocation natively.
- Phase skill directories drop the `evolve-` prefix. The namespace already supplies it:
  `/evolve-loop:build`, not `/evolve-loop:evolve-build`.

| Old dir | New dir | Surface |
|---|---|---|
| skills/evolve-spec | skills/scout | /evolve-loop:scout |
| skills/evolve-plan-review | skills/plan-review | /evolve-loop:plan-review |
| skills/evolve-tdd | skills/tdd | /evolve-loop:tdd |
| skills/evolve-build | skills/build | /evolve-loop:build |
| skills/evolve-audit | skills/audit | /evolve-loop:audit |
| skills/evolve-ship | skills/ship | /evolve-loop:ship |
| skills/evolve-retro | skills/retro | /evolve-loop:retro |
| skills/evolve-intent | skills/intent | /evolve-loop:intent |
| skills/evolve-loop | skills/loop | /evolve-loop:loop (macro) |

`evolve-spec → scout` also fixes the dir-name/phase mismatch. Utility skills (13) were already
compliant and keep their names. All on-disk skills are now explicitly listed in
`plugin.json:skills[]`.

**Naming rule (codified in [docs/conventions/skill-naming.md](../../conventions/skill-naming.md)):**
- Single-word skill names = the closed builtin phase vocabulary (scout, tdd, build, audit, ship,
  retro, loop, intent, plan-review†).
- Utility / user / minted names = `<object>-<action>` kebab-case (e.g. `verify-release`,
  `phase-create`).
- A skill directory MUST NOT repeat the plugin namespace (no `evolve-` prefix inside the
  `evolve-loop` plugin).
- Frontmatter `name:` MUST equal the directory name (enforced by the drift test, §3).

† `plan-review` is two words but is itself a member of the closed phase vocabulary
(matches the phase name in the registry).

### 2. Phase SKILL.md = projection, not source

Phase skill docs previously hand-duplicated facts whose authoritative homes already exist. Each
fact now has ONE home, projected into SKILL.md by `evolve skills generate`:

| Generated section | Single source of truth | Loader reused |
|---|---|---|
| Output-contract headings | `phasecontract.FromSpec` / `phasecontract.All` | direct pkg call |
| Artifact paths, inputs, gates, archetype | `docs/architecture/phase-registry.json` (+ user `phase.json` via merge) | `phasespec.Load` + `Merge` |
| Description / when-to-invoke facts | `agents/<agent>.md` frontmatter | `prompts.Agent` |
| Fan-out / CLI / budget facts | `.evolve/profiles/<role>.json` | `internal/profiles` |

Generated regions are **marker-delimited**
(`<!-- GENERATED:<section> BEGIN -->` … `<!-- GENERATED:<section> END -->`); hand-written prose
outside markers is preserved verbatim on regeneration. The template lives once at
`go/cmd/evolve/templates/skill.md.tmpl` (go:embed). Rejected alternative: full-file generation —
it would force voice/nuance prose into data files, bloating the registry with prose that has no
runtime consumer.

### 3. Drift enforcement

`evolve skills check` (exit 2 on drift) plus a CI test
(`go/cmd/evolve/cmd_skills_drift_test.go`, same `runtime.Caller` pattern as
`phasecontract/contract_test.go`) assert:
1. regenerating produces byte-identical generated regions, and
2. every skill's frontmatter `name` equals its directory name.

A hand edit inside a generated region fails CI; the fix is to edit the SSOT and regenerate.

## Consequences

- Breaking surface rename → **v17.0.0**. Installed users pick up the new skill set via
  marketplace propagation; stale plugin-cache slash entries (`/evolve-loop:evolve-*`) may linger
  for one session until the cache refreshes.
- User-minted phases under `.evolve/phases/` are unaffected (no SKILL.md projection; invoked via
  `evolve phase <name>`); they follow `<object>-<action>` naming per the convention doc.
- `versionbump`/`releasepipeline` now target `skills/loop/SKILL.md` (was
  `skills/evolve-loop/SKILL.md`).
- Adding or changing a phase fact (heading, artifact, fan-out) is a one-place edit in its SSOT
  followed by `evolve skills generate` — drift is structurally impossible to ship.
