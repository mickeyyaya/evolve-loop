# Comment history: `internal/treefence`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d9

### `go/internal/treefence/apicover_named_test.go:9` — above `func TestAPICoverNamedExports(t *testing.T) {`

```text
// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) through the shapes its consumers
// — the phase runner's and the retro phase's worktree fence — rely on: the
// one-shot Take/Restore pair, and the Begin/End lifecycle with its Outcome
// rendered as diagnostics.
```

### `go/internal/treefence/fence.go:1` — above `package treefence`

```text
// Package treefence is a content fence around a read-only phase's worktree.
//
// A phase that is not a declared source writer (registry writes_source=false:
// audit, adversarial-review, retrospective, …) must hand downstream the exact
// tree it was given. The agents behind those phases have a shell, and cycles
// 1603, 1604 and 1605 each failed the deterministic build-explanation gate
// because the auditor's mutation probes rewrote the builder's material files
// in place (`cp /tmp/digest.go.bak …`, "wrote reverted tdd.go", probe tests
// dropped into the material packages) — the sealed diff digest no longer
// matched the tree, and no LLM round could repair a defect the auditor itself
// had introduced. See docs/incidents/2026-09-03-auditor-mutates-the-worktree.md.
//
// Take records the worktree as a git tree object — tracked changes and
// untracked files, .gitignore respected — through a throwaway index, so the
// real index is never touched. Restore diffs a fresh tree against it and
// undoes every difference: added paths are removed, modified/deleted/retyped
// paths are written back from the snapshot with their mode, and the
// directories the phase created around its files are pruned. Ignored paths
// (build outputs, the worktree's own .evolve/ scratch) are outside the fence
// by construction — the same set the explanation digest ignores.
```

### `go/internal/treefence/fence_test.go:66` — above `write(t, dir, "src/mat.go", "package src // MUTATED by a probe\n")`

```text
// The auditor's mutation probes (cycles 1603-1605): rewrite a material
// file in place, revert a builder edit, drop a probe test into the package,
// delete a builder-added file, chmod a file, and build a binary (ignored).
```
