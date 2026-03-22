# Runtime Guardrails

Pre-execution and runtime enforcement mechanisms that prevent unsafe agent behavior before it reaches the codebase. Based on AgentSpec (arXiv:2503.18666, ICSE 2026) and AEGIS (arXiv:2603.12621).

---

## The Runtime Safety Gap

The evolve-loop's current safety mechanisms operate **after** execution: eval graders verify output, the Auditor reviews changes, checksums detect tampering. None operate **before** or **during** execution. Runtime guardrails fill this gap with pre-execution checks and enforcement hooks.

---

## AgentSpec: Declarative Enforcement DSL

AgentSpec provides a domain-specific language for specifying runtime constraints with trigger/predicate/enforcement semantics. Results: 90%+ unsafe execution prevention at millisecond overhead.

**Mapping to evolve-loop phase-gate system:**

| AgentSpec Concept | Evolve-Loop Equivalent |
|-------------------|----------------------|
| Trigger (event that activates a rule) | Phase boundary (Scout→Builder, Builder→Auditor) |
| Predicate (condition to check) | `scripts/phase-gate.sh` checks |
| Enforcement (action on violation) | HALT, WARN, retry |
| Temporal constraints | Token budget checks, cycle time limits |

**Example phase-gate guardrails (trigger → predicate → enforce):**

| Trigger | Predicate | Enforcement |
|---------|-----------|-------------|
| Builder starts | `git status --porcelain` is clean | HALT if dirty |
| Builder edits file | File is in `filesToModify` list | WARN if out-of-scope |
| Auditor starts | Eval checksums match Phase 1 capture | HALT if tampered |
| Ship phase | Health check passes (11 signals) | HALT on anomaly |

---

## AEGIS: Three-Stage Pre-Execution Pipeline

AEGIS adds a pre-execution firewall with tamper-evident logging. Three stages execute before any agent action:

1. **Intent classification** — Categorize the proposed action (read, write, execute, network)
2. **Policy check** — Verify the action against the agent's allowed scope
3. **Audit trail** — Log the decision with Ed25519 signature for tamper evidence

Results: blocks 48/48 known attack patterns at 1.2% false positive rate.

**Evolve-loop integration opportunity:** The orchestrator already runs `phase-gate.sh` at phase boundaries. AEGIS extends this to **within-phase** checks — monitoring Builder tool calls in real-time rather than only at phase transitions.

---

## Anti-Patterns

| Anti-Pattern | Risk | Mitigation |
|-------------|------|------------|
| Over-restrictive guardrails | Builder cannot complete legitimate tasks | Allow-list by task scope, not deny-list globally |
| Silent enforcement | Agent doesn't know why it was blocked | Include enforcement reason in retry context |
| Post-hoc only | Damage already done when detected | Pre-execution checks at phase boundaries |

---

## Research References

- AgentSpec (arXiv:2503.18666): declarative runtime enforcement DSL, ICSE 2026
- AEGIS (arXiv:2603.12621): three-stage pre-execution firewall with tamper-evident audit
- Safiron (arXiv:2510.09781): pre-execution risk classification (deferred)

See [research-paper-index.md](research-paper-index.md) for the full citation index.
