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

## Self-Evaluation Bias Mitigation

As of 2026, LLM-as-a-judge patterns suffer from documented systematic biases. The Evolve Loop employs the SURE (Selective Uncertainty-based Re-Evaluation) pipeline to mitigate these:

1. **Verbosity Bias:** LLMs naturally favor longer responses. The Auditor and Calibrate phases are explicitly instructed to resist verbosity bias and score strictly on qualitative merit, penalizing unnecessary complexity.
2. **Self-Preference Bias:** LLMs favor outputs from their own model family. The loop mitigates this by relying on objective bash/grep eval graders (Phase 3) as the primary hard gate, using the LLM Auditor only for semantic checks.
3. **Calibration Gap:** LLMs often present incorrect evaluations with high confidence. The Auditor is required to output an explicit `confidence` score (0.0-1.0). If confidence falls below `0.8`, the Auditor MUST issue a `WARN` verdict, triggering a Builder retry or operator escalation rather than a false `PASS`.

## Uncertainty Acknowledgment

Agents should explicitly state when confidence is low rather than generating plausible-sounding content that may be incorrect. Uncertainty acknowledgment is distinct from hedging — it is a specific signal ("confidence: low because X") rather than a vague qualifier ("this might work").

**Evolve-loop mapping:**

- **Builder — build-report Risks section:** The Risks section already captures uncertainty signals for the Auditor. Risks should include a confidence indicator when the Builder is unsure about an approach: "approach untested on files > 200 lines — confidence: low."
- **Scout — deferred-task counterfactuals:** When the Scout defers a task, it already produces a rationale. Deferral is an uncertainty acknowledgment at the task-selection level — the Scout signals that it lacks sufficient information to scope the task safely.
- **Builder — capability gap detection (Step 7):** When the Builder identifies a capability gap, reporting it explicitly ("no tool available to validate YAML schema") is uncertainty acknowledgment at the tooling level. This prevents the Builder from generating a plausible-looking workaround that silently fails.
- **Auditor — WARN vs FAIL thresholds:** The Auditor's two-tier verdict system (WARN for uncertain risk, FAIL for confirmed violation) encodes uncertainty acknowledgment into the review protocol.

---

## Anti-Conformity Audit Protocol (Free-MAD-Inspired)

Free-MAD (arXiv:2509.11035) identifies a critical failure mode in LLM evaluation: **agent conformity**. LLMs have an inherent tendency to align with their initial assessment even when evidence contradicts it. This "self-herding" means a single-pass Auditor can confirm an incorrect PASS simply because its first impression was positive.

**Split-role adversarial check:** Before finalizing any audit verdict, run an explicit anti-conformity step:

1. **Initial assessment:** Auditor forms a preliminary verdict (PASS/WARN/FAIL) based on normal review
2. **Adversarial challenge:** The Auditor then adopts the opposite stance:
   - If initial = PASS → "actively search for one strong reason to FAIL"
   - If initial = FAIL → "actively search for one strong reason to PASS"
3. **Score-based decision:** Compare the strength of evidence for both positions. The verdict follows the stronger evidence, not the initial assessment

**Anti-conformity injection prompt** (add to Auditor system context):
> "Before finalizing your verdict, explicitly state your initial leaning. Then argue the opposite case with at least one concrete piece of evidence. Only finalize after you have genuinely considered both sides."

**Trajectory consistency check** (for Phase 5 self-evaluation):
If the LLM-as-a-Judge's chain-of-thought lists specific failures or concerns but then assigns a high score (>= 0.8), flag this as a conformity signal — the Judge is conforming to its own initial bias rather than following its evidence. Trigger recalibration when this pattern appears in 2+ consecutive cycles.

**Token cost:** This is a single-round technique (no multi-round debate needed). Adds ~200-400 tokens to each audit verdict — minimal cost for substantially improved robustness.

---

## Cross-Cutting Application

| Technique | Scout | Builder | Auditor | Operator |
|-----------|-------|---------|---------|----------|
| Chain-of-Thought | Task rationale reasoning | Step 3 design, build-report risks | Finding justification | HALT/CONTINUE reasoning |
| Multi-Stage Verification | Criteria decomposition | Per-grader self-verify | Segment→verify→reflect | Ledger claim checks |
| Context Alignment | Task spec vs codebase | filesToModify coverage | Changes table vs task spec | Score vs outcome alignment |
| Uncertainty Acknowledgment | Deferred-task rationale | Confidence indicators, capability gaps | WARN vs FAIL thresholds | Operator warnings |

---

## Implementation Patterns

Concrete examples of the techniques above as runnable graders and procedural flows.

### CoT-Enforcing Audit Grader

A grep-based grader that checks whether a Builder build-report contains at least three numbered reasoning steps (indicating chain-of-thought was applied during design):

```bash
# Pass if build-report contains 3 or more numbered list items (e.g., "1. ", "2. ", "3. ")
grep -cE '^[0-9]+\.' .evolve/workspace/build-report.md | awk '{exit ($1 < 3)}'
```

This grader can be added to any eval file where CoT compliance matters. It fails (exit 1) if fewer than three numbered steps appear, signaling the Builder skipped explicit reasoning. The Auditor can run this as a structural check independently of functional graders.

### Multi-Stage Verification Flow

Example: an Auditor reviewing a build that modifies 3 files (`config.json`, `lib/runner.js`, `docs/guide.md`).

**Stage 1 — Segment.** Extract each row from the build-report Changes table as an independent claim:

- Claim A: `config.json` was modified to add a `timeout` field
- Claim B: `lib/runner.js` was modified to read the new `timeout` field
- Claim C: `docs/guide.md` was updated to document the `timeout` option

**Stage 2 — Verify.** For each claim, run a targeted tool check:

```bash
# Claim A: field exists in config
grep -q '"timeout"' config.json && echo "A: PASS" || echo "A: FAIL"

# Claim B: runner reads timeout
grep -q 'timeout' lib/runner.js && echo "B: PASS" || echo "B: FAIL"

# Claim C: docs mention timeout
grep -qi 'timeout' docs/guide.md && echo "C: PASS" || echo "C: FAIL"
```

**Stage 3 — Reflect.** If Claim A passes but Claim B fails, surface the conflict: "config declares timeout but runner does not consume it — partial implementation, likely a WARN." Do not silently pick a verdict; state the contradiction explicitly in the Auditor findings.

### Groundedness Check Example

A one-liner that cross-references the files listed in the build-report Changes table against the `filesToModify` list in the scout-report, catching ungrounded file modifications:

```bash
# Extract changed files from build-report (assumes "| ACTION | path/to/file |" format)
changed=$(grep -oP '(?<=\| \w{4,6} \| )[^|]+' .evolve/workspace/build-report.md | tr -d ' ')

# Extract filesToModify from scout-report
allowed=$(grep -A20 'filesToModify' .evolve/workspace/scout-report.md | grep -oP '`[^`]+`' | tr -d '`')

# Flag any changed file not in the allowed list
for f in $changed; do
  echo "$allowed" | grep -qF "$f" || echo "UNGROUNDED: $f not in filesToModify"
done
```

Any `UNGROUNDED:` line indicates the Builder modified a file outside the task scope — a direct groundedness failure that the Auditor should escalate to WARN or FAIL depending on severity.

For token optimization techniques that complement accuracy work (e.g., how context window management prevents mid-cycle truncation that can degrade output quality), see `docs/token-optimization.md`.

For pipeline security mechanisms that protect accuracy (eval tamper detection, state integrity, prompt injection defense), see `docs/security-considerations.md`.

For per-step evidence decomposition that improves self-evaluation calibration (+15% AUC-ROC), see `docs/self-learning.md` § Stepwise Confidence Scoring.

For confidence-correctness alignment that detects when the LLM-as-a-Judge's stated confidence drifts from actual correctness rates and triggers automatic recalibration, see `docs/self-learning.md` § Confidence-Correctness Alignment.
