# Meta-Cycle Self-Improvement

The meta-cycle is a self-evaluation pass that runs every 5 cycles (`cycle % 5 === 0`). It reviews pipeline performance, agent effectiveness, and eval quality, then proposes targeted improvements.

## Trigger

The meta-cycle runs automatically during Phase 5 (LEARN) when `cycle % 5 === 0`. It executes after instinct extraction and before the Operator check.

## Split-Role Critique

The split-role critique uses three specialized critics to review the last 5 cycles independently, avoiding blind spots:

| Critic | Focus | Key Question |
|--------|-------|-------------|
| **Efficiency Critic** | Cost, token usage, task sizing, model routing | "Are we spending tokens wisely?" |
| **Correctness Critic** | Eval pass rates, audit verdicts, regression trends | "Are we shipping quality code?" |
| **Novelty Critic** | Instinct diversity, task variety, stagnation | "Are we learning new things?" |

The orchestrator synthesizes findings, prioritizing correctness > efficiency > novelty.

## Agent Effectiveness Review

For each agent:
- **Scout** — Are selected tasks the right size? Are they shipping?
- **Builder** — How many attempts per task? Self-verify pass rate?
- **Auditor** — Are WARN/FAIL verdicts being resolved or accumulating?
- **Operator** — Are recommendations being followed?

## Automated Prompt Evolution

Based on findings, the orchestrator may refine agent prompts using a critique-synthesize loop:

1. **Critique** — Identify specific weaknesses from cycle outcomes
2. **Synthesize** — Propose targeted prompt edits (additions, rewording, examples)
3. **Validate** — Check against existing instincts and policies
4. **Apply** — Make the edit, log before/after in meta-review
5. **Track** — Add `prompt-evolution` entry to ledger

**TextGrad-style optimization:** Each edit includes a "textual gradient" describing observed behavior, desired behavior, the specific text change, and expected impact on process rewards.

**Safety constraints:**
- Only modify non-structural sections (guidance, examples, strategy handling)
- Maximum 2 prompt edits per meta-cycle
- All edits are committed and revertable via `git revert`
- Auto-revert if next meta-cycle shows degradation

## Self-Generated Mutation Testing

Tests eval quality by generating code mutations:

1. For each completed task, generate 2-3 small mutations (remove validation, change return value, delete import)
2. Run existing eval graders against mutated code
3. Mutations NOT caught by evals → eval is weak, propose stronger criteria
4. Track **mutation kill rate** (caught / generated)

Target: >80% kill rate. Below 60% triggers eval improvement as priority next cycle.

## Workflow Topology Review

Evaluates whether the current phase ordering is optimal:

1. **Phase skip analysis** — Were any phases redundant?
2. **Phase merge candidates** — Could phases be combined?
3. **Phase addition candidates** — Is there a gap (e.g., recurring security issues)?
4. **Parallel opportunities** — Could independent tasks build in parallel?

Topology changes are proposals only — they require human approval.

## LLM-as-a-Judge Self-Evaluation

Added in cycle 27, the LLM-as-a-Judge mechanism integrates automated self-evaluation into the meta-cycle review process. Rather than relying solely on grep-based eval graders, the pipeline uses a judge LLM to assess build quality on subjective dimensions:

- **Correctness** — Does the implementation satisfy the task intent, not just the letter of the eval graders?
- **Coherence** — Are the changes internally consistent and well-integrated with existing code?
- **Regression risk** — Does the change introduce fragility or break adjacent contracts?

The judge produces a structured verdict (PASS / WARN / FAIL) with a reasoning trace. This verdict feeds into the meta-cycle's agent effectiveness review and informs prompt evolution priorities.

See [docs/self-learning.md](docs/self-learning.md) for the full self-evaluation rubric, scoring weights, and integration with the learn process reward.

## Output

The meta-cycle produces `workspace/meta-review.md` with:
- Pipeline metrics (success rate, audit iterations, stagnation)
- Agent effectiveness table
- Recommended changes
- Mutation testing results
- Topology recommendations (if any)
- LLM-as-a-Judge self-evaluation summary
