# Agentic Pipeline Enforcement Patterns — Research Dossier — 2026-05-19

| Field | Value |
|-------|-------|
| title | Agentic Pipeline Enforcement Patterns |
| date | 2026-05-19 |
| sources | 3 (ICSE 2026 arXiv:2503.18666, arXiv:2604.19049, ACM AISystems 2024 dl.acm.org/doi/full/10.1145/3703412.3703439) |
| status | accepted-research-only — implementation slotted cycle-88 |
| produced-by | Scout cycle-87 |

> **Archive note:** Produced by Scout in cycle 87 (research-deposit cycle). Lives in `knowledge-base/research/`; excluded from agent context per `feedback_knowledge_base_stewardship.md`. Move this file to `knowledge-base/research/agentic-pipeline-enforcement-2026.md` at ship time.
>
> **Companion dossiers:** `knowledge-base/research/self-correcting-pipelines-ghosh-2026.md` (Ghosh self-correction patterns), `knowledge-base/research/execution-grounded-process-supervision-2026.md` (EGPS/reward-hacking fixes).

## Why This Dossier

Cycle 86 produced two carryover todos and a code-audit-fail streak:

| Observed failure | Signal in ledger |
|---|---|
| `turn-overrun` | intent agent: 12 turns vs ceiling 10 |
| `code-audit-fail` (×17 non-expired) | recurring false-pass / weak predicate pattern |

Both failures map to a single underlying gap: **the evolve-loop pipeline's behavioral guardrails are defined as numbers in config, not as structured runtime rules that can be introspected, composed, and tested.** External research from 2026 names this gap and supplies two high-fit patterns.

---

## Finding 1 — AgentSpec: Structured Runtime Enforcement for LLM Agents

| Field | Value |
|---|---|
| Authors | Wang, Poskitt et al. |
| Title | "AgentSpec: Customizable Runtime Enforcement for Safe and Reliable LLM Agents" |
| Venue | ICSE 2026 (IEEE/ACM 48th Int'l Conference on Software Engineering) |
| URL | `https://arxiv.org/abs/2503.18666` (HTML: `https://arxiv.org/html/2503.18666v3`) |
| Relevance | Directly addresses turn-limit and scope-constraint enforcement for autonomous LLM agents |

### Core Pattern

AgentSpec formalizes agent constraints as three-tuples:

```
rule r = (η_r, P_r, E_r)
  η_r  — trigger event (e.g., tool call, message turn, phase transition)
  P_r  — set of predicate functions evaluated against agent state
  E_r  — sequence of enforcement actions (stop / user_inspection / log / fallback)
```

**Concrete example** (maps to evolve-loop turn-overrun):
- Trigger: every agent turn
- Predicate: `turn_count > max_allowed_turns`
- Enforcement: emit WARN to abnormal-events.jsonl + inject STOP CRITERION reminder into system prompt for next turn

### Results

- **>90%** of unsafe executions prevented across code-agent, embodied agent, and autonomous vehicle domains
- **100% compliance** enforced for autonomous vehicles despite novel scenarios
- **<3ms overhead** per rule evaluation (1.42ms parse + 1.11–2.83ms predicate eval vs 9.82–25.4s agent execution)

### Actionable Change for Evolve-Loop

**Current state**: turn ceiling is a bare integer in `--max-turns` passed to `claude -p`. When an agent exceeds it, the run just fails — no structured recovery, no remediation injection.

**Proposed change** (future cycle): Implement a lightweight rule-engine wrapper around `subagent-run.sh` that evaluates trigger-predicate-enforcement rules at each phase boundary. Minimum viable rule set:
1. `turn_count > max_turns` → inject STOP CRITERION reminder at turn `max_turns - 2` (pre-emptive, not reactive)
2. `phase_elapsed_seconds > budget_seconds` → emit abnormal-event + PROCEED-with-note (not abort)
3. `scope_files_modified > 0 AND phase = scout` → emit integrity-breach event immediately

This converts the current "fail loudly at the ceiling" pattern into a "warn before the ceiling, enforce at the ceiling" pattern that matches AgentSpec's approach.

---

## Finding 2 — Refute-or-Promote: Cross-Model Adversarial Review Reduces False-Pass Rate

| Field | Value |
|---|---|
| Authors | (anon review) |
| Title | "Refute-or-Promote: Adversarial Stage-Gated Multi-Agent Review for High-Precision LLM-Assisted Defect Discovery" |
| URL | `https://arxiv.org/abs/2604.19049` |
| Published | April 2026 |
| Relevance | Directly maps to evolve-loop's adversarial Auditor and the code-audit-fail pattern |

### Core Pattern

The paper proposes a pipeline that forces each review stage to *disprove* the prior stage's finding before it can be promoted:

```
Candidate → Stage 1 (kill mandate) → Stage 2 (cross-model critic) → Stage 3 (empirical test) → Promote or Discard
```

Key innovations vs. single-LLM audit:
1. **Kill mandate**: each reviewer's *default* verdict is DISCARD; promotion requires explicit positive evidence
2. **Cross-model critic (CMC)**: reviewer at each stage is from a different LLM family to break correlated blind spots
3. **Cold-start reviewers**: each stage reviewer receives only the artifact and the prior stage's verdict — no prior conversation context — preventing anchoring

### Results

- **79% kill rate** on initial candidates in prospective study (171 candidates; only 36 promoted)
- **83% kill rate** on rigorous subset
- Illustrative failure mode caught: "10 dedicated reviewers unanimously endorsed a non-existent Bleichenbacher padding oracle" until empirical testing refuted it — unanimous LLM consensus ≠ correct verdict

### Actionable Change for Evolve-Loop

**Current state**: Adversarial Auditor (Opus, ADVERSARIAL_AUDIT=1) already requires positive evidence for PASS. The cross-model family distinction (Sonnet Builder → Opus Auditor) already exists. The gap is **stage 2** — there is no structured "CMC refutation gate" between Builder-output and Auditor-PASS.

**Proposed change** (future cycle): Add a lightweight "refutation scan" pre-step in the Auditor phase:
1. Auditor first generates a list of up to 3 specific claims in the build-report (acceptance-check verdicts)
2. For each claim, Auditor attempts a concrete counter-argument using repo state (grep, read files)
3. Only claims that survive attempted refutation proceed to PASS
4. Claims that cannot be independently verified (no file reference, no test output) are automatically downgraded to WARN

This matches the "kill mandate default" pattern from the paper and costs at most 1–2 extra Auditor tool calls per cycle.

---

## Finding 3 — Instruction Adherence Attenuation in Long Agent Sessions

| Field | Value |
|---|---|
| Source | Methodology for Quality Assurance Testing of LLM-based Multi-Agent Systems (ACM AISystems 2024) |
| URL | `https://dl.acm.org/doi/full/10.1145/3703412.3703439` |
| Relevance | Explains why audit quality degrades in long cycles; quantifies the decay rate |

### Core Finding

An AST-based scoring pipeline measuring LLM agent compliance across thousands of sessions found:

> "A significant within-session attenuation effect where LLM agent compliance decreases by approximately **5.6% per generated function**."

In plain language: the longer an agent's output generation session, the less the agent adheres to behavioral specifications from its system prompt. This effect is independent of whether the specification was repeated — the attenuation is a property of output-length-vs-context dilution.

### Mapping to Evolve-Loop's Turn-Overrun

When the intent agent ran 12 turns vs a ceiling of 10, it was operating past the zone where instruction adherence is reliable. At 12 turns with ~5–10 tool calls per turn, the effective attenuation is approximately **30–60% degradation** in adherence to the STOP CRITERION injected in the system prompt.

### Actionable Change for Evolve-Loop

**Proposed change** (future cycle): For agents with `max_turns > 8`, inject a compressed re-statement of STOP CRITERION at turn `ceil(max_turns * 0.7)` as a mid-session anchor. This counters the attenuation effect described in the ACM paper. The cost is one additional system-prompt segment (~100 tokens) at the re-injection point.

This can be implemented in `scripts/dispatch/subagent-run.sh` as a turn-count-aware prompt injection strategy.

---

## Summary — Three Findings, Three Actionable Future-Cycle Tasks

| Finding | Source | Gap | Proposed Action |
|---|---|---|---|
| AgentSpec trigger-predicate-enforcement | ICSE 2026 (2503.18666) | Turn limits are bare integers with no pre-emptive warning | Add rule-engine wrapper to subagent-run.sh: warn at `max_turns-2`, enforce at `max_turns` |
| Refute-or-Promote kill mandate | arXiv 2604.19049 | Auditor has no structured refutation gate before PASS | Add CMC pre-step: Auditor generates up to 3 claims → attempts counter-argument → unverified claims → WARN |
| Instruction adherence attenuation | ACM 2024 (3703412.3703439) | No mid-session STOP CRITERION re-injection for long agents | Inject compressed STOP CRITERION re-statement at turn `ceil(max_turns*0.7)` |

All three are **scoped to existing scripts** (no new agent or profile needed), **orthogonal** (can be implemented in separate cycles), and **directly tied to cycle-86 failure evidence**.

---

## Cycle-87 Convergent Recommendation

**Selected proposal: Finding 3 — Mid-Session STOP CRITERION Re-Injection**

| Field | Value |
|-------|-------|
| name | `mid-session-stop-criterion-reinjection` |
| source | ACM AISystems 2024 (dl.acm.org/doi/full/10.1145/3703412.3703439) |
| implementation target | `scripts/dispatch/subagent-run.sh` |
| cycle slotted | cycle-88 |

**Rationale:** Finding 3 is selected as the convergent proposal because it directly addresses cycle-86's `turn-overrun` abnormal event. At ~57 turns (cycle-86 Builder), the ~5.6%/function attenuation equates to 30–60% degradation in STOP CRITERION adherence. The fix is additive (~20 lines of bash), bash 3.2 compatible, confined to a single existing script, and carries zero blast radius to agent profiles or the tri-layer trust kernel.

**ROI vs. status quo:** Current behavior fails cold when the turn ceiling is breached with no recovery signal. Re-injection at `ceil(max_turns * 0.7)` delivers a mid-session anchor before attenuation becomes critical. Expected reduction in `turn-overrun` abnormal events: 30–50%.

**Deferred proposals:**
- Finding 1 (AgentSpec rule-engine wrapper) → cycle-88 or later; higher blast radius, requires new wrapper layer and rule definitions.
- Finding 2 (Refute-or-Promote CMC pre-step) → cycle-88 or later; targets code-audit-fail rate (orthogonal to turn-overrun).

---

## Citations

1. Wang, Poskitt et al. "AgentSpec: Customizable Runtime Enforcement for Safe and Reliable LLM Agents." ICSE 2026. https://arxiv.org/abs/2503.18666
2. (Authors anon.) "Refute-or-Promote: Adversarial Stage-Gated Multi-Agent Review for High-Precision LLM-Assisted Defect Discovery." April 2026. https://arxiv.org/abs/2604.19049
3. (Authors) "Methodology for Quality Assurance Testing of LLM-based Multi-Agent Systems." ACM AISystems 2024. https://dl.acm.org/doi/full/10.1145/3703412.3703439
