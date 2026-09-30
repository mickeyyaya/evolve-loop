# Comment history: `acs/cycle488`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle488/predicates_test.go:3` — above `package cycle488`

```text
// Package cycle488 materialises the cycle-488 acceptance criteria.
//
// TRIAGE COMMITTED TWO ## top_n TASKS, both materializable this cycle:
//
//	tighten-carryover-todo-creation-length  (go/internal/core/failure_learning.go)
//	  → C488_001 (drop boilerplate prefix), C488_002 (bound defect length)
//	cap-carryover-todo-render-length        (go/internal/core/phase_advisor.go)
//	  → C488_003 (per-item render cap)
//	both → C488_004 (core+router CI parity, covers the two "tests green" ACs)
//
// Root cause (scout Key Finding "Router/advisor context injection"): the
// `## Carryover todos` section is 7,627 bytes — ~23% of the 33 KB router prompt
// — because two creation paths write unbounded/redundant CarryoverTodo.Action
// strings and the sole render site (writeCarryoverTodos) caps only the COUNT,
// not the per-item length.
//
// Predicate strategy (mirrors cycle480): behavioral predicates EXERCISE the
// system under test — never a source grep (the cycle-85 degenerate-predicate
// trap). C488_002 calls the exported core.ApplyDefectsAsCarryoverTodos directly;
// C488_001/003/004 drive the unexported code paths through the in-package
// go/internal/core tests via `go test` subprocesses and assert exit 0 +
// "tests actually ran" (the cycle480 no-tests-to-run guard).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C488_002 feeds a 5000-rune defect that a no-op renders unbounded;
//	            C488_003's in-package test feeds a 4000-rune Action a no-op
//	            passes through in full.
//	Edge/OOD:   the in-package suite pins len(todos)==0 (empty omit) and the
//	            25-todo ">20 omitted" trailer boundary (regression pins).
//	Semantic:   the creation-time cap (C488_001/002) and the render-time cap
//	            (C488_003) are DISTINCT surfaces — satisfying one must not
//	            silently satisfy the other.
//
// RED strategy (see test-report.md "RED Run Output"): all four predicates are
// RED before the Builder edits failure_learning.go / phase_advisor.go. The
// in-package RED tests (TestApplyDefectsAsCarryoverTodosBoundsLength,
// TestCarryoverTodoActionDropsBoilerplatePrefix,
// TestWriteCarryoverTodosCapsPerItemLength) fail, so every subprocess exits
// non-zero and the direct-call C488_002 assertion trips.
```
