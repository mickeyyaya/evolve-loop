# Agent Governance and Compliance

> Reference document for governance frameworks, audit trails, and regulatory requirements
> for autonomous agent systems. Apply these patterns to ensure accountability, traceability,
> and regulatory readiness in evolve-loop and similar multi-agent pipelines.

## Table of Contents

1. [Governance Framework](#governance-framework)
2. [Audit Trail Requirements](#audit-trail-requirements)
3. [Regulatory Landscape](#regulatory-landscape)
4. [Control Architecture](#control-architecture)
5. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## Governance Framework

| Accountability Layer | Role | Decision Ownership | Escalation Path |
|---|---|---|---|
| **Model provider** | Supply foundation model, enforce usage policies, publish model cards | Training data curation, safety fine-tuning, capability restrictions | Report unsafe outputs to provider safety team |
| **Deployer** | Configure agent pipeline, set system prompts, define tool access | Architecture decisions, tool allowlisting, context window strategy | Escalate model-level failures to provider |
| **Operator** | Run day-to-day cycles, monitor outputs, intervene on failures | Approve/reject agent outputs, trigger HALT, adjust cycle parameters | Escalate pipeline failures to deployer |
| **Agent (Scout/Builder/Auditor)** | Execute assigned phase, produce artifacts, report results | Task-scoped decisions within phase boundaries | Escalate ambiguity or constraint violations to Operator |

| Principle | Requirement |
|---|---|
| **Separation of duties** | No single agent scores its own work; Auditor is independent of Builder |
| **Least privilege** | Grant each agent only the tools required for its phase |
| **Human-in-the-loop** | Operator retains HALT authority at every phase boundary |
| **Traceability** | Every decision maps to a logged event with timestamp and actor |
| **Proportionality** | Governance overhead scales with risk level of the task |

---

## Audit Trail Requirements

### What to Log

| Event Type | Required Fields | Example |
|---|---|---|
| **Agent decision** | timestamp, agent_role, decision, rationale, cycle_id | Scout selects task from backlog |
| **Tool invocation** | timestamp, tool_name, parameters, result_summary, duration_ms | Builder calls `Edit` on `src/lib.ts` |
| **State change** | timestamp, entity, previous_state, new_state, actor | Phase transitions from `build` to `audit` |
| **Human override** | timestamp, operator_id, action, reason, affected_cycle | Operator issues HALT on cycle 145 |
| **Integrity check** | timestamp, check_type, expected_value, actual_value, pass/fail | Challenge token verification |
| **Error/failure** | timestamp, error_type, stack_trace, recovery_action | Build failure triggers retry |

### Retention Policies

| Data Category | Minimum Retention | Storage Format | Access Control |
|---|---|---|---|
| Agent decisions and tool calls | 90 days | Append-only JSONL | Read: all agents; Write: owning agent only |
| Human overrides and HALTs | 1 year | Append-only JSONL | Read: operator+deployer; Write: operator only |
| Integrity check results | 1 year | Append-only JSONL | Read: all; Write: phase-gate script only |
| Incident reports | 2 years | Markdown in `docs/incidents/` | Read: all; Write: operator only |
| Model outputs (full) | 30 days | Structured logs | Read: deployer; Write: system only |

### Integrity Guarantees

| Guarantee | Implementation |
|---|---|
| **Append-only** | Use append-only file formats (JSONL); reject overwrites |
| **Tamper detection** | Compute checksums over log segments; verify at phase boundaries |
| **Completeness** | Validate that every phase transition has a corresponding log entry |
| **Non-repudiation** | Include agent identity and cycle ID in every log record |

---

## Regulatory Landscape

| Regulation | Jurisdiction | Status | Key Requirements for Agent Systems |
|---|---|---|---|
| **EU AI Act** | European Union | Effective Aug 2024; full enforcement Aug 2026 | Risk classification, transparency obligations, human oversight for high-risk systems, technical documentation, conformity assessments |
| **Colorado AI Act (SB 24-205)** | Colorado, USA | Effective June 2026 | Algorithmic impact assessments, disclosure of AI decision-making, opt-out rights for consumers, annual compliance reports |
| **NIST AI RMF 1.0** | USA (voluntary) | Published Jan 2023 | Govern, Map, Measure, Manage lifecycle; risk tiering; documentation of AI system behaviors and limitations |
| **ISO/IEC 42001** | International | Published Dec 2023 | AI management system requirements, risk assessment, policy controls, continual improvement, internal audit |
| **Executive Order 14110** | USA (federal) | Signed Oct 2023 | Safety testing for dual-use models, red-teaming, reporting of safety incidents, watermarking AI-generated content |
| **OECD AI Principles** | International | Updated May 2024 | Transparency, accountability, robustness, human-centered values, fairness |

### Compliance Mapping

| Requirement Category | EU AI Act | Colorado AI Act | NIST AI RMF | ISO 42001 |
|---|---|---|---|---|
| Risk assessment | Mandatory for high-risk | Algorithmic impact assessment | Map function | Risk assessment clause |
| Human oversight | Required for high-risk | Opt-out rights | Govern function | Management review |
| Audit trail | Technical documentation | Annual compliance reports | Measure function | Internal audit |
| Transparency | Disclosure to users | Disclosure of AI use | Map function | Communication clause |
| Incident reporting | Serious incident notification | Not explicit | Manage function | Nonconformity handling |

---

## Control Architecture

| Control Layer | Type | Enforcement | Bypass Risk |
|---|---|---|---|
| **System prompts** | Model-layer | Probabilistic; depends on model adherence | High — prompt injection, context overflow, model drift |
| **Phase-gate scripts** | Data-layer | Deterministic; runs as shell process | Low — requires filesystem access to circumvent |
| **Checksums/hashes** | Data-layer | Deterministic; cryptographic verification | Very low — requires hash collision or log tampering |
| **Append-only ledger** | Data-layer | Deterministic; file-append semantics | Low — requires filesystem-level attack |
| **Challenge tokens** | Data-layer | Deterministic; random token injected and verified | Low — token must match exactly |
| **Operator HALT** | Human-layer | Absolute; human override stops pipeline | None — highest authority |

### Key Principle

> System prompts are NOT compliance controls. Treat them as guidelines that
> improve behavior but can fail. Build deterministic, data-layer controls as
> the compliance boundary. Use system prompts for defense-in-depth only.

| Control Strategy | Use For | Do NOT Use For |
|---|---|---|
| System prompts | Behavioral guidance, output formatting, tone | Access control, safety-critical gates, audit integrity |
| Phase-gate scripts | Phase transitions, prerequisite checks, artifact validation | Nuanced judgment calls, creative decisions |
| Checksums | Artifact integrity, tamper detection | Semantic validation of content quality |
| Human oversight | Ambiguous decisions, high-risk actions, incident response | Routine low-risk operations |

---

## Mapping to Evolve-Loop

| Evolve-Loop Component | Governance Function | Regulatory Alignment |
|---|---|---|
| **ledger.jsonl** | Audit trail — append-only log of every cycle, phase, decision, and outcome | EU AI Act technical documentation; NIST Measure function; ISO 42001 internal audit |
| **phase-gate.sh** | Control gate — deterministic script that validates prerequisites before phase transitions | EU AI Act human oversight; NIST Govern function; ISO 42001 operational control |
| **Challenge tokens** | Integrity verification — random tokens injected into agent context and verified at phase boundaries | EU AI Act robustness; NIST Manage function; ISO 42001 risk treatment |
| **Operator HALT** | Human escalation — operator can stop any cycle at any phase boundary | EU AI Act human oversight; Colorado AI Act opt-out; NIST Govern function |
| **operatorWarnings** | Incident log — structured warnings emitted when anomalies or violations are detected | EU AI Act incident reporting; NIST Manage function; ISO 42001 nonconformity |
| **Scout agent** | Risk identification — scans codebase and backlog to identify highest-value tasks | NIST Map function; ISO 42001 risk assessment |
| **Builder agent** | Execution under constraint — implements changes within phase-gate boundaries | ISO 42001 operational control; NIST Manage function |
| **Auditor agent** | Independent verification — reviews Builder output without access to Builder rationale | EU AI Act conformity assessment; ISO 42001 internal audit; NIST Measure function |

### Compliance Checklist for Each Cycle

| Step | Verification | Owner |
|---|---|---|
| Pre-cycle | Confirm ledger.jsonl is writable and append-only | phase-gate.sh |
| Scout phase | Log task selection rationale to ledger | Scout |
| Scout-to-Build gate | Verify scout-report.md exists and challenge token matches | phase-gate.sh |
| Build phase | Log every tool invocation and file change to ledger | Builder |
| Build-to-Audit gate | Verify build-report.md exists and checksums match | phase-gate.sh |
| Audit phase | Log audit findings independently; no access to Builder rationale | Auditor |
| Audit-to-Ship gate | Verify audit-report.md exists and all evals pass | phase-gate.sh |
| Post-cycle | Append final cycle summary to ledger; emit operatorWarnings if anomalies found | Orchestrator |

---

## Prior Art

| Source | Contribution | Reference |
|---|---|---|
| **Kiteworks** | Governance framework for AI data handling; emphasizes data provenance and access control in agent pipelines | Kiteworks AI Governance Framework (2024) |
| **ISACA** | Guidelines for auditing AI systems; risk-based approach to AI governance; control objectives for autonomous systems | ISACA AI Audit Program (2024) |
| **Microsoft Responsible AI** | Six principles (fairness, reliability, safety, privacy, inclusiveness, transparency); HAX toolkit for human-AI interaction | Microsoft Responsible AI Standard v2 (2024) |
| **NIST AI RMF** | Four-function framework (Govern, Map, Measure, Manage); AI risk taxonomy; profiles for generative AI | NIST AI 100-1 (2023), NIST AI 600-1 (2024) |
| **Anthropic RSP** | Responsible Scaling Policy; capability evaluations before deployment; commitment to safety levels | Anthropic RSP v1.1 (2024) |
| **OpenAI Preparedness** | Preparedness framework; risk scoring for model capabilities; deployment thresholds | OpenAI Preparedness Framework (2023) |
| **EU AI Office** | Codes of practice for general-purpose AI; guidance on systemic risk assessment | EU AI Office GPAI Code of Practice (2025) |

---

## Anti-Patterns

| Anti-Pattern | Risk | Mitigation |
|---|---|---|
| **No audit trail** | Cannot reconstruct what happened during a cycle; no evidence for regulators | Enforce append-only ledger.jsonl logging for every phase and decision |
| **Trusting model-layer controls alone** | System prompts are probabilistic and bypassable; false sense of security | Layer deterministic data-layer controls (phase-gate, checksums) beneath model-layer guidance |
| **Missing human escalation** | Agent pipeline runs unsupervised; errors compound across cycles | Implement Operator HALT at every phase boundary; emit operatorWarnings on anomaly detection |
| **Compliance theater** | Governance artifacts exist but are never verified or enforced | Run phase-gate.sh at every transition; validate artifacts contain substantive content, not boilerplate |
| **Single agent self-assessment** | Builder grades its own work; no independent verification of quality or correctness | Separate Auditor agent with no access to Builder rationale; cross-validate scores |
| **Immutable-in-name-only logs** | Ledger file exists but agents can overwrite or truncate it | Enforce file-append permissions; checksum log segments at phase boundaries |
| **Retroactive justification** | Agent generates rationale after the fact to match a desired outcome | Log decisions and rationale at decision time, before outcomes are known |
| **Governance without teeth** | Policies exist but violations have no consequences; agents learn to ignore constraints | Fail the phase gate on violations; block cycle progression until issues are resolved |
