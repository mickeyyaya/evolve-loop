# HITL Trust Calibration

> Reference document for human-in-the-loop patterns and trust calibration in
> agent systems. Use these models to determine when agents operate autonomously,
> when they escalate to humans, and how trust graduates over time within the
> evolve-loop Scout, Builder, and Auditor pipeline.

## Table of Contents

1. [Trust Calibration Model](#trust-calibration-model)
2. [HITL Handoff Patterns](#hitl-handoff-patterns)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Trust Calibration Model

Assign a confidence score (0-100%) to every agent action. Map the score to an
operating mode using the thresholds below.

### Confidence Threshold Table

| Confidence Range | Operating Mode | Human Role | Agent Behavior | Example Scenario |
|---|---|---|---|---|
| **0-50%** | Human Required | Direct control; human performs the action | Pause execution, present context, wait for human decision | Novel architecture choice with no prior art in codebase |
| **50-80%** | Human Review | Approve or reject agent proposal before execution | Generate proposal with rationale, block until reviewed | Refactor touches 5+ files; agent drafts PR for human approval |
| **80-95%** | Autonomous with Logging | Async review; human monitors logs post-hoc | Execute action, emit structured log, flag for batch review | Routine bug fix matching a known pattern |
| **95-100%** | Fully Autonomous | None required; human audits periodically | Execute immediately, log minimally | Formatting fix, dependency version bump |

### Confidence Scoring Factors

| Factor | Weight | High Confidence Signal | Low Confidence Signal |
|---|---|---|---|
| Pattern match to prior successful actions | 30% | Exact match in action history | No prior match |
| Test coverage of affected code | 25% | 90%+ coverage, all passing | No tests or failing tests |
| Scope of change (files, lines) | 20% | Single file, < 50 lines | 10+ files, 500+ lines |
| Reversibility | 15% | Easily reverted (git revert) | Irreversible (data migration, public API) |
| Domain risk | 10% | Internal tooling | Security, auth, payments |

---

## HITL Handoff Patterns

Define four handoff patterns. Select the pattern based on who initiates the
transition and when it occurs.

| Pattern | Trigger | Initiator | Timing | Mechanism | Use Case |
|---|---|---|---|---|---|
| **Proactive Handoff** | Agent recognizes uncertainty | Agent | Before action | Agent emits `NEEDS_REVIEW` signal with context bundle | Builder confidence drops below 50% mid-implementation |
| **Reactive Handoff** | Human observes problem | Human | During action | Operator sends `HALT` command; agent checkpoints state | Human spots incorrect approach in build-report |
| **Scheduled Handoff** | Predetermined review cadence | System | Periodic | Timer or cycle-count gate triggers mandatory review | Every 10 evolve-loop cycles, human reviews trajectory |
| **Invisible Handoff** | Seamless agent-human collaboration | Either | Continuous | Shared workspace; human edits alongside agent | Human tweaks a gene file while Builder runs next cycle |

### Handoff State Machine

| Current State | Event | Next State | Action |
|---|---|---|---|
| Autonomous | Confidence < 50% | Blocked (Human Required) | Emit context bundle, pause pipeline |
| Autonomous | Confidence 50-80% | Pending Review | Queue proposal, continue non-dependent work |
| Pending Review | Human approves | Autonomous | Resume with approved plan |
| Pending Review | Human rejects | Blocked (Human Required) | Present rejection reason, request new approach |
| Blocked (Human Required) | Human provides directive | Autonomous | Execute directive, recalibrate confidence |
| Any | Operator HALT | Blocked (Human Required) | Checkpoint state, stop all agents |

---

## Mapping to Evolve-Loop

Map trust calibration concepts to existing evolve-loop mechanisms.

### Mechanism Mapping

| Evolve-Loop Mechanism | Trust Calibration Role | Confidence Effect | Configuration |
|---|---|---|---|
| `warnAfterCycles` | Scheduled handoff trust gate | Forces human review after N cycles | Set in `config/loop-config.yaml` |
| Operator `HALT` | Reactive handoff trigger | Drops confidence to 0%, blocks all agents | Manual command during active loop |
| Mastery levels (Novice-Master) | Trust graduation ladder | Higher mastery = higher base confidence | Earned through successful cycle completions |
| `bypass-permissions` | Trust override | Sets operating mode to Fully Autonomous | CLI flag; skips interactive prompts, not integrity checks |
| Phase gate (`phase-gate.sh`) | Trust verification checkpoint | Blocks advancement if gate fails regardless of confidence | Runs at every Scout-Builder-Auditor transition |
| Audit score | Confidence feedback signal | High audit score raises next-cycle confidence | Auditor agent output in `audit-report.md` |

### Agent-Specific Trust Profiles

| Agent | Default Confidence Floor | Escalation Target | Trust Graduation Criteria |
|---|---|---|---|
| **Scout** | 70% (read-only, low risk) | Human reviewer | 10 consecutive accurate scout-reports |
| **Builder** | 40% (write actions, high risk) | Human developer | 5 consecutive cycles with audit score >= 8 |
| **Auditor** | 60% (evaluation, medium risk) | Human auditor | 15 consecutive cycles with no false positives |

### Mastery-to-Confidence Mapping

| Mastery Level | Base Confidence Bonus | Max Autonomous Scope | Review Cadence |
|---|---|---|---|
| Novice (0-10 cycles) | +0% | Single file changes | Every cycle |
| Apprentice (11-30 cycles) | +10% | Multi-file within module | Every 3 cycles |
| Journeyman (31-60 cycles) | +20% | Cross-module changes | Every 5 cycles |
| Expert (61-100 cycles) | +30% | Architectural changes | Every 10 cycles |
| Master (100+ cycles) | +40% | Full system scope | Every 20 cycles |

---

## Implementation Patterns

### Confidence Scoring in Build Reports

Embed confidence metadata in every `build-report.md` output.

| Field | Type | Description | Example |
|---|---|---|---|
| `confidence_score` | float (0-1) | Overall action confidence | `0.82` |
| `confidence_factors` | object | Per-factor breakdown | `{ pattern_match: 0.9, test_coverage: 0.75, ... }` |
| `operating_mode` | enum | Derived from threshold table | `autonomous_with_logging` |
| `escalation_needed` | boolean | True if confidence < handoff threshold | `false` |
| `escalation_reason` | string | null | Why escalation triggered | `null` |

### Escalation Protocol

Follow this sequence when confidence drops below the handoff threshold.

| Step | Action | Owner | Timeout | Fallback |
|---|---|---|---|---|
| 1 | Emit `NEEDS_REVIEW` with context bundle | Agent | Immediate | N/A |
| 2 | Notify human via configured channel | System | 5 minutes | Retry notification |
| 3 | Block dependent pipeline stages | Agent | Until resolved | N/A |
| 4 | Continue independent non-blocked work | Agent | N/A | N/A |
| 5 | Human provides decision | Human | 24 hours | Auto-escalate to HALT |
| 6 | Agent resumes with updated confidence | Agent | Immediate | N/A |

### Human Feedback Integration

Capture structured feedback to improve future confidence calibration.

| Feedback Type | Capture Point | Data Collected | Calibration Effect |
|---|---|---|---|
| Approval/Rejection | Pending Review state | Binary decision + optional comment | Adjust pattern-match weights |
| Correction | Post-execution review | Diff between agent output and human fix | Lower confidence for similar patterns |
| Endorsement | Periodic audit | Human confirms agent trajectory | Raise base confidence for agent |
| Override | Any state | Human overrides agent decision | Flag pattern for retraining |

---

## Prior Art

### Research and Industry References

| Source | Year | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|---|
| Mosqueira-Rey et al., "Human-in-the-loop ML: a taxonomy and tutorial" | 2023 | HITL taxonomy across active learning, interactive ML, and ML teaching | Framework for classifying Scout/Builder/Auditor handoff patterns |
| Wu et al., "AutoGen: Enabling Next-Gen LLM Applications" | 2023 | Multi-agent conversation patterns with human proxy agents | Validates invisible handoff pattern for agent-human collaboration |
| SAE J3016 Levels of Driving Automation | 2021 | 6-level autonomy scale (0=manual to 5=full automation) | Direct analog: map driving levels to agent confidence thresholds |
| Parasuraman et al., "Humans and Automation: Use, Misuse, Disuse, Abuse" | 2024 (updated) | Trust calibration framework for automation reliance | Informs anti-patterns: over-trust, under-trust, misuse |
| OpenAI Preparedness Framework | 2025 | Risk-tiered autonomy with human oversight gates | Validates phase-gate approach for high-risk agent actions |
| Anthropic RSP (Responsible Scaling Policy) | 2025 | Commitment levels tied to capability evaluations | Maps to mastery-level trust graduation |
| Google DeepMind RLHF with Constitutional AI | 2025 | Structured human feedback for model alignment | Informs human feedback integration patterns |

### Autonomous Vehicle Trust Analogy

| SAE Level | Driving Equivalent | Agent Equivalent | Human Role |
|---|---|---|---|
| Level 0 | No automation | Manual coding, no agents | Human does everything |
| Level 1 | Driver assistance | Agent suggests, human executes | Human performs all actions |
| Level 2 | Partial automation | Agent executes with constant oversight | Human monitors every action |
| Level 3 | Conditional automation | Autonomous with logging (80-95%) | Human available for escalation |
| Level 4 | High automation | Fully autonomous within domain (95%+) | Human handles edge cases only |
| Level 5 | Full automation | Fully autonomous, all domains | Human audits periodically |

---

## Anti-Patterns

Avoid these failure modes when implementing trust calibration.

| Anti-Pattern | Description | Symptom | Mitigation |
|---|---|---|---|
| **Over-Trusting Agents** | Set confidence thresholds too high; skip human review for risky actions | Silent failures accumulate; agent drifts from intent | Enforce phase-gate.sh at every transition; require audit scores >= 7 |
| **Alert Fatigue** | Escalate too frequently; human ignores notifications | Approval latency increases; human rubber-stamps | Tune confidence thresholds upward as trust graduates; batch low-priority reviews |
| **Rubber-Stamp Approvals** | Human approves without reviewing context bundle | Bad agent decisions pass review gates unchallenged | Require structured approval (checklist, not single click); rotate reviewers |
| **Missing Escalation Paths** | No mechanism for agent to signal uncertainty | Agent proceeds with low confidence; failures increase | Implement proactive handoff with `NEEDS_REVIEW` signal in all agents |
| **Trust Ceiling** | Never graduate trust upward; agent stays in human-review mode permanently | Throughput stays low; human becomes bottleneck | Use mastery levels; auto-graduate after N consecutive successes |
| **Trust Floor Absence** | No minimum confidence enforced; agent claims 100% on everything | Overconfident actions bypass safety checks | Set per-agent confidence floors; cap maximum confidence below 100% |
| **Handoff Context Loss** | Agent escalates without sufficient context for human decision | Human cannot act on escalation; round-trips multiply | Mandate context bundles: affected files, rationale, alternatives, risk assessment |
| **Feedback Loop Neglect** | Collect human feedback but never use it to recalibrate | Trust thresholds remain static; same mistakes repeat | Pipe approval/rejection data into confidence scoring weights each cycle |
