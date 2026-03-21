# Evolve Loop — Phase 6: META-CYCLE

Meta-cycle self-improvement runs ONLY when `cycle % 5 === 0`. This phase evaluates the evolve-loop's own effectiveness and proposes improvements to agent prompts, strategies, and pipeline topology.

---

### Meta-Cycle Self-Improvement (every 5 cycles)

7. **Meta-Cycle Self-Improvement** (every 5 cycles):
   If `cycle % 5 === 0`, run a meta-evaluation of the evolve-loop's own effectiveness:

   a. **Collect metrics** from the last 5 cycles in `evalHistory` and `ledger.jsonl`:
      - Tasks shipped vs attempted (success rate)
      - Average audit iterations per task (Builder efficiency)
      - Stagnation pattern count
      - Instinct confidence trend (are instincts getting confirmed?)

   b. **Split-role critique** — use three specialized critic perspectives to avoid blind spots:

      | Critic | Focus | Key Question |
      |--------|-------|-------------|
      | **Efficiency Critic** | Cost, token usage, task sizing, model routing | "Are we spending tokens wisely? Could tasks be smaller?" |
      | **Correctness Critic** | Eval pass rates, audit verdicts, regression trends | "Are we shipping quality code? Are evals catching issues?" |
      | **Novelty Critic** | Instinct diversity, task variety, stagnation patterns | "Are we learning new things? Or repeating the same work?" |

      Each critic reviews the last 5 cycles independently and produces 1-3 findings. The orchestrator synthesizes findings into the meta-review, resolving conflicts by prioritizing correctness > efficiency > novelty.

   c. **Evaluate agent effectiveness** — for each agent, ask:
      - Scout: Are selected tasks the right size? Are they shipping?
      - Builder: How many attempts per task? What's the self-verify pass rate?
      - Auditor: Are WARN/FAIL verdicts being resolved or accumulating?
      - Operator: Are recommendations being followed?

   c. **Propose improvements** — write a `meta-review.md` to the workspace:
      ```markdown
      # Meta-Cycle Review — Cycles {N-4} to {N}

      ## Pipeline Metrics
      - Success rate: X/Y tasks (Z%)
      - Avg audit iterations: N
      - Stagnation patterns: N active
      - Instinct trend: growing/stable/stale

      ## Agent Effectiveness
      | Agent | Assessment | Suggested Change |
      |-------|-----------|-----------------|
      | Scout | ... | ... |
      | Builder | ... | ... |
      | Auditor | ... | ... |
      | Operator | ... | ... |

      ## Recommended Changes
      1. <specific change to agent prompt, strategy, or process>
      ```

   d. **Automated Prompt Evolution** — based on meta-review findings, the orchestrator may refine agent prompts using a critique-synthesize loop:

      1. **Critique:** Identify specific weaknesses in agent prompts based on cycle outcomes. For example, if the Builder frequently needs 3 attempts, its design step may need stronger guidance.
      2. **Synthesize:** Propose specific prompt edits (additions, rewording, new examples) that address the weakness. Each edit must be small and targeted — do not rewrite entire agent definitions.
      3. **Validate:** Before applying, check that the proposed edit doesn't contradict existing instincts or orchestrator policies.
      4. **Apply:** Make the edit to the agent file. Log the change in the meta-review with before/after and rationale.
      5. **Track:** Add a `prompt-evolution` entry to the ledger:
         ```json
         {"ts":"<ISO-8601>","cycle":<N>,"role":"orchestrator","type":"prompt-evolution","data":{"agent":"<name>","section":"<section changed>","rationale":"<why>","change":"<summary>"}}
         ```

      **TextGrad-style optimization:** For each proposed edit, generate a "textual gradient" — a natural language critique describing:
      - What the current prompt produces (observed behavior)
      - What it should produce (desired behavior)
      - The specific text change that bridges the gap (the "gradient")
      - Expected impact on process rewards for the affected phase

      This is more rigorous than free-form critique. The gradient must reference specific prompt text and specific cycle outcomes.

      **Safety constraints:**
      - Only modify non-structural sections (guidance, examples, strategy handling) — never change the agent's tools, model, or core responsibilities
      - Maximum 2 prompt edits per meta-cycle
      - All edits are committed and can be reverted with `git revert`
      - If an evolved prompt leads to worse performance in the next meta-cycle, auto-revert the change

   d2. **Skill Synthesis Check** (meta-cycle only — after prompt evolution, before mutation testing):

      After the split-role critique and prompt evolution, check whether any instinct clusters are ready to graduate into executable artifacts. This closes the loop between learning and capability expansion — instincts become structurally durable genes or skill fragments rather than passive hints. (Research basis: continuous-learning-v2 instinct→skill graduation, self-learning-agent-patterns pattern graduation pipelines.)

      **Step 1: Identify synthesis candidates**
      Query `instinctSummary` in state.json for clusters of 3+ instincts that meet ALL criteria:
      - Same `category` (all procedural, all semantic, or all episodic)
      - Related patterns (overlapping keywords, shared target file types, or similar domain)
      - All confidence >= 0.8
      - None marked `graduated: true` already (avoid re-synthesizing)

      **Step 2: Determine synthesis target** for each qualifying cluster:

      **Option A — Synthesize a Gene** (when instincts describe a repeatable fix/implementation pattern):
      Write to `.evolve/genes/<pattern-name>.md`:
      ```markdown
      # Gene: <pattern-name>
      <!-- Synthesized from instincts: inst-012, inst-017, inst-023 at cycle N -->

      ## Selector
      When to apply: <conditions under which this gene matches — task type, file patterns, error signatures>

      ## Steps
      1. <concrete implementation step>
      2. <concrete implementation step>
      3. <concrete implementation step>

      ## Validation
      - `<bash command to verify the gene was applied correctly>`
      - `<bash command to check for regressions>`

      ## Source Instincts
      - inst-012: <pattern summary>
      - inst-017: <pattern summary>
      - inst-023: <pattern summary>
      ```

      **Option B — Synthesize a Skill Fragment** (when instincts describe agent behavior improvements):
      Propose an addition to the relevant agent's prompt. Gate through the same TextGrad validation used for prompt evolution:
      1. Draft the skill fragment as a new markdown section (max 20 lines)
      2. Generate a textual gradient: current behavior → desired behavior → text change
      3. Validate against existing instincts and policies (no contradictions)
      4. Apply via the prompt evolution mechanism (counts toward the 2-edit-per-meta-cycle limit)

      **Step 3: Record synthesis** in `state.json.synthesizedTools`:
      ```json
      {
        "name": "<pattern-name>",
        "path": ".evolve/genes/<pattern-name>.md",
        "purpose": "<one-line description>",
        "sourceInstincts": ["inst-012", "inst-017", "inst-023"],
        "cycle": <N>,
        "useCount": 0,
        "type": "gene|skill-fragment"
      }
      ```

      **Step 4: Update source instincts**
      - Mark all source instincts as `graduated: true` in `instinctSummary`
      - These instincts stop decaying (confidence is preserved) but are no longer individually injected — the gene/skill fragment subsumes them

      **Safety constraints:**
      - Maximum 1 synthesis per meta-cycle (avoid bulk generation of untested artifacts)
      - Synthesized genes must pass the eval-quality-check rigor filter (no trivial validation commands)
      - Skill fragments are subject to the same auto-revert rule as prompt evolution edits
      - Log synthesis in the ledger:
        ```json
        {"ts":"<ISO-8601>","cycle":<N>,"role":"orchestrator","type":"skill-synthesis","data":{"name":"<pattern-name>","type":"gene|skill-fragment","sourceInstincts":["inst-012","inst-017","inst-023"]}}
        ```

   e. **Self-Generated Evaluation (mutation testing):**

      Test the quality of our evals by generating mutations:
      1. For each task completed in the last 5 cycles, generate 2-3 small code mutations (e.g., remove a validation, change a return value, delete an import)
      2. Run the existing eval graders against the mutated code
      3. If evals DON'T catch a mutation → the eval is weak. Propose stronger eval criteria.
      4. Track **mutation kill rate** (mutations caught / mutations generated)

      ```markdown
      ## Mutation Testing Results
      - Mutations generated: <N>
      - Mutations killed (caught by evals): <N>
      - Kill rate: <percentage>
      - Weak evals identified: <list>
      - Proposed improvements: <list>
      ```

      Target: >80% mutation kill rate. Below 60% triggers eval improvement as a priority task in the next cycle.

   f. **Workflow Topology Review:**

      Evaluate whether the current phase ordering and agent configuration is optimal:

      1. **Phase skip analysis** — were any phases redundant this meta-cycle? (e.g., Auditor always PASS → consider lighter audit)
      2. **Phase merge candidates** — could two phases be combined? (e.g., if Builder self-verify is reliable, reduce Auditor scope)
      3. **Phase addition candidates** — is there a gap? (e.g., if security issues keep recurring, add a dedicated security scan phase)
      4. **Parallel opportunities** — could independent tasks be built in parallel instead of sequentially?

      Propose topology changes in the meta-review:
      ```markdown
      ## Topology Recommendations
      - **Current:** DISCOVER → BUILD → AUDIT → SHIP → LEARN
      - **Proposed:** DISCOVER → BUILD(parallel) → AUDIT(light) → SHIP → LEARN
      - **Rationale:** <why this change would improve performance>
      ```

      **Safety:** Topology changes are proposals only — they require human approval before the orchestrator applies them. Never auto-apply topology changes.

   g. **Apply remaining changes** — update default strategy, token budgets, or other configuration based on meta-review findings. Archive the `meta-review.md` to history.

   h. **Regenerate project digest** — during meta-cycle (every 5 cycles), regenerate `.evolve/project-digest.md` (shared location) to capture any structural changes.
