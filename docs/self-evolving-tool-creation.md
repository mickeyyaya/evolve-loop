# Self-Evolving Tool Creation

> Reference document for agents that detect capability gaps and create their own
> tools at runtime. Covers the full tool lifecycle from need detection through
> retirement, mapped to evolve-loop's synthesizedTools infrastructure.

## Table of Contents

1. [Tool Lifecycle](#tool-lifecycle)
2. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
3. [Implementation Patterns](#implementation-patterns)
4. [Prior Art](#prior-art)
5. [Anti-Patterns](#anti-patterns)

---

## Tool Lifecycle

| Phase | Action | Owner | Gate Criteria | Output |
|---|---|---|---|---|
| **Detect Need** | Identify a repeated manual step or capability gap during task execution | Scout, Builder | Gap appears in 2+ cycles or blocks a build step | Capability gap entry in scout-report.md |
| **Generate** | Write a reusable script or function to `.evolve/tools/<tool-name>.sh` | Builder | Script has usage comment, input validation, error handling | Tool file with executable permissions |
| **Validate** | Run tool against test inputs; verify idempotency and error paths | Builder, Auditor | All test inputs produce expected output; no side effects on failure | Validation log in build-report.md |
| **Register** | Add tool metadata to `state.json` under `synthesizedTools` | Builder | Validation passed; no duplicate tool name | `synthesizedTools` entry with `name`, `path`, `purpose`, `cycle`, `useCount` |
| **Version** | Update tool when requirements change; increment in-place or create v2 | Builder | Existing tool fails new use case; backward compatibility preserved | Updated tool file; `useCount` reset if interface changed |
| **Retire** | Remove tool when `useCount` stays 0 for 10+ cycles or superseded | Meta-cycle (Phase 6) | Zero usage confirmed; no downstream dependencies | Tool file deleted; `synthesizedTools` entry removed |

---

## Mapping to Evolve-Loop

| Evolve-Loop Component | Role in Tool Creation | Details |
|---|---|---|
| **Scout** | Detect capability gaps | Flag repeated manual steps or missing automation in scout-report.md |
| **Builder** | Generate, validate, register tools | Write tool to `.evolve/tools/`, run validation, register in `state.json.synthesizedTools` |
| **Auditor** | Verify tool quality | Check tool meets validation protocol in audit-report.md Section D |
| **Phase 5 (Learn)** | Track tool effectiveness | Aggregate `useCount` trends; flag underused tools for retirement |
| **Phase 6 (Meta-cycle)** | Graduate or retire tools | Promote high-use tools to skills/genes; retire zero-use tools |
| **`state.json.synthesizedTools`** | Tool registry | Array of `{name, path, purpose, cycle, useCount}` entries |
| **`.evolve/tools/`** | Tool storage | Directory containing generated tool scripts |
| **Ledger** | Audit trail | `tool-synthesis` entries log creation, updates, and retirement |

### synthesizedTools Schema

```json
{
  "synthesizedTools": [
    {
      "name": "check-eval-coverage",
      "path": ".evolve/tools/check-eval-coverage.sh",
      "purpose": "Verify all tasks have at least one eval grader",
      "cycle": 45,
      "useCount": 12
    }
  ]
}
```

---

## Implementation Patterns

### Tool Template Structure

Every generated tool must follow this structure:

| Section | Requirement | Example |
|---|---|---|
| **Shebang** | Use `#!/usr/bin/env bash` | First line of file |
| **Usage comment** | Describe purpose, inputs, outputs | `# Usage: check-eval-coverage.sh <task-file>` |
| **Input validation** | Validate all arguments; fail fast with clear message | `[[ -z "$1" ]] && echo "Error: missing task file" && exit 1` |
| **Core logic** | Single responsibility; under 50 lines | Main processing block |
| **Error handling** | Trap errors; return nonzero on failure | `set -euo pipefail` |
| **Output format** | Structured output (JSON or key=value) for downstream parsing | `echo '{"status":"pass","count":5}'` |

### Validation Protocol

| Step | Action | Pass Criteria |
|---|---|---|
| 1 | Run with valid inputs | Exit code 0; expected output |
| 2 | Run with missing arguments | Exit code nonzero; usage message printed |
| 3 | Run with malformed input | Exit code nonzero; error message (no stack trace) |
| 4 | Run twice in sequence | Identical output (idempotency check) |
| 5 | Check file permissions | Executable bit set |

### Registration in state.json

| Step | Action |
|---|---|
| 1 | Verify no existing tool with same `name` in `synthesizedTools` |
| 2 | Add entry: `{name, path, purpose, cycle: currentCycle, useCount: 0}` |
| 3 | Log `tool-synthesis` event to ledger with tool name, purpose, and cycle |
| 4 | Update Builder report with tool registration confirmation |

---

## Prior Art

| Source | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|
| **arXiv:2508.07407** — Self-Evolving Agents Survey | Taxonomy of self-evolving agent capabilities: self-reflection, self-training, self-tool-creation | Provides theoretical framework; evolve-loop implements the self-tool-creation axis via synthesizedTools |
| **LATM** (LLMs as Tool Makers) — Cai et al., 2023 | Two-phase pattern: tool-maker LLM creates tools, tool-user LLM applies them; amortizes cost over reuse | Maps to Builder (maker) creating tools that Scout/Builder (user) invoke in later cycles |
| **Gorilla** — Patil et al., 2023 | Fine-tuned LLM for accurate API call generation; retrieval-augmented tool use | Informs tool discovery phase: search existing tools/APIs before generating new ones |
| **ToolACE** — Liu et al., 2024 | Automated tool-calling training data generation; self-evolution of tool proficiency | Validates the validate-then-register pattern; quality gates prevent tool hallucination |
| **Voyager** — Wang et al., 2023 | Minecraft agent that writes and stores reusable skill functions in a library | Direct analog: skill library = `.evolve/tools/`; curriculum = evolve-loop task selection |
| **CREATOR** — Qian et al., 2023 | Four-stage tool creation: abstraction, decision, implementation, validation | Maps to Detect → Generate → Validate → Register lifecycle |

---

## Anti-Patterns

| Anti-Pattern | Description | Symptom | Mitigation |
|---|---|---|---|
| **Tool Sprawl** | Generating a new tool for every minor task variation | `synthesizedTools` array grows past 20 entries; most have `useCount < 3` | Enforce minimum reuse threshold (3+ uses predicted) before generation; retire at 10 cycles unused |
| **Untested Tools** | Registering tools that skip validation protocol | Tools fail silently in production cycles; Auditor flags regressions | Gate registration behind validation protocol completion; Auditor checks validation log |
| **Tool Duplication** | Creating a tool that overlaps with an existing tool or built-in command | Multiple tools with similar `purpose` strings; redundant code | Search `synthesizedTools` and system tools before generating; Builder must document uniqueness |
| **Over-Generalization** | Building a complex multi-purpose tool instead of focused single-purpose tools | Tool has 5+ flags; usage comment exceeds 10 lines; core logic exceeds 50 lines | Enforce single-responsibility; split into composable tools if scope grows |
| **Phantom Registration** | Adding tool to `synthesizedTools` without creating the actual tool file | Registry entry exists but `path` points to nonexistent file | Auditor validates file existence during audit; phase-gate checks file presence |
| **Stale Tools** | Keeping tools registered after their use case no longer applies | `useCount` flatlines at 0 for 10+ cycles; tool references outdated file paths | Meta-cycle retirement sweep; auto-flag tools with 0 uses in last 10 cycles |
