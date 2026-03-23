# Synthetic Data Generation

> Reference document for agents generating synthetic data for eval bootstrapping
> and training. Use multi-agent pipelines to produce high-quality, diverse datasets
> that drive evaluation coverage and model improvement.

## Table of Contents

1. [Pipeline Architecture](#pipeline-architecture)
2. [Generation Techniques](#generation-techniques)
3. [Quality Control](#quality-control)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Pipeline Architecture

Use a five-stage multi-agent pipeline to generate synthetic data. Each stage has a dedicated agent with a single responsibility.

| Stage | Agent Role | Input | Output | Description |
|---|---|---|---|---|
| **1. Seed** | Seed Curator | Raw corpus, existing evals, failure logs | `seed-set.jsonl` | Extract seed examples from production data, prior eval results, and known failure cases |
| **2. Generate** | Generator | `seed-set.jsonl`, generation config | `raw-synthetic.jsonl` | Produce candidate examples using configured techniques (see Generation Techniques) |
| **3. Filter** | Filter Agent | `raw-synthetic.jsonl`, quality rules | `filtered-synthetic.jsonl` | Remove duplicates, malformed entries, and examples below quality thresholds |
| **4. Validate** | Validator | `filtered-synthetic.jsonl`, ground truth refs | `validated-synthetic.jsonl` | Verify correctness of labels, check factual accuracy, run consistency checks |
| **5. Augment** | Augmentor | `validated-synthetic.jsonl`, augmentation rules | `final-synthetic.jsonl` | Apply transformations (paraphrase, difficulty scaling, format variation) to increase diversity |

### Pipeline Flow

```
Seed Curator → Generator → Filter Agent → Validator → Augmentor
     ↓              ↓            ↓             ↓            ↓
 seed-set.jsonl  raw-syn.jsonl  filtered.jsonl  valid.jsonl  final.jsonl
```

### Stage Contracts

| Contract | Format | Required Fields | Validation Rule |
|---|---|---|---|
| Seed → Generator | JSONL | `id`, `content`, `category`, `source` | Each seed must have a unique `id` and non-empty `content` |
| Generator → Filter | JSONL | `id`, `content`, `label`, `seed_id`, `technique` | Every generated example must reference its originating `seed_id` |
| Filter → Validator | JSONL | `id`, `content`, `label`, `quality_score` | `quality_score` must exceed configured threshold (default 0.7) |
| Validator → Augmentor | JSONL | `id`, `content`, `label`, `validation_status` | Only entries with `validation_status: "pass"` proceed |

---

## Generation Techniques

| Technique | Description | Best For | Diversity | Cost | Example |
|---|---|---|---|---|---|
| **Template-Based** | Fill structured templates with variable slots | Uniform coverage of known patterns | Low | Low | `"Write a {language} function that {action} on {data_type}"` |
| **LLM-Generated** | Prompt an LLM with seed examples and instructions to produce novel variants | Open-ended tasks, natural language evals | High | Medium | Provide 3 seed examples; ask for 50 diverse variants |
| **Adversarial Generation** | Instruct a red-team agent to craft inputs that exploit known weaknesses | Robustness testing, edge case coverage | Medium | High | "Generate inputs that cause the Auditor to miss a failing test" |
| **Mutation-Based** | Apply systematic mutations to existing examples (swap, delete, insert, permute) | Regression testing, boundary conditions | Medium | Low | Swap variable names, reorder function arguments, inject off-by-one errors |
| **Failure-Induced Exploration** | Analyze past failures and generate similar but novel failure-triggering inputs | Targeted improvement of weak areas | High | Medium | Parse `incidents/` logs; generate inputs matching failure signatures |

### Technique Selection Matrix

| Task Type | Primary Technique | Secondary Technique | Rationale |
|---|---|---|---|
| Code generation evals | Template-Based | Mutation-Based | Ensure structural coverage, then stress boundaries |
| Natural language evals | LLM-Generated | Adversarial | Maximize linguistic diversity, then harden |
| Regression evals | Mutation-Based | Failure-Induced | Systematically vary known cases, then target past failures |
| Security evals | Adversarial | Failure-Induced | Prioritize attack surface, then learn from incident history |
| New task bootstrap | LLM-Generated | Template-Based | Explore space broadly, then fill gaps with structured templates |

---

## Quality Control

### Deduplication

| Method | Description | When to Use | Tool |
|---|---|---|---|
| Exact hash | SHA-256 of normalized content | First pass; remove identical entries | `hashlib` / built-in |
| Fuzzy match | Jaccard similarity on token n-grams, threshold 0.85 | Remove near-duplicates after exact dedup | `datasketch` MinHash |
| Semantic dedup | Cosine similarity on embeddings, threshold 0.92 | Remove semantically equivalent but lexically different entries | Embedding model + FAISS |

### Diversity Scoring

| Metric | Definition | Target Range | Action if Below Target |
|---|---|---|---|
| Category coverage | Fraction of defined categories with at least 5 examples | > 0.9 | Run targeted generation for uncovered categories |
| Lexical diversity | Type-token ratio across all examples | > 0.4 | Increase paraphrase augmentation |
| Difficulty distribution | Entropy of difficulty labels (easy/medium/hard) | > 1.0 (bits) | Calibrate difficulty; add underrepresented levels |
| Source diversity | Fraction of distinct `seed_id` values used | > 0.6 | Expand seed set; reduce over-reliance on popular seeds |

### Difficulty Calibration

| Difficulty Level | Calibration Method | Target Pass Rate (by baseline agent) |
|---|---|---|
| Easy | Select examples baseline agent solves consistently | 85-100% |
| Medium | Select examples baseline agent solves intermittently | 40-70% |
| Hard | Select examples baseline agent rarely solves | 0-25% |
| Adversarial | Craft examples specifically to defeat baseline | 0-5% |

### Human Validation Sampling

| Dataset Size | Sample Size | Sampling Strategy | Minimum Agreement |
|---|---|---|---|
| < 500 | 100% | Review all | N/A (full review) |
| 500-5,000 | 10% (min 50) | Stratified by category and difficulty | 90% inter-annotator |
| 5,000-50,000 | 5% (min 200) | Stratified random | 85% inter-annotator |
| > 50,000 | 2% (min 500) | Stratified random + adversarial oversample | 85% inter-annotator |

---

## Mapping to Evolve-Loop

### Generating Eval Test Cases

| Agent | Synthetic Data Role | Workflow |
|---|---|---|
| **Scout** | Identify gaps in current eval coverage by analyzing `experiments.jsonl` | Scout flags categories with < 5 eval cases; triggers Seed Curator |
| **Builder** | Consume generated test cases as acceptance criteria | Builder reads `final-synthetic.jsonl` to validate implementation against generated evals |
| **Auditor** | Run generated adversarial inputs to stress-test Builder output | Auditor executes adversarial subset; flags regressions and new failure modes |

### Bootstrapping Evals for New Task Types

| Step | Action | Agent | Output |
|---|---|---|---|
| 1 | Define new task schema (input format, expected output, grading rubric) | Scout | `task-schema.json` |
| 2 | Collect 5-10 manual seed examples conforming to schema | Human / Scout | `seed-set.jsonl` |
| 3 | Run generation pipeline with LLM-Generated + Template-Based techniques | Generator | `final-synthetic.jsonl` |
| 4 | Human-validate a stratified sample | Human | Validation annotations |
| 5 | Register validated dataset as eval suite in `experiments.jsonl` | Orchestrator | Updated `experiments.jsonl` |

### Creating Adversarial Inputs for Auditor

| Adversarial Category | Generation Method | Target Weakness |
|---|---|---|
| Ambiguous instructions | LLM-Generated with conflicting constraints | Instruction-following precision |
| Edge-case inputs | Mutation-Based on boundary values | Off-by-one, empty input, max-length handling |
| Deceptive correctness | Adversarial generation of plausible but wrong outputs | Auditor false-negative rate |
| Format violations | Template-Based with intentional schema breaks | Schema validation robustness |

### Augmenting experiments.jsonl

| Augmentation | Method | Frequency | Trigger |
|---|---|---|---|
| Add new eval entries | Append validated synthetic examples | Per cycle | Scout detects coverage gap |
| Retire stale entries | Mark entries consistently passed (> 10 cycles) as retired | Weekly | Automated staleness check |
| Rebalance difficulty | Adjust distribution to maintain entropy > 1.0 | Per cycle | Difficulty audit by Auditor |
| Inject adversarial | Add adversarial examples targeting recent failures | Per cycle | Failure-Induced Exploration output |

---

## Prior Art

| System | Authors / Org | Year | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|---|---|
| **Self-Instruct** | Wang et al. (Stanford) | 2023 | Bootstrap instruction-following data from a seed set using the model itself | Foundation for LLM-Generated technique; seed-to-generation pipeline |
| **Evol-Instruct** | Xu et al. (Microsoft/WizardLM) | 2023 | Iteratively evolve instructions for complexity and diversity | Informs mutation-based and difficulty calibration stages |
| **AgentInstruct** | Crouse et al. (Microsoft) | 2024 | Multi-agent pipeline for generating diverse, high-quality synthetic data | Direct inspiration for multi-agent pipeline architecture |
| **MetaSynth** | Li et al. | 2024 | Meta-learning approach to synthetic data generation with quality feedback loops | Informs quality control and feedback-driven generation |
| **MALLM-GAN** | Chen et al. | 2024 | GAN-style adversarial training between generator and discriminator agents | Basis for adversarial generation technique |

---

## Anti-Patterns

| Anti-Pattern | Description | Symptom | Mitigation |
|---|---|---|---|
| **Mode Collapse** | Generator produces narrow variations of the same pattern | Low lexical diversity score; most examples look similar | Increase temperature; diversify seeds; use multiple generation techniques |
| **Data Contamination** | Generated data leaks into training set or overlaps with held-out evals | Suspiciously high eval scores; low generalization | Maintain strict train/eval splits; hash-check against eval set before inclusion |
| **Easy-Example Bias** | Generator defaults to simple, well-formed examples | Difficulty distribution skewed toward easy; baseline passes > 90% | Enforce difficulty quotas; use adversarial and failure-induced techniques |
| **Quality-Quantity Tradeoff** | Prioritize volume over correctness, flooding pipeline with low-quality data | Validator rejection rate > 50%; downstream eval noise increases | Set minimum quality thresholds; prefer smaller validated sets over large unvalidated ones |
| **Seed Overfitting** | All generated examples closely mirror the small seed set | Source diversity score < 0.3; generated examples are paraphrases of seeds | Expand seed set from multiple sources; add random perturbation to generation |
| **Circular Validation** | Use the same model to generate and validate data | Validator misses systematic errors the generator makes | Use a different model or human validation for at least a sample |
| **Stale Generation** | Continue generating from outdated seeds after system capabilities change | Generated examples no longer challenge the system | Refresh seeds from recent failure logs and production data each cycle |