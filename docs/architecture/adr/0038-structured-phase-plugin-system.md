# ADR-0038: Structured Phase Plugin System — Index, Multi-Root Discovery, Conversational Registration

- Status: Accepted
- Date: 2026-06-05
- Extends: ADR-0035 (spec-derived deliverable contracts), ADR-0034 (unified deliverable contract), ADR-0024 (dynamic phase routing / PhaseAdvisor)
- Companion design doc: [phase-plugin-system.md](../phase-plugin-system.md) · Research: [micro-phase-catalog-research-2026-06-05.md](../../../knowledge-base/research/micro-phase-catalog-research-2026-06-05.md)

## Context

ADR-0035 made a phase fully definable as config (`.evolve/phases/<name>/phase.json` +
persona), but three gaps kept phases from being a true *plugin* surface:

1. **No discovery beyond the project.** `DiscoverUserSpecs` walked exactly one root
   (`.evolve/phases`), so a third party could not ship phases as an installable bundle.
2. **The advisor selected blind.** `PhaseCard` carried only name/role/tier/writes_source —
   no description, no when-to-use, no goal-type categories. SELECT-over-MINT asked the
   advisor to prefer phases it knew nothing about, and there was no index artifact an LLM
   could read offline to learn what phases exist (skills have `skill-inventory.json`).
3. **Registration was manual.** The "LLM builds its own phase" flow (mint) was ephemeral
   per-cycle; persisting one meant hand-writing files. There was no validated, self-correcting
   entry point an arbitrary LLM CLI could drive conversationally.

## Decision

Extend the existing phasespec system (no parallel system) with four layers:

| Layer | Mechanism |
|---|---|
| **Metadata** | `PhaseSpec` gains `description`, `when_to_use`, `categories[]` (closed goal-type vocab: bugfix/feature/refactor/security/performance/release/docs). Soft-validated: unknown categories are lint warnings, never load gates. |
| **Discovery** | `DiscoverUserSpecsFromRoots(roots)` over the single env seam `EVOLVE_PHASE_ROOTS` (colon-separated, default `.evolve/phases`, the `EVOLVE_KB_SEARCH_PATHS` idiom). Built-ins win; among user roots, left-most wins with a shadowing warning. Plugin bundles register by adding their path — evolve never scans CLI-private plugin caches. |
| **Index** | `evolve phase-inventory build` → `.evolve/phase-inventory.json` (gitignored cache, skillinventory pattern): per-phase spec metadata + contract-derived facts (artifact, required_sections, emits_verdict) + provenance (source, root) + `category_index`. **Advisory-only**: execution always reads the live merged catalog, so a stale index can never break a cycle. |
| **Registration** | `evolve phases create --spec <file\|-> [--persona …] [--mint …]` — floor-validate, collision-check (built-ins + all roots + persona overwrite), transactional scaffold, force inventory rebuild, and a **machine-parseable JSON envelope on stdout** (`ok/errors/warnings/hint`) so any LLM CLI self-corrects without screen-scraping. The thin `phase-create` skill is documentation around this command. |

Advisor side: `PhaseCard` carries the new metadata; `writeCatalog` renders enriched lines
(`- name [role, writes-source] (categories) — when: <hint ≤140 runes>`) with at most 12
enriched cards — optional cards bearing metadata take the slots first — and ALL remaining
names stay selectable via a name-only overflow line (a phase absent from the prompt cannot
be SELECTed). Relevance judgment stays with the advisor LLM; Go only bounds tokens.

## Alternatives considered

- **New skills-style plugin system (PHASE.md frontmatter):** uniform with skills but
  duplicates a working schema and forces a migration. Rejected — extend, don't fork.
- **Metadata in agent .md frontmatter:** splits the SSOT across two files and makes 3p
  bundles ship both coherently. Rejected — phase.json is the machine contract.
- **Scanning CLI plugin caches for `phases/` dirs:** couples evolve to three CLIs' private
  cache layouts. Rejected — one env seam, bundles opt in explicitly.
- **Deterministic goal-type relevance filter in writeCatalog:** no `goal_type` signal exists
  in the routing plane, and classification is LLM work (Core Rule 5). Rejected in favor of
  LLM-side relevance + Go-side caps.
- **Mid-cycle visibility of new phases:** would touch the per-transition Propose path and
  complicate floor clamping. Rejected — next-cycle-start visibility (composition root
  re-reads the catalog every cycle; `create` force-rebuilds the inventory).

## Trust floor (unchanged, restated)

Plugin/LLM-created phases are pure declarative JSON + persona markdown: `optional:true`,
`kind:"llm"` only, spine (build→audit→ship) untouchable, same gates/policies as built-ins.
`writes_source:true` phases get the sandbox role-gate. Operators should review a plugin
root's phases before adding it to `EVOLVE_PHASE_ROOTS` — a `writes_source` phase runs in
the worktree.

## Consequences

- Any LLM CLI can register a phase at any time by shelling out to the Go binary; the
  envelope contract makes self-correction mechanical. Mints can be persisted (`--mint`).
- The advisor's SELECT decisions are informed (when-to-use + categories) and the prompt
  cost is bounded regardless of how many plugin phases exist.
- The seed `reproduce-bug` phase (highest-leverage micro-phase per the 2026-06-05 research;
  since renamed `bug-reproduction` by the two-tier naming rule)
  was registered THROUGH the new flow as the end-to-end proof; the real-catalog e2e test
  (`seed_phase_e2e_test.go`) guards the merged-catalog → advisor-card → rendering chain.
- Known accepted risk: `.gitignore`'s `**/.evolve/` shadows *new* `phase.json` files
  despite the documented allowlist (`!.evolve/phases/*/phase.json`) — committing a new
  phase requires `git add -f` until the ignore rules are reworked.
