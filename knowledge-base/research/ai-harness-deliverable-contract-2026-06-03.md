# AI-harness research: fault-proof deliverable contracts (2026-06-03)

External research backing [ADR-0034](../../docs/architecture/adr/0034-unified-deliverable-contract.md).
All URLs were **re-fetched and verified June 2026**; subagent-supplied citations that could not be
re-fetched (e.g. `arXiv 2602.16977`, `geodocs.dev`) were dropped — only re-fetchable sources are
preserved here.

## Question

How do production AI-agent harnesses make agent deliverables fault-proof, and how does an
enforcement gate avoid bricking an autonomous loop when it defaults to enforce (fail-closed)?

## Findings → how they shaped the design

### 1. Validation vs guardrail (anti-Goodhart)
"Validation answers *well-formed*; guardrails answer *allowed* — both are needed." Schema/format
compliance ≠ semantic correctness; a model can pick the wrong enum while respecting the schema.
→ Layer 3 (`evolve phase verify`) is scoped to **well-formedness only**; correctness stays the
auditor's LLM-judged job. A contract PASS never implies a semantic PASS.
- https://www.digitalapplied.com/blog/llm-guardrails-production-safety-layers-reference-2026
- https://orq.ai/blog/llm-guardrails
- https://toolhalla.ai/blog/ai-agent-guardrails-io-validation-2026
- https://futureagi.com/blog/what-is-llm-input-output-validation-2026/

### 2. Circuit breaker for agents (trip on quality, not exit codes)
"A circuit breaker for agents needs to track *quality* failures — outputs that violate schema or
semantic invariants — even when the API returns success." Fail-open vs fail-closed is a per-rule
dial (`POLICY_CHECK_FAIL_MODE` closed default; open allows through with bypass signals).
→ Layer 4 breaker trips on **contract violations** (a cleanly-exited but malformed deliverable),
which the loop's budget/exit-code stops are blind to; after N consecutive blocks it demotes
enforce→advisory so it cannot brick the loop.
- https://cordum.io/blog/ai-agent-circuit-breaker-pattern
- https://brandonlincolnhendricks.com/research/circuit-breaker-patterns-ai-agent-reliability
- https://martinfowler.com/bliki/CircuitBreaker.html

### 3. Prompt caching (stable prefix / volatile suffix)
Cache hits require a byte-identical prefix; keep system/rules stable and push per-request variable
content (paths, IDs) to the end. Default 5-min TTL; workspace-isolated as of 2026-02-05.
→ Layer 2 keeps the invariant contract block in the cacheable prefix and the per-cycle path in a
footer (last line).
- https://platform.claude.com/docs/en/build-with-claude/prompt-caching
- https://www.mager.co/blog/2026-04-29-claude-prompt-caching/

### 4. Deterministic enforcement + formal-verification trend
Probabilistic filters can't guarantee compliance; the trend is deterministic/formal checks for the
parts code can verify (schema, presence), reserving the LLM for judgment.
→ The verifier is pure deterministic Go shared by agent + host (one source, no drift).
- https://arxiv.org/html/2604.01483 (Lean-4 type-checked compliance for agentic systems)

### 5. Strangler Fig / Tolerant Reader (safe migration + evolving contracts)
Add the new path (verdict sentinel) alongside the legacy parser, retire later; accept unknown
fields gracefully (Postel) for the evolving `routing-plan.json`.
- https://martinfowler.com/bliki/StranglerFigApplication.html
- https://martinfowler.com/bliki/TolerantReader.html

## Synthesised principles applied
1. Schema/path stated explicitly in the prompt, then validated post-hoc (necessary + sufficient).
2. Deterministic for well-formedness; LLM-judge for semantics.
3. Defense in depth: prompt instruction → agent self-check → host gate (no single load-bearing layer).
4. Fail-open on infra/ambiguity, fail-closed on confirmed violation.
5. Circuit breaker so the gate is never the single point of failure.
6. Stable-prefix/volatile-suffix prompt layout for cache safety.
7. Strangler-fig migration for the verdict parser; tolerant reader for JSON contracts.

## Anti-patterns avoided
Monolithic catch-all gate · LLM-only validation with no deterministic backbone · block-forever
with no fallback · retry without feedback · no observability on gate decisions · blocking
parse-errors and policy-violations equally.
