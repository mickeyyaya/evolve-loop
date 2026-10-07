# Structured Phase Plugin System

**Status:** Shipped (branch `feat/phase-plugin-system`, 2026-06-05)
**ADR:** [0038-structured-phase-plugin-system.md](adr/0038-structured-phase-plugin-system.md)
**Research:** [micro-phase-catalog-research-2026-06-05.md](../research/micro-phase-catalog-research-2026-06-05.md) · [micro-phase-catalog.md](micro-phase-catalog.md)

## 1. The request

> Build a structured phase agent plugin feature (which allows 3p and LLMs to build and
> register phases depending on their needs) with a structured index file the advisor can
> use to learn what phases are available (the same architecture as skills), or it can
> trigger a skill to build its own.

Decomposed with the user into ten pinned decisions:

1. Extend the existing phasespec system — no parallel plugin framework.
2. Distribution: drop-in `.evolve/phases/` + plugin-bundle roots + **primary UX =
   conversational creation from any LLM CLI** ("just ask the LLM to add a phase, like
   adding a skill").
3. Good ephemeral mints become persistent phases via the create flow.
4. Index = generated `phase-inventory.json`, modeled on `skill-inventory.json`.
5. Trust floor unchanged: phases are pure declarative JSON + persona markdown; the
   pipeline framework, rules, and policies are identical for built-in and plugin phases.
6. Rich metadata lives in `phase.json` (schema extension), not agent frontmatter.
7. New phases become visible at the **next cycle start** (no mid-cycle plan mutation).
8. Scope: infrastructure + one seed phase (`reproduce-bug`, since renamed
   `bug-reproduction` by the two-tier naming rule) registered through the new
   flow as the end-to-end proof.
9. Go CLI does the work; the skill is a thin conversational wrapper.
10. Built directly as feature-dev in an isolated worktree.

## 2. Approaches considered

| Decision | Chosen | Rejected (why) |
|---|---|---|
| Foundation | Extend phasespec | New PHASE.md skills-clone (duplicates a working schema); hybrid phase.json+PHASE.md (two SSOTs) |
| Plugin discovery | One env seam `EVOLVE_PHASE_ROOTS` (KB-paths idiom) | Scanning CLI plugin caches (couples to 3 CLIs' private layouts); a config-file key (second mechanism = flag sprawl); marketplace index (later wave) |
| Index | Generated, gitignored cache; advisory-only | Enriching phase-registry.json in place (no cross-root aggregation); live in-memory only (nothing for LLMs to read offline) |
| Advisor relevance | LLM-side judgment + Go-side token caps | Deterministic goal-type filter in Go (no `goal_type` signal exists; classification is LLM work — Core Rule 5) |
| Create flow | Go command + JSON envelope; thin skill | Rich Claude-only skill (breaks the any-CLI requirement); advisor-internal mint persistence only (no human entry point) |
| Visibility | Next cycle start | Mid-cycle live insertion (touches Propose path + floor clamping for marginal value) |
| Trust | Current floor for everyone | Trust tiers / signed verified lists; per-phase operator approval gates (breaks "LLM adds a phase mid-run") |

## 3. Architecture

```
                       ┌──────────────────────────────────────────────┐
 EVOLVE_PHASE_ROOTS    │  discovery (every cycle start + on demand)   │
 .evolve/phases:/plug… │                                              │
        │              │  phasespec.Load(registry)      built-ins win │
        ├─ root 1 ─────►  DiscoverUserSpecsFromRoots ── left-most wins│
        └─ root 2 ─────►  Merge → live Catalog                        │
                       └───────┬──────────────────────┬───────────────┘
                               │ (execution: ALWAYS    │ (advisory)
                               │  the live catalog)    ▼
                               │            phaseinventory.Build
                               │            .evolve/phase-inventory.json
                               ▼            (TTL cache; category_index;
                    orchestrator runners     provenance; contract facts)
                    + specrunner per phase            │
                               │                      ▼
                               │            phaseCardsFromCatalog
                               │            → enriched PhaseCards
                               ▼            → writeCatalog (≤12 enriched,
                    build → audit → ship       overflow name-only line)
                    (spine untouchable)               │
                                                      ▼
                                              PhaseAdvisor (LLM)
                                              SELECT > MINT
                                                      │ mint worth keeping?
                                                      ▼
   any LLM CLI ──► evolve phases create ◄── skills/phase-create (thin)
                   --spec - | --mint -
                   validate → collide-check → scaffold → rebuild index
                   stdout: {"ok":…} JSON envelope (self-correction loop)
```

### 3.1 Schema additions (`phasespec.PhaseSpec`)

```jsonc
{
  "description": "one line: what the phase produces",
  "when_to_use": "the signal/goal that should trigger SELECTing it (≤140 chars rendered)",
  "categories": ["bugfix"]   // closed vocab: bugfix|feature|refactor|security|performance|release|docs
}
```

All `omitempty`; the tolerant parser is unaffected; absence degrades to the legacy
name-only card. Unknown categories: `phasespec.UnknownCategories` → lint **warning**
(`evolve phase lint`, `phases create` envelope) — never a load gate.
Schema doc: [phase-descriptor.schema.json](phase-descriptor.schema.json).

### 3.2 Multi-root discovery

- `phasespec.DiscoverUserSpecsFromRoots(roots) (specs, sources, warnings)` — ordered
  concat; inter-root collision keeps the left-most with a shadowing warning; `sources`
  maps name → root (provenance).
- `phasespec.Roots(projectRoot)` reads `paths.phase_roots` from `.evolve/policy.json` —
  colon-separated, relative entries resolved against the project root, default
  `.evolve/phases`. `phasespec.RootsWithWarnings` also returns a
  `PHASE_ROOTS_POLICY_UNREADABLE` warning when policy.json is malformed, so the fallback
  to the default root is visible; `MergedCatalog` (`phases list`, preflight, skillcheck),
  `phases validate` and `phase lint` print it. A missing policy.json is silent.
- Wired at: composition root (`cmd_cycle.go`), `phases list/validate`, `phase lint`,
  `phases create` collision check, `phase-inventory build`.
- `phases list` shows SOURCE + ROOT columns.

### 3.3 The index (`.evolve/phase-inventory.json`)

Built by `go/internal/phaseinventory` (clone of the skillinventory pattern: mtime/TTL
cache, `atomicwrite.JSON`, `--force`). Per-phase entry: identity + archetype/kind/tier/
agent + the metadata trio + **contract-derived facts** (`artifact`,
`required_sections`, `emits_verdict` via `phasecontract.FromSpec`) + provenance
(`source: builtin|user`, `root`) + `after`. Plus `category_index` (uncategorized
bucket for metadata-less phases).

**Advisory-only invariant:** routing/execution always reads the live merged catalog;
the inventory only informs advisor cards and offline readers. A stale index can weaken
a SELECT hint, never break a cycle. Gitignored as a generated cache (same as
`skill-inventory.json`).

### 3.4 Advisor enrichment

`router.PhaseCard` += `optional, description, when_to_use, categories`.
`writeCatalog` renders:

```
- bug-reproduction [evaluate] (bugfix) — when: bugfix cycles, after fault-localization …
```

Token control without a deterministic classifier: at most **12 enriched** cards —
stable three-bucket priority (optional+metadata > optional > spine) — then ALL
remaining names on one `also available …` line (a phase absent from the prompt cannot
be SELECTed). Hints truncate at 140 runes (`truncateRunes`).

### 3.5 Registration (`evolve phases create`)

Pipeline: parse → `ValidateUserSpec` (hard floor) + soft lint → collision check
(built-ins fail-open on registry read errors; all roots; persona overwrite refusal) →
transactional scaffold (`<root>/<name>/phase.json` + `agents/<agent>.md`, rollback on
partial failure) → derived-contract report → `phaseinventory.Build(Force)`.

stdout is ONE JSON envelope; exit 0/2/10/1:

```jsonc
// failure (exit 2)
{"ok":false,"phase":"x","errors":["user phase must be optional:true — …"],
 "warnings":["unknown category \"foo\" — known: …"],
 "hint":"fix errors and re-run: evolve phases create --spec -"}
// success (exit 0)
{"ok":true,"phase":"x","artifact":"x-report.md","required_sections":["## Findings"],
 "emits_verdict":true,"phase_json":".evolve/phases/x/phase.json",
 "persona":"agents/evolve-x.md","inventory_rebuilt":true}
```

`--mint <file|->` accepts `{name, prompt, tier, cli, writes_source, description,
when_to_use}` (the advisor's MintSpec + a name) and synthesizes spec + persona — the
mint-persistence path. The runtime mint flow stays ephemeral and untouched.

`description`/`when_to_use` are the SELECT metadata (ADR-0038) and use the same JSON
keys as `phasespec.PhaseSpec`, so `mintConfigsFrom` threads them straight onto the
minted spec. Both are optional (`omitempty`) — a metadata-less mint block mints exactly
as before. Supplying them is how a minted phase satisfies the catalog metadata gate
(`TestPhaseCatalog_OptionalPhasesHaveSelectMetadata`) on its own, instead of having its
name padded into `metadataAllowlist` after the fact (#404, #406). Already-allowlisted
legacy stubs predate this and are backfilled separately.

### 3.6 The skill (`skills/phase-create/`)

Thin by design: interview (purpose → archetype → categories → trigger → sections →
writes_source → position) → synthesize spec+persona → call the command → self-correct
from the envelope (≤3 passes). All enforcement lives in the binary, which is what makes
the flow identical on claude/codex/gemini (the binary is tier-1 everywhere; skills are
not).

## 4. Seed phase: `bug-reproduction` (née `reproduce-bug`)

Registered THROUGH `evolve phases create` (envelope: ok:true) as the e2e proof, then
renamed `bug-reproduction` when the two-tier naming rule landed (2026-06-06; signal
namespace `repro.*` kept). Evaluate archetype, `categories:["bugfix"]`,
`fail_if_signal {"repro.failing":"==false"}` — a repro that doesn't fail is a failed
phase. Evidence: +9.4–12.9% relative issue-resolution (TestPrune), SWE-Tester, the
SWE-bench FAIL_TO_PASS oracle. Guarded by `seed_phase_e2e_test.go`, which loads the
REAL repo catalog and asserts the phase reaches an enriched advisor card — this test
caught a real bug (metadata-less optional built-ins starving metadata-rich plugin
phases out of the enriched slots).

## 4b. A pinned user phase: `code-review` (ADR-0124, 2026-10-07)

A user phase stays optional-only and can never satisfy or displace the floor, but the registry may still **pin** one. `docs/architecture/phase-registry.json` `config.conditional_mandatory["code-review"] = "deliverable_kind==code && build.files_touched>0"` makes the router run the `code-review` user phase after build on every code cycle that touched files, whatever the advisor planned.

Why a pin and not `routing.insert_when`: at the Advisory stage `insert_when` only *gates* the advisor's plan (`router.shouldRunFromPlan`). It removes a planned phase whose trigger does not fire, but never adds an unplanned one. A pinned phase also does not count against `max_optional_insertions`, and PSMAS cannot skip it. Pinned by `TestCodeReviewPin_*` (`internal/router/code_review_pin_test.go`), which loads the real registry and the real `.evolve/phases`.

**Report grammars.** A user phase may declare `classify.grammars`, a list of named report grammars. The deliverable gate runs each registered grammar's check on the report, and a violation (`bad_grammar`) gets the contract-correction rung like a missing section. The grammar names are `phasespec`'s vocabulary (`phasespec.Grammars()`). A user spec naming any other grammar fails `ValidateUserSpec`, so the catalog load skips it with a WARN, the same as any invalid spec. A typo is therefore a load error and never reaches an agent's correction rung. The gate's `unbound_grammar` stays for a spec that bypassed validation. `TestGrammars_TheGateBindsExactlyTheGrammarsASpecMayDeclare` pins the gate's registry to that vocabulary in both directions. `code-review` declares `code-review-report` (`codereview.ValidateReport`): the Review Plan, the quality-index Scores, their agreement, and a cited finding for every gap ([design §3](review-loop-and-quality-index.md)).

**The Task Contract on request.** A user phase that lists `task_contract` in `prompt_context` is seeded the Task Contract block, as tdd, build and audit are; `code-review` does, so the reviewer sees the acceptance it judges against. A spec that sets `prompt_context` replaces the evaluate default `["goal"]`, so `code-review` lists both: `["goal", "task_contract"]`. The spec runner renders a multi-line value as its own `## Task Contract` block.

Open (OQ1 in the [plan §11](../plans/code-review-phase-2026-10.md)): user phases anchored `after: build` splice in reverse-alphabetical order, so `code-review` is not guaranteed to run first after build. Shadow does not need it to, because no fix round changes the diff. Enforce does, and Q6 adds an explicit order.

## 5. Operational notes

- **Add a phase by talking:** invoke `/phase-create` (or just ask) → the LLM designs
  the spec → `evolve phases create` validates/registers → next cycle's advisor can
  SELECT it.
- **Ship phases as a plugin:** bundle `<name>/phase.json` dirs; consumers append the
  bundle path to `EVOLVE_PHASE_ROOTS`. Review `writes_source` phases before adding a
  root (they run in the worktree, sandboxed by the role-gate).
- **Gotcha:** `.gitignore`'s `**/.evolve/` shadows NEW `phase.json` files despite the
  documented allowlist — `git add -f .evolve/phases/<name>/phase.json` until the
  ignore rules are reworked.
- **Wave 1–3 micro-phases** remain queued in `carryoverTodos` (see
  [micro-phase-catalog.md](micro-phase-catalog.md)); each is now a pure
  `phases create` invocation.
