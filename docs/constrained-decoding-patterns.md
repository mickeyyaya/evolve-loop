# Constrained Decoding Patterns

> Reference document for schema-constrained generation techniques applied to agent outputs.
> Use these patterns to enforce structured output from Scout, Builder, and Auditor agents,
> ensuring valid JSON ledger entries, handoff files, and state transitions without post-hoc repair.

## Table of Contents

1. [Constrained Decoding Techniques](#constrained-decoding-techniques)
2. [Performance Characteristics](#performance-characteristics)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Constrained Decoding Techniques

| Technique | Mechanism | Constraint Granularity | Strengths | Limitations |
|---|---|---|---|---|
| **JSON Mode** | Provider-level flag forces output to be valid JSON | Structural (valid JSON only) | Zero-config; supported by OpenAI, Anthropic, Google | No schema enforcement; any valid JSON passes |
| **Grammar-Constrained (GBNF/CFG)** | Define a context-free grammar; decoder only emits tokens matching grammar | Token-level; each token must extend a valid parse | Arbitrary structure enforcement; works offline | Complex grammar authoring; vendor-specific syntax |
| **Regex-Constrained** | Define regex pattern; decoder masks tokens violating the pattern | Token-level; character-by-character matching | Simple for flat formats (IDs, enums, dates) | Cannot express nested or recursive structures |
| **Schema-Guided (JSON Schema)** | Provide JSON Schema; provider validates/constrains output to match | Field-level; enforces types, required fields, enums | Declarative; reusable across tools; wide ecosystem | Schema complexity limits vary by provider |
| **Logit Masking** | Modify logit distribution at each decoding step to zero out invalid tokens | Token-level; direct probability manipulation | Maximum control; composable with other techniques | Requires model access; high implementation complexity |

---

## Performance Characteristics

| Dimension | Token-Level Constraints | Sequence-Level Constraints |
|---|---|---|
| **When Applied** | Every decoding step; mask invalid tokens before sampling | After full generation; validate and reject/retry |
| **Latency Impact** | 1-15% overhead per token (grammar checking) | Zero generation overhead; retry cost on failure |
| **Guarantee Strength** | 100% structural validity by construction | Probabilistic; depends on retry budget |
| **Failure Mode** | May force low-probability tokens, reducing output quality | May exhaust retry budget on complex schemas |
| **Best For** | Critical schemas (state.json, ledger entries) | Advisory formats (reports, summaries) |

| Optimization | Approach | Speedup | Trade-off |
|---|---|---|---|
| **XGrammar precompilation** | Compile grammar to token-level bitmask ahead of time | ~100x vs naive grammar check | Upfront compilation cost; memory for bitmask tables |
| **Incremental parsing** | Maintain parser state across tokens; avoid re-parsing from start | ~10x vs full re-parse per token | Parser state memory; complexity in batched inference |
| **Speculative constrained decoding** | Generate k tokens speculatively, validate batch, accept valid prefix | ~2-4x throughput | Wasted computation on rejected speculative tokens |
| **Schema caching** | Cache compiled schema constraints across requests | Amortized compilation to near-zero | Cache invalidation when schemas change |

---

## Mapping to Evolve-Loop

| Component | Schema Target | Constraint Type | Enforcement Level |
|---|---|---|---|
| **Scout report** | `scout-report.md` frontmatter + task table | Schema-guided JSON frontmatter; regex for table format | Token-level for frontmatter; sequence-level for prose sections |
| **Builder output** | `build-report.md` + code diffs | Schema-guided for structured sections; unconstrained for code | Hybrid: constrain metadata fields, leave code blocks free |
| **Auditor verdict** | `audit-report.md` pass/fail table + scores | Schema-guided with enum constraints (pass/fail/warning) | Token-level; audit verdicts must be structurally valid |
| **Ledger entries** | `ledger.jsonl` | Strict JSON Schema with required fields, types, enums | Token-level; every entry must parse and validate |
| **State file** | `state.json` | JSON Schema with phase enum, cycle number, timestamps | Token-level; invalid state file breaks the entire loop |
| **Handoff files** | `scout-report.md`, `build-report.md` | Schema-guided frontmatter; markdown body unconstrained | Token-level for handoff metadata; sequence-level for body |

| Agent | Constrained Fields | Free-Form Fields | Rationale |
|---|---|---|---|
| **Scout** | task_id, priority, risk_level, estimated_tokens | analysis, reasoning, search_results | Constrain routing metadata; allow creative analysis |
| **Builder** | files_changed, tests_added, build_status | implementation_notes, code_diffs | Constrain verifiable facts; allow flexible implementation narrative |
| **Auditor** | verdict, score, criteria_results | justification, recommendations | Constrain machine-readable verdicts; allow nuanced justification |

---

## Implementation Patterns

### Define Output Schemas

| Step | Action | Example |
|---|---|---|
| 1 | Define JSON Schema for each output type | `ledger-entry.schema.json` with required `cycle`, `phase`, `timestamp` |
| 2 | Store schemas alongside the code that consumes them | `schemas/` directory in repo root |
| 3 | Version schemas with semver; reject unknown schema versions | `"$schema": "ledger-entry.v2.schema.json"` |
| 4 | Generate human-readable docs from schemas automatically | Use `json-schema-to-markdown` or equivalent |

### Generation-Time vs Post-Hoc Validation

| Strategy | When to Use | Implementation | Recovery |
|---|---|---|---|
| **Generation-time constraint** | Critical machine-readable outputs (state.json, ledger) | Pass schema to provider; use JSON mode + schema | N/A; output is valid by construction |
| **Post-hoc validation** | Advisory outputs (reports, summaries) | Validate after generation with `ajv` or equivalent | Re-prompt with validation errors appended to context |
| **Hybrid** | Outputs with both structured and free-form sections | Constrain structured prefix; validate full output post-hoc | Re-prompt only the failed section |

### Fallback Strategies

| Failure Scenario | Fallback Action | Max Retries |
|---|---|---|
| Schema constraint produces degenerate output | Fall back to post-hoc validation with retry | 3 |
| Post-hoc validation fails after max retries | Extract partial valid fields; log warning; proceed with defaults | N/A |
| Provider does not support schema-guided mode | Use JSON mode + post-hoc schema validation | 3 |
| Grammar compilation fails (too complex) | Simplify schema; split into multiple constrained calls | 1 (then simplify) |
| Constraint causes generation timeout | Reduce schema complexity; increase timeout budget | 2 |

---

## Prior Art

| Library / System | Approach | Key Innovation | Integration Model |
|---|---|---|---|
| **XGrammar** | Precompiled grammar-to-bitmask | 100x speedup via ahead-of-time grammar compilation to token masks | C++ engine; Python bindings; integrates with vLLM, TGI |
| **llguidance (OpenAI)** | Server-side constrained decoding | Native provider integration; zero client overhead | API parameter (`response_format`) |
| **Outlines** | Python library for structured generation | Regex and CFG constraints with HuggingFace models | Python; wraps HF `generate()` |
| **LMQL** | Query language for LLM interaction | Declarative constraints in SQL-like syntax; mixed prompting and constraints | Python DSL; compiles to constrained decoding |
| **Guidance (Microsoft)** | Template-based structured generation | Interleave fixed text with constrained LLM fills | Python; template syntax with `{{gen}}` blocks |
| **Instructor** | Pydantic-based output validation | Schema extraction from Python types; automatic retry on validation failure | Python; wraps OpenAI/Anthropic clients |
| **TypeChat (Microsoft)** | TypeScript type-guided generation | Use TypeScript types as schema; validate with `tsc` | TypeScript; compile-time type checking of LLM output |
| **Guardrails AI** | Rail specification language | XML-based output schema with validators and re-ask logic | Python; middleware between app and LLM |

---

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Mitigation |
|---|---|---|---|
| **Over-constraining** | Apply token-level constraints to free-form outputs (analysis, reasoning) | Degrades output quality; forces low-probability tokens; loses nuance | Constrain only machine-readable fields; leave reasoning unconstrained |
| **Schema-reality mismatch** | Schema does not match what downstream consumers actually parse | Outputs pass validation but break consumers; false confidence | Derive schemas from consumer code; test with real consumer parsing |
| **Ignoring constraint failures** | Silently swallow validation errors; proceed with invalid data | Corrupt state propagates through pipeline; hard-to-debug failures | Fail loudly; log full validation errors; halt pipeline on critical schema violations |
| **Performance overhead denial** | Use complex grammars without measuring latency impact | 10-50% latency increase goes unnoticed; accumulates across cycles | Benchmark constrained vs unconstrained generation; set latency budgets |
| **Schema drift** | Schema evolves but old producers/consumers are not updated | Version mismatch causes silent data loss or parsing errors | Version schemas; reject unknown versions; run schema compatibility checks in CI |
| **Constraint cargo-culting** | Apply constrained decoding everywhere because it exists | Unnecessary overhead on outputs that never fail validation | Measure validation failure rate first; add constraints only where failures occur |
| **Single-call monolith** | Force entire complex output through one constrained generation call | Schema too complex; timeout or degenerate output | Split into multiple focused calls: metadata call (constrained) + content call (free) |
