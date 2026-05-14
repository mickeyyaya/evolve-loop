# Token-Reduction Roadmap (Cycles 15–19+)

> **Status:** v9.1.1 baseline — Cycle 15 research deliverable + opt-in audit-advisory hook. Cycles 16+17 originally shipped advisory subagents (`code-simplifier`, `evolve-code-reviewer`) but their reports were orphans — no downstream consumer. Cycle 20 refactor deleted both and replaced them with a Builder self-review skill loop (`EVOLVE_BUILDER_SELF_REVIEW=1`, default OFF) that invokes review skills mid-build via the Skill tool and converges before Auditor handoff. See "Cycle 20 Refactor" section below.
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
| **LoC delta** | 0 (shipped v9.0.3; cap in profile) + ~35 LoC persona stop-criterion (cycle 40) |
| **Risk** | Low |
| **Target cycle** | ~~DONE (v9.0.3)~~ **REOPENED** — `max_turns` is advisory-only; Claude CLI has no `--max-turns` flag. Cycle-39 scout ran **68 turns ($2.54)** despite `scout.json:max_turns=15`. Fix: `## STOP CRITERION` persona section (cycle 40). |
| **Verification** | `jq .turns .evolve/runs/cycle-N/scout-usage.json`; assert ≤20 (relaxed from ≤15; persona-enforced not CLI-enforced) |

**Source:** Anthropic multi-agent research system (2025–2026) — subagents must return 1–2K condensed summaries from 10K+ internal token work. [[1]](#sources)

**Cycle-40 fix:** `agents/evolve-scout.md` now has an explicit `## STOP CRITERION` section with four named completion gates (`system-health-complete`, `inbox-audit-complete`, `backlog-complete`, `build-plan-written`). Once all gates are satisfied, Scout calls `Write` and halts. Banned-post-report patterns enumerated. See P-NEW-10 below.

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

**Source:** PSMAS — Phase-Scheduled Multi-Agent Systems for Token-Efficient Coordination (arXiv:2604.17400, April 2026): **27.3%** mean token reduction via phase scheduling (range 21.4–34.8%); beats learned routing. [[2]](#sources)

---

## P7 — TOON-format structured outputs for audit-report + triage-decision

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-auditor.md` + `agents/evolve-triage.md` + `scripts/observability/verify-eval.sh` |
| **Expected saving** | ~$0.10/cycle (40–65% structured-output token reduction vs JSON per TOON 2026 benchmark suite; was 30–60%) |
| **LoC delta** | ~50 LoC: audit-report TSV template + triage-decision template + parser |
| **Risk** | Medium |
| **Target cycle** | 18 |
| **Verification** | Parse output in `verify-eval.sh`; assert TSV/TOON parse success; no FAIL regression |

**Source:** TOON format (DEV.to 2026): 40–65% structured-output token reduction vs JSON (2026 benchmark suite update). [[7]](#sources)

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
| **Target cycle** | 21+ (after Builder self-review loop verified across multiple cycles — Builder's mid-build skill convergence reduces the residual defect rate Auditor faces, increasing Sonnet's adequacy) |
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

## Cycle 20 Refactor: Builder Self-Review Skill Loop

### P-C20 — replace cycles 16+17 advisory subagents with Builder Skill-tool convergence loop

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-builder.md` Step 5 self-review loop spec; pluggable via `EVOLVE_BUILDER_REVIEW_SKILLS` env-var list (default `code-review-simplify`); invocation via `Skill` tool, no separate subagent profile |
| **What it replaces** | Cycles 16 (`code-simplifier` profile/persona/`builder-simplify-advisory.sh`/`EVOLVE_SIMPLIFY_ENABLED` hook) and 17 (`code-reviewer` profile/persona/`EVOLVE_FANOUT_AUDITOR_CODE_REVIEWER` fan-out hook) — both DELETED in this cycle |
| **Expected saving** | Net-same or cheaper per-cycle (~$0.30-1.00 opt-in vs. ~$0.50-1.30 combined cycle-16+17 overhead). Primary win: eliminates orphan reports; same-cycle feedback closes the loop. |
| **LoC delta** | ~436 LoC deleted (5 files) + ~60 LoC added (`evolve-builder.md`) = ~376 net subtraction across ~8 files |
| **Risk** | Low — `EVOLVE_BUILDER_SELF_REVIEW=0` default preserves byte-equivalence. Sycophancy break primary defense (Builder Sonnet → Auditor Opus) untouched. |
| **Target cycle** | 20 (SHIPPED — v9.2.0) |
| **Verification** | Cycle-20 end-to-end verification: `EVOLVE_BUILDER_SELF_REVIEW=1` activated with `code-review-simplify` skill; `## Self-Review` section confirmed in `build-report.md` with convergence verdict + per-skill composite scores. See cycle-20 build-report for cost delta (flag=1 overhead ~$0.10–0.25 per iteration; baseline flag=0 avg $1.07/cycle cycles 17–19). Git ref contract closed: SKILL.md Step 1 now uses adaptive `HEAD`/`HEAD~1` detection (was `HEAD~1` hardcoded — would have reviewed previous cycle's commit, not Builder's in-flight changes). Skill catalog assessed: `code-review-simplify` confirmed as the only compatible built-in skill; `simplify` system skill is action-based (no composite score); `review` is PR-centric; `refactor` spawns parallel worktrees (single-writer violation). See `docs/architecture/review-skill-catalog.md` for full compatibility table. Default-OFF behavior preserved: flag=0 produces no `## Self-Review` section (byte-equivalent to pre-v9.2.0). |

**Why this refactor.** Cycles 16+17 template-copied cycle-15's advisory pattern (skill invoked as a separate subagent around the Builder) but cycle 15's pattern advised an *already-bound* audit verdict — useful as observability. Cycles 16+17 instead advised mid-build work that Builder could still revise, but nothing wired their reports back to Builder. The reports (`code-simplifier-report.md`, `workers/code-reviewer.md`) became orphan forensic artifacts. The Skill primitive exists precisely so an agent can invoke a skill mid-task for self-review — moving the reviewers into Builder's own loop closes the feedback gap.

**Pluggability.** `EVOLVE_BUILDER_REVIEW_SKILLS` is a comma-separated skill name list. Operators add review skills (`refactor`, `security-review`, custom) without touching evolve-loop source.

**Env vars introduced (all default-OFF):**
- `EVOLVE_BUILDER_SELF_REVIEW=0` — master switch
- `EVOLVE_BUILDER_REVIEW_SKILLS=code-review-simplify` — invocation list
- `EVOLVE_BUILDER_REVIEW_MAX_ITERS=3` — convergence cap
- `EVOLVE_BUILDER_REVIEW_THRESHOLD=0.85` — clean/dirty threshold

**Env vars removed:** `EVOLVE_SIMPLIFY_ENABLED`, `EVOLVE_FANOUT_AUDITOR_CODE_REVIEWER`. (`EVOLVE_AUDIT_ADVISORY_REVIEW` from cycle 15 STAYS — operates post-verdict, a different lifecycle slot.)

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
2. PSMAS — Phase-Scheduled Multi-Agent Systems for Token-Efficient Coordination (arXiv:2604.17400, April 2026): https://arxiv.org/abs/2604.17400
3. Zylos — AI Agent Context Compression Strategies (2026-02-28): https://zylos.ai/research/2026-02-28-ai-agent-context-compression-strategies
4. SupervisorAgent — Obvious Works (2026): https://www.obviousworks.ch/en/token-optimization-saves-up-to-80-percent-llm-costs/
5. Finout — Claude Opus 4.7 Pricing (2026): https://www.finout.io/blog/claude-opus-4.7-pricing-the-real-cost-story-behind-the-unchanged-price-tag
6. ACON — NeurIPS-track (OpenReview, 2024–2025): https://openreview.net/pdf?id=7JbSwX6bNL
7. TOON format (DEV.to 2026): https://dev.to/pockit_tools/llm-structured-output-in-2026-stop-parsing-json-with-regex-and-do-it-right-34pk
8. LLMLingua 2026 / TokenMix: https://tokenmix.ai/blog/llmlingua-prompt-compression-2026
9. Anthropic — Effective context engineering for AI agents (2025–2026): https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
10. Progressive Disclosure (MindStudio 2025): https://www.mindstudio.ai/blog/progressive-disclosure-ai-agents-context-management
11. TACO — Self-Evolving Terminal Agent Compression (arXiv:2604.19572, Apr 2026): training-free, model-agnostic tool-result compression framework: https://arxiv.org/abs/2604.19572
12. Token Economics for LLM Agents (arXiv:2605.09104, May 2026): dual-view study — sequential pipelines achieve 20× lower token usage vs. parallel; combined orchestration+caching+routing achieves 70–80% total savings: https://arxiv.org/abs/2605.09104

---

## Status as of Cycle 47 (2026-05-14)

> Updated by Builder cycle 47. Previous snapshot: cycle 43 (2026-05-14).

**Cycle-46 cost snapshot (VERIFIED from usage sidecars):** Scout $0.64 (18 turns) / Triage $0.30 (6 turns) / Builder $3.72 (95 turns — **REGRESSION 4×**) / Auditor $0.84 (29 turns) / Orchestrator $2.53 (26 turns) / Memo $0.35 (9 turns) / Retrospective $0.73 (21 turns). **Total: $9.11 (+75% regression vs cycle-41 baseline $5.21).** Root cause: Builder 95 turns — scope overrun (Phase B: 15 ACS predicates + test + doc + abnormal event pipeline). P-NEW-22 schema filter + P-NEW-29 parallel batching shipped cycle 47 to address this pattern.

**Cycle-47 shipped items:** P-NEW-22 Phase 2 (dispatch-layer schema filter enforcement in `claude.sh`); P-NEW-29 (parallel tool-call batching guidance in Builder + Scout personas); T2 turn-overrun observability (`subagent-run.sh` abnormal event on `num_turns > max_turns`); P-NEW-30 roadmap entry (TACO arXiv:2604.19572); Sources entries 11–12.

**Expected cycle-48 baseline (if P-NEW-29 effective):** Builder turns ≤20 (from 95), saving ~$2.00/cycle on builder. Combined P-NEW-22 + P-NEW-29: target $5.00–6.00/cycle (vs $9.11 cycle-46 regression).

---

## Status as of Cycle 43 (2026-05-14)

> Updated by Scout/Builder cycle 43. Previous snapshot: cycle 42 (2026-05-14).

**Cycle-42 cost snapshot (VERIFIED):** Scout $1.24 (28 turns) / Triage $0.32 (5 turns) / Builder $1.06 (33 turns) / Auditor $1.55 (49 turns, Sonnet) / Orchestrator $1.15 (32 turns) / Memo $0.48 (14 turns). **Total: $5.80 (+11% regression vs cycle-41 $5.21).** Scout regression: 20→28 turns (+40%, from P-NEW-17 web search cost). Auditor regression: 35→49 turns (+40%, no stop-criterion — target of P-NEW-19). Memo improvement: 23→14 turns (−39%, P-NEW-16 STOP CRITERION working). Running shipped savings vs cycle-11 $6.70 baseline: ~$0.90/cycle.

**Cycle-41 cost snapshot (VERIFIED):** Scout $0.83 (20 turns) / Triage $0.40 (8 turns) / Builder $0.99 (34 turns) / Auditor $1.12 (35 turns, Sonnet — P-NEW-2 ✓) / Orchestrator $1.08 (42 turns) / Memo $0.79 (23 turns). **Total: $5.21 (−22% from cycle-11 $6.70 baseline, −$0.27 vs cycle-40 $5.48).** P-NEW-2 Auditor Sonnet: $0.98/cycle actual saving. P-NEW-10 Scout: 68→20 turns. P-NEW-9 Orchestrator: 50KB→10KB accumulated context. Running shipped savings: ~$2.15/cycle.

**Cycle-40 cost snapshot:** Scout $1.57 (40 turns) / Triage $0.37 / Builder $1.09 / Auditor $2.09 (Opus 4.7 — regression) / Retrospective $0.36. **Total: $5.48.** Auditor Opus regression (+$0.96 vs cycle-39 Sonnet $1.13) erased P-NEW-10 gains. Cumulative from cycle-11 $6.70 baseline: **>18% reduction achieved** (29% target not yet reached due to auditor regression).

**Cycle-41 fixes shipped:** P-NEW-2 (auditor default Sonnet — expected to recover $0.97/cycle); P-NEW-9 (orchestrator 3-bullet summarization, 50KB→10KB accumulated context); `scout-stop-tighten` (emergency exit turn 12 + max-3 WebSearch cap — targets 40→≤20 turns, ~$0.50/cycle). Builder worktree isolation enforcement promoted to default-ON (`EVOLVE_BUILDER_ISOLATION_CHECK=1` + `EVOLVE_BUILDER_ISOLATION_STRICT=1`) — cost-prevention: breach incidents cost ~$5.48/cycle in wasted retries (cycles 6, 40, 41); `git diff --quiet HEAD` replaces narrow evals/instincts directory scan. Tester allowlist added to `subagent-run.sh` — prevents 241s watchdog kills when orchestrator advances to test phase.

**P-NEW-10 confirmed:** $0.97/cycle actual saving (cycle 39→40 delta). Target ≤20 turns still pending; cycle-41 scout ran 40 turns — further improvement on track.

**Cache-safety validation (arXiv:2601.06007):** "Don't Break the Cache: Agentic Task Evaluation" (2026) confirms static-content-first / dynamic-content-last as a multi-turn cache-safety requirement. evolve-loop's `role-context-builder.sh` already implements this via `EVOLVE_CONTEXT_DIGEST` + `EVOLVE_ANCHOR_EXTRACT` (P4 DONE, P-NEW-1 DONE). No new action item — paper validates existing design.

**Inbox audit (cycle 41):** c45 (P-NEW-6 Branch B tool-result clearing) and C1 (ship-gate tree-SHA binding) confirmed ALREADY DONE — do NOT re-implement.

**Auditor model discrepancy:** cycle-39 used `claude-sonnet-4-6` despite `auditor.json:model_tier_default=opus`. Whether intentional (`ADVERSARIAL_AUDIT=0` or model-tier override) or accidental needs operator verification. If intentional, P-NEW-2 is effectively shipping $0.97/cycle savings; document it. If accidental, adversarial quality may be degraded.

| Item | State | Evidence anchor |
|------|-------|----------------|
| P1 Scout turn cap (≤15) | **REOPENED** (cycle 40) | `max_turns` advisory-only; claude CLI has no `--max-turns`; cycle-39 scout ran 68 turns ($2.54). Cycle-40 fix: `## STOP CRITERION` persona section in `agents/evolve-scout.md`. |
| P2 Builder turn cap (≤20 → actual 25) | DONE | `builder.json max_turns=25`; v9.0.4; update roadmap target to ≤25 |
| P3 Triage right-sizing | DONE (cycle 24) | Context savings delivered via EVOLVE_CONTEXT_DIGEST=1 default-on; triage gets compact intent; 123-line persona already lean |
| P4 Auditor anchor mode | DONE | `auditor.json:context_anchors` 4 anchors configured; v8.63.0 |
| P5 Retrospective YAML template | DONE (cycle 24) | `lesson-template.yaml` created at `.agents/skills/evolve-loop/`; `evolve-retrospective.md` trimmed −19 lines |
| P6 PSMAS phase-skip | PENDING | No implementation; benchmark updated to 34.8% (was 27.3%) |
| P7 TOON structured outputs | PENDING | No TSV template or parser; benchmark updated to 40–65% (was 30–60%) |
| P8 LLMLingua integration | PENDING | No integration; external dep |
| P-NEW-1 Flags A–D default-on | DONE (cycle 24) | `EVOLVE_CONTEXT_DIGEST` + `EVOLVE_ANCHOR_EXTRACT` promoted to `default=1` in `role-context-builder.sh`; v9.4.0 |
| P-NEW-2 Auditor Sonnet right-sizing | **VERIFIED (cycle 41)** | Cycle-41 empirical: $2.10→$1.12 auditor cost, **actual saving $0.98/cycle** (exceeded $0.97 estimate). `auditor.json:model_tier_default=sonnet`. Shipped `bb4e52d`. |
| P-NEW-3 evolve-scout.md Layer-3 split | DONE (cycle 24) | `agents/evolve-scout-reference.md` created; `evolve-scout.md` trimmed 334→167 lines |
| P-NEW-4 EVOLVE_REQUIRE_* consolidation | PENDING | `EVOLVE_REQUIRED_PHASES` not implemented |
| P-NEW-5 Deprecated flag removal | BRIDGES-ACTIVE | 5 flags w/ bridges; removal target v8.61+ MISSED; cycle 26+ |
| P-NEW-6 Tool-result clearing | DONE (cycle 36) | Profile field `context_clear_trigger_tokens` added to builder/scout/auditor; Tool-Result Hygiene subsection in 3 persona files; `subagent-run.sh` advisory log; `scripts/observability/tool-result-saturation.sh` NEW. |
| P-NEW-7 SkillReducer Layer-3 split | PARTIAL | `phases.md` split done; other skill files pending |
| P-NEW-8 AgentDiet filtering | **DONE (cycle 40)** | `jq` filter in `role-context-builder.sh` builder section: `select(.classification \| test("code-build-fail\|code-quality"))`. FSE 2026 benchmark: 39.9–59.7% input token reduction. Next-cycle telemetry confirms. |
| P-NEW-9 Orchestrator summarization | **DONE (cycle 41)** | `## Phase-Report Reading Protocol (P-NEW-9)` section added to `agents/evolve-orchestrator.md`; 3-bullet summary protocol (verdict + SHA + top defects); SHA preservation rule. Shipped `2522dbc`. Expected reduction: orchestrator accumulated context 50KB→10KB (~$0.10–0.30/cycle). |
| P-NEW-10 Scout stop-criterion | **DONE (cycle 40)** | `## STOP CRITERION` section added to `agents/evolve-scout.md`; 4 completion gates; banned post-report pattern list. **Cycle-40 actual saving: $0.97/cycle** (scout turns 68→40, cost $2.54→$1.57, −38%). Target ≤20 turns still pending (cycle-41 scout: 40 turns — progress confirmed). |
| P-NEW-11 MCP Compaction | RESEARCH | Cycle 45+; new external API dependency |
| P-NEW-12 RLM context folding | RESEARCH | Cycle 50+; paradigm-level; no prod deployments |
| P-NEW-13 Verbatim semantic compaction | **DONE (cycle 42)** | `subagent-run.sh` autotrim: `head -c`/`tail -c` → `head -n`/`tail -n` (line-boundary cut). ~25 LoC. Commit 183406e. |
| P-NEW-16 Orchestrator stop-criterion | **DONE (cycle 42)** | `## STOP CRITERION` section added to `agents/evolve-orchestrator.md`; 3 named gates; targets 42→25 orchestrator turns (~$0.40/cycle). Commit 183406e. |
| P-NEW-17 Explicit Cache TTL for cross-phase reuse | **INVESTIGATION-COMPLETE (cycle 43)** | Path A CLOSED: no `--cache-ttl` CLI flag. CRITICAL CORRECTION: Claude CLI uses 1h TTL (not 5m) per cycle-42 `ephemeral_1h_input_tokens` telemetry (`ephemeral_5m_input_tokens=0` for all phases). TTL concern is API SDK-specific, not CLI-specific. `claude -p` invocations already use 1h TTL. $2.00/cycle estimate was overstated for CLI path. True cross-phase reuse opportunity: shared system-prompt prefix via `EVOLVE_CACHE_PREFIX_V2` (addressed by P-NEW-18). |
| P-NEW-18 EVOLVE_CACHE_PREFIX_V2 default-on | **DONE (cycle 43)** | Default changed from `:-0` to `:-1` in `scripts/dispatch/subagent-run.sh` and `scripts/cli_adapters/claude.sh`. `docs/architecture/control-flags.md` updated. Overdue since v8.62 target (shipped v10.6). Expected saving: $0.10–0.30/cycle. |
| P-NEW-19 Auditor stop-criterion | **DONE (cycle 43)** | `## STOP CRITERION` section added to `agents/evolve-auditor.md`; 3 named gates + banned post-report patterns. ~30 LoC. Expected saving: $0.30–0.50/cycle. |
| P-NEW-20 Builder stop-criterion | **DONE (cycle 43)** | `## STOP CRITERION` section added to `agents/evolve-builder.md`; 4 named gates + banned post-report patterns. ~40 LoC. Expected saving: $0.40–0.60/cycle (cycle-43 builder: 39 turns / $1.22). |
| P-NEW-21 AgentDiet full trajectory compression | **DONE (cycle 45) — REVISED cycle 46** | `context_compact_expired_tool_results` and `context_compact_threshold_tokens` fields removed from `builder.json` in cycle 46 — P-NEW-21 is persona-level only (`agents/evolve-builder.md` Tool-Result Trajectory Compression section). No CLI flag exists for `--compact` (confirmed P-NEW-25 CLOSED). Profile fields were dead config. |
| P-NEW-22 Selective MCP tool-schema measurement + dispatch-layer enforcement | **DONE (cycle 47)** | Phase 1 (cycle 46): `schema_filter_enabled: true` field added to scout/triage/memo profiles. Phase 2 (cycle 47): `claude.sh` reads `schema_filter_enabled`; auto-injects `--strict-mcp-config` when field is true and flag is absent — making it the declarative source of truth. Profiles already had the flag in extra_flags; adapter now enforces it structurally. Expected: 5–20% per-turn input reduction for narrow-toolset roles (scout/triage/memo). |
| P-NEW-23 Token-budget-aware turn hints | **DONE (cycle 44)** | `emit_budget_hint()` in `role-context-builder.sh`; `turn_budget_hint` in 6 profiles (scout:12, builder:20, auditor:30, orchestrator:45, memo:8, triage:12). Preemptive budget declaration; arXiv:2412.18547. Expected: 10–20% turn reduction. |
| P-NEW-24 Observational context compression for Builder | **PENDING (cycle 47+)** | Remove expired tool-results from Builder multi-turn trajectory; arXiv:2604.19572 (Apr 2026); 40–60% input reduction on tool-output bloat. Profile-level contract changes required. Deferred pending P-NEW-27 baseline measurement (1-2 cycles). |
| P-NEW-25 Anthropic native compaction (compact-2026-01-12) | **CLOSED (cycle 46)** | `claude -p --help` (v2.1.140) confirms no `--compact` flag exists. Path A CLOSED. Path B (SDK-level compaction) out of scope for CLI pipeline. P-NEW-21 profile fields removed as dead config. |
| P-NEW-26 Per-role `--effort` flag dispatch | **DONE (cycle 44)** | `effort_level` field added to 6 profiles (scout/triage/memo/orchestrator=medium, builder/auditor=high); `scripts/cli_adapters/claude.sh` reads field + appends `--effort` to `claude -p` invocation. Guard: only appended when field non-empty. Expected saving: ~$0.66/cycle (~25% on medium-effort phases). |
| P-NEW-27 Scout tool-call discipline (Bash→native) | **DONE (cycle 46)** | BANNED patterns table added to `agents/evolve-scout.md` Tool-Result Hygiene section; 5 before/after examples; Bash-only-for-shell-operations rule. Root cause: cycle-45 scout made 36 Bash calls vs. 4 WebSearch ($1.30 actual vs $0.50 target). Expected saving: ≥$0.50/cycle when scout Bash ≤8. |
| P-NEW-28 RE-TRAC recursive trajectory compression | **PENDING (cycle 47+)** | arXiv:2602.02486 (RE-TRAC, Feb 2026): recursive summarization of oldest M turns at N-turn boundaries. Complementary to P-NEW-24 (removes tail bloat; RE-TRAC compresses head bloat). Expected: 40–60% input reduction on Builder 35-turn sessions. Needs spec before implementation. |
| P-NEW-29 Parallel tool-call batching (multi-tool-use) | **DONE (cycle 47)** | "Parallel Tool-Call Batching" section with 3 before/after examples added to `agents/evolve-builder.md` (Turn budget section) and `agents/evolve-scout.md` (BANNED patterns section). Rule: emit independent tool calls in a single turn. Expected: 20–40% turn reduction for read-heavy phases. |
| P-NEW-30 TACO terminal observation compression | **PENDING (cycle 48+)** | arXiv:2604.19572 (TACO, Apr 2026): training-free tool-result compression framework that discovers and refines compression rules from agent trajectories. Zero training cost, model-agnostic. Upgrade path for P-NEW-24; applies compression BEFORE tool results enter the context. Expected: 30–50% terminal observation token reduction. [[11]](#sources) |
| P-C20 Builder self-review skill loop | DONE | v9.2.0 + v9.3.0 --plugin-dir fix; `EVOLVE_BUILDER_SELF_REVIEW=0` intentional |

---

## 2026 Delta — Patterns Not Yet In P1–P8

Research scan: 3 sources (Anthropic Claude Cookbook 2026, SmolAgents 2026, Redis/GetMaxim 2026). Conducted cycle 24.

### Confirmed: P1–P8 + P-NEW remain canonical roster

No paradigm shifts found. The field in 2026 has converged on the same layering evolve-loop already implements: context compaction (= P-NEW-1 EVOLVE_CONTEXT_DIGEST), selective context clearing (= EVOLVE_ANCHOR_EXTRACT), and memory/instinct persistence (= `state.json:instinctSummary[]`).

### Net-new candidate: P-NEW-6 — Tool-result clearing

Anthropic's 2026 production cookbook documents "tool-result clearing": surgically remove old `tool_result` blocks (keeping `tool_use`) at a configurable token threshold. In a baseline research agent example, this freed ~164K tokens (67% reduction) from re-fetchable file reads, keeping peak context at 173K vs 335K — critical for staying under 200K windows.

**Relevance to evolve-loop:** Builder's multi-file-read phases accumulate large `tool_result` blocks in context. A profile field `context_clear_trigger_tokens: 30000` + `context_clear_keep_recent_tool_results: 4` could be added to `builder.json` (similar to auditor's `context_anchors` pattern). Expected saving: 20–40% Builder context reduction. Risk: Low (surgical, not destructive). Target: cycle 26+ after P-NEW-1 promotion.

Source: https://platform.claude.com/cookbook/tool-use-context-engineering-context-engineering-tools

---

## P-NEW-7 — SkillReducer-Style Layer-3 Split for Large Skill Files

| Field | Value |
|-------|-------|
| **Subsystem** | `skills/evolve-loop/phases.md`, `SKILL.md`, `online-researcher.md`, `benchmark-eval.md` |
| **Expected saving** | $0.10–0.40/cycle: phases.md 28,911→~14KB = 16KB saved per load × 2+ loads/cycle ≈ 8,000 tokens × $3/MTok = $0.024/cycle minimum; scales 5–10× when EVOLVE_BUILDER_SELF_REVIEW becomes default-ON |
| **LoC delta** | ~0 LoC code change; ~60 LoC moved to `skills/evolve-loop/reference/<name>-detail.md`; core bodies trimmed to <14KB each |
| **Risk** | Low — read-only content reorganization; skill invocation path unchanged |
| **Target cycle** | 24 (I1 shipped — `phases.md` split done) |
| **Verification** | `wc -c .agents/skills/evolve-loop/phases.md` < 14000; `test -f skills/evolve-loop/reference/phases-detail.md` |
| **Source** | SkillReducer (arXiv:2603.29919, March 2026) + addyosmani/agent-skills (GitHub, Oct 2025) |

**Anti-gaming:** Moving content to reference sub-files doesn't change tool grants. Builder cannot self-mark PASS on "skill still works" — Auditor re-invokes and checks exit 0 independently.

---

## P-NEW-8 — AgentDiet-Style failedApproaches Classification Filtering

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/lifecycle/role-context-builder.sh` (builder + auditor role sections) |
| **Expected saving** | $0.03–0.10/cycle; **FSE 2026 paper benchmark: 39.9–59.7% input token reduction, 21.1–35.9% cost reduction** (AgentDiet continuous-control benchmark). Evolve-loop projection: ~6KB Builder context reduction by filtering 14/23 non-expired entries (code-audit-warn + unknown-classification). |
| **LoC delta** | ~14 LoC in `role-context-builder.sh` builder-role section: add `jq` filter `select(.classification \| test("code-build-fail\|code-quality"))` |
| **Risk** | Low — filtering is idempotent; worst case Builder sees fewer failure examples (all existing ones are audit-domain anyway) |
| **Target cycle** | ~~26+~~ **DONE (cycle 40)** — filter implemented in builder section |
| **Verification** | `context-monitor.json` `builder.input_bytes` delta: assert reduction ≥ 2KB vs prior cycle (next cycle telemetry) |
| **Source** | AgentDiet (arXiv:2509.23586, FSE 2026) — trajectory expired/useless content removal |

**Anti-gaming:** Classification labels set by `record-failure-to-state.sh` (kernel script, not Builder). Auditor verifies: `jq '[.failedApproaches[] | .classification] | unique' .evolve/state.json` shows no new classification strings.

---

## P-NEW-9 — OpenHands-Style Orchestrator Phase-Report Summarization

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-orchestrator.md` (phase-report reading protocol) |
| **Expected saving** | $0.10–0.30/cycle: orchestrator reads scout-report (~15KB) + build-report (~20KB) + audit-report (~15KB) = 50KB accumulated. With 3-bullet summarization between phases: 50KB → ~10KB in orchestrator context. Direct saving: 40KB × (1/4 tokens/byte) × $3/MTok = $0.03/cycle plus ~40% fewer orchestrator turns |
| **LoC delta** | ~40 LoC in orchestrator persona: add "phase-report reading → 3-bullet summary → pass summary to next phase" protocol section |
| **Risk** | Medium — LLM-generated summaries could miss audit defect IDs needed for SHA verification. Mitigation: summary MUST include verbatim `## Verdict` line + SHA8 from each report |
| **Target cycle** | 26+ (after P-NEW-7 verified) |
| **Verification** | `context-monitor.json` `orchestrator.input_bytes` delta: assert reduction ≥ 10KB vs prior cycle; Auditor verifies SHA in orchestrator-report matches audit-report on-disk |
| **Source** | OpenHands context condensation (arXiv:2511.03690, Nov 2025) — LLMSummarizingCondenser; quadratic→linear context scaling |

**Anti-gaming:** Orchestrator could summarize away its own audit defects. Mitigation: SHA verification preserved in summary (auditor SHA is 8 chars, always fits). Auditor independently cross-checks SHA.

---

## P-NEW-10 — Scout Stop-Criterion Persona Enforcement

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-scout.md` — `## STOP CRITERION` section |
| **Expected saving** | $0.50–$1.00/cycle (68→~20 turns based on GAIA 29.68% reduction applied to multi-turn research) |
| **LoC delta** | ~35 LoC added to `agents/evolve-scout.md` |
| **Risk** | Low — prompt-level only; no structural change |
| **Target cycle** | **DONE (cycle 40)** |
| **Verification** | `jq .turns .evolve/runs/cycle-N/scout-usage.json`; assert ≤20 across 3 consecutive cycles |

**Problem:** `max_turns` in `scout.json` is advisory-only — the Claude CLI has no `--max-turns` flag (`claude --help` confirms). Cycle-39 scout ran 68 turns ($2.54) despite `max_turns=15`. The P1 roadmap item claimed DONE in v9.0.3, but the profile field documents intent only; it has no mechanical effect.

**Fix:** `## STOP CRITERION` section with four named completion gates (`system-health-complete`, `inbox-audit-complete`, `backlog-complete`, `build-plan-written`). Once all gates satisfied: call `Write`, halt immediately. Banned post-report patterns enumerated (no "Let me also check…" exploration after report written).

**Source:** Anthropic SupervisorAgent + observation purification (GAIA benchmark, 2026) — LLM-free adaptive filtering; model terminates own session once report written. Expected saving: 29.68% turn reduction applied to scout multi-turn research profile.

---

## P-NEW-11 — MCP Native Compaction API for Inter-Phase Artifacts

| Field | Value |
|-------|-------|
| **Subsystem** | Phase artifact pipeline; replaces bash preprocessing in `scripts/dispatch/subagent-run.sh` |
| **Expected saving** | TBD — runtime-native compaction replaces current bash preprocessing (EVOLVE_CONTEXT_DIGEST). Potential 30–50% context reduction on accumulated cross-phase artifacts. |
| **LoC delta** | High (~100+ LoC + new external API dependency) |
| **Risk** | Medium-High (new external API dependency; requires architectural evaluation) |
| **Target cycle** | 45+ |
| **Verification** | Isolated test: same cross-phase artifacts through MCP Compaction vs. current bash preprocessing; assert output quality preserved |

**Source:** Anthropic MCP + Compaction (2026 engineering blog) — native compaction API enables memory-efficient cross-agent communication; combined with tool-result clearing, eliminates token waste from execution trace bloat. evolve-loop already does compaction-equivalent via EVOLVE_CONTEXT_DIGEST; MCP Compaction could provide a runtime-native mechanism.

---

## P-NEW-12 — Recursive Language Model (RLM) Context Folding

| Field | Value |
|-------|-------|
| **Subsystem** | Long-term replacement for EVOLVE_CONTEXT_DIGEST bash preprocessing |
| **Expected saving** | TBD — models actively manage their own context folding, shifting from predefined compression to learned, task-adaptive strategies |
| **LoC delta** | High (paradigm-level change; no LoC estimate possible) |
| **Risk** | High — paradigm-level; no production deployments to reference in 2026 H1 |
| **Target cycle** | 50+ (research-only until production deployments emerge) |
| **Verification** | N/A in 2026 H1 — monitor arXiv for production-ready implementations |

**Source:** Emerging 2026 paradigm (multiple sources) — models actively manage their own context folding. Long-term replacement for EVOLVE_CONTEXT_DIGEST. Research-only; no production implementation target in 2026 H1.

---

## P-NEW-13 — Verbatim Semantic Compaction for EVOLVE_CONTEXT_AUTOTRIM

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/dispatch/subagent-run.sh` autotrim block (lines 651–689) |
| **Expected saving** | ~10% of cycles that hit autotrim-induced re-reads (reduces false-positive autotrim fragmentation of file paths, function signatures, and JSON structures) |
| **LoC delta** | ~25 LoC: extend autotrim to cut at line-break boundaries instead of byte boundaries; preserve complete semantic units (full JSON objects, complete file paths) |
| **Risk** | Low — improves correctness; no structural change to the pipeline |
| **Target cycle** | **DONE (cycle 42)** — commit 183406e |
| **Verification** | Enable `EVOLVE_CONTEXT_AUTOTRIM=1`; verify autotrim output has no truncated JSON objects or partial file paths; assert no autotrim-induced re-read events in `subagent-run.sh` log |

**Source:** Production agentic pipeline analysis (2026) — operating on complete semantic units (statements/blocks) rather than partial truncation ensures file paths and critical references remain intact or absent entirely — critical safety property for code agents.

**Current gap:** `EVOLVE_CONTEXT_AUTOTRIM=1` uses head-60%/tail-35% truncation (byte-boundary cut) that can split file paths, function signatures, and JSON structures mid-token — triggering parse failures in Builder and requiring expensive re-read loops.

**Validation:** arXiv:2601.06007 "Don't Break the Cache: Agentic Task Evaluation" (2026) confirms static-content-first / dynamic-content-last as a cache-safety constraint. Semantic-unit preservation is the complementary compaction-safety constraint — both are required for safe multi-turn agent context management.

---

## P-NEW-14 — Agentic Plan Caching (APC)

| Field | Value |
|-------|-------|
| **Subsystem** | Builder pre-build phase + `scripts/dispatch/subagent-run.sh` |
| **Expected saving** | **76.42% cost reduction** on recurring task classes (GAIA benchmark: $69.02→$16.27/query, 0.61% accuracy drop) |
| **LoC delta** | ~50 LoC: `builder-plan-cache.json` template store + lightweight similarity check in orchestrator pre-Builder |
| **Risk** | Medium — false-positive plan reuse degrades build quality; requires similarity threshold tuning |
| **Target cycle** | 46+ (post P-NEW-2 verification) |
| **Verification** | Builder cost delta on recurring token-reduction task types; assert plan-reuse accuracy ≥99% (no incorrect ACs shipped) |

**Source:** Agentic Plan Caching (arXiv:2506.14852, 2026) — cache structured plan templates from initial Builder reasoning; reuse across semantically similar task classes via lightweight similarity matching. Benchmark: 76.42% cost reduction on GAIA, 0.61% accuracy drop.

**Applicability to evolve-loop:** Builder repeatedly solves similar task archetypes (profile edit, persona section add, roadmap update, ACS predicate write). Plan templates for these archetypes could save 5–8 planning turns per cycle on recognized task types.

---

## P-NEW-15 — Hierarchical Caching for Agentic Workflows

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/dispatch/subagent-run.sh` (MCP tool schema caching at dispatcher level) |
| **Expected saving** | **50.31% cost reduction, 27.28% latency improvement** vs. no caching (MDPI 2026 benchmark) |
| **LoC delta** | ~80 LoC in `subagent-run.sh`: workflow-level + tool-level cache with dependency-aware graph invalidation |
| **Risk** | Medium — MCP spec compliance required; cache invalidation correctness critical |
| **Target cycle** | 48+ |
| **Verification** | MCP tool schema re-send count per cycle (assert 0 on cached invocations); latency delta in `*-timing.json` sidecars |

**Source:** Hierarchical Caching for Agentic Workflows (MDPI 2026) — multi-level caching (workflow-level + tool-level) with dependency-aware graph invalidation. MCP tool schema re-sending is 30–50% of context waste in ≥40-tool workflows.

**Applicability to evolve-loop:** evolve-loop uses strict MCP config per subagent (`--strict-mcp-config`); tool schemas are re-sent on each `subagent-run.sh` invocation. With ~6 subagents/cycle each receiving the same tool schema set, dispatcher-level caching could eliminate most of that redundancy.

---

## P-NEW-16 — Orchestrator Stop-Criterion Persona Section

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-orchestrator.md` — `## STOP CRITERION` section |
| **Expected saving** | ~$0.40/cycle (orchestrator 42→~25 turns; cycle-41 orchestrator $1.08; 25/42 × $1.08 = ~$0.65 target, delta ~$0.40) |
| **LoC delta** | ~30 LoC in `agents/evolve-orchestrator.md` |
| **Risk** | Low — prompt-level only; no structural change |
| **Target cycle** | **DONE (cycle 42)** |
| **Verification** | `jq .turns .evolve/runs/cycle-N/orchestrator-usage.json`; assert ≤30 across 3 consecutive cycles |

**Problem:** Orchestrator ran 42 turns in cycle-41 despite P-NEW-9's 3-bullet summary protocol. P-NEW-9 reduces re-read overhead but doesn't bound total turn count. Without explicit completion gates, orchestrator continues exploring after its work is done — re-reading ledger, memos, and instincts after `orchestrator-report.md` is written.

**Fix:** `## STOP CRITERION` section in `agents/evolve-orchestrator.md` (analogous to `agents/evolve-scout.md`). Three named completion gates:
- `phase-sequence-complete` — all required phases invoked, each produced an artifact in `$WORKSPACE`
- `verdict-written` — `orchestrator-report.md` contains `## Verdict` line
- `cycle-state-advanced` — cycle-state phase reflects final state (ship/retrospective/blocked)

Once all three gates satisfied: `Write` the report and halt. Banned post-report patterns enumerated (re-reading audit-report, additional ledger reads, "Let me verify one more time…" loops).

**Source:** Analogous to P-NEW-10 (Scout stop-criterion, DONE cycle 40) which delivered $0.97/cycle actual saving (68→20 turns). Orchestrator exhibits the same post-completion accumulation pattern at lower base cost.

---

## P-NEW-17 — Explicit Cache TTL Configuration for Cross-Phase Reuse

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/dispatch/subagent-run.sh` + claude CLI capability investigation |
| **Expected saving** | Up to $2.00/cycle (eliminate per-phase cache-creation re-cost when cross-phase TTL > 5 min) |
| **LoC delta** | TBD pending CLI capability investigation; estimated ~15 LoC if flag available |
| **Risk** | Medium (depends on claude CLI exposing ttl configuration) |
| **Target cycle** | 43 (investigation) → 44 (implementation if feasible) |

**Problem:** Anthropic silently changed prompt cache TTL from 60 min → 5 min on 2026-03-06. For multi-agent pipelines where sequential phases span 10–30+ minutes total, this means every phase re-pays cache-creation costs (~$0.40–0.63/phase × 5 phases ≈ $2/cycle fixed overhead). Cross-phase cache reuse — which was theoretically possible under the 60-min TTL — is now structurally impossible.

Per cycle-11 forensics (`docs/architecture/token-economics-2026.md`): cache-creation is ~30% of per-phase cost. Eliminating cross-phase cache misses (restoring 1-hour TTL) could recover most of this.

**Three mitigation paths:**

| Path | Mechanism | Feasibility |
|------|-----------|-------------|
| A — CLI flag | Pass `--cache-ttl=3600` or equivalent to `claude -p` subagent invocation | Requires CLI support; investigate cycle 43 |
| B — API migration | Document as prerequisite for API-based subagent dispatch (SDK exposes `"ttl": 3600` in `cache_control`) | Long-term; requires API auth + non-CLI dispatch |
| C — Phase timing | Minimize inter-phase delays (orchestrator turn reduction via P-NEW-16; parallel-eligible phases) | Already in motion via P-NEW-9/P-NEW-10/P-NEW-16 |

**Investigation tasks (cycle 43):**
1. Run `claude --help | grep -i cache` to confirm if `--cache-ttl` flag exists in installed version
2. Inspect `~/.claude/config.json` or equivalent for TTL overrides
3. Test: invoke `claude -p` with explicit cache breakpoint and measure TTL via API metadata
4. Document result in `knowledge-base/research/cache-ttl-march-2026-impact.md`

**Source:** Anthropic TTL change 2026-03-06 (GitHub claude-code issue #56307); Anthropic SDK `cache_control.ttl` (SDK docs 2026); arXiv:2601.06007 "Don't Break the Cache: Agentic Task Evaluation" (2026) — cache-safety constraints for multi-turn agents.

See `knowledge-base/research/cache-ttl-march-2026-impact.md` for full research dossier.

---

## P-NEW-18 — EVOLVE_CACHE_PREFIX_V2 Promotion to Default-On

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/dispatch/subagent-run.sh` + `scripts/cli_adapters/claude.sh` + `docs/architecture/control-flags.md` |
| **Expected saving** | $0.10–0.30/cycle (cleaner prompt structure; static bedrock cached in system-prompt slot rather than user-message position — better cache hit rate for role bedrock content) |
| **LoC delta** | ~2 LoC code change (default value in two guards) + ~5 LoC docs update |
| **Risk** | Low (deployed stable since v8.61.0; `--exclude-dynamic-system-prompt-sections` already in all profiles; 18+ versions of opt-in testing without known breakage) |
| **Target cycle** | **DONE (cycle 43)** — overdue since v8.62 target (now v10.6) |
| **Verification** | `grep 'EVOLVE_CACHE_PREFIX_V2:-1' scripts/dispatch/subagent-run.sh` exits 0; `grep 'EVOLVE_CACHE_PREFIX_V2:-1' scripts/cli_adapters/claude.sh` exits 0 |

**What the promotion does:**
- (Cycle A1) `subagent-run.sh` emits a compact `## INVOCATION CONTEXT` user prompt instead of the verbose v1 header
- (Cycle A2) `claude.sh` attaches role-specific bedrock via `--append-system-prompt` (system-prompt slot is cached automatically; byte-stable per role across runs)
- `--exclude-dynamic-system-prompt-sections` already flows through profiles' `extra_flags` in both v1 and v2

**Promotion ladder satisfied:** default-off (v8.61) → verify (v8.61–v10.5.x, 18+ versions, no known breakage) → **default-on (v10.6, this cycle)**.

---

## P-NEW-19 — Auditor Stop-Criterion Persona Section

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-auditor.md` — `## STOP CRITERION` section |
| **Expected saving** | $0.30–0.50/cycle (auditor 49→~30 turns; cycle-42 auditor $1.55; 30/49 × $1.55 = ~$0.95 target, delta ~$0.50) |
| **LoC delta** | ~30 LoC in `agents/evolve-auditor.md` |
| **Risk** | Low — prompt-level only; no structural change |
| **Target cycle** | 44 |
| **Verification** | `jq .turns .evolve/runs/cycle-N/auditor-usage.json`; assert ≤35 across 3 consecutive cycles |

**Status: DONE (cycle 43).** `## STOP CRITERION` section added to `agents/evolve-auditor.md` with 3 named completion gates (`predicates-run`, `verdict-decided`, `report-written`) and banned post-report patterns. ~30 LoC.

**Problem:** Cycle-42 auditor ran 49 turns ($1.55) vs cycle-41's 35 turns ($1.12). No turn-count bound existed for the auditor. Without explicit completion gates, auditor continues exploring defects and verifying predicates after its verdict is decided.

**Fix:** `## STOP CRITERION` section in `agents/evolve-auditor.md` (analogous to P-NEW-10 for Scout, P-NEW-16 for Orchestrator). Completion gates:
- `predicates-run` — all `acs/cycle-N/*.sh` predicates executed (or noted absent)
- `verdict-decided` — PASS/FAIL decision made from `acs-verdict.json` + predicate results
- `report-written` — `audit-report.md` + `acs-verdict.json` written

Banned post-report patterns: re-running predicates after verdict written, additional grep searches after report written, "let me also check…" loops.

**Source:** Analogous to P-NEW-10 (Scout, $0.97/cycle actual) and P-NEW-16 (Orchestrator, $0.40/cycle expected). Same post-completion accumulation pattern at auditor cost.

**Verification:** `jq .turns .evolve/runs/cycle-N/auditor-usage.json`; assert ≤35 across 3 consecutive cycles.

## P-NEW-20 — Builder Stop-Criterion Persona Section

| Field | Value |
|-------|-------|
| **Subsystem** | `agents/evolve-builder.md` — `## STOP CRITERION` section |
| **Expected saving** | $0.40–0.60/cycle (cycle-43 builder: 39 turns / $1.22; analogous to P-NEW-10 Scout $0.97/cycle actual) |
| **LoC delta** | ~40 LoC in `agents/evolve-builder.md` |
| **Risk** | Low — prompt-level only; no structural change |
| **Target cycle** | 43 |
| **Verification** | `jq .turns .evolve/runs/cycle-N/builder-usage.json`; assert ≤25 across 3 consecutive cycles |

**Status: DONE (cycle 43).** `## STOP CRITERION` section added to `agents/evolve-builder.md` with 4 named completion gates (`worktree-verified`, `implementation-complete`, `self-verify-passed`, `report-written`) and banned post-report patterns.

**Problem:** `agents/evolve-builder.md` had zero STOP CRITERION / Completion Gates (431 lines, confirmed by grep). Builder ran 39 turns ($1.22) in cycle-43. Analogous gap to pre-P-NEW-10 Scout (68 turns) and pre-P-NEW-16 Orchestrator.

**Source:** Same post-completion accumulation pattern. P-NEW-10 (Scout, DONE, $0.97/cycle actual), P-NEW-16 (Orchestrator, DONE). Builder is the highest-cost phase and the last one without a stop criterion.

---

## P-NEW-21 — AgentDiet Full Trajectory Compression for Builder

| Field | Value |
|-------|-------|
| **Subsystem** | Builder profile + persona — expired tool-result removal during multi-turn read phases |
| **Status** | **DONE (cycle 45)** |
| **Expected saving** | 20–30% Builder cost reduction; cache_read 9.9M → ≤5M tokens |
| **LoC delta** | ~15 LoC (profile fields + persona section) |
| **Risk** | Low — persona-level guidance + profile fields; no pipeline logic change |
| **Shipped cycle** | 45 |
| **Verification** | Compare `builder-usage.json:input_tokens` in cycle-46 vs cycle-44 baseline (expect ≥15% reduction) |

**Problem:** P-NEW-8 (DONE cycle 40) applied AgentDiet-style filtering to `failedApproaches[]` only. Builder's multi-turn read phases accumulate large `tool_result` blocks (intermediate file reads) in context. These expired reads add input token overhead without contributing to the build decision.

**Fix:** Profile field `context_compact_expired_tool_results: true` + builder persona guidance to summarize tool results after each Read and discard full content. The Tool-Result Hygiene section (P-NEW-6, DONE cycle 36) partially addresses this; AgentDiet extends to auto-compaction at the profile level.

**Source:** AgentDiet (FSE 2026, arXiv:2509.23586v2): 39.9–59.7% input token reduction, 21.1–35.9% total cost reduction, no performance regression on SWE-bench. Full trajectory compression targets useless/redundant/expired tool results.

---

## P-NEW-22 — Selective MCP Tool-Schema Measurement and Reduction

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/cli_adapters/claude.sh` — dispatch-layer schema filter enforcement |
| **Status** | **DONE (cycle 47)** |
| **Expected saving** | 5–20% per-turn input token reduction for scout/triage/memo |
| **LoC delta** | ~3 LoC (profile fields added cycle 46); ~23 LoC adapter enforcement (cycle 47) |
| **Risk** | Low |
| **Shipped cycle** | 47 |
| **Verification** | `grep -q "schema_filter_enabled" .evolve/profiles/scout.json` AND `grep -q "SCHEMA_FILTER_ENABLED" scripts/cli_adapters/claude.sh` |

**Measurement result (cycle 46):** `--allowedTools` restricts invocations but does NOT reduce schema token serialization. Full MCP schema is serialized by the Claude CLI on every turn regardless of the `--allowedTools` filter. For a session with 10+ MCP tools, this adds ~2,000–4,000 tokens per turn overhead on narrow-toolset roles (scout, triage, memo).

**Phase 1 (DONE cycle 46):** Added `schema_filter_enabled: true` field to `profiles/scout.json`, `profiles/triage.json`, `profiles/memo.json`. This marks these roles as targets for dispatch-layer schema filtering.

**Phase 2 (DONE cycle 47):** `scripts/cli_adapters/claude.sh` reads `schema_filter_enabled` from the profile. When `true`, adapter checks whether `--strict-mcp-config` is already in `EXTRA_FLAGS_ARR`; if not, auto-injects it. This makes `schema_filter_enabled: true` the declarative source of truth — the adapter enforces it structurally rather than relying on each profile to manually include the flag in `extra_flags`. All three target profiles already had `--strict-mcp-config` in their extra_flags; the adapter now guarantees it for any future profile that sets `schema_filter_enabled: true`.

**Source:** GitHub Blog (2026-05-13) "Improving token efficiency in agentic workflows"; MindStudio "10 MCP Optimization Techniques"; MCP SEP-1576 "Mitigating Token Bloat". Tool schema compression: 30–60% per-request overhead reduction reported.

---

## P-NEW-23 — Token-Budget-Aware Turn Hints via role-context-builder.sh

**Status: DONE (cycle 44)**

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/lifecycle/role-context-builder.sh` — `emit_budget_hint()` added to `header_block()` |
| **Expected saving** | 10–20% additional turn reduction on top of stop-criterion |
| **Actual LoC delta** | 16 LoC in `role-context-builder.sh` + 6 one-line profile edits |
| **Risk** | Low — prompt-level only; no structural change |
| **Shipped cycle** | 44 |
| **Verification** | Compare `turns` in usage sidecars with/without hint across 3 cycles; assert ≥10% reduction |

**Problem:** Stop-criterion sections (P-NEW-10, P-NEW-16, P-NEW-19, P-NEW-20) are gate-based — they tell the agent when to stop *after* work is done. Budget hints are *preemptive*: injecting a turn budget in the context primes the agent to be concise from turn 1, preventing over-elaboration before gates are even reached.

**Implementation:** `emit_budget_hint()` in `role-context-builder.sh` reads `turn_budget_hint` from the role's profile JSON via `jq`. When set, it appends a `## Budget` block to `header_block()` output. Per-role advisory values: scout:12, builder:20, auditor:30, orchestrator:45, memo:8, triage:12 (all roughly 75–80% of `max_turns` to induce conservative self-regulation per arXiv:2412.18547).

---

## P-NEW-24 — Observational Context Compression for Builder

| Field | Value |
|-------|-------|
| **Subsystem** | Builder multi-turn trajectory; expired tool-result removal |
| **Research source** | arXiv:2604.19572 (Observational Context Compression, Apr 2026) |
| **Expected saving** | 40–60% input token reduction on tool-output trajectory bloat |
| **LoC delta** | Profile-level contract changes + subagent-run.sh trajectory filter |
| **Risk** | Medium — requires tracking "expired" tool results without losing active state |
| **Target cycle** | 46+ (after P-NEW-23 baseline established and measured) |
| **Verification** | Compare Builder `input_tokens` per-turn in usage sidecars before/after; assert ≥30% reduction on multi-file builds |

**Problem:** Builder multi-turn read phases accumulate tool results (Read, Grep, Bash) in the conversation trajectory. After a file is read and acted upon, its contents remain in the context forever — pure token waste. In cycle-43, Builder ran 69 turns at $3.12; a significant portion was repeated file-content in context.

**Fix:** Automated identification and removal of tool-result entries that have already been summarized or acted upon (agent has moved past them in its reasoning chain). Analogous to AgentDiet (arXiv:2509.23586, FSE 2026) but targeting the observation layer rather than the action layer.

---

## P-NEW-25 — Anthropic Native Compaction via compact-2026-01-12 API

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/dispatch/subagent-run.sh` / `scripts/cli_adapters/claude.sh` — dispatcher flags |
| **Research source** | Anthropic Compaction API (compact-2026-01-12, Jan 2026) |
| **Expected saving** | 40–60% cost reduction on long Orchestrator/Builder sessions |
| **LoC delta** | ~5 LoC in dispatcher (add `--compact` flag gated by `EVOLVE_COMPACTION=1`) |
| **Risk** | Low (if flag exists) — native API, zero persona changes; investigation-gated |
| **Target cycle** | 45 (investigation — analogous to P-NEW-17 TTL probe) |
| **Verification** | 1. Probe `claude -p --help` for `--compact` flag (like P-NEW-17 probed `--cache-ttl`); 2. If exists, run one cycle with/without; compare cost |

**Problem:** Orchestrator (48 turns / $1.68, cycle-43) and Builder (69 turns / $3.12, cycle-43) are the two highest-cost phases. Both run long sessions where context accumulates. Anthropic's compaction API (compact-2026-01-12) provides automatic context compaction for long-running agent sessions with 40–60% cost reduction and zero changes to persona prompts.

**Investigation first (cycle 45):** Determine whether `claude -p` exposes a `--compact` flag (or equivalent env var). If Path A is open: wire `EVOLVE_COMPACTION=1` in dispatcher profiles for orchestrator and builder. If Path A is closed: explore SDK-level compaction as Path B.

---

## P-NEW-26 — Per-Role `--effort` Flag Dispatch

| Field | Value |
|-------|-------|
| **Subsystem** | `scripts/cli_adapters/claude.sh` + `.evolve/profiles/*.json` |
| **Research source** | `claude -p --help` — `--effort <level>` flag (low/medium/high/xhigh/max) |
| **Expected saving** | ~$0.66/cycle (~25% on medium-effort phases: scout $1.66 + triage $0.35 + memo $0.64 = $2.65 × 25%) |
| **LoC delta** | ~10 LoC in claude.sh + 1 field per profile (6 profiles) |
| **Risk** | Low — additive flag, omitted when field absent, preserves pre-P-NEW-26 behavior |
| **Target cycle** | 44 (SHIPPED) |
| **Verification** | `grep -q "\-\-effort" scripts/cli_adapters/claude.sh` + all 6 profiles have non-empty `effort_level` field |

**Effort assignments:**
- `medium`: scout (research/discovery), triage (JSON decision), memo (short summary), orchestrator (phase sequencing/shell)
- `high`: builder (multi-file code changes), auditor (verification)

**Implementation:** `EFFORT_LEVEL=$(jq -r '.effort_level // empty' "$PROFILE_PATH")` read in adapter; `--effort "$EFFORT_LEVEL"` appended to CMD only when non-empty. Guard ensures profiles without the field behave identically to pre-P-NEW-26.

**Source:** "Token-Budget-Aware LLM Reasoning" (arXiv:2412.18547v1, 2026): pre-declaring reasoning budget induces self-regulation and stops excessive elaboration. Claude responds to explicit budget declarations. Complementary to gate-based stop criteria.

---

## P-NEW-29 — Parallel Tool-Call Batching via Multi-Tool-Use Pattern

| Field | Value |
|-------|-------|
| **Subsystem** | Agent personas (`agents/evolve-builder.md`, `agents/evolve-scout.md`) — turn-level tool dispatch |
| **Status** | **DONE (cycle 47)** |
| **Research source** | Anthropic Multi-Tool-Use Parallel pattern (Claude API Cookbook 2026); MindStudio "10 MCP Optimization Techniques" (2026) |
| **Expected saving** | 20–40% turn reduction for read-heavy phases; additional ~5–10% per-turn input reduction from reduced schema serialization overhead |
| **LoC delta** | ~28 LoC (persona guidance sections in 2 files — Builder 3 before/after examples + rule; Scout compact guidance) |
| **Risk** | Low — guidance-only change; no pipeline structure change |
| **Shipped cycle** | 47 |
| **Verification** | `grep -q "Parallel Tool-Call Batching" agents/evolve-builder.md agents/evolve-scout.md` |

**Problem:** Scout and Builder agents issue sequential single-tool calls when multiple independent reads could be parallelized. In cycle-45, Builder made 58 turns — a significant portion were sequential `Read` + `Read` + `Bash` calls that have no ordering dependency. Each sequential call adds a round-trip schema serialization overhead (~500–2,000 tokens) and costs a full turn.

**Fix:** Add a "Parallel Tool Use" guidance section to `agents/evolve-builder.md` and `agents/evolve-scout.md` instructing agents to:
1. Identify groups of independent tool calls (multiple `Read` for different files, multiple `Grep` for different patterns).
2. Emit them as a single parallel tool-use block (Claude natively supports multi-tool-use-parallel in a single response).
3. Wait for all results before proceeding — do NOT split into multiple turns.

Example pattern:
```
# SLOW (2 turns):
Read(file_a)  →  result_a
Read(file_b)  →  result_b

# FAST (1 turn):
Read(file_a), Read(file_b)  →  result_a, result_b (parallel)
```

**Scope:** Builder (multi-file reads during codebase analysis phase) and Scout (parallel codebase + research sub-tasks). Not applicable to Auditor (sequential by design) or Orchestrator (shell-heavy, sequential state machine).

**Source:** Anthropic Multi-Tool-Use Parallel documentation (2026): "When multiple tool calls are independent, emit them in a single response for 2–5× turn reduction." MindStudio "10 MCP Optimization Techniques" (2026): Tool-call batching listed as technique #3 with 20–35% latency reduction in production multi-agent benchmarks. Complementary to P-NEW-24 (context compression) and P-NEW-28 (RE-TRAC trajectory summarization).

---

## P-NEW-30 — TACO Terminal Observation Compression

| Field | Value |
|-------|-------|
| **Subsystem** | Builder tool-result stream; subagent-run.sh or builder prompt trajectory |
| **Status** | **PENDING (cycle 48+)** |
| **Research source** | arXiv:2604.19572 (TACO — Self-Evolving Terminal Agent Compression, Apr 2026) [[11]](#sources) |
| **Expected saving** | 30–50% terminal observation token reduction; complements P-NEW-24 (expired tool-result removal) |
| **LoC delta** | TBD — depends on implementation approach (pre-context filter vs. in-context summarization) |
| **Risk** | Low (training-free, model-agnostic) → Medium (requires integration point in trajectory) |
| **Target cycle** | 48+ |
| **Verification** | Compare Builder `input_tokens` per-turn before/after; assert ≥20% reduction on tool-result-heavy builds |

**Problem:** Builder's multi-turn trajectory accumulates raw tool-result observations (Read, Grep, Bash output) that grow quadratically with turn count. P-NEW-24 addresses *expired* results; TACO addresses *noisy* results — tool outputs with irrelevant content that could be compressed at ingestion time without losing task-critical signals.

**TACO framework:** Discovers compression rules from agent trajectories; refines them iteratively. Training-free and plug-and-play — no fine-tuning required. Applies before tool results enter the context window, preventing accumulation rather than retroactively removing it. Directly applicable to evolve-loop Builder's tool-result bloat identified in cycle-46 (95-turn regression).

**Implementation path:**
1. Evaluate `--context-compression` CLI flag availability (check claude.sh `--help` output).
2. If unavailable: implement post-observation summarization hook in Builder persona (prompt-level, similar to Tool-Result Hygiene section in `agents/evolve-builder.md`).
3. TACO compression rules can be seeded from evolve-loop trajectory archives (`.evolve/runs/cycle-*/builder-stdout.log`).

**Source:** arXiv:2604.19572 — TACO: Self-Evolving Terminal Agent Compression (Apr 2026). Token Economics for LLM Agents (arXiv:2605.09104, May 2026) confirms combined orchestration+caching+routing achieves 70–80% total savings; TACO is the observation-compression component of that stack. [[12]](#sources)
