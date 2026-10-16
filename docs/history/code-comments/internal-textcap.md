# Comment history: `internal/textcap`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero k2

### `go/internal/textcap/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — textcap is a stdlib-only leaf (ADR-0103 unit 04 §2):
// the two rune-cap rules must never grow a dependency, or the advisor and the
// carryover lifecycle would inherit it (signalcenter/importgraph_test.go idiom).
```

### `go/internal/textcap/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 04 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/textcap/textcap.go:1` — above `package textcap`

```text
// Package textcap is the ONE home of the two rune-cap text rules the phase
// advisor's prompt (unit 04 of ADR-0103) and the carryover-todo lifecycle
// (unit 03) both apply: TruncateRunes trims and marks with " …[truncated]",
// CapRunes marks with the bare ellipsis rune. Stdlib only; the two consumers
// project it so neither imports the other for a string rule. Design:
// docs/architecture/decomposition/04-advisor.md.
```

### `go/internal/textcap/textcap_test.go:8` — above `func TestTruncateRunes_TrimsCapsAndMarks(t *testing.T) {`

```text
// Test 10 — the two rune-cap rules the advisor prompt and the carryover
// lifecycle project (ADR-0103 unit 04): distinct markers, rune-safe bounds,
// TrimSpace on one rule only.
```
