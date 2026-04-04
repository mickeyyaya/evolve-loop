> Read this file during Phase 1 scan. SonarQube-derived cognitive complexity algorithm with scoring rules, thresholds, and worked example.

# Cognitive Complexity Scoring

Compute cognitive complexity for every function in scope. This measures how hard a function is to *understand*, not just how long it is.

## Algorithm

| Increment | Condition | Nesting penalty |
|-----------|-----------|-----------------|
| +1 | `if`, `else if`, `else` | +1 per nesting level |
| +1 | `switch` | +1 per nesting level |
| +1 | `for`, `while`, `do-while`, `for...of`, `for...in` | +1 per nesting level |
| +1 | `catch` | +1 per nesting level |
| +1 | Ternary operator `? :` | +1 per nesting level |
| +1 | Mixed logical operator sequence (`a && b \|\| c`) | No nesting penalty |
| +1 | Recursion (function calls itself) | No nesting penalty |

## What Does NOT Count

| Construct | Reason |
|-----------|--------|
| Null-coalescing (`??`, `?.`) | Simplifies code, not complexity |
| Early returns / guard clauses | Reduce nesting, improve readability |
| Lambda/arrow function definitions | Definition is not control flow |
| Simple `try` blocks (without logic) | Structural, not cognitive |
| `break`, `continue` | Flow interruption already counted at loop level |

## Thresholds

| Score | Rating | Action |
|-------|--------|--------|
| 0-10 | Good | No action required |
| 11-15 | Moderate | Consider refactoring if in hot path |
| 16-25 | High | Refactor — extract methods, simplify conditionals |
| 26+ | Critical | Must refactor — function is unmaintainable |

## Scoring Example

```javascript
function processOrder(order, user) {        // function declaration: 0
  if (order.items.length === 0) {            // +1 (if)
    return null;                             // early return: 0
  }
  for (const item of order.items) {          // +1 (for)
    if (item.quantity > 0) {                 // +2 (if + 1 nesting)
      if (item.price > 100                   // +3 (if + 2 nesting)
          && user.isPremium                  // +0 (same operator)
          || item.isOnSale) {               // +1 (mixed operators)
        applyDiscount(item);
      }
    }
  }
}                                            // Total: 8 (Good)
```
