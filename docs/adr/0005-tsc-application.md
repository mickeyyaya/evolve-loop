# ADR 0005 — TSC Application to Persona Files

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-12 |
| Cycle | 24–26 |
| Affects | evolve-scout.md, evolve-builder.md, token economics |

## Context

Agent persona files were written for human readers — full sentences, articles, auxiliary verbs, filler phrases. Loaded into every subagent invocation, they inflated token costs with no semantic value for machine consumers that parse intent directly from keywords and structure.

The token-reduction campaign (cycles 15–24) identified persona verbosity as a high-ROI target. Research in `knowledge-base/research/tsc-prompt-compression-2026.md` evaluated several techniques:

- **LLMLingua-2**: neural compression; runtime latency; additional dependency; degrades prompt-cache hit rates.
- **Manual summarization**: lossy; destroys precision; cannot be mechanically applied or verified.
- **Telegraphic Semantic Compression (TSC)**: rule-based removal of grammar elements while preserving domain vocabulary, structure, and machine-critical content.

TSC is preferred for static persona files because: zero runtime latency, no additional dependencies, static files benefit from prompt-cache hits, and compression is reversible via git.

## Decision

Apply TSC to evolve-loop persona files following a defined rule table:

**Remove:** articles (a, an, the), auxiliary verbs (is, are, should, will, must) when implied by context, filler phrases ("In order to", "Note that", "It is important that"), redundant adverbs.

**Preserve verbatim:** code blocks, JSON, regex patterns, numbers, `EVOLVE_*` env var names, SHA values, file paths, tool names, quoted strings.

**Hard constraints (anti-goals):** never compress fenced code or JSON blocks; never alter `EVOLVE_*` variable names; never compress ADR section headers or table structure.

First application:
- Commit `44ebf09` (2026-05-12, cycle 24): `evolve-scout.md` 1271 → 1017 words (−20.0%).
- Commit `7105685` (2026-05-13, cycle 26): `evolve-builder.md` 2730 → 2045 words (−25.1%), combined with caveman merge.

## Consequences

**Positive:**
- Measurable token reduction per invocation: Scout −20%, Builder −25.1% on persona load.
- Prompt-cache effectiveness increased: shorter, stable static content caches better.
- Compression is deterministic and reviewable; git diff shows exactly what was removed.
- Technique generalizes to remaining persona files (Auditor, Orchestrator, Retrospective).

**Negative:**
- Compressed text is less readable for human contributors; a human editing `evolve-scout.md` must understand TSC conventions before making changes.
- "Caveman merge" (cycle 26 commit `7105685`) combined TSC with content restructuring in one commit, making it harder to attribute specific changes to TSC vs structural decisions.
- Compression quality depends on judgment; aggressive compression can destroy precision if domain vocabulary is inadvertently simplified.

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| LLMLingua-2 automated compression | Runtime dependency; latency; reduces prompt-cache hit rate |
| Manual summarization | Lossy; imprecise; cannot be mechanically applied |
| Truncate at token limit | Loses unpredictable content; no semantic boundary |
| Layer-3 split only (ADR 0003) | Orthogonal: splits reduce what is loaded; TSC reduces verbosity of what must be loaded regardless |

## Cross-reference

- `docs/architecture/token-reduction-roadmap.md` — P-NEW-3 (Scout TSC, cycle 24), P-NEW-5 (Builder TSC, cycle 26).
- `knowledge-base/research/tsc-prompt-compression-2026.md` — canonical rule table and rationale (excluded from agent context by archival policy; reference only).
