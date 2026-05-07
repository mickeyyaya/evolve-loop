---
name: reference
description: Reference doc.
---

> Read this file to understand the full refactoring pipeline end-to-end. Demonstrates all 5 phases on a hypothetical UserService class.

# Worked Example: End-to-End Pipeline

Demonstrates the full pipeline on a hypothetical `UserService` class.

## Phase 1: Scan Results

```
| # | File(s) | Smell/Issue | Severity | Category | Complexity | Fan-in/out |
|---|---------|------------|----------|----------|------------|------------|
| 1 | src/services/user.ts | Long Method (processRegistration: 85 lines) | High | Bloater | 23 | 4/8 |
| 2 | src/services/user.ts | Feature Envy (validateAddress uses AddressService 80%) | Medium | Coupler | 12 | 2/5 |
| 3 | src/services/user.ts | Long Parameter List (createUser: 7 params) | Medium | Bloater | 5 | 8/2 |
| 4 | src/utils/validation.ts | Duplicate Code (email regex in 3 files) | Medium | Dispensable | 3 | 0/3 |
| 5 | src/models/user.ts ↔ src/models/order.ts | Circular Dependency | Critical | Architecture | — | — |
```

## Phase 2: Prioritized with Weighted Scoring

```
| # | Issue | Priority Score | Fix Technique | Difficulty | Risk |
|---|-------|---------------|---------------|------------|------|
| 5 | Circular Dependency | 25 (5x critical) | Extract shared interface | Medium | Medium |
| 1 | Long Method | 18 (3x maintenance + 2x churn) | Extract Method | Easy | Low |
| 3 | Long Parameter List | 14 (3x maintenance + 2x centrality) | Introduce Parameter Object | Easy | Low |
| 2 | Feature Envy | 12 (2x coupling + 2x churn) | Move Method | Medium | Medium |
| 4 | Duplicate Code | 8 (2x frequency) | Extract Method | Easy | Low |
```

## Phase 3: Partition Result

```
| Group | Slug | Issues | Write Set | Read Set | Isolated? |
|-------|------|--------|-----------|----------|-----------|
| A | break-circular-dep | #5 | src/models/user.ts, src/models/order.ts, src/models/shared.ts (new) | — | ✓ |
| B | refactor-user-service | #1, #2, #3 | src/services/user.ts, src/services/address.ts | src/models/user.ts | ✗ (reads A's write) |
| C | extract-email-validation | #4 | src/utils/validation.ts, src/utils/email.ts (new) | — | ✓ |
```

Groups A and B share `src/models/user.ts` (A writes, B reads) → **merge into sequential group AB**.
Group C is independent → **parallel with AB**.

## Phase 4: Execution

| Group | Mode | Outcome |
|-------|------|---------|
| AB | Sequential worktree | #5 first (break cycle), then #1, #2, #3 |
| C | Parallel worktree | #4 (extract email validation) |

## Phase 5: Validation

```
| Metric | Before | After | Delta | Status |
|--------|--------|-------|-------|--------|
| Max cognitive complexity | 23 | 8 | -15 | ✓ Improved |
| Circular dependencies | 1 | 0 | -1 | ✓ Fixed |
| Duplicate code blocks | 3 | 0 | -3 | ✓ Fixed |
| Test count | 24 | 28 | +4 | ✓ More tests |
| Architecture violations | 1 | 0 | -1 | ✓ Fixed |
```

All metrics improved or held → **SHIP**.
