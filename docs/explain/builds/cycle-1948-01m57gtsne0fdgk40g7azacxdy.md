# Build Explanation — Cycle 1948

## Build Binding
- Cycle: 1948
- Base SHA: 2f1d2eb5017eeffa571133ff45374a312203f988

## Summary
`convergence.Decide` now signals `CONVERGENCE_DISPUTED` when the decided round holds a DISPUTED row. The signal names who must rule on it.

## Rationale
A disputed CRITICAL never blocks a land, so the console saw no sign that someone had to adjudicate. A reason line is the smallest fix: it adds no field to `Decision` and changes no action.

## Changed Areas
- `go/internal/convergence/reasons.go` — adds `disputedReasons` and `adjudicator`: count of DISPUTED rows, count of CRITICAL ones, operator for `console-lane`, audit for other loops.
- `go/internal/convergence/decide.go` — places the new reason after concentration and before the settle reasons.
- `go/internal/convergence/disputed_test.go` — tests the reason fields, the unchanged decision, absence, round scope and order.
- `go/acs/cycle1948/predicates_test.go` — acceptance predicates through the `evolve convergence decide` verb and the docs.
- `.evolve/evals/convergence-disputed-critical-names-its-adjudicator.md` — eval graders for the task.
- `docs/architecture/convergence-policy.md` — lists the new signal in the §9 table.
- `docs/operations/runtime-reference.md` — names the signal in the `convergence decide` entry.

## Design Decisions
Only the decided round is read, so an old dispute does not repeat. Non-console loops name `audit` because the code-review test requires it.

## Verification
`go test -count=1 ./internal/convergence` and the cycle 1948 ACS predicates pass.

## Compatibility
No exported symbol and no decision changes. Rounds without a DISPUTED row print the same reasons as before.

## Limitations
The adjudicator for loops other than `console-lane` and `code-review` defaults to `audit`.
