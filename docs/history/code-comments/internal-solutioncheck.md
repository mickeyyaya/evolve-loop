# Comment history: `internal/solutioncheck`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/solutioncheck/solutioncheck.go:1` — above `package solutioncheck`

```text
// Package solutioncheck is the ONE deterministic, LLM-free judge of a document
// cycle's deliverable shape (ADR-0099 slice 2). A `document` cycle delivers
// solutions/<slug>/ — at least N candidate options, a recommendation and an
// assumptions-and-evidence file — and this engine is projected three ways: the
// build handoff floor (core), the `evolve solution check` CLI (the eval [code]
// grader and the agent's self-check) and the audit gate line. The contract
// itself is config (phase-registry.json:config.deliverable_kinds); judgment of
// QUALITY (rubrics, evidence strength) stays in the audit phase — never here.
```

### `go/internal/solutioncheck/solutioncheck_test.go:3` — above `import (`

```text
// solutioncheck_test.go — ADR-0099 slice 2 RED contract. A `document` cycle's
// deliverable is solutions/<slug>/ with ≥N candidate options, a recommendation
// and an assumptions-and-evidence file; this package is the ONE deterministic,
// LLM-free engine that judges its shape (the build floor, the `evolve solution
// check` CLI and the audit gate line all project it). Judgment (rubric quality)
// stays in audit — never here.
```
