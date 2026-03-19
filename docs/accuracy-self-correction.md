# Accuracy and Self-Correction Techniques

Techniques for improving output accuracy and catching errors across the evolve-loop pipeline. Each technique is mapped to the specific agent and phase where it applies.

---

## Chain-of-Thought (CoT) Prompting

Chain-of-thought prompting requires an agent to produce explicit step-by-step reasoning before arriving at an answer or decision. Research shows a +35% accuracy improvement on reasoning tasks when outputs include numbered reasoning steps with mandatory source citation and explicit uncertainty acknowledgment.

**How it works:**

1. Break the problem into numbered steps before producing any output
2. Cite the source or evidence for each step (file path, instinct ID, eval grader)
3. Acknowledge uncertainty explicitly rather than generating confident-sounding guesses

**Evolve-loop mapping:**

- **Builder — Step 3 (Design):** The Builder's inline design phase should enumerate reasoning steps before selecting an approach. When multiple approaches exist, list trade-offs per step rather than jumping to a conclusion.
- **Auditor — Verdict reasoning:** The Auditor must produce chain-of-thought justification for each WARN or FAIL finding. A finding without a reasoning chain is not actionable and risks being a false positive.
- **LLM-as-a-Judge (LEARN phase):** Self-evaluation scores already require chain-of-thought justification before a numeric value is assigned (see `docs/self-learning.md`). This is the CoT pattern applied to meta-evaluation.

---

## Multi-Stage Verification (HaluAgent Pattern)

Single-pass review misses errors that are only detectable by decomposing output into discrete claims and verifying each claim independently. The HaluAgent pattern uses three stages:

1. **Segmentation** — Split the output into individual atomic claims (one assertion per segment)
2. **Tool-based verification** — Verify each claim against ground truth using tools (grep, test execution, file existence checks)
3. **Reflective reasoning** — Reconcile conflicts between claims; if two claims contradict each other, surface the conflict rather than silently picking one

**Evolve-loop mapping:**

- **Auditor — complex task reviews:** The Auditor currently performs single-pass review. For tasks touching more than 3 files or flagged as `complexity: M+`, the Auditor should apply segment→verify→reflect: segment the build-report's change table into individual claims, verify each claim against the actual diff, then reflect on any discrepancies before issuing the verdict.
- **Builder — self-verification (Step 5):** Running eval graders sequentially is already a form of claim-by-claim verification. The Builder should treat each grader as one segment and report per-grader results rather than a single aggregate check.
- **Scout — acceptance criteria review:** Each acceptance criterion in a task is a claim. Scout should verify each criterion is testable and non-overlapping before finalizing the task object.

---

## Context Alignment Scoring

Context alignment scoring measures how well a generated output is grounded in the provided input context. Two sub-scores:

- **Input coverage:** Did the output address all relevant parts of the input (task spec, instincts, eval graders)?
- **Output groundedness:** Does every claim in the output trace back to something in the input context, or did the agent introduce unsupported assumptions?

Low groundedness is a hallucination indicator. High coverage with low groundedness is a particularly risky pattern — the output looks complete but is partially fabricated.

**Evolve-loop mapping:**

- **Eval graders:** Graders currently verify behavioral correctness (exit codes, file existence, content checks) but do not score context alignment. A complementary grader pattern is to check that each file changed in the build report corresponds to a file listed in `filesToModify` in the task spec.
- **Benchmark dimensions:** The `correctness` and `completeness` dimensions in LLM-as-a-Judge self-evaluation partially cover context alignment. Groundedness maps to correctness; coverage maps to completeness.
- **Builder — build-report:** The Changes table (Action / File / Description) functions as a groundedness check surface. The Auditor can cross-reference each row against the task's `filesToModify` list to detect ungrounded additions.

---

## Uncertainty Acknowledgment

Agents should explicitly state when confidence is low rather than generating plausible-sounding content that may be incorrect. Uncertainty acknowledgment is distinct from hedging — it is a specific signal ("confidence: low because X") rather than a vague qualifier ("this might work").

**Evolve-loop mapping:**

- **Builder — build-report Risks section:** The Risks section already captures uncertainty signals for the Auditor. Risks should include a confidence indicator when the Builder is unsure about an approach: "approach untested on files > 200 lines — confidence: low."
- **Scout — deferred-task counterfactuals:** When the Scout defers a task, it already produces a rationale. Deferral is an uncertainty acknowledgment at the task-selection level — the Scout signals that it lacks sufficient information to scope the task safely.
- **Builder — capability gap detection (Step 7):** When the Builder identifies a capability gap, reporting it explicitly ("no tool available to validate YAML schema") is uncertainty acknowledgment at the tooling level. This prevents the Builder from generating a plausible-looking workaround that silently fails.
- **Auditor — WARN vs FAIL thresholds:** The Auditor's two-tier verdict system (WARN for uncertain risk, FAIL for confirmed violation) encodes uncertainty acknowledgment into the review protocol.

---

## Cross-Cutting Application

| Technique | Scout | Builder | Auditor | Operator |
|-----------|-------|---------|---------|----------|
| Chain-of-Thought | Task rationale reasoning | Step 3 design, build-report risks | Finding justification | HALT/CONTINUE reasoning |
| Multi-Stage Verification | Criteria decomposition | Per-grader self-verify | Segment→verify→reflect | Ledger claim checks |
| Context Alignment | Task spec vs codebase | filesToModify coverage | Changes table vs task spec | Score vs outcome alignment |
| Uncertainty Acknowledgment | Deferred-task rationale | Confidence indicators, capability gaps | WARN vs FAIL thresholds | Operator warnings |

For token optimization techniques that complement accuracy work (e.g., how context window management prevents mid-cycle truncation that can degrade output quality), see `docs/token-optimization.md`.
