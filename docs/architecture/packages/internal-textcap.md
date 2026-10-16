# internal/textcap

## Purpose

`internal/textcap` is the one home of the two rune-cap text rules. The phase advisor's prompt and the carryover-todo lifecycle each apply one of them. Each consumer projects the rule, so neither imports the other for a string rule.

## Design

- **Two rules, two markers.** `TruncateRunes` trims surrounding whitespace, caps the string at the rune bound, and marks the cut with ` …[truncated]`. That is the advisor's goal and card-hint rule, and the remediation title's rule. `CapRunes` marks the cut with the bare ellipsis rune and never trims. That is the carryover action rule.
- **Rune-safe bounds.** Both rules count runes, so a multi-byte character is never split.
- **Stdlib only.** The package is a leaf. Its two rules must never grow a dependency, or the advisor and the carryover lifecycle inherit it.

## Invariants

- **The markers stay distinct.** `TestCapRunes_CapsWithTheEllipsisRune` pins that the two rules never produce the same output, so they are never folded into one.
- **Trim happens before the bound, and only in one rule.** `TestTruncateRunes_TrimsCapsAndMarks` pins the order for `TruncateRunes`, and `CapRunes` never trims.
- **Stdlib only.** `TestImportGraph_StdlibOnly` reads the package's imports.
- **The clean-code bar.** `TestLimits_FunctionsFilesAndNestingStayWithinTheBar` enforces the function, file and nesting limits for the package.

## Findings

- None.
