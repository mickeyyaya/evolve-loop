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
.claude/evolve/genes/
  gene-001-fix-missing-export.yaml
  gene-002-add-test-file.yaml
  capsule-001-add-component.yaml
```

For an annotated example, see [examples/gene-example.yaml](../examples/gene-example.yaml).

## Relationship to Instincts

| Aspect | Instincts | Genes |
|--------|-----------|-------|
| Nature | Declarative (what to know) | Imperative (how to fix) |
| Trigger | Read at start of cycle | Triggered by error pattern match |
| Format | Description + confidence | Selector + steps + validation |
| Evolution | Confidence scoring | Success/fail counting |
