# Memory subsystem: instincts, genes, and learning configuration

> The durable reference for evolve-loop's **cross-cycle memory** — how the loop
> learns. Distilled from the bash-era `docs/reference/{instincts,genes,configuration}.md`
> into one Go-runtime-grounded place. The Retro/Memo phase (formerly "Phase 6 / LEARN")
> writes these structures; Scout and Builder read them at cycle start.
>
> Related: [the-dev-workflow](../guides/the-dev-workflow.md) ·
> [decision-digest](../evolution/decision-digest.md) (the *why* behind the trust
> governance gates) · [state-and-ledger](../architecture/state-and-ledger.md)
> (where the summaries live).

Memory has two complementary stores: **instincts** (declarative — *what to know*)
and **genes** (imperative — *how to fix*). Both are evidence-weighted, decay when
unused, and graduate when repeatedly confirmed.

---

## 1. Instincts — declarative lessons

An instinct captures a single durable pattern with a confidence score. Stored as YAML
under `.evolve/instincts/personal/`, read by Scout and Builder at the start of each
cycle, written by the Retro/Memo phase.

```yaml
- id: inst-001
  pattern: "short-identifier"
  description: "Specific, actionable: what to do/avoid and WHY it matters and when."
  confidence: 0.7        # 0.5 (new) → 1.0 (fundamental)
  source: "cycle-N/task-slug"
  type: "anti-pattern"   # anti-pattern | successful-pattern | convention | architecture | domain | process | technique
  category: "episodic"   # episodic | semantic | procedural
```

**Three memory categories** drive targeted retrieval:

| Category | Contains | Queried by |
|---|---|---|
| **Episodic** | What happened — what worked / failed | Scout (avoid past failures) |
| **Semantic** | Domain knowledge — conventions, architecture, codebase facts | Builder (follow patterns) |
| **Procedural** | How-to — techniques, process optimizations | Builder (during implementation) |

**Confidence scale.** 0.5 new · 0.6 seen once with evidence · 0.7 confirmed by a
passing audit · 0.8 confirmed across 2+ cycles · 0.9 proven, no contradictions ·
1.0 always applies. Confidence rises on confirmation, falls on contradiction.

### Lifecycle gates (do not conflate them)

| Gate | Effect | Confidence | Cycles | Extra |
|---|---|---|---|---|
| **Graduation** | Instinct → *mandatory* Builder guidance (no deliberation) | ≥ 0.75 | cited 3+ distinct cycles | no contradiction in `failedApproaches` |
| **Global promotion** | Project → cross-project (`~/.evolve/instincts/personal/`) | ≥ 0.8 | 2+ cycles | loop age ≥ 5 cycles; generalizable |
| **Trust governance** | External instinct → accepted | ≥ 0.8 | 3 confirmations | provenance check; no eval/prompt refs |

Why the differing bars: graduated instincts *bypass* Builder deliberation, so a wrong
one directly causes failures → stricter cycle count. Trust governance is strictest on
provenance because external instincts carry injection risk (community skills have a
non-trivial vulnerability rate).

**Reversal.** 2+ consecutive build failures where a graduated instinct was applied →
`graduated: false`, −0.2 confidence; below 0.5 → archived (`archivedReason: "reversal"`);
logged in the ledger as `instinct-reversal`.

### Memory operations (three cadences, complementary)

| Operation | Frequency | Window | Action |
|---|---|---|---|
| **Dormant flagging** | per-cycle (Scout) | 3+ cycles uncited | soft signal in Scout report |
| **Consolidation + decay** | every 3 cycles (or count > 20) | last 5 cycles unreferenced | cluster (>85% similar), −0.1/pass, archive < 0.3 |
| **Forgetting** | every 10 cycles | last 10 cycles, 0 citations | causal review → archive (`zero-use-discard`) |

A 5-cycle decay window (not 3) gives a new instinct two consolidation passes before
decay can touch it — matching the 2+ cycle promotion bar so a promotable instinct is
never simultaneously decayed. **Graduated instincts are exempt from forgetting.** The
causal-review step protects rarely-used-but-critical instincts (e.g. one learned from a
CRITICAL audit finding).

Files: `.evolve/instincts/personal/cycle-N-instincts.yaml` (append per cycle);
`.evolve/instincts/archived/` (superseded/stale — never deleted, provenance preserved).
Annotated example: [examples/instinct.yaml](../../examples/instinct.yaml).

---

## 2. Genes — imperative fix templates

A gene is an executable fix recipe: a selector that matches an error, ordered steps, and
pre/post validation commands. More actionable than an instinct.

```yaml
- id: gene-001
  name: "fix-missing-export"
  selector:
    errorPattern: "Module.*has no exported member"
    fileGlob: "src/**/index.ts"
  action:
    steps: ["Find the missing export", "Add export to the nearest barrel file"]
    commands: ["grep -r 'export' ${file} | head -5"]
  validation:
    pre:  "grep '${symbol}' ${barrel} | wc -l"   # expect 0 (bug present)
    post: "grep '${symbol}' ${barrel} | wc -l"   # expect 1 (fixed)
  confidence: 0.8
  source: "cycle-5/fix-export-bug"
  successCount: 3
  failCount: 0
```

**Selection.** On a build error the Builder matches `selector.errorPattern`; multiple
matches rank by `confidence * successCount / (successCount + failCount)`; the winner runs
`validation.pre` (confirm the bug), applies steps, runs `validation.post` (confirm the
fix), then increments `successCount`/`failCount`.

**Capsules** bundle genes into an ordered/parallel workflow:

```yaml
- id: capsule-001
  name: "add-new-component"
  genes: ["gene-003", "gene-005", "gene-008"]
  sequence: "ordered"
```

Genes are extracted during Retro/Memo after instinct extraction; genes with
`failCount > successCount` are archived. Stored under `.evolve/genes/`. Annotated
example: [examples/gene.yaml](../../examples/gene.yaml).

| Aspect | Instincts | Genes |
|---|---|---|
| Nature | declarative (what to know) | imperative (how to fix) |
| Trigger | read at cycle start | matched on error pattern |
| Evolution | confidence scoring | success/fail counting |

---

## 3. Learning-related state (`.evolve/state.json`)

The orchestrator keeps compact summaries in `state.json` so agents never read the full
ledger or every instinct YAML.

- **`failedApproaches[]`** — after 3 failed Builder attempts an approach is logged
  (`feature`, `approach`, `error`, `reasoning`, `filesAffected`, `cycle`, `alternative`).
  Scout reads it to avoid repeating dead ends. (Also the failure-adapter's input — see
  [decision-digest](../evolution/decision-digest.md) cluster C.)
- **`instinctSummary[]`** — `{id, pattern, confidence, type, graduated?}` per active
  instinct; Scout/Builder read this instead of the YAML files.
- **`ledgerSummary`** — aggregate counts (`scoutRuns`, `builderRuns`, `totalTasksShipped`,
  …) so agents avoid reading `ledger.jsonl`.
- **`processRewards`** — per-phase 0–1 efficiency scores (discover/build/audit/ship/learn),
  feeding meta-cycle reviews.
- **`researchCache` / research cooldown** — web research is cached with a TTL
  (gated by `EVOLVE_RESEARCH_CACHE_ENABLED`; see [env-vars](./env-vars.md)).

### Domain + model configuration

- **Domain detection** auto-classifies the project (`coding`/`writing`/`research`/`design`)
  and selects eval grader, build isolation, and ship mechanism. Override with
  `.evolve/domain.json`.
- **Model tiers** are a 3-tier abstraction (tier-1 deep reasoning · tier-2 balanced
  coding · tier-3 fast/cheap) resolved to concrete models per provider. Override with
  `.evolve/models.json`. The authoritative router is now `.evolve/llm_config.json`
  (ADR-0001) — see [cli-matrix-and-drivers](../architecture/cli-matrix-and-drivers.md)
  and [cli-capability-matrix](./cli-capability-matrix.md).

---

## Where the full bash-era source lives

The original long-form docs remain at `docs/reference/instincts.md`,
`docs/reference/genes.md`, and `docs/reference/configuration.md` (kept in place because
CI asserts their existence and several skill/agent prompts link them by path). This page
is the curated knowledge-base entry point; the source docs carry the exhaustive field
tables and provider model-mapping matrix.
