# ADR 0004 — CONTEXT.md Canonical Glossary Adoption

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-12 |
| Cycle | 24 |
| Affects | All agent personas, orchestrator context, CONTEXT.md, terminology standards |

## Context

Terminology drift across agent personas caused repeated misunderstandings in evolve-loop. The word "memo" was the most severe case: used in four distinct senses across agents — handoff document, merge-lesson dependency, carryover-todo source, and cycle summary. This created 13 consecutive `code-audit-warn` ledger entries where orchestrators incorrectly wired the `merge-lesson-into-state.sh` dependency, believing it required `memo.md` (it does not; it reads `handoff-retrospective.json`).

Other terms with drift: "batch", "carryover", "cycle", "gate", "instinct", "kernel hook", "ledger", "persona", "phase", "pipeline", "retrospective", "role", "ship", "triage", "worktree".

No single authoritative definition existed; agents inferred meaning from context, and different agents inferred differently.

## Decision

Create `CONTEXT.md` at the project root: 21 canonical term definitions, 598 words. Each entry follows a fixed schema: term name, canonical definition, and an "Avoid:" list of deprecated synonyms.

Canonical term for "memo": **"Post-PASS role that writes a cycle summary and emits carryover todos for the next cycle."** Deprecated synonyms: "summary", "handoff", "recap".

`CONTEXT.md` is loaded into orchestrator context every calibrate phase, making the glossary available to the orchestrator before it sequences any phase subagent.

Adding new domain terms to `CONTEXT.md` is designated the preferred first-step when introducing new concepts to the pipeline.

Commit `172f665` (2026-05-12) created the file. Note: `172f665` is a commit SHA, not a cycle number; the cycle was 24.

## Consequences

**Positive:**
- Single authoritative source for domain vocabulary; agents can resolve ambiguity by reading `CONTEXT.md`.
- Deprecated synonyms listed explicitly, making it possible to grep for stale usage.
- Downstream: the 13 consecutive `code-audit-warn` entries (merge-lesson dependency misidentification) were resolved once the "memo" canonical definition clarified the independence of `merge-lesson-into-state.sh`.
- Pattern establishes a scalable extension point: new concepts get a canonical definition before they proliferate.

**Negative:**
- `CONTEXT.md` adds ~600 words to the orchestrator's calibrate-phase context load every cycle.
- Terms can become stale if updated in code but not in `CONTEXT.md`; requires maintenance discipline.

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| Inline definitions in each persona file | Definitions diverge over time; no single source of truth |
| Comment block at the top of CLAUDE.md | CLAUDE.md already overlong; cross-CLI canonical instructions belong in AGENTS.md; a separate glossary file is more maintainable |
| README-based glossary | README is user-facing documentation; agents don't load README during calibrate |
| Rely on git commit messages for terminology history | Commit messages are not loaded into agent context; agents cannot self-correct from history |
