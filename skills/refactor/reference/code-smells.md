> Read this file during Phase 1 scan. Complete 22-smell catalog with detection signals and numeric thresholds.

# Code Smell Detection Catalog

Complete catalog of 22 code smells organized by category.

## Bloaters

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 1 | Long Method | Line count or cognitive complexity | >20 lines OR complexity >15 |
| 2 | Large Class | Line count, field count, method count | >300 lines, >10 fields, OR >20 methods |
| 3 | Primitive Obsession | Repeated primitive params representing a concept | 3+ primitives that belong together |
| 4 | Long Parameter List | Parameter count per function | >3 parameters |
| 5 | Data Clumps | Same group of fields appearing together | Same 3+ fields in 2+ locations |

## Object-Orientation Abusers

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 6 | Switch Statements | Switch/if-else chains on type codes | >3 cases dispatching on same value |
| 7 | Temporary Field | Fields only set/used in certain paths | Field null/undefined in >50% of methods |
| 8 | Refused Bequest | Subclass ignores parent methods/fields | Overrides >50% of inherited interface |
| 9 | Alt Classes, Different Interfaces | Two classes doing the same thing differently | Similar method bodies, different signatures |

## Change Preventers

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 10 | Divergent Change | One class changed for many different reasons | >3 unrelated change reasons in git history |
| 11 | Shotgun Surgery | One change requires edits across many files | Change spans >5 files |
| 12 | Parallel Inheritance | Creating subclass in one hierarchy forces subclass in another | 1:1 subclass correspondence across hierarchies |

## Dispensables

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 13 | Duplicate Code | Token-level similarity between code blocks | >25 tokens duplicated |
| 14 | Dead Code | Unreachable or unused code | Zero references (fan-in = 0, not entry point) |
| 15 | Lazy Class | Class does too little to justify its existence | <3 methods AND <50 lines |
| 16 | Speculative Generality | Abstractions created for future use that never came | Abstract class/interface with single implementation |
| 17 | Data Class | Class with only fields and getters/setters, no behavior | 0 business-logic methods |
| 18 | Excessive Comments | Comments compensating for unclear code | Comment-to-code ratio >0.5 in a function |

## Couplers

| # | Smell | Detection Signal | Threshold |
|---|-------|-----------------|-----------|
| 19 | Feature Envy | Method uses another class's data more than its own | >50% of references are to external class |
| 20 | Inappropriate Intimacy | Two classes access each other's internals | Bidirectional private/protected access |
| 21 | Message Chains | Long chains of method calls (a.b().c().d()) | >3 chained calls |
| 22 | Middle Man | Class delegates most work to another class | >50% of methods are pure delegation |
