# Eval Grader Best Practices

Eval graders are bash commands that exit 0 (pass) or non-zero (fail). Their quality determines whether the evolve-loop can reliably distinguish correct from incorrect output. Poorly designed graders allow bad output to slip through; overly brittle graders cause false failures that stall progress.

---

## Grader Precision

The precision of a grader determines how well it discriminates signal from noise.

**Prefer specific patterns over broad matches.**

```bash
# Too broad — matches any file with the word "plan" anywhere
grep -q "plan" docs/foo.md

# Precise — matches the exact section heading
grep -q "^## Plan Cache Schema" docs/foo.md
```

**Case sensitivity matters — choose deliberately.**

- Use exact case for structural checks (headings, identifiers, file paths):
  ```bash
  grep -q "^## Installation" README.md
  ```
- Use `-i` (case-insensitive) for concept checks where capitalization may vary:
  ```bash
  grep -qi "authentication" docs/security.md
  ```

**Reference: inst-007 (case-sensitive-eval-graders anti-pattern)** — a known failure mode where graders use case-insensitive matching for structural elements (headings, export names), causing false passes when content is malformed.

**Line-anchored patterns are more robust than substring matches.**

```bash
# Matches "## Goals" anywhere in a line — could match "### My Goals"
grep -q "## Goals" docs/foo.md

# Matches only a top-level heading
grep -q "^## Goals$" docs/foo.md
```

---

## Anti-Patterns

### Too-Broad Graders

`grep -q "test"` matches any file with "test" anywhere — including comment text, variable names, or prose. It tells you nothing about whether the required content is present.

```bash
# BAD: trivially satisfied
grep -q "cache" docs/architecture.md

# GOOD: checks for the specific concept
grep -qi "plan cache" docs/architecture.md
```

### Too-Narrow Graders

Matching an exact sentence fails on minor rewording, reformatting, or synonym substitution. The output may be semantically correct but the grader rejects it.

```bash
# BAD: breaks on any rephrasing
grep -q "The builder runs in an isolated git worktree" docs/architecture.md

# GOOD: checks the concept keyword
grep -qi "worktree isolation" docs/architecture.md
```

### No Negative Checks

Checking only that something is present misses cases where unwanted content also appears (placeholder text, debug output, accidental deletions elsewhere). Add at least one negative check per task.

```bash
# Incomplete: only positive
grep -q "^## Summary" docs/foo.md

# Complete: positive + negative
grep -q "^## Summary" docs/foo.md
! grep -q "TODO\|FIXME\|PLACEHOLDER" docs/foo.md
```

### Ignoring Exit Codes

Piping `grep` output without checking `$?` silently swallows failures. Always structure graders so the exit code flows through cleanly.

```bash
# BAD: exit code of grep is lost
grep "pattern" file.md | wc -l

# GOOD: grep exit code is preserved
grep -q "pattern" file.md

# GOOD: awk check that surfaces failure correctly
wc -l < file.md | awk '{exit ($1 > 200)}'
```

---

## Grader Composition Patterns

Strong evals combine multiple lightweight checks rather than relying on a single complex grader.

### Existence + Content + Size Triple

Verify the file exists, has the required content, and stays within expected size bounds. This triple catches creation failures, content omissions, and accidental over-generation.

```bash
# 1. Existence
test -f docs/foo.md

# 2. Content
grep -q "^## Required Section" docs/foo.md

# 3. Size
wc -l < docs/foo.md | awk '{exit ($1 > 200)}'
```

### Positive + Negative Pair

Check that required content IS present AND unwanted content is NOT present. Pairs are especially valuable when output could pass by adding filler rather than correct content.

```bash
# Positive: required concept present
grep -qi "eval grader" docs/foo.md

# Negative: no stale placeholder text
! grep -q "TODO\|FIXME\|PLACEHOLDER" docs/foo.md
```

### Structural Graders

Check section headings using `^## ` anchors rather than searching for content keywords. Structural graders validate document shape independently of prose, making them resilient to content rewrites.

```bash
# Count top-level sections — require at least 3
grep -c "^## " docs/foo.md | awk '{exit ($1 < 3)}'

# Verify a specific required section exists
grep -q "^## Anti-Patterns" docs/foo.md
```

---

## Worked Example

Task: "Add a new section to `docs/foo.md` covering the concept of plan caching."

```bash
# Existence — file was created or already exists
test -f docs/foo.md

# Content — specific section heading present (structural grader)
grep -q "^## Plan Cache" docs/foo.md

# Content — key concept present (case-insensitive concept check)
grep -qi "plan cache" docs/foo.md

# Content — cross-reference link is present
grep -qi "eval-runner" docs/foo.md

# Size — under line limit (negative size check)
wc -l < docs/foo.md | awk '{exit ($1 > 200)}'

# Negative — no broken placeholder text
! grep -q "TODO\|FIXME\|PLACEHOLDER" docs/foo.md
```

This set covers: file existence, structure (section heading), concept coverage, cross-referencing, size constraint, and placeholder cleanup. Each check catches a distinct class of failure.

---

## Mutation Resistance

A grader set is only as strong as its ability to detect deliberate defects. Mutation testing measures this by introducing small, targeted changes to a completed task's output and checking whether at least one grader catches each change.

**Design each grader to catch at least one mutation type:**

| Mutation Type | Grader That Catches It |
|---------------|----------------------|
| Deletion — section heading removed | `grep -q "^## Section Name"` |
| Value change — threshold altered | `grep -q "80%"` or `awk` numeric check |
| Logic inversion — condition reversed | `grep -q "exit ($1 > 200)"` size check |
| Import removal — cross-reference deleted | `grep -qi "linked-file"` |
| Over-generation — file grown beyond limit | `wc -l` size check |

**Target kill rate: 80%+** — if fewer than 80% of generated mutations are caught by the grader set, the evals are too coarse. The fix is to add more targeted grep or assertion checks.

| Kill Rate | Action |
|-----------|--------|
| >= 80% | Graders are robust. No action needed. |
| 60-79% | Review weak graders. Add more specific checks. |
| < 60% | PRIORITY: Propose eval improvement task for next cycle. |

For the full mutation testing execution protocol, including how to generate mutations, apply them to temp copies, and compute kill rates, see the **Mutation Testing** section in `skills/evolve-loop/eval-runner.md`.
