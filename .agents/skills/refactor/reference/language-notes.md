---
name: reference
description: Reference doc.
---

> Read this file during Phase 3-4 for language-specific refactoring guidance. Covers TypeScript/JavaScript, Python, Go, and Java.

# Language-Specific Refactoring Notes

## TypeScript/JavaScript

| Concern | Guidance |
|---------|----------|
| Type narrowing | Prefer discriminated unions over type assertions when replacing conditionals |
| Barrel exports | When extracting classes/modules, update `index.ts` barrel exports |
| React components | Extract Method → Extract Component; watch for hook rules (no conditional hooks) |
| Async patterns | When refactoring promise chains, prefer async/await; avoid mixing styles |

## Python

| Concern | Guidance |
|---------|----------|
| Dataclasses | Replace Data Class smell with `@dataclass` + methods |
| Type hints | Add type hints when applying Extract Method or Introduce Parameter Object |
| Dunder methods | When encapsulating fields, use `@property` not Java-style getters |
| Module structure | Python favors flat module hierarchies; avoid deep nesting when extracting |

## Go

| Concern | Guidance |
|---------|----------|
| Interfaces | Extract Interface → define small interfaces at the consumer side |
| Error handling | When simplifying conditionals, preserve explicit error handling (no swallowing) |
| Packages | When extracting classes, prefer package-level organization over deep nesting |
| Exported names | Extracted public functions must have doc comments |

## Java

| Concern | Guidance |
|---------|----------|
| Records | Replace Data Class smell with Java records (Java 16+) |
| Sealed classes | Use sealed classes when replacing type codes with subclasses |
| Streams | When refactoring loops, consider Stream API but avoid nested streams |
| Dependency injection | When moving methods, update DI container configuration |
