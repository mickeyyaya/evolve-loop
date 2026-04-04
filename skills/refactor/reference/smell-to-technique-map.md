> Read this file for quick smell-to-fix lookup. Maps each of the 22 code smells to primary and secondary refactoring techniques with skill references.

# Smell-to-Technique Quick Reference

Fast lookup: given a detected smell, which technique(s) to apply.

| Smell | Primary Technique | Secondary Technique | Skill |
|-------|------------------|--------------------|----|
| Long Method | Extract Method | Replace Temp with Query | `refactor-composing-methods` |
| Large Class | Extract Class | Extract Subclass | `refactor-moving-features` |
| Primitive Obsession | Replace Data Value with Object | Introduce Parameter Object | `refactor-organizing-data` |
| Long Parameter List | Introduce Parameter Object | Preserve Whole Object | `refactor-simplifying-method-calls` |
| Data Clumps | Extract Class | Introduce Parameter Object | `refactor-moving-features` |
| Switch Statements | Replace Conditional with Polymorphism | Replace Type Code with Subclasses | `refactor-simplifying-conditionals` |
| Temporary Field | Extract Class | Introduce Null Object | `refactor-moving-features` |
| Refused Bequest | Replace Inheritance with Delegation | Push Down Method | `refactor-generalization` |
| Divergent Change | Extract Class | Move Method | `refactor-moving-features` |
| Shotgun Surgery | Move Method | Inline Class | `refactor-moving-features` |
| Parallel Inheritance | Replace Inheritance with Delegation | — | `refactor-generalization` |
| Duplicate Code | Extract Method | Pull Up Method | `refactor-composing-methods` |
| Dead Code | Remove (delete it) | — | — |
| Lazy Class | Inline Class | Collapse Hierarchy | `refactor-moving-features` |
| Speculative Generality | Collapse Hierarchy | Inline Class | `refactor-generalization` |
| Data Class | Move Method | Encapsulate Field | `refactor-organizing-data` |
| Feature Envy | Move Method | Extract Method | `refactor-moving-features` |
| Inappropriate Intimacy | Move Method | Hide Delegate | `refactor-moving-features` |
| Message Chains | Hide Delegate | Extract Method | `refactor-moving-features` |
| Middle Man | Remove Middle Man | Inline Method | `refactor-moving-features` |
| Excessive Comments | Extract Method | Rename Method | `refactor-composing-methods` |
| High Complexity (>15) | Decompose Conditional | Replace Nested Conditional with Guard Clauses | `refactor-simplifying-conditionals` |
