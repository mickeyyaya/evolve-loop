# Gene/Capsule Library

Genes are structured, reusable fix templates — more actionable than instincts. While instincts describe *what was learned*, genes describe *how to fix it* with executable steps and validation.

## Gene Schema

```yaml
- id: gene-001
  name: "fix-missing-export"
  selector:
    errorPattern: "Module.*has no exported member"
    fileGlob: "src/**/index.ts"
  action:
    steps:
      - "Find the missing export in the source file"
      - "Add export to the nearest barrel file (index.ts)"
    commands:
      - "grep -r 'export' ${file} | head -5"
  validation:
    pre: "grep '${symbol}' ${barrel} | wc -l"   # should be 0
    post: "grep '${symbol}' ${barrel} | wc -l"  # should be 1
  confidence: 0.8
  source: "cycle-5/fix-export-bug"
  successCount: 3
  failCount: 0
```

## Fields

| Field | Description |
|-------|-------------|
| `id` | Unique identifier (gene-NNN) |
| `name` | Human-readable name |
| `selector.errorPattern` | Regex matching error messages that trigger this gene |
| `selector.fileGlob` | File patterns this gene applies to |
| `action.steps` | Ordered list of fix steps (natural language) |
| `action.commands` | Optional shell commands to assist the fix |
| `validation.pre` | Command to verify precondition (the bug exists) |
| `validation.post` | Command to verify postcondition (the fix worked) |
| `confidence` | Reliability score (0.5-1.0), increases with successful applications |
| `successCount` | Times this gene was applied successfully |
| `failCount` | Times this gene was applied but failed |

## Capsules

Capsules are composite bundles combining multiple genes into a workflow:

```yaml
- id: capsule-001
  name: "add-new-component"
  genes: ["gene-003", "gene-005", "gene-008"]
  sequence: "ordered"  # or "parallel"
  description: "Full workflow for adding a new React component with tests and exports"
  successCount: 2
```

## How Genes Are Used

1. **Builder encounters an error** → checks gene library for matching `selector.errorPattern`
2. **Multiple matches** → ranked by `confidence * successCount / (successCount + failCount)`
3. **Best gene selected** → Builder follows `action.steps`, runs `validation.pre` to confirm bug, applies fix, runs `validation.post` to confirm fix
4. **Outcome tracked** → increment `successCount` or `failCount`

## Gene Extraction

During Phase 5 (LEARN), after instinct extraction:
- If a Builder successfully fixed a recurring error type, extract the fix as a gene
- If a sequence of genes was applied together successfully, bundle as a capsule
- Genes with `failCount > successCount` are archived

## Storage

```
.evolve/genes/
  gene-001-fix-missing-export.yaml
  gene-002-add-test-file.yaml
  capsule-001-add-component.yaml
```

For an annotated example, see [examples/gene.yaml](../examples/gene.yaml).

## Relationship to Instincts

| Aspect | Instincts | Genes |
|--------|-----------|-------|
| Nature | Declarative (what to know) | Imperative (how to fix) |
| Trigger | Read at start of cycle | Triggered by error pattern match |
| Format | Description + confidence | Selector + steps + validation |
| Evolution | Confidence scoring | Success/fail counting |

See [memory-hierarchy.md](memory-hierarchy.md) for how genes fit into the broader memory architecture (Layer 6).

## Gene Self-Play Evolution (Tool-R0-Inspired)

Tool-R0 (arXiv:2602.21320) demonstrates that co-evolutionary self-play — where a Generator proposes challenges and a Solver attempts them — produces 92.5% improvement with zero labeled data. This adversarial curriculum maps directly to the evolve-loop's gene lifecycle.

**Mapping to gene evolution:**

| Tool-R0 Concept | Gene Equivalent |
|-----------------|----------------|
| Generator proposes hard tasks | Scout identifies capability gaps (gene selectors that don't match any existing gene) |
| Solver attempts tasks | Builder applies gene steps to fix matched patterns |
| Adversarial curriculum | Genes with low `successCount` are re-proposed with harder selectors |
| Self-play feedback | Gene validation (pre/post checks) provides verifiable success signal |

**Self-play gene refinement protocol:**
1. After a gene fires but its validation fails, the gene enters "adversarial refinement" — the LEARN phase proposes a targeted mutation to the gene's steps
2. The mutated gene is tested against the same error pattern in the next cycle
3. Genes that survive 3+ mutations with consistent success are promoted to high-confidence capsules
4. Genes that fail 3 consecutive mutations are archived with `archivedReason: "self-play-failure"`

This creates a curriculum where genes progressively improve through adversarial pressure — the evolve-loop equivalent of Tool-R0's Generator/Solver co-evolution.
