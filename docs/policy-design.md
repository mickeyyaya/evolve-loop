# Policy & Skill Design Guide

How to write effective agent skills, rules, and policies for the evolve-loop pipeline and similar agentic systems.

## Core Principles

### 1. Specificity Over Vagueness

Actionable rules must be specific enough that an agent can follow them without interpretation.

| Bad (vague) | Good (specific) |
|-------------|-----------------|
| "Write clean code" | "Functions must be under 50 lines; files under 800 lines" |
| "Handle errors properly" | "All bash commands must check exit code; non-zero triggers HALT" |
| "Keep things modular" | "Extract any section exceeding 300 lines into its own file with a reference link" |
| "Document your work" | "Every agent workspace file must include a ledger entry in JSONL format" |

The test: could two different agents interpret this rule the same way? If not, make it more specific.

### 2. Minimal, Non-Overlapping Rule Sets

Each agent should receive only the rules relevant to its phase. Overlapping rules create ambiguity about which takes precedence.

- **Layer 0 (shared values):** Universal rules all agents follow (immutability, scope discipline, blast radius)
- **Agent-specific rules:** Only in that agent's definition file
- **Strategy overrides:** Modify behavior without duplicating rules

The anti-pattern is "defensive redundancy" — repeating the same rule in 4 different files. When it needs updating, you update 1 and miss 3.

### 3. Guardrail Design: Rules vs LLM Judgment

Choose the right enforcement mechanism:

| Use deterministic rules when... | Use LLM judgment when... |
|--------------------------------|--------------------------|
| The check can be expressed as a bash command | The check requires understanding intent |
| False positives are unacceptable | Some false positives are tolerable |
| The rule applies uniformly to all cases | Context matters (strategy, task type, history) |
| The cost of violation is high (data loss, security) | The cost of violation is low (style, naming) |

**Examples:**
- File size limit → deterministic (`wc -l < file | awk '{exit ($1 > 800)}'`)
- Eval checksum verification → deterministic (`sha256sum -c checksums.json`)
- "Is this task well-scoped?" → LLM judgment (requires understanding the task)
- "Does this change improve the codebase?" → LLM judgment (benchmark eval)

The evolve-loop uses a 70/30 split in benchmark evaluation: 70% automated (deterministic bash checks) + 30% LLM judgment (anchored rubric). This ratio prevents gaming while allowing nuanced assessment.

## Policy Graduation Lifecycle

See [skill-building.md](skill-building.md) for the full lifecycle reference. Policies evolve through a confidence-gated pipeline:

```
Observation → Instinct (0.5) → Confirmed Instinct (0.8+) → Orchestrator Policy (0.9+)
```

### Stage 1: Instinct (confidence 0.5)
A pattern observed during a single cycle. Stored in `.evolve/instincts/personal/` as YAML with source provenance. Not yet trusted enough to influence other agents.

### Stage 2: Confirmed Instinct (confidence 0.8+)
The pattern has been confirmed across multiple cycles (confidence increases by +0.05 each time it's cited by an agent). Agents read these via `instinctSummary` in state.json and may apply them.

### Stage 3: Orchestrator Policy (confidence 0.9+)
The pattern is so reliable it becomes a named policy in SKILL.md (e.g., "Inline S-complexity tasks", "Grep-based evals"). Policies are hard rules the orchestrator enforces, not suggestions.

### Decay & Archival
Instincts not cited for 5+ cycles decay by -0.1 per consolidation pass. Below 0.3 → archived as stale. This prevents policy bloat from outdated patterns.

## Writing Effective Rules

1. **Lead with the constraint, not the rationale.** Agents process rules, not essays. Put the "do this" before the "because."
2. **Include the check command.** If a rule can be verified with a bash command, include it in the rule definition.
3. **Specify the scope.** Which agents does this rule apply to? Which strategies? Which task types?
4. **Define the exception.** Every rule has edge cases. State them explicitly rather than letting agents guess.
5. **Version through git.** Rules are code. They should be committed, reviewed, and revertible.

## Anti-Patterns

- **Rule explosion:** Adding a new rule for every edge case instead of writing one general rule with exceptions
- **Aspirational rules:** Rules that describe ideal behavior no agent can consistently achieve
- **Circular references:** Rule A says "see Rule B", Rule B says "see Rule A"
- **Implicit rules:** Behavior expected but never written down (e.g., "obviously the Builder should...")
- **Stale rules:** Rules that reference removed features or outdated schemas
