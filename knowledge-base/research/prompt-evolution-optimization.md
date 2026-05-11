> **Prompt Evolution Optimization** — Reference doc on automated prompt improvement techniques. Covers evolutionary search, iterative refinement, meta-prompting, and gradient-free optimization applied to evolve-loop agent prompts.

## Table of Contents

1. [Technique Taxonomy](#technique-taxonomy)
2. [Selection Matrix](#selection-matrix)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Technique Taxonomy

| Technique | Category | Mechanism | Strengths | Limitations |
|-----------|----------|-----------|-----------|-------------|
| MIPROv2 | Evolutionary search | Generate candidate prompts via LLM proposals; evaluate on training set; select top-k via Bayesian surrogate model | Systematic exploration of prompt space; handles multi-stage pipelines | Requires large eval dataset; high token cost for search |
| SIMBA | Evolutionary search | Segment-level optimization; mutate individual prompt segments independently; recombine best segments | Fine-grained control; avoids catastrophic prompt rewrites | Segment boundary selection is manual; combinatorial explosion risk |
| GEPA | Evolutionary search | Grammar-guided evolutionary prompt assembly; use formal grammar to constrain prompt structure during mutation | Structural consistency across generations; prevents degenerate prompts | Requires grammar definition upfront; less flexible for free-form prompts |
| Prochemy | Iterative refinement | Analyze eval failures; generate targeted prompt patches; verify improvement on held-out set | Low token cost per iteration; fast convergence on specific failure modes | Local optima risk; may miss global prompt structure improvements |
| Meta-prompting | Meta-optimization | Use an LLM to generate and critique prompts for another LLM; outer loop selects best candidates | No eval dataset required for initial exploration; leverages LLM reasoning about prompts | Circular dependency on LLM quality; hard to validate without evals |
| Gradient-free optimization | Numerical search | Treat prompt tokens as discrete variables; apply CMA-ES, random search, or bandit algorithms over prompt variants | Model-agnostic; works with black-box APIs | Slow convergence in high-dimensional prompt spaces; token-expensive |
| Few-shot selection | Example curation | Retrieve or rank few-shot examples by similarity to test input; optimize example ordering and count | Direct performance gain on structured tasks; composable with other techniques | Requires example bank; diminishing returns past 5-8 examples |
| Databricks APO | Iterative refinement | Automated prompt optimization via eval-driven rewriting; replace manual prompt engineering with systematic search | 90x cost reduction vs manual tuning; production-proven at scale | Requires well-defined eval metrics; less effective for open-ended tasks |

## Selection Matrix

Use this matrix to select the right technique based on your constraints.

| Constraint | Evolutionary Search (MIPROv2/SIMBA/GEPA) | Iterative Refinement (Prochemy/APO) | Meta-Prompting | Gradient-Free | Few-Shot Selection |
|------------|------------------------------------------|-------------------------------------|----------------|---------------|-------------------|
| Task complexity: low | Overkill | Good fit | Good fit | Overkill | Best fit |
| Task complexity: high | Best fit | Good for targeted fixes | Starting point only | Viable if budget allows | Complementary |
| Eval signal: strong (numeric score) | Best fit | Best fit | Viable | Best fit | Good fit |
| Eval signal: weak (LLM-as-judge) | Viable with large sample | Viable with careful thresholds | Best fit | Poor — noisy gradients | Viable |
| Eval signal: none | Not viable | Not viable | Only option | Not viable | Manual curation only |
| Token budget: < 100K | Not viable | Best fit | Viable (1-2 rounds) | Not viable | Best fit |
| Token budget: 100K-1M | Viable (small search) | Best fit | Good fit | Viable | Good fit |
| Token budget: > 1M | Best fit (full search) | Complementary | Complementary | Viable | Complementary |
| Pipeline stages: single | Any technique works | Best fit | Good fit | Viable | Good fit |
| Pipeline stages: multi (3+) | MIPROv2 designed for this | Apply per-stage independently | Good for initial design | Apply per-stage | Select per-stage |
| Latency tolerance: low | Pre-compute offline | Pre-compute offline | Pre-compute offline | Pre-compute offline | Runtime-viable |
| Latency tolerance: high | Runtime search viable | Runtime patching viable | Runtime viable | Not recommended | Runtime viable |

## Mapping to Evolve-Loop

| Evolve-Loop Concept | Prompt Evolution Analog | Technique to Apply | Implementation Location |
|---------------------|------------------------|-------------------|------------------------|
| Instinct evolution | Prompt evolution across generations | Evolutionary search (MIPROv2) | `state.json` instinct field; mutate per cycle |
| Gene mutation | Discrete prompt parameter variation | SIMBA segment-level mutation | Gene definitions in `docs/reference/genes.md`; vary gene values as prompt segments |
| Scout prompt refinement | Iterative prompt improvement based on eval feedback | Prochemy-style patching | Scout system prompt; patch based on audit-report failures |
| Builder prompt refinement | Targeted improvement of code generation instructions | Iterative refinement (APO) | Builder system prompt; optimize based on build success rate |
| Auditor prompt refinement | Calibrate evaluation criteria over cycles | Meta-prompting + eval feedback | Auditor system prompt; adjust scoring rubrics based on false positive/negative rates |
| Strategy playbook | Accumulated prompt knowledge base | Few-shot selection from past successes | `workspace/strategy-playbook.md`; retrieve relevant strategies as few-shot examples |
| Fitness function | Eval signal for prompt optimization | Drives all techniques above | Cycle eval delta; quality scores from audit reports |
| Cross-cycle learning | Population-level prompt evolution | GEPA grammar-guided assembly | Aggregate winning prompt patterns across cycles into reusable templates |

## Implementation Patterns

### A/B Testing Prompts Across Cycles

| Step | Action | Tool/Location | Success Criteria |
|------|--------|---------------|------------------|
| 1 | Generate prompt variant | Meta-prompting or mutation operator | Variant differs from baseline in >= 1 segment |
| 2 | Store variant in state.json | `state.json` → `prompt_variants[]` | Variant ID, parent ID, diff summary recorded |
| 3 | Assign variant to cycle | Cycle config selects variant by ID | Single variant per cycle (no confounding) |
| 4 | Run Scout with variant prompt | Scout agent uses assigned variant | Scout completes without error |
| 5 | Run Builder with variant prompt | Builder agent uses assigned variant | Builder produces valid output |
| 6 | Run Auditor with baseline prompt | Auditor uses stable prompt (control) | Auditor scores are comparable across variants |
| 7 | Record eval delta | Compare cycle score to baseline mean | Delta recorded in `state.json` → `prompt_results[]` |
| 8 | Select or discard variant | Promote if delta > threshold; discard otherwise | Winning variant becomes new baseline |

### Tracking Prompt Variants in state.json

| Field | Type | Description |
|-------|------|-------------|
| `prompt_variants` | Array | All active and historical prompt variants |
| `prompt_variants[].id` | String | Unique variant identifier (e.g., `scout-v12`) |
| `prompt_variants[].parent_id` | String | ID of the variant this was derived from |
| `prompt_variants[].agent` | String | Target agent: `Scout`, `Builder`, or `Auditor` |
| `prompt_variants[].diff` | String | Human-readable summary of what changed |
| `prompt_variants[].created_cycle` | Number | Cycle number when variant was created |
| `prompt_variants[].status` | String | `active`, `promoted`, `discarded` |
| `prompt_results` | Array | Eval results per variant per cycle |
| `prompt_results[].variant_id` | String | Which variant was used |
| `prompt_results[].cycle` | Number | Cycle number |
| `prompt_results[].eval_delta` | Number | Score difference vs baseline |
| `prompt_results[].promoted` | Boolean | Whether this result triggered promotion |

### Measuring Prompt Effectiveness via Eval Delta

| Metric | Formula | Interpretation |
|--------|---------|----------------|
| Raw eval delta | `cycle_score - baseline_mean` | Positive = improvement; negative = regression |
| Normalized delta | `(cycle_score - baseline_mean) / baseline_stddev` | Z-score; > 1.0 = statistically meaningful |
| Win rate | `wins / total_comparisons` | Fraction of cycles where variant beat baseline |
| Prompt cost ratio | `variant_tokens / baseline_tokens` | > 1.0 = more expensive; weight against eval gain |
| Net effectiveness | `normalized_delta - (cost_ratio - 1.0)` | Balance quality gain against token cost increase |

## Prior Art

| System | Author/Org | Year | Key Contribution | Relevance to Evolve-Loop |
|--------|-----------|------|------------------|--------------------------|
| MIPROv2 | Stanford NLP (DSPy team) | 2024 | Bayesian surrogate model for multi-stage prompt optimization; jointly optimizes instructions and few-shot examples | Direct model for instinct evolution; handles Scout-Builder-Auditor pipeline as multi-stage system |
| SIMBA | Microsoft Research | 2024 | Segment-level prompt optimization; mutate and recombine individual prompt sections | Model for gene-level mutation; each gene maps to a prompt segment |
| GEPA | Tsinghua University | 2024 | Grammar-guided evolutionary assembly; formal grammar constrains valid prompt structures | Enforce structural consistency in instinct prompts; prevent degenerate mutations |
| Prochemy | Prochemy Labs | 2024 | Failure-driven prompt patching; analyze eval errors to generate targeted fixes | Model for Scout prompt refinement; patch based on audit findings |
| Databricks APO | Databricks | 2024 | Automated prompt optimization achieving 90x cost reduction vs manual engineering | Validate that automated optimization is production-viable; benchmark for cost savings |
| DSPy | Stanford NLP | 2023-2024 | Programming framework for LLM pipelines; compile high-level signatures into optimized prompts | Foundation library; potential runtime for evolve-loop prompt compilation |
| PromptBreeder | DeepMind | 2023 | Self-referential self-improvement of prompts; mutation prompts that evolve themselves | Model for meta-level instinct evolution; instincts that improve their own evolution strategy |
| EvoPrompt | Tsinghua University | 2023 | Apply genetic algorithms (GA) and differential evolution (DE) to discrete prompt optimization | Baseline evolutionary approach; validate GA-style operators on prompt strings |
| OPRO | Google DeepMind | 2023 | Use LLM as optimizer; feed past prompts and scores, ask for better prompt | Simplest meta-prompting pattern; viable as starting point for Scout refinement |

## Anti-Patterns

| Anti-Pattern | Description | Symptom | Mitigation |
|-------------|-------------|---------|------------|
| Prompt overfitting | Optimize prompt to maximize score on training evals; performance degrades on novel tasks | High eval scores but poor real-world performance; score drops when eval set rotates | Hold out 20%+ of evals for validation; rotate eval sets across cycles |
| Ignoring prompt length costs | Add instructions, examples, and context without tracking token cost increase | Prompt grows monotonically across cycles; token spend increases without proportional quality gain | Track prompt cost ratio; enforce token budget per prompt; prune low-impact sections |
| Manual prompt tweaking at scale | Hand-edit prompts for each agent based on intuition rather than eval signal | Inconsistent prompt quality; changes not tracked; no rollback capability | Use automated optimization; track all variants in state.json; require eval evidence for changes |
| Prompt drift | Accumulated small changes cause prompt to diverge from original intent without detection | Agent behavior shifts gradually; no single change is flagged but cumulative effect is significant | Periodically diff current prompt against original; set semantic similarity threshold for drift alert |
| Eval gaming | Prompt evolves to exploit eval metric weaknesses rather than improve actual task quality | Eval scores increase but human review shows declining output quality | Use multiple diverse eval metrics; include human-in-the-loop spot checks; rotate eval rubrics |
| Premature convergence | Search settles on a local optimum; stop exploring after early gains | Eval scores plateau early; diversity of prompt variants collapses | Maintain population diversity; inject random mutations; use temperature scheduling in search |
| Ignoring prompt interactions | Optimize each agent prompt independently; miss cross-agent prompt dependencies | Scout improvement causes Builder regression; overall pipeline quality stagnates | Optimize end-to-end pipeline score (MIPROv2 approach); test prompt changes across full cycle |
| Cargo-cult few-shot | Add few-shot examples without measuring their impact; assume more examples = better | Prompt length increases; performance flat or decreasing; latency increases | A/B test example count; measure marginal gain per example; remove examples with negative impact |
