# Token Economics — Findings, Forensics, Optimization Roadmap

> Canonical reference for evolve-loop's token-economics analysis as of
> v9.0.2 (2026-05-11). Pairs with [`token-floor-history.md`](token-floor-history.md)
> (static context-floor measurements) and [`control-flags.md`](control-flags.md)
> (flag inventory). This doc covers *runtime* token economics: where per-cycle
> dollars actually go, the duplication patterns the data exposes, and the
> ROI-ordered optimization roadmap that follows from the evidence.

## Status

- Forensics source: cycle 11 (2026-05-11). The cycle ran intent → scout
  → triage → build → audit, totalling **$6.70** across 5 phases / 142
  turns. Build and audit landed as a WARN-ship at commit `1c2c511`; the
  orchestrator-report.md write failed afterwards, causing dispatcher
  rc=1, but the per-phase usage telemetry was preserved.
- Pricing source: [Anthropic pricing docs (2026)](https://platform.claude.com/docs/en/about-claude/pricing)
  and [Claude Opus 4.7 Pricing Analysis — Finout](https://www.finout.io/blog/claude-opus-4.7-pricing-the-real-cost-story-behind-the-unchanged-price-tag).
- All telemetry preserved at `.evolve/runs/cycle-11/{intent,scout,triage,
  builder,auditor}-usage.json`.

## Per-phase cost breakdown (cycle 11 evidence)

Applied to the cycle-11 raw token counts with Anthropic 2026 per-MTok
rates:

| Phase | Total | Output | Cache-create | Cache-read | Turns | Model |
|---|---|---|---|---|---|---|
| intent | $1.05 | $0.26 (25%) | **$0.55 (53%)** | $0.23 (22%) | 7 | Opus 4.7 |
| scout | $1.32 | $0.25 (19%) | $0.34 (26%) | **$0.73 (55%)** | 49 | Sonnet 4.6 |
| triage | $0.27 | $0.03 (12%) | **$0.17 (64%)** | $0.05 (20%) | 6 | Sonnet 4.6 |
| builder | $1.95 | $0.30 (15%) | $0.34 (18%) | **$1.30 (67%)** | 58 | Sonnet 4.6 |
| auditor | $2.10 | $0.43 (20%) | $0.63 (30%) | **$1.04 (50%)** | 22 | Opus 4.7 |
| **cycle** | **$6.70** | **$1.27** (19%) | **$2.03** (30%) | **$3.34** (50%) | 142 | — |

### Reading the table

- **No single category dominates.** The intuitive frame "output tokens
  drive cost" is wrong post Opus 4.7. With output at $25/MTok instead
  of the older $75/MTok, output is 12–25% of phase cost. Cache-read
  dominates at 50% cycle-wide.
- **Cache creation is paid 5× per cycle** — each phase rebuilds its
  cache from scratch (~95K tokens × 5 phases × ~$5/M blended = **~$2/cycle
  fixed overhead** before any work happens).
- **Turn count drives cache-read cost.** Scout (49 turns) reads 2.4M
  cached tokens. Builder (58 turns) reads 4.3M. Each turn reads back
  the cached prefix.
- **Triage is over-provisioned**: it pays 64% in cache-creation for a
  6-turn run that barely uses the cache it builds.

## Cross-phase duplication audit

### What's well-managed (no action needed)

| Resource | Pattern | Why it's already optimal |
|---|---|---|
| `state.json` | Pre-emitted via `role-context-builder.sh:emit_state_field` and `emit_carryover_todos` | Zero tool-call duplication; v8.62 digest mode collapsed this further |
| `ledger.jsonl` | Pre-emitted last-5 entries; auditor explicit Read for verification | Intent gets pre-digested; auditor's read is purpose-specific |
| Persona prose | Zero duplicated paragraphs across 5 personas | Single-source-of-truth discipline + v9.0.0 Layer 1/3 split |
| `scout-report.md`, `build-report.md` | Pre-emitted whole or anchor-scoped | v8.63 anchor-extract infrastructure handles this |

### What's still duplicated (action items)

| # | Resource | Duplication | Fix | Savings |
|---|---|---|---|---|
| D1 | `intent.md` to Auditor | Full 8.2 KB emitted; Auditor only uses `acceptance_criteria` | Add `intent.md:acceptance_criteria` to `auditor.json:context_anchors` | ~7 KB / cycle, ~$0.05 |
| D2 | Cache CREATION per phase | Every phase pays ~$0.40–$0.63 cache-creation independently (5 phases × ~95K tokens) | Architectural — see Roadmap §6 (PSMAS-style phase scheduling) | Up to $2/cycle on skipped phases |
| D3 | `retrospective.md` persona embeds ~3 KB YAML template | Loaded every retrospective invocation; static schema | Externalize to `skills/evolve-loop/lesson-template.yaml` + on-demand Read | ~$0.05 / retrospective |
| D4 | `agent-mailbox.md` 3× reads per cycle (Scout/Builder/Auditor) | Intentional sequential signaling | **Not** a dedup target — communication protocol | n/a |

## 2026 external research applied to evolve-loop

### Phase-Scheduled Multi-Agent Systems (PSMAS)

Source: [Stop Wasting Your Tokens (arxiv 2510.26585)](https://arxiv.org/html/2510.26585v2)

Mean 27.3% token reduction (range 21.4–34.8%) with task performance
within 2.1 pp of fully-activated baseline. Beats best learned routing
baseline by 5.6 pp.

**Pattern applied:** instead of running every phase for every cycle,
triage decides which downstream phases are needed. For a "doc-only"
cycle, skip auditor's adversarial framing (Opus tier-1 → Sonnet, half
the rate). For a "trivial refactor" cycle, skip retrospective (no
failure to learn from on PASS).

**Why it fits evolve-loop:** triage already classifies cycle scope as
`small | medium | large`; the next step is letting that classification
adjust downstream phase selection. This is the architectural follow-up
to v9.0.0–v9.0.2's per-phase tuning.

### SupervisorAgent

Source: [Obvious Works — Token optimization 2026](https://www.obviousworks.ch/en/token-optimization-saves-up-to-80-percent-llm-costs/)

43% step reduction + 70% token cost reduction via tighter orchestrator-
side decisions.

**Pattern applied:** the orchestrator persona itself is a SupervisorAgent
candidate. v9.0.0 already reduced its persona size by ~6% via Layer 3
extraction. The next gain is in *decision quality* — the orchestrator
currently runs every phase mechanically; it could skip phases based on
the failure-adapter's classification.

### Prompt compression — LLMLingua 2026

Source: [TokenMix — LLMLingua 2026: 20x Prompt Compression](https://tokenmix.ai/blog/llmlingua-prompt-compression-2026)

20× compression in production deployments ($42K → $2.1K monthly savings
documented).

**Applicability:** prompt-side; orthogonal to the per-phase work.
Speculative for evolve-loop — would require integrating LLMLingua's
prompt-compressor as a pre-processor on the bedrock + persona prompt
before sending to claude. Defer to v9.1+.

### Structured-output compression — TOON format

Source: [LLM Structured Output in 2026 (TOON) — DEV](https://dev.to/pockit_tools/llm-structured-output-in-2026-stop-parsing-json-with-regex-and-do-it-right-34pk)

TOON cuts structured-output tokens 30–60% vs JSON. TSV equivalent
~50% reduction.

**Applicability:** audit-report.md's defects table + triage-decision.md's
top_n structures are JSON-shaped today. Converting to TOON or TSV would
reduce output tokens 30–60% for those sections.

**Cost-benefit:** small ($0.10/cycle saved), needs downstream parser
updates. Defer until easier wins land.

### Opus 4.7 tokenizer verbosity tax

Source: [Finout — Claude Opus 4.7 Pricing](https://www.finout.io/blog/claude-opus-4.7-pricing-the-real-cost-story-behind-the-unchanged-price-tag)

Opus 4.7 uses up to 35% more tokens for the same text vs older Opus
tokenizers. The published per-MTok rate is unchanged, but *effective*
output cost rises ~25-35%.

**Implication for evolve-loop:** intent + auditor (the Opus phases) pay
an invisible verbosity tax. Mitigations:

1. Short artifacts (v9.0.2 cut intent.md from 50–200 → 30–80 lines).
2. Terse persona prose (Campaign D Layer 1/3 split).
3. Prefer Sonnet for non-judgment-critical work. Sonnet at $3/$15 vs
   Opus's $5/$25 means switching saves 40% on equivalent token volume.

### General research (for reference)

- [Redis — LLM Token Optimization 2026](https://redis.io/blog/llm-token-optimization-speed-up-apps/) — caching, batching, structured-output patterns
- [Optimizing Tokens for Structured LLM Outputs — newline](https://www.newline.co/@Dipen/optimizing-tokens-for-better-structured-llm-outputs--adf4cfea)
- [How I Reduced LLM Token Costs by 90% — Medium](https://medium.com/@ravityuval/how-i-reduced-llm-token-costs-by-90-using-prompt-rag-and-ai-agent-optimization-f64bd1b56d9f)
- [Aussie AI — Token Reduction](https://www.aussieai.com/research/token-reduction)
- [ProjectPro — LLM Compression Techniques](https://www.projectpro.io/article/llm-compression/1179)

## Optimization roadmap (ROI-ordered)

| Priority | Change | Cost saved / cycle | Effort | Status | Reference |
|---|---|---|---|---|---|
| **P1** | **v9.0.3 — apply v9.0.2 playbook to scout** (49 → ≤8–10 turns) | **~$0.80** (60% of scout) | Low | Planned | This doc §"Per-phase" |
| **P2** | **v9.0.4 — apply v9.0.2 playbook to builder** (58 → ≤15–20 turns) | **~$1.00** (50% of builder) | Medium | Identified | This doc §"Per-phase" |
| P3 | Triage bedrock right-sizing (shorter persona + slimmer context) | ~$0.10 (40% of triage) | Low | Identified | D2 above |
| P4 | Auditor anchor mode for intent.md (`intent.md:acceptance_criteria`) | ~$0.05 + 7 KB context budget | **Tiny** (1 JSON edit) | Identified | D1 above |
| P5 | Retrospective YAML template externalization | ~$0.05 | Small | Identified | D3 above |
| P6 | PSMAS-style phase-skip via triage classification | up to $2.10 on skipped cycles | High (new orchestration) | Research-stage | PSMAS research above |
| P7 | TOON-format structured outputs for audit/triage tables | ~$0.10 | Medium (downstream parser updates) | Speculative | TOON research above |
| P8 | LLMLingua prompt compression integration | TBD | High | Speculative | LLMLingua research above |

### Realistic near-term target

**Items P1 + P2 + P3 + P4 + P5 land $2.00–$2.20 saved per $6.70 cycle = 30–33% reduction.**

Items P6–P8 push further (50%+) but require new architecture / external
dependencies. Defer until P1–P5 ship + verify.

### Trust-kernel invariants (unchanged across all P1–P8)

Every item in this roadmap operates *above* the trust kernel layer
(personas, profiles, role-context-builder). The kernel — phase-gate,
role-gate, ship-gate, ledger SHA chain, mutate-eval — is not touched.
Per the v9.0.0 release notes: "Context restructuring above the kernel
does not weaken structural integrity."

## Methodology — how to reproduce the per-phase cost math

For any cycle N with usage telemetry preserved:

```bash
# Per-phase totals
for f in intent scout triage builder auditor retrospective; do
    [ -f ".evolve/runs/cycle-${N}/${f}-usage.json" ] || continue
    jq -r --arg p "$f" '"\($p): cost=$\(.total_cost_usd) turns=\(.num_turns) " +
        "out=\(.usage.output_tokens) cc=\(.usage.cache_creation_input_tokens) " +
        "cr=\(.usage.cache_read_input_tokens)"' \
        ".evolve/runs/cycle-${N}/${f}-usage.json"
done

# Cost-share by category (apply 2026 per-MTok rates from Finout / Anthropic):
# Opus 4.7:    input $5    output $25    cache_create $6.25   cache_read $0.50
# Sonnet 4.6:  input $3    output $15    cache_create $3.75   cache_read $0.30
# (× 10⁻⁶ for per-token rate)
#
# Validate by summing: output × rate_out + cache_create × rate_cc + cache_read × rate_cr
# should equal modelUsage.<model>.costUSD from the same JSON.
```

## When to revisit this doc

- After v9.0.3 ships (scout fix) — append a "before/after" table for
  scout phase to the per-phase section.
- After v9.0.4 ships (builder fix) — same.
- When a new cycle's per-phase cost deviates materially (>±25%) from the
  v9.0.2 baseline above — investigate cause; update the table.
- When Anthropic updates Opus/Sonnet pricing — recompute share
  percentages; the absolute dollar figures rescale linearly.

## See also

- [`token-floor-history.md`](token-floor-history.md) — static context
  floor measurements per campaign (input-side dataset; pairs with this
  doc's runtime-side dataset)
- [`control-flags.md`](control-flags.md) — full flag inventory
  including the v9.0.0 opt-in `EVOLVE_*` flags referenced here
- [`docs/release-protocol.md`](../release-protocol.md) — publish
  vocabulary (push / tag / release / propagate / publish / ship)
- [Memory: `reference_token_optimization_research.md`](file:///Users/danleemh/.claude/projects/-Users-danleemh-ai-claude-evolve-loop/memory/reference_token_optimization_research.md)
  — 2026 production-state research (earlier campaign — pairs with this doc)
- Plan file: [`~/.claude/plans/let-s-pause-all-task-snug-bentley.md`](file:///Users/danleemh/.claude/plans/let-s-pause-all-task-snug-bentley.md)
