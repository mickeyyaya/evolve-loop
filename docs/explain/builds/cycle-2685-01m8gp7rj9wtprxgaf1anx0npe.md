# Build Explanation — Cycle 2685

## Build Binding
- Cycle: 2685
- Base SHA: 4e7c0689ffd6b63dda29ab4baea9ed59b44e9310

## Summary
The program name `go` is one named constant, `goTool`, in the acsassert package.

## Rationale
Five bare `"go"` literals named the same tool in three files. One constant gives the name one home.

## Changed Areas
- `go/pkg/acsassert/subprocess_bound.go` — declares `goTool` and uses it in the bound checks and the bound line.
- `go/pkg/acsassert/assertions.go` — the timeout rewrite compares the name with `goTool`.
- `go/pkg/acsassert/go_tests.go` — both test runners pass `goTool` to the process runner.

## Design Decisions
The constant is unexported. No caller outside the package needs it.

## Verification
Predicates TestC2685_001 and TestC2685_002 pass. The acsassert package tests pass.

## Compatibility
Behavior does not change. The value is still `go`.

## Limitations
Test files keep their `"go"` literals.
