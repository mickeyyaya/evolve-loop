# Telegraphic Semantic Compression (TSC) — Research Dossier — 2026-05-12

> **Archive note:** This file lives in `knowledge-base/research/` and is excluded from agent context per `feedback_knowledge_base_stewardship.md`. Persistent reference for future Scouts evaluating prompt-compression work in evolve-loop personas, skills, and reference docs.
>
> **Companion dossier:** `knowledge-base/research/token-reduction-2026-may.md` (cycle 15) covered LLMLingua family, PSMAS, ACON, TOON, MindStudio Progressive Disclosure. This file *extends* that dossier specifically for the **Telegraphic Semantic Compression (TSC)** technique flagged by operator on 2026-05-12 (end of $50 batch that shipped 3 cycles: `d73eabf`, `c7b49bc`, `0e4bff1`).

## 1. Operator request (verbatim)

> "Shorten sentences by removing grammar to make sentence concise and straight forward for the production md which will be loaded. Research for more details for shortening the prompt by removing grammar."

This is a request to apply prompt-compression to evolve-loop's **production MD files** — persona files (`agents/*.md`), SKILL.md files, reference docs (`agents/*-reference.md`) — that are loaded into subagent context every cycle. The technique should compress *text* without changing *behavior*.

## 2. Canonical name in literature

The technique is called **Telegraphic Semantic Compression (TSC)**. It is the manual, deterministic, rule-based cousin of LLMLingua / LLMLingua-2 (which are automated, BERT-encoder-based).

### TSC core principle

> "Remove what an LLM can reliably predict (grammar, filler words, structural glue); preserve the information it cannot reconstruct from context (nouns, verbs, numbers, entity names, domain vocabulary)."

## 3. TSC rules (canonical)

| Rule | Example BEFORE | Example AFTER | Tokens saved |
|---|---|---|---|
| Drop definite/indefinite articles (`the`, `a`, `an`) | "the Builder reads the report" | "Builder reads report" | 2 |
| Drop auxiliary verbs (`is`, `are`, `was`, `were`, `have`, `has`) | "you should verify that the flag is set" | "verify flag set" | 4 |
| Drop fillers (`It is important to note that`, `In order to`, `Please`) | "It is important to note that..." | "Note:" | 5 |
| Compress relative clauses | "the file which contains the manifest" | "manifest file" | 3 |
| Imperative form | "you must invoke Skill" | "Invoke Skill" | 1 |
| Drop modal padding (`should`, `may`, `might` when intent is firm) | "you should always check" | "check" | 2 |
| Compress prepositional chains | "in the context of the build phase" | "during build" | 4 |
| Domain vocab preserved (case + punctuation) | `EVOLVE_BUILDER_SELF_REVIEW=1` | (unchanged) | 0 |
| Numbers preserved | `max_turns: 25` | (unchanged) | 0 |
| Code/JSON/regex preserved verbatim | `` `awk '/^## [A-Z]/'` `` | (unchanged) | 0 |

### Concrete sample (from existing evolve-loop persona content)

**BEFORE** (32 words):

> "When the flag `EVOLVE_BUILDER_SELF_REVIEW` is set to 1, after self-verify passes, run a convergence loop that invokes the configured review skill(s) against your diff and revises until clean OR iteration cap hit."

**AFTER** (19 words, ~40% reduction):

> "On `EVOLVE_BUILDER_SELF_REVIEW=1`, post-self-verify: run convergence loop invoking review skill(s) against diff; revise until clean OR iter-cap hit."

Meaning preserved. Same instructions. Same domain vocab. Fewer tokens.

## 4. LLMLingua / LLMLingua-2 (companion technique — NOT recommended for our use)

| Aspect | TSC (manual) | LLMLingua-2 (automated) |
|---|---|---|
| Method | Rule-based edits at file-write time | BERT-encoder token classification, runtime pass |
| Compression ratio | 2x–3x typical | 2x–5x typical |
| Latency cost | Zero (one-time edit) | ~50–100ms per invocation (BERT inference) |
| Dependency surface | None | New Python package + model download |
| Human readability | High (still grammatical-ish, just terse) | Variable (sometimes ungrammatical) |
| Cache friendliness | High (static file, cache hits) | Lower (dynamic per invocation) |
| Reversibility | Manual edits in git | Need uncompressed source-of-truth |
| **Best fit** | **Static persona/skill files** (our case) | Dynamic context (e.g., long doc retrieval) |

For evolve-loop's MD persona/skill files, **manual TSC is strictly better than runtime LLMLingua-2**. The files don't change per invocation, so compress once, save every load.

## 5. Application surface in evolve-loop (post-cycle-24 batch)

The 2026-05-12 $50 batch shipped Layer-3 splits for `evolve-scout.md` (cycle `d73eabf`) and `phases.md` (cycle `0e4bff1`, −51.6%), plus default flipping of `EVOLVE_CONTEXT_DIGEST`/`EVOLVE_ANCHOR_EXTRACT` (cycle `c7b49bc`). Remaining TSC-applicable surface:

| File | Approx. lines × words | Est. compressible % (prose-only) | Est. tokens saved at 30% TSC |
|---|---|---|---|
| `agents/evolve-scout.md` (post-split) | reduced ~30% from cycle d73eabf | ~50% remaining prose | ~300 tokens |
| `agents/evolve-builder.md` | 397 × 2730 | ~55% prose | ~590 tokens |
| `agents/evolve-auditor.md` | 299 × 2286 | ~55% prose | ~500 tokens |
| `agents/evolve-orchestrator.md` | (TBD) | ~50% prose | ~400 tokens |
| `agents/evolve-retrospective.md` | (TBD) | ~55% prose | ~350 tokens |
| `agents/evolve-tdd-engineer.md` | (TBD) | ~55% prose | ~300 tokens |
| `agents/evolve-triage.md` | (TBD) | ~50% prose | ~250 tokens |
| `agents/evolve-intent.md` | (TBD) | ~50% prose | ~300 tokens |
| `skills/evolve-loop/SKILL.md` | 245 × 2482 | ~50% prose | ~490 tokens |
| `agents/evolve-builder-reference.md` | (Layer-3 reference) | ~60% prose | ~300 tokens |
| `agents/evolve-scout-reference.md` (just created) | (Layer-3 reference) | ~60% prose | ~300 tokens |
| **Subtotal (11 core files)** | ~10K words est. | | **~4080 tokens** |

## 6. Per-cycle savings (estimate)

- Cycle-11 forensics baseline (`docs/architecture/token-economics-2026.md`): $6.70/cycle, ~50% cache-read share.
- Persona files dominate cache-read content.
- 4080 token reduction × cache-read cost rate (per-1K tokens, Sonnet) ≈ **$0.05–0.12 direct savings per cycle**.
- Cumulative across 50-cycle batches: **$2.50–6.00 per batch**.
- Stacks with cycle-24 batch wins (`c7b49bc` saved ~$1.25/cycle via flag default flip; phases.md reduction will save more on every reference doc load).

## 7. Application strategy (proposed for cycle 25+)

### Composition with Layer-3 splits (already shipped in cycle 24 batch)

Cycle 24's batch shipped Layer-3 splits for `evolve-scout.md` and `phases.md`. That trimmed hot-path content. TSC operates on what remains. **Layer-3 split first (done), TSC second (next)** — the order matters because TSC on already-trimmed content has the cleanest signal.

### Single-writer / one-file-per-cycle discipline

ONE persona or SKILL file per cycle. Multi-file TSC in one cycle increases blast radius without benefit; pre-existing project anti-goal.

### Section-level discipline

Apply TSC **only** to:
- Prose paragraphs in narrative sections (overview, rationale, "why" blocks)
- Step-by-step instructions
- Long bullet-list explanations

**Do NOT** apply TSC to:
- Fenced code blocks (` ``` ` ... ` ``` `)
- JSON schemas / examples
- Eval grader command lines
- Regex patterns
- Table headers (these guide LLM parsing; preserve exact wording)
- `## Anti-Goals` / `## Constraints` sections (over-compression here enables reward-hacking — preserve full text)
- Kernel-hook code / shell scripts (out of scope; this is MD-only)

### Mandatory verification per cycle

- `wc -w` baseline vs post-edit delta in `build-report.md`
- Mutation testing at `gate_discover_to_build` (already kernel-enforced)
- Adversarial Auditor mode (default-on) catches sycophancy/drift
- A/B verification: run 1 cycle with compressed persona vs baseline cycle, compare cost + audit verdict
- Pass criterion: ≥20% token reduction on the modified file AND audit verdict not worse than baseline

## 8. Risks and anti-patterns

| Risk | Severity | Mitigation |
|---|---|---|
| Over-compression breaks parser-dependent content | HIGH | Section-level discipline (anti-list above); manual review of diff |
| Semantic drift changes instruction meaning | HIGH | Mutation testing + adversarial Auditor + A/B verification |
| Aesthetic loss (human readability) | MEDIUM | Add a top-of-file comment: "TSC applied — see knowledge-base/research/tsc-prompt-compression-2026.md" |
| Cache invalidation per edit | LOW | Batch all TSC for a single file in ONE commit; the next cycle gets a stable cache prefix |
| Reward-hacking enabler if `## Anti-Goals` compressed too aggressively | HIGH | Permanent anti-goal: do NOT TSC behavioral discipline sections |
| Default-on promotion premature | MEDIUM | Require 2 verification cycles + measurable token delta + no quality regression before promoting |

## 9. Anti-goals (for cycle 25 Intent)

- Do NOT apply TSC to multiple files in ONE cycle. One file per cycle.
- Do NOT touch fenced code blocks, JSON schemas, regex, eval-grader command lines.
- Do NOT modify kernel hook code, scripts, or `.evolve/profiles/*.json`.
- Do NOT remove text from `## Anti-Goals`, `## Constraints`, `## Risks`, or any section whose tokens encode behavioral discipline. Over-compression there is a reward-hacking enabler.
- Do NOT promote TSC default-on without A/B verification + 2 verification cycles.
- Do NOT compress eval files in `.evolve/evals/`.
- Do NOT introduce LLMLingua-2 runtime dependency. Manual TSC suffices for our static MD surface.

## 10. Verification approach (per-cycle)

| Step | Command / check | Pass criterion |
|---|---|---|
| Pre-edit baseline | `wc -w <target-file>` | Recorded in build-report |
| Edit | TSC applied per Section 3 rules | Diff reviewed |
| Post-edit delta | `wc -w <target-file>` | ≥20% word reduction on prose-only sections |
| Mutation test | `gate_discover_to_build` runs `mutate-eval.sh` (existing) | Kill rate ≥0.7 |
| Adversarial audit | Auditor profile Opus + framing | Verdict not worse than last clean baseline |
| A/B cost compare | `show-cycle-cost.sh` pre-TSC vs post-TSC, same task | ≥$0.05/cycle measurable savings |
| No quality regression | Audit verdict + Builder turn count | Both within ±10% of baseline |

## 11. Sources

- [Telegraphic Semantic Compression (TSC) — developer-service.blog](https://developer-service.blog/telegraphic-semantic-compression-tsc-a-semantic-compression-method-for-llm-contexts/) — primary canonical definition; rule taxonomy.
- [LLMLingua — Microsoft Research blog](https://www.microsoft.com/en-us/research/blog/llmlingua-innovating-llm-efficiency-with-prompt-compression/) — automated compression family.
- [LLMLingua: Compressing Prompts for Accelerated Inference of LLMs — arXiv 2310.05736](https://arxiv.org/abs/2310.05736) — EMNLP 2023 paper.
- [LLMLingua-2: Data Distillation for Efficient and Faithful Task-Agnostic Prompt Compression — arXiv 2403.12968](https://arxiv.org/abs/2403.12968) — ACL 2024 paper; 2x–5x compression with measurable benchmark improvements.
- [LongLLMLingua: Accelerating and Enhancing LLMs in Long Context Scenarios via Prompt Compression — ACL Anthology](https://aclanthology.org/2024.acl-long.91/) — long-context variant.
- [Prompt Compression for Large Language Models: A Survey — arXiv 2410.12388](https://arxiv.org/html/2410.12388v2) — comprehensive 2024 survey.
- [FrugalPrompt: Reducing Contextual Overhead via Token Attribution — arXiv 2510.16439](https://arxiv.org/html/2510.16439v1) — 2025 saliency-based filtering, training-free.
- [LLMLingua GitHub — microsoft/LLMLingua](https://github.com/microsoft/LLMLingua) — reference implementation.
- [Sahin Ahmed — Making Every Token Count](https://medium.com/@sahin.samia/prompt-compression-in-large-language-models-llms-making-every-token-count-078a2d1c7e03) — practitioner-oriented overview.

## 12. Related prior work in this repo

- `knowledge-base/research/token-reduction-2026-may.md` — cycle-15 dossier covering broader 2026 prompt-compression ecosystem.
- `docs/architecture/token-economics-2026.md` — cycle-11 forensics; baseline cost model.
- `docs/architecture/token-reduction-roadmap.md` — P1-P8 + P-NEW-1..5 + P-C20 + P-NEW-7/8/9 actionable roadmap (cycle 24 batch shipped P-NEW-1, P-NEW-3 in part via `c7b49bc` and `d73eabf`; P-NEW-7 via `0e4bff1`). `P-NEW-6 (TSC application)` candidate for cycle 25.
- `agents/evolve-builder-reference.md` — example of a Layer-3 reference file.
- `agents/evolve-scout-reference.md` — new Layer-3 reference (cycle `d73eabf`).

## 13. Recommendation for cycle 25 (operator note)

When cycle 25 fires, its Scout should:
1. Read this dossier (per knowledge-base stewardship rule).
2. Treat the `carryoverTodos` entry `tsc-application-cycle25` as the top-priority secondary work item.
3. Propose ONE target file (suggest: `agents/evolve-scout.md` post-Layer-3-split, since cycle 24-d73eabf trimmed it).
4. Build report includes mandatory `wc -w` delta + A/B cost projection.
5. Auditor verifies: ≥20% reduction, mutation gate pass, no quality regression vs. baseline.
6. If PASS: ship as cycle-25 commit. Cycle 26 candidate: next persona (`evolve-builder.md` only after v9.3.0 maturity proven; otherwise `skills/evolve-loop/SKILL.md`).

## 14. Cycle-24 batch context (2026-05-12)

The $50 batch terminated at exit code 2 (Layer-P memo contract violation) after shipping 3 cycles for $24.51 total. Substantive shipped work:

- `d73eabf` — Scout persona Layer-3 split (P-NEW-3) + roadmap currency refresh
- `c7b49bc` — P-NEW-1 + P5: `EVOLVE_CONTEXT_DIGEST` + `EVOLVE_ANCHOR_EXTRACT` defaults flipped to 1; retrospective YAML template extracted (~$1.25/cycle savings expected)
- `0e4bff1` — P-NEW-7: phases.md Layer-3 split (28,911→13,987 bytes, −51.6%) + roadmap P-NEW-7/8/9 added

The TSC item proposed in this dossier (`P-NEW-6`) is the natural composition: Layer-3 splits trim hot-path content; default-flag flips reduce per-phase context; TSC compresses remaining prose.

Combined campaign trajectory (cycles 15–24+25): baseline $6.70/cycle → projected $4.50–5.00/cycle post-TSC. ~30% total reduction.
