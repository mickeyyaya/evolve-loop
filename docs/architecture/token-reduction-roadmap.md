# Token-Reduction Roadmap (Cycles 15–19+)

> **Status:** v9.1.1 baseline — Cycle 15 research deliverable.
> For context-floor history see [token-floor-history.md](token-floor-history.md).
> For Cycle-11 cost forensics see [token-economics-2026.md](token-economics-2026.md).

## Baseline (Cycle 11 forensics — $6.70 total per cycle)

| Phase | Cost | Cache-read % | Cache-create % | Output % |
|-------|------|-------------|----------------|---------|
| Intent | $1.05 | ~50% | ~30% | ~19% |
| Scout | $1.32 | ~50% | ~30% | ~19% |
| Triage | $0.27 | ~50% | ~30% | ~19% |
| Builder | $1.95 | ~50% | ~30% | ~19% |
| Auditor | $2.10 | ~50% | ~30% | ~19% |

Near-term target (Cycles 15–18 combined): **−48% = ~$3.20/cycle saved**.

---

## P1 — Scout turn-count cap (max_turns → ≤15)

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-scout.md` + `profiles/scout.json` (`max_turns`) |
| **Expected saving** | ~$0.80/cycle (−60% Scout cost; 49→≤15 turns) |
| **LoC delta** | 0 (shipped v9.0.3; cap already in profile) |
| **Risk** | Low |
| **Target cycle** | DONE (v9.0.3) |
| **Verification** | `jq .turns .evolve/runs/cycle-N/scout-usage.json`; assert ≤15 |

**Source:** Anthropic multi-agent research system (2025–2026) — subagents must return 1–2K condensed summaries from 10K+ internal token work. [[1]](#sources)

---

## P2 — Builder turn-count cap (max_turns → ≤20)

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-builder.md` + `profiles/builder.json` (`max_turns`) |
| **Expected saving** | ~$1.00/cycle (−50% Builder cost; 58→≤20 turns) |
| **LoC delta** | ~30 LoC in persona stop-criteria + profile `max_turns` field |
| **Risk** | Medium |
| **Target cycle** | 16 |
| **Verification** | Before/after `builder-usage.json` turn count across 3 cycles; assert ≤20 |

**Rationale:** Cycle-11 forensics show Builder ran 58 turns ($1.95) — the single largest reduction lever after P1. A structured stop-criteria section (write-draft → verify-acceptance → emit-report → stop) prevents turn-budget exhaustion.

---

## P3 — Triage right-sizing (persona trim + EVOLVE_CONTEXT_DIGEST default-on for triage)

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-triage.md` + `profiles/triage.json` + dispatcher flag |
| **Expected saving** | ~$0.10/cycle (−40% triage context = 27,733B → 16,677B) |
| **LoC delta** | ~15 LoC trim in triage persona + `EVOLVE_CONTEXT_DIGEST` promotion ladder update |
| **Risk** | Low |
| **Target cycle** | 16 |
| **Verification** | `context-monitor.json` triage `input_bytes`; assert <17KB on anchored cycles |

**Source:** ACON NeurIPS-track (OpenReview) — 26–54% peak-token reduction via failure-driven guideline updates; gradient-free, model-agnostic. [[6]](#sources)

---

## P4 — Auditor anchor mode for intent.md acceptance criteria

| Field | Value |
|-------|-------|
| **Subsystem** | `profiles/auditor.json` (`context_anchors` field) |
| **Expected saving** | ~$0.05/cycle + 7 KB context-floor reduction for Auditor |
| **LoC delta** | 1 JSON edit (`auditor.json:context_anchors`) |
| **Risk** | Low |
| **Target cycle** | 15 (piloted via advisory hook infrastructure) |
| **Verification** | `role-context-builder.sh auditor …` output; assert intent section ≤500 chars when anchor active |

**Source:** Anthropic — Effective context engineering for AI agents (2025–2026): static-content-first / dynamic-content-last maximizes prompt-cache hit rate. [[9]](#sources)

---

## P5 — Retrospective YAML template externalization

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-retrospective.md` + new `skills/evolve-loop/lesson-template.yaml` |
| **Expected saving** | ~$0.05/cycle (reduce retrospective persona from 12,988B by ~2KB inline template) |
| **LoC delta** | ~30 LoC: extract template to `skills/evolve-loop/lesson-template.yaml`; persona reads on demand |
| **Risk** | Low |
| **Target cycle** | 17 |
| **Verification** | `retrospective-usage.json` `input_tokens` before/after; assert −15% |

**Source:** Progressive Disclosure (MindStudio 2025): three-layer persona (card/manual/reference) prevents context rot. [[10]](#sources)

---

## P6 — PSMAS-style phase-skip via triage `cycle_size_estimate`

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-orchestrator.md` + `agents/evolve-triage.md` + `scripts/lifecycle/phase-gate.sh` |
| **Expected saving** | Up to $2.10/cycle on skip-eligible cycles (Auditor cost = 0) |
| **LoC delta** | ~80 LoC in orchestrator persona skip-branch + triage `cycle_size_estimate=skip` path |
| **Risk** | High (new orchestration branch) |
| **Target cycle** | 19+ |
| **Verification** | A/B test: 5 cycles with skip-eligible tasks; assert auditor cost=0 on skipped cycles |

**Source:** PSMAS — Phase-Scheduled Multi-Agent Systems (arXiv:2510.26585, 2025): 27.3% mean token reduction via phase scheduling; beats learned routing by 5.6pp. [[2]](#sources)

---

## P7 — TOON-format structured outputs for audit-report + triage-decision

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-auditor.md` + `agents/evolve-triage.md` + `scripts/observability/verify-eval.sh` |
| **Expected saving** | ~$0.10/cycle (30–60% structured-output token reduction vs JSON per TOON research) |
| **LoC delta** | ~50 LoC: audit-report TSV template + triage-decision template + parser |
| **Risk** | Medium |
| **Target cycle** | 18 |
| **Verification** | Parse output in `verify-eval.sh`; assert TSV/TOON parse success; no FAIL regression |

**Source:** TOON format (DEV.to 2026): 30–60% structured-output token reduction vs JSON. [[7]](#sources)

---

## P8 — LLMLingua prompt compression integration

| Field | Value |
|-------|-------|
| **Subsystem** | Pre-processor on role-context-builder.sh output + `scripts/dispatch/subagent-run.sh` |
| **Expected saving** | TBD (20× theoretical; ~10–30% realistic for evolve-loop prose given existing caching) |
| **LoC delta** | High (~200 LoC + external dependency) |
| **Risk** | High |
| **Target cycle** | 20+ |
| **Verification** | Isolated test: same cycle prompt through LLMLingua vs. raw; assert PASS verdict unchanged |

**Source:** LLMLingua 2026 / TokenMix: 20× compression in production deployments ($42K → $2.1K monthly). [[8]](#sources)

---

## Additional Items Discovered Cycle 15

### P-NEW-1 — Flags A–D promotion (EVOLVE_CONTEXT_DIGEST + EVOLVE_ANCHOR_EXTRACT default-on)

| Field | Value |
|-------|-------|
| **Subsystem** | Dispatcher env defaults + flag promotion markers in `docs/architecture/control-flags.md` |
| **Expected saving** | ~$1.20/cycle (all users at cycle-11 rates: −85% scout, −40% triage, −43% builder) |
| **LoC delta** | ~10 LoC: update promotion ladder markers |
| **Risk** | Low (flags verified across multiple production cycles) |
| **Target cycle** | 16 (verification) → 17 (default-on) |
| **Verification** | Run 3 default cycles with flags ON; assert N/N regression tests pass |

### P-NEW-2 — Auditor model-tier adaptive right-sizing (Sonnet on consecutiveClean ≥ 3)

| Field | Value |
|-------|-------|
| **Subsystem** | `profiles/auditor.json` (`model_tier_overrides`) + eligibility check in orchestrator |
| **Expected saving** | ~$0.80/cycle (−40% auditor cost on clean cycles; Opus→Sonnet) |
| **LoC delta** | ~20 LoC in profile + eligibility check |
| **Risk** | Medium (requires validation that Sonnet ADVERSARIAL_AUDIT quality matches Opus) |
| **Target cycle** | 17+ (after code-reviewer Opus second-opinion wired per Cycle 17 plan) |
| **Verification** | Compare audit quality on 5 Sonnet vs. 5 Opus cycles; no CRITICAL miss-rate increase |

### P-NEW-3 — evolve-scout.md Layer-3 extraction (Campaign D gap)

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-scout.md` → extract ~2–4 KB to `agents/evolve-scout-reference.md` |
| **Expected saving** | ~$0.03/cycle (10–20% scout persona; currently 0% Campaign D extraction) |
| **LoC delta** | ~20 LoC (extract Phase 4 DEBRIEF algorithm + concept-card template + task-scoring rubric) |
| **Risk** | Medium (scout has integrated discovery logic; must verify no behavioral change) |
| **Target cycle** | 18+ |
| **Verification** | Role-context-builder scout output size; `swarm-architecture-test.sh` N/N PASS |

### P-NEW-4 — EVOLVE_REQUIRE_* → EVOLVE_REQUIRED_PHASES consolidation

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/dispatch/run-cycle.sh` + `scripts/dispatch/subagent-run.sh` |
| **Expected saving** | Operator ergonomics (2 active flags → 1; reduces documentation surface) |
| **LoC delta** | ~30 LoC + backward-compat bridge |
| **Risk** | Low |
| **Target cycle** | 18 |
| **Verification** | `evolve-loop-dispatch-test.sh`; assert existing behavior unchanged |

### P-NEW-5 — Deprecated flag removal (5 flags past removal target)

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/dispatch/claude.sh`, `scripts/lifecycle/ship.sh`, `scripts/failure/failure-adapter.sh` |
| **Expected saving** | Shell overhead reduction + dead-code removal |
| **LoC delta** | ~50 LoC removed (bridge code for FORCE_INNER_SANDBOX, BUDGET_CAP, STRICT_FAILURES, DISPATCH_STOP_ON_FAIL, DISPATCH_VERIFY) |
| **Risk** | Low (bridges emit stderr WARN; removal is planned; operators warned) |
| **Target cycle** | 16 |
| **Verification** | `guards-test.sh` + `dispatch-test.sh`; assert no regression |

---

## Realistic Near-Term Savings Summary

| Items | Mechanism | Saving/cycle | Target |
|-------|-----------|-------------|--------|
| P1 (done) | Scout turn cap | $0.80 | DONE v9.0.3 |
| P2 | Builder turn cap | $1.00 | Cycle 16 |
| P3 | Triage right-sizing | $0.10 | Cycle 16 |
| P-NEW-1 | Flags A–D default-on | $1.20 | Cycle 16–17 |
| P-NEW-5 | Deprecated flag removal | negligible | Cycle 16 |
| **Subtotal Cycles 16–17** | | **$2.30** | |
| P4 + P5 | Anchor mode + retro template | $0.10 | Cycle 17 |
| P-NEW-2 | Auditor right-sizing | $0.80 | Cycle 17+ |
| **Combined Cycles 15–18** | | **~$3.20/cycle** | **−48% on $6.70 baseline** |

Items P6–P8 and P-NEW-3/4 push further to 60–70% but require new architecture or cross-cycle verification.

---

## Sources

<a name="sources"></a>

1. Anthropic — How we built our multi-agent research system (2025–2026): https://www.anthropic.com/engineering/multi-agent-research-system
2. PSMAS — Phase-Scheduled Multi-Agent Systems (arXiv:2510.26585, 2025): https://arxiv.org/abs/2510.26585
3. Zylos — AI Agent Context Compression Strategies (2026-02-28): https://zylos.ai/research/2026-02-28-ai-agent-context-compression-strategies
4. SupervisorAgent — Obvious Works (2026): https://www.obviousworks.ch/en/token-optimization-saves-up-to-80-percent-llm-costs/
5. Finout — Claude Opus 4.7 Pricing (2026): https://www.finout.io/blog/claude-opus-4.7-pricing-the-real-cost-story-behind-the-unchanged-price-tag
6. ACON — NeurIPS-track (OpenReview, 2024–2025): https://openreview.net/pdf?id=7JbSwX6bNL
7. TOON format (DEV.to 2026): https://dev.to/pockit_tools/llm-structured-output-in-2026-stop-parsing-json-with-regex-and-do-it-right-34pk
8. LLMLingua 2026 / TokenMix: https://tokenmix.ai/blog/llmlingua-prompt-compression-2026
9. Anthropic — Effective context engineering for AI agents (2025–2026): https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
10. Progressive Disclosure (MindStudio 2025): https://www.mindstudio.ai/blog/progressive-disclosure-ai-agents-context-management
