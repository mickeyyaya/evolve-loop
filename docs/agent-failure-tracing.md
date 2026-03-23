# Agent Failure Tracing

> Reference document for debugging and tracing agent failures in multi-agent systems.
> Use these techniques to identify root causes, attribute failures to specific steps,
> and prevent recurrence across evolve-loop cycles.

## Table of Contents

1. [Error Taxonomy](#error-taxonomy)
2. [Tracing Techniques](#tracing-techniques)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Error Taxonomy

Classify every agent failure into one of five categories before attempting a fix.

| Category | Definition | Typical Symptoms | Example |
|---|---|---|---|
| **Planning Error** | Agent selects wrong task, wrong ordering, or infeasible plan | Irrelevant changes, dependency violations, circular task selection | Scout picks a task that conflicts with an in-progress gene |
| **Tool Use Error** | Agent invokes a tool incorrectly — wrong parameters, wrong tool, or missing invocation | Command failures, malformed output, file-not-found errors | Builder calls `Edit` with a non-unique `old_string`, causing a no-op |
| **Reasoning Error** | Agent reaches an incorrect conclusion despite correct inputs | Logically unsound justifications, hallucinated constraints, wrong trade-off analysis | Auditor approves code that violates an explicit rubric criterion |
| **Context Error** | Agent operates on stale, missing, or excessive context | Outdated file references, contradicting prior cycle state, context window overflow | Builder modifies a file that was already refactored in a previous cycle |
| **Integration Error** | Individually correct steps produce incorrect combined result | Tests pass in isolation but fail together, merge conflicts, broken cross-file dependencies | Scout report is valid, Builder output is valid, but combined changes break the build |

### Severity Matrix

| Severity | Detection Timing | Blast Radius | Required Action |
|---|---|---|---|
| **Critical** | Post-ship (production) | Cross-cycle corruption | Immediate rollback, incident report, structural fix |
| **High** | Audit phase | Single cycle failure | Block ship, fix in same cycle |
| **Medium** | Build phase (self-detected) | Single file or function | Retry with corrected approach |
| **Low** | Scout phase | Plan-only, no code impact | Log and adjust task selection |

---

## Tracing Techniques

### Step-Level Attribution

Trace each failure back to the exact agent step that introduced it.

| Technique | How It Works | When to Use |
|---|---|---|
| **Step tagging** | Assign a unique ID to each agent action; reference the ID in error reports | Always — foundational for all other tracing |
| **Input-output diffing** | Compare the input context and output artifact of each step to find the divergence point | When the failure is subtle and multiple steps are suspect |
| **Counterfactual replay** | Re-run a single step with corrected input to confirm it was the root cause | When attribution is ambiguous between two adjacent steps |

### Violation Logs

Record every deviation from expected behavior in a structured format.

| Field | Purpose | Example Value |
|---|---|---|
| `violation_id` | Unique identifier | `v-2024-0142-003` |
| `cycle` | Cycle number where violation occurred | `142` |
| `phase` | Scout, Builder, or Auditor | `Builder` |
| `category` | Error taxonomy category | `Tool Use Error` |
| `description` | Human-readable summary | `Edit tool failed: old_string not unique` |
| `root_cause` | Attributed cause after investigation | `Builder context missing recent file changes` |
| `resolution` | Fix applied | `Added file-read step before edit` |

### Cascading Failure Detection

Identify when a single root cause produces multiple downstream symptoms.

| Signal | Detection Method | Response |
|---|---|---|
| **Multiple failures in one cycle** | Count distinct errors per cycle; flag if > 2 | Trace backward to find shared root cause |
| **Same error across cycles** | Match `category + description` patterns across cycle history | Escalate to structural fix, not point fix |
| **Phase-boundary corruption** | Diff phase handoff artifacts against expected schema | Validate handoff format before next phase starts |

### Trajectory Replay

Reconstruct the full agent decision path to understand failure context.

| Step | Action | Artifact |
|---|---|---|
| 1 | Collect all agent actions from the cycle | Raw action log |
| 2 | Reconstruct the decision tree (which branch was taken at each choice point) | Decision tree diagram |
| 3 | Identify the first divergence from expected behavior | Divergence point annotation |
| 4 | Validate whether the divergence was caused by input or reasoning | Root cause classification |

---

## Mapping to Evolve-Loop

Map each tracing concept to concrete evolve-loop artifacts.

| Tracing Concept | Evolve-Loop Artifact | Role in Tracing |
|---|---|---|
| **Trajectory log** | `experiments.jsonl` | Record of every experiment attempted — serves as the sequential action trace across cycles |
| **Violation log** | `audit-report.md` issues section | Structured list of violations found by Auditor — primary source for post-cycle failure analysis |
| **Root cause database** | `failedApproaches` in genes | Accumulated knowledge of what did not work and why — prevents repeated failures |
| **Event trace** | `ledger.jsonl` | Append-only log of cycle events (task selected, build started, audit verdict) — enables timeline reconstruction |
| **Context snapshot** | `scout-report.md`, `build-report.md` | Phase handoff artifacts — diff these to detect context corruption at phase boundaries |

### Agent-Specific Failure Patterns

| Agent | Common Failure Mode | Tracing Approach | Prevention |
|---|---|---|---|
| **Scout** | Selects task already attempted in `failedApproaches` | Check task ID against `failedApproaches` before selection | Add pre-selection filter in Scout prompt |
| **Scout** | Misestimates task complexity or risk | Compare Scout risk score against actual Auditor verdict | Calibrate risk heuristics using historical accuracy |
| **Builder** | Edits fail due to stale file content in context | Diff Builder's assumed file state against actual file state | Force file re-read immediately before every edit |
| **Builder** | Implementation diverges from scout-report spec | Diff build-report deliverables against scout-report requirements | Add spec-compliance check to Builder post-step |
| **Auditor** | Approves code that fails tests | Verify Auditor ran tests and parsed output correctly | Make test execution a hard prerequisite for audit pass |
| **Auditor** | Flags false positives, blocking valid cycles | Track false-positive rate across cycles; tune rubric thresholds | Add appeal mechanism with evidence requirements |

---

## Implementation Patterns

### Adding Trace Metadata to Build Reports

Embed structured trace data in every `build-report.md` to enable post-hoc analysis.

| Metadata Field | Type | Description |
|---|---|---|
| `trace_id` | `string` | Unique identifier for this build trace |
| `steps` | `array` | Ordered list of actions taken, each with `{action, tool, target, result, duration_ms}` |
| `files_read` | `array` | Files read during the build, with line ranges |
| `files_modified` | `array` | Files created or edited, with diff summary |
| `failed_attempts` | `array` | Actions that failed, with error message and retry count |
| `context_tokens` | `number` | Estimated token count at each major step |

### Automated Root Cause Attribution

Use pattern matching to auto-classify failures without manual investigation.

| Pattern | Matched Category | Confidence | Auto-Resolution |
|---|---|---|---|
| `Edit tool failed: old_string not unique` | Tool Use Error | High | Re-read file, use longer context string |
| `Test failed: expected X, got Y` after correct implementation | Integration Error | Medium | Check for import conflicts, run tests in isolation |
| `File not found` for a path referenced in scout-report | Context Error | High | Validate all file paths in scout-report before build |
| `Audit pass` followed by post-ship regression | Reasoning Error | Medium | Add regression test, expand audit rubric |
| Task in `failedApproaches` selected again | Planning Error | High | Block task, log Scout prompt for review |

### Failure Correlation Across Cycles

Detect systemic issues by correlating failures across multiple cycles.

| Correlation Method | Implementation | Output |
|---|---|---|
| **Category frequency** | Count errors per taxonomy category per 10-cycle window | Trend chart showing category spikes |
| **File hotspot** | Track which files appear in `failed_attempts` most often | Ranked list of fragile files needing refactoring |
| **Agent accuracy** | Compare each agent's predictions against outcomes over time | Per-agent reliability score |
| **Cascade chain length** | Measure average steps between root cause and final symptom | Metric for system coupling — shorter is better |

---

## Prior Art

| Source | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|
| **AgentDebug** (arXiv:2509.25370) | Systematic taxonomy of agent failures with step-level attribution; introduces trajectory-based debugging | Directly informs the error taxonomy and step-level attribution techniques in this document |
| **AgentRx** | Reactive failure recovery framework; agents detect and self-correct failures using violation patterns | Model for automated root cause attribution and the violation log structure |
| **OpenTelemetry** (adapted for agents) | Distributed tracing with spans, traces, and context propagation across service boundaries | Adapt span/trace model: each agent phase is a span, each cycle is a trace, context propagation happens via handoff artifacts |
| **LangSmith / LangFuse** | LLM observability platforms with token tracking, latency measurement, and prompt versioning | Inform the trace metadata schema (token counts, step durations, prompt versions) |
| **Reflexion** (Shinn et al., 2023) | Self-reflective agents that maintain failure memory and use it to improve subsequent attempts | Direct parallel to `failedApproaches` gene — both accumulate failure knowledge to prevent repetition |

---

## Anti-Patterns

Avoid these common mistakes when implementing failure tracing.

| Anti-Pattern | Description | Consequence | Correct Approach |
|---|---|---|---|
| **Silent failure** | Agent encounters an error but continues without logging it | Root cause becomes invisible; downstream failures have no traceable origin | Log every error with category, context, and step ID — even if the agent recovers |
| **Trace overload** | Record every micro-action (each token, each API call) in the trace log | Trace becomes too large to analyze; important signals buried in noise | Trace at the step level (tool invocations, phase transitions), not the token level |
| **Blaming wrong step** | Attribute failure to the step where the symptom appeared, not the step where the root cause originated | Fix addresses symptom, not cause; failure recurs in a different form | Always trace backward from symptom to first divergence point using input-output diffing |
| **Missing context in error logs** | Log the error message but not the input state, agent reasoning, or attempted action | Investigation requires reproducing the exact scenario, which may be impossible | Include input snapshot, agent reasoning summary, and full action parameters in every error log entry |
| **Single-cycle analysis** | Analyze each failure in isolation without checking for cross-cycle patterns | Systemic issues go undetected; same root cause produces repeated point fixes | Run failure correlation analysis every 10 cycles to detect category spikes and file hotspots |
| **Unvalidated handoffs** | Pass artifacts between phases without schema validation | Corrupted or incomplete handoff causes downstream phase to fail unpredictably | Validate every handoff artifact against its expected schema before the next phase begins |
